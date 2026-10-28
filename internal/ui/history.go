package ui

import (
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/group"
	"github.com/aidanlsb/perch/internal/md"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) beginNav() {
	if m.navReplace {
		return
	}
	m.snapshotCurrent()
	if m.histPtr >= 0 && m.histPtr < len(m.hist)-1 {
		m.hist = m.hist[:m.histPtr+1]
	}
}

func (m *Model) commitNav() {
	if m.navReplace {
		m.navReplace = false
		m.snapshotCurrent()
		return
	}
	fr := m.captureFrame()
	if m.histPtr < 0 || len(m.hist) == 0 {
		m.hist = []histFrame{fr}
		m.histPtr = 0
		return
	}
	m.hist = append(m.hist, fr)
	m.histPtr = len(m.hist) - 1
}

func (m *Model) seedHomeUnder() {
	m.hist = []histFrame{{Screen: kindHome}}
	m.histPtr = 0
	m.homeSec = 0
	m.homeCur = [homeSectionCount]int{}
	m.screen = kindHome
	m.overlay = overlayNone
}

func (m *Model) snapshotCurrent() {
	if m.histPtr < 0 || m.histPtr >= len(m.hist) {
		return
	}
	m.hist[m.histPtr] = m.captureFrame()
}

func (m Model) captureFrame() histFrame {
	fr := histFrame{
		Screen:  m.screen,
		HomeSec: m.homeSec,
		HomeCur: m.homeCur,
	}
	switch m.screen {
	case kindNote:
		fr.NoteID = m.noteID()
		if fr.NoteID == "" && m.histPtr >= 0 && m.histPtr < len(m.hist) {
			fr.NoteID = m.hist[m.histPtr].NoteID
			fr.Line = m.hist[m.histPtr].Line
		} else if m.histPtr >= 0 && m.histPtr < len(m.hist) {
			fr.Line = m.hist[m.histPtr].Line
		}
		fr.Scroll = m.noteScroll
		fr.Pane = m.pane
		fr.LinkIdx = m.linkIdx
		fr.BackIdx = m.backIdx
		fr.HasView = m.note != nil
	case kindBrowse:
		fr.Browse = m.captureBrowse()
	case kindHome:
		// home cursors already copied
	}
	return fr
}

func (m Model) captureBrowse() *browseSnap {
	panes := make([]*viewState, 0, len(m.panes))
	for _, p := range m.panes {
		panes = append(panes, cloneViewState(p))
	}
	return &browseSnap{
		currentView:   m.currentView,
		currentLayout: m.currentLayout,
		tree:          cloneTree(m.tree),
		panes:         panes,
		focus:         m.focus,
		lastMainFocus: m.lastMainFocus,
		zoomed:        m.zoomed,
		previewOn:     m.previewOn,
		previewID:     m.previewID,
		previewLine:   m.previewLine,
		previewScroll: m.previewScroll,
		searchQ:       m.searchQ,
		searchHits:    append([]int(nil), m.searchHits...),
		searchAt:      m.searchAt,
	}
}

func (m *Model) restoreFrame(fr histFrame) tea.Cmd {
	m.overlay = overlayNone
	m.pending = ""
	m.homeSec = fr.HomeSec
	m.homeCur = fr.HomeCur
	switch fr.Screen {
	case kindHome:
		m.applyHome()
		return nil
	case kindBrowse:
		m.applyBrowse(fr.Browse)
		var cmd tea.Cmd
		m2, c := m.previewCmd()
		*m = m2
		cmd = c
		return cmd
	case kindNote:
		m.screen = kindNote
		m.clearBrowseChrome()
		if fr.HasView && m.note != nil && m.note.ObjectID == fr.NoteID {
			m.noteScroll = max(0, fr.Scroll)
			m.pane = fr.Pane
			m.linkIdx = fr.LinkIdx
			m.backIdx = fr.BackIdx
			return m.backlinkPreviewCmd()
		}
		if rd, ok := m.readCache[fr.NoteID]; ok && rd != nil {
			m.applyCachedNote(rd, fr)
			return m.backlinkPreviewCmd()
		}
		m.loading = true
		m.status = "reading " + fr.NoteID + "…"
		gen := m.nextGen()
		return fetchRead(m.client, gen, fr.NoteID, "note")
	}
	m.applyHome()
	return nil
}

func (m *Model) applyCachedNote(rd *api.ReadResult, fr histFrame) {
	m.note = rd
	m.noteDoc = frontmatter.Split(rd.Content)
	width := m.noteBodyWidth()
	m.noteBody = md.Render(m.noteDoc.Body, width)
	if fr.HasView {
		nLinks, nBack := 0, 0
		if rd != nil {
			nLinks = len(rd.Links)
			nBack = len(rd.Backlinks)
		}
		m.noteScroll = max(0, fr.Scroll)
		m.pane = fr.Pane
		if m.pane == paneBacklinks && nBack == 0 {
			m.pane = paneMain
		}
		m.linkIdx = clamp(fr.LinkIdx, 0, max(0, nLinks-1))
		m.backIdx = clamp(fr.BackIdx, 0, max(0, nBack-1))
	} else {
		m.noteScroll = sourceLineScroll(rd.Content, m.noteBody, fr.Line)
		m.linkIdx = 0
		m.backIdx = 0
		m.pane = paneMain
		if len(rd.Links) == 0 && len(rd.Backlinks) > 0 {
			m.pane = paneBacklinks
		}
	}
	m.status = rd.ObjectID
}

func (m *Model) applyBrowse(snap *browseSnap) {
	m.screen = kindBrowse
	m.clearNoteDisplay()
	if snap == nil {
		return
	}
	m.currentView = snap.currentView
	m.currentLayout = snap.currentLayout
	m.tree = cloneTree(snap.tree)
	m.panes = make([]*viewState, 0, len(snap.panes))
	for _, p := range snap.panes {
		m.panes = append(m.panes, cloneViewState(p))
	}
	m.focus = snap.focus
	m.lastMainFocus = snap.lastMainFocus
	m.zoomed = snap.zoomed
	m.previewOn = snap.previewOn
	m.previewID = snap.previewID
	m.previewLine = snap.previewLine
	m.previewScroll = snap.previewScroll
	m.searchQ = snap.searchQ
	m.searchHits = append([]int(nil), snap.searchHits...)
	m.searchAt = snap.searchAt
	if m.previewOn && m.previewID != "" {
		if rd, ok := m.readCache[m.previewID]; ok {
			m.setPreview(rd)
			m.previewScroll = snap.previewScroll
		}
	}
	if m.focus != "" {
		m.setFocus(m.focus)
	}
	m.status = m.currentLayoutName()
}

func (m *Model) applyHome() {
	m.screen = kindHome
	m.clearNoteDisplay()
	m.overlay = overlayNone
	m.pending = ""
	m.zoomed = false
	m.focus = ""
	m.status = "home"
}

func (m *Model) clearNoteDisplay() {
	m.note = nil
	m.noteBody = ""
	m.noteScroll = 0
	m.linkIdx = 0
	m.backIdx = 0
	if m.pane == paneBacklinks {
		m.pane = paneMain
	}
}

func (m *Model) clearBrowseChrome() {
	m.searchQ = ""
	m.searchHits = nil
	m.zoomed = false
}

func (m Model) historyGo(delta int) (Model, tea.Cmd) {
	m.snapshotCurrent()
	next := m.histPtr + delta
	if next < 0 {
		if m.screen != kindHome {
			m.hist = append([]histFrame{{Screen: kindHome}}, m.hist...)
			m.histPtr = 0
			m.applyHome()
			return m, m.ensureCorpus()
		}
		return m, nil
	}
	if next >= len(m.hist) {
		return m, nil
	}
	m.histPtr = next
	cmd := m.restoreFrame(m.hist[m.histPtr])
	return m, cmd
}

func (m Model) goHome() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.pending = ""
	m.fromNav = false
	if m.screen == kindHome {
		m.status = "home"
		return m, m.ensureCorpus()
	}
	m.beginNav()
	m.applyHome()
	m.commitNav()
	return m, m.ensureCorpus()
}

func (m Model) ensureCorpus() tea.Cmd {
	if m.corpusDone || m.corpusLoading {
		return nil
	}
	m.corpusLoading = true
	return m.corpusCmd()
}

func cloneViewState(p *viewState) *viewState {
	if p == nil {
		return nil
	}
	cp := *p
	cp.objects = append([]api.Object(nil), p.objects...)
	cp.filtered = append([]api.Object(nil), p.filtered...)
	cp.listFields = append([]string(nil), p.listFields...)
	cp.board = cloneColumns(p.board)
	cp.dateCols = cloneColumns(p.dateCols)
	if p.rawInputs != nil {
		cp.rawInputs = copyStringMap(p.rawInputs)
	}
	if p.sourceInputs != nil {
		cp.sourceInputs = copyStringMap(p.sourceInputs)
	}
	if p.savedQuery != nil {
		q := *p.savedQuery
		q.Args = append([]string(nil), p.savedQuery.Args...)
		cp.savedQuery = &q
	}
	return &cp
}

func cloneColumns(cols []group.Column) []group.Column {
	if cols == nil {
		return nil
	}
	out := make([]group.Column, len(cols))
	for i, c := range cols {
		out[i] = c
		out[i].Items = append([]api.Object(nil), c.Items...)
	}
	return out
}

func cloneTree(n layoutTree) layoutTree {
	out := layoutTree{split: n.split, id: n.id}
	if len(n.kids) == 0 {
		return out
	}
	out.kids = make([]layoutTree, len(n.kids))
	for i, k := range n.kids {
		out.kids[i] = cloneTree(k)
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m Model) currentNoteLine() int {
	if m.histPtr >= 0 && m.histPtr < len(m.hist) {
		return m.hist[m.histPtr].Line
	}
	return 0
}

func (m *Model) setCurrentNoteLine(line int) {
	if m.histPtr >= 0 && m.histPtr < len(m.hist) {
		m.hist[m.histPtr].Line = line
		m.hist[m.histPtr].HasView = false
	}
}

func (m *Model) ensureNoteFrame(id string, line int) {
	if id == "" {
		return
	}
	if m.histPtr >= 0 && m.histPtr < len(m.hist) {
		fr := m.hist[m.histPtr]
		if fr.Screen == kindNote && fr.NoteID == id {
			return
		}
	}
	m.beginNav()
	m.hist = append(m.hist, histFrame{Screen: kindNote, NoteID: id, Line: line})
	m.histPtr = len(m.hist) - 1
}
