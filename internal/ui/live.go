package ui

import (
	"time"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultPollInterval = time.Second
	refreshDebounce     = 300 * time.Millisecond
	maxPollBackoff      = 30 * time.Second
	pollTimeout         = 3 * time.Second
	liveErrPoll         = "poll err"
	liveErrRefresh      = "refresh err"
)

func (m Model) pollCmd() tea.Cmd {
	d := m.pollDelay()
	if d <= 0 || m.client == nil {
		return nil
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

func (m Model) pollDelay() time.Duration {
	if m.pollInterval <= 0 {
		return 0
	}
	d := m.pollInterval
	for i := 0; i < m.pollFails && d < maxPollBackoff; i++ {
		if d > maxPollBackoff/2 {
			d = maxPollBackoff
			break
		}
		d *= 2
	}
	if d > maxPollBackoff {
		d = maxPollBackoff
	}
	return d
}

func (m Model) debounceCmd(seq int) tea.Cmd {
	return tea.Tick(refreshDebounce, func(time.Time) tea.Msg {
		return debounceMsg{seq: seq}
	})
}

func (m Model) onPollTick() (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	return m, fetchRevision(m.client)
}

func (m Model) onRevision(msg revisionMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.pollFails++
		m.liveErr = liveErrPoll
		return m, m.pollCmd()
	}
	m.pollFails = 0
	m.liveErr = ""
	if msg.rev == nil || msg.rev.Generation == "" {
		m.pollFails++
		m.liveErr = liveErrPoll
		return m, m.pollCmd()
	}
	if m.revGeneration == "" {
		m.revGeneration = msg.rev.Generation
		m.revCounter = msg.rev.Counter
		return m, m.pollCmd()
	}
	if msg.rev.Generation != m.revGeneration {
		m.refreshSeq++
		m.revGeneration = msg.rev.Generation
		m.revCounter = msg.rev.Counter
		m.dropCaches()
		return m, batchCmds([]tea.Cmd{fetchStartup(m.client, true), m.pollCmd()})
	}
	if msg.rev.Counter != m.revCounter {
		m.revCounter = msg.rev.Counter
		m.refreshSeq++
		seq := m.refreshSeq
		return m, batchCmds([]tea.Cmd{m.debounceCmd(seq), m.pollCmd()})
	}
	return m, m.pollCmd()
}

func (m Model) onDebounce(msg debounceMsg) (Model, tea.Cmd) {
	if msg.seq != m.refreshSeq {
		return m, nil
	}
	return m.liveRefetch(true)
}

func (m *Model) dropCaches() {
	m.queryCache = map[string]queryCacheEntry{}
	m.readCache = map[string]*api.ReadResult{}
	m.noteIndex = map[string]api.Object{}
	m.corpusDone = false
	m.needPrefetch = true
	m.corpusLoading = false
}

func (m *Model) applyRevision(rev *api.Revision) {
	if rev == nil {
		return
	}
	m.revGeneration = rev.Generation
	m.revCounter = rev.Counter
}

func (m *Model) rebindPanesFromSchema() {
	for _, p := range m.panes {
		if p == nil {
			continue
		}
		if p.viewName != "" && m.cfg != nil {
			v, ok := m.cfg.Views[p.viewName]
			if !ok {
				continue
			}
			applyViewSource(p, m.schema, v)
			p.applySpec(v.Spec(p.viewName), m.typeSchemaFor(p))
			continue
		}
		if p.typeName != "" {
			if ts, ok := m.schema.Types[p.typeName]; ok {
				p.applySpec(p.spec, ts)
			}
		}
	}
}

type liveChrome struct {
	focus         string
	lastMainFocus string
	screen        string
	pane          pane
	zoomed        bool
}

func (m Model) snapshotLiveChrome() liveChrome {
	return liveChrome{
		focus:         m.focus,
		lastMainFocus: m.lastMainFocus,
		screen:        m.screen,
		pane:          m.pane,
		zoomed:        m.zoomed,
	}
}

func (m *Model) restoreLiveChrome(s liveChrome) {
	m.zoomed = s.zoomed
	m.lastMainFocus = s.lastMainFocus
	m.focus = s.focus
	m.pane = s.pane
	m.screen = s.screen
}

func (m Model) liveRefetch(keep bool) (Model, tea.Cmd) {
	snap := m.snapshotLiveChrome()
	var cmds []tea.Cmd
	if m.screen == kindNote && m.note != nil && m.note.ObjectID != "" {
		id := m.note.ObjectID
		if m.readCache != nil {
			delete(m.readCache, id)
		}
		gen := m.nextGen()
		cmds = append(cmds, fetchReadFull(m.client, gen, id, "note", keep, true))
	}
	if m.previewOn && m.previewID != "" {
		if m.readCache != nil {
			delete(m.readCache, m.previewID)
		}
		m.previewLoading = true
		cmds = append(cmds, fetchReadFull(m.client, m.gen, m.previewID, "preview", keep, true))
	}
	for _, p := range m.panes {
		if p == nil {
			continue
		}
		if m.queryCache != nil {
			delete(m.queryCache, cacheKey(p.sourceQuery, p.sourceInputs))
		}
		if p.bind != "" {
			continue
		}
		var cmd tea.Cmd
		m, cmd = m.queryPane(p, "", keep, true)
		cmds = append(cmds, cmd)
	}
	m.restoreLiveChrome(snap)
	return m, batchCmds(cmds)
}

func (m *Model) restorePaneCursor(p *viewState) {
	if p == nil || !p.keepCursor {
		return
	}
	if p.restoreID == "" || !p.selectID(p.restoreID) {
		if p.restoreID != "" {
			p.resetCursor()
		}
	} else {
		p.listOff = p.restoreOff
	}
	p.keepCursor = false
	p.restoreID = ""
}

func (m Model) reloadFromSchema(msg schemaMsg) (Model, tea.Cmd) {
	snap := m.snapshotLiveChrome()
	if msg.schema != nil {
		m.schema = *msg.schema
	}
	m.applyRevision(msg.revision)
	if m.cfg != nil {
		m.cfgWarnings = m.cfg.Validate(m.schema)
	}
	m.rebindPanesFromSchema()
	m, cmd := m.liveRefetch(false)
	m.restoreLiveChrome(snap)
	return m, cmd
}
