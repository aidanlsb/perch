package ui

import (
	"fmt"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/editor"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/layout"
	"github.com/aidanlsb/perch/internal/md"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.update(msg)
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recomputeRegions()
		return m, nil
	case schemaMsg:
		return m.onSchema(msg)
	case queryMsg:
		return m.onQuery(msg)
	case readMsg:
		return m.onRead(msg)
	case searchMsg:
		return m.onSearch(msg)
	case objectsMsg:
		return m.onObjects(msg)
	case prefetchMsg:
		return m.onPrefetch(msg)
	case editorDoneMsg:
		return m.onEditorDone(msg)
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m Model) onSchema(msg schemaMsg) (Model, tea.Cmd) {
	m.applySession(msg.session)
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		m.loading = false
		return m, nil
	}
	if msg.schema != nil {
		m.schema = *msg.schema
	}
	m.rebuildNav()
	m.ready = true
	m.loading = false
	m.needPrefetch = true
	m.corpusDone = false
	m.status = fmt.Sprintf("%d types", len(m.schema.Types))
	if m.cfg != nil {
		m.cfgWarnings = m.cfg.Validate(m.schema)
		if len(m.cfgWarnings) > 0 {
			m.status = "config: " + m.cfgWarnings[0]
		}
	}
	if m.open != "" {
		return m.openNamed(m.open)
	}
	names := m.schema.SortedTypeNames()
	if len(names) == 0 {
		m.corpusLoading = true
		return m, m.corpusCmd()
	}
	return m.loadType(names[0])
}

func (m Model) openNamed(name string) (Model, tea.Cmd) {
	m.viewsOpen = true
	m.rebuildNav()
	if m.cfg != nil {
		if _, ok := m.cfg.Views[name]; ok {
			return m.loadNamedView(name)
		}
		if _, ok := m.cfg.Layouts[name]; ok {
			return m.loadNamedLayout(name)
		}
	}
	m.errMsg = "unknown view or layout " + name
	names := m.schema.SortedTypeNames()
	if len(names) == 0 {
		m.corpusLoading = true
		return m, m.corpusCmd()
	}
	return m.loadType(names[0])
}

func (m Model) corpusCmd() tea.Cmd {
	return fetchObjectsPage(m.client, 0, objectsPageSize)
}

func (m Model) prefetchCmd() tea.Cmd {
	names := m.schema.SortedTypeNames()
	if len(names) == 0 {
		return nil
	}
	rest := names[1:]
	return fetchPrefetch(m.client, names[0], rest, m.queryLimit, m.schema.Types[names[0]])
}

func (m Model) onObjects(msg objectsMsg) (Model, tea.Cmd) {
	if msg.fallback || (msg.err != nil && len(msg.objects) == 0) {
		m.corpusDone = true
		m.corpusLoading = false
		return m, m.prefetchCmd()
	}
	m.mergeNotes(msg.objects)
	if m.overlay == overlayPicker && m.pickerKind == pickerNotes {
		m.rebuildNotePicks()
		m.refilterPicker()
	}
	if msg.hasMore && msg.nextOffset > msg.offset {
		return m, fetchObjectsPage(m.client, msg.nextOffset, objectsPageSize)
	}
	m.corpusDone = true
	m.corpusLoading = false
	return m, nil
}

func (m Model) onPrefetch(msg prefetchMsg) (Model, tea.Cmd) {
	m.mergeNotes(msg.objects)
	if len(msg.rest) == 0 {
		return m, nil
	}
	next := msg.rest[0]
	if _, ok := m.queryCache["type:"+next]; ok {
		return m, fetchPrefetch(m.client, "", msg.rest[1:], m.queryLimit, api.TypeSchema{})
	}
	return m, fetchPrefetch(m.client, next, msg.rest[1:], m.queryLimit, m.schema.Types[next])
}

func (m Model) onQuery(msg queryMsg) (Model, tea.Cmd) {
	p := m.paneForQuery(msg)
	if p == nil || p.gen != msg.gen {
		return m, nil
	}
	p.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		p.objects = nil
		p.applyFilterAndGroup(m.typeSchemaFor(p))
		return m, nil
	}
	m.errMsg = ""
	p.objects = msg.objects
	if msg.typeName != "" {
		p.typeName = msg.typeName
	}
	p.hasMore = msg.hasMore
	p.total = msg.total
	if msg.vault != "" {
		m.vaultPath = msg.vault
	}
	p.resetCursor()
	ts := m.typeSchemaFor(p)
	if len(p.listFields) == 0 {
		p.listFields = defaultFields(ts)
		p.spec.Fields = p.listFields
	}
	p.applyFilterAndGroup(ts)
	m.mergeNotes(p.objects)
	if m.queryCache != nil {
		m.queryCache[cacheKey(p.sourceQuery, p.sourceInputs)] = queryCacheEntry{
			objects:  p.objects,
			typeName: p.typeName,
			hasMore:  p.hasMore,
			total:    p.total,
		}
	}
	n := len(p.filtered)
	m.status = fmt.Sprintf("%d objects", n)
	if p.hasMore {
		m.status = fmt.Sprintf("%d of %d (truncated)", n, p.total)
	}
	var cmds []tea.Cmd
	if c := m.previewCmd(); c != nil {
		cmds = append(cmds, c)
	}
	if m.needPrefetch {
		m.needPrefetch = false
		m.corpusLoading = true
		if c := m.corpusCmd(); c != nil {
			cmds = append(cmds, c)
		}
	}
	if p.bind == "" {
		m2, bound := m.refreshBound(p.id)
		m = m2
		if bound != nil {
			cmds = append(cmds, bound)
		}
	}
	return m, batchCmds(cmds)
}

func (m *Model) paneForQuery(msg queryMsg) *viewState {
	if msg.pane != "" {
		if p := m.viewByID(msg.pane); p != nil {
			return p
		}
	}
	for _, p := range m.panes {
		if p.gen == msg.gen {
			return p
		}
	}
	if len(m.panes) == 1 {
		return m.panes[0]
	}
	return nil
}

func (m Model) onRead(msg readMsg) (Model, tea.Cmd) {
	if msg.purpose == "preview" {
		return m.onPreviewRead(msg)
	}
	if msg.gen != m.gen {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.errMsg = ""
	m.note = msg.result
	if msg.result != nil && m.readCache != nil {
		m.readCache[msg.result.ObjectID] = msg.result
	}
	if msg.vault != "" {
		m.vaultPath = msg.vault
	}
	m.noteDoc = frontmatter.Split(msg.result.Content)
	width := m.noteBodyWidth()
	m.noteBody = md.Render(m.noteDoc.Body, width)
	m.noteScroll = 0
	m.linkIdx = 0
	m.backIdx = 0
	m.screen = kindNote
	m.pane = paneMain
	if len(msg.result.Links) == 0 && len(msg.result.Backlinks) > 0 {
		m.pane = paneBacklinks
	}
	m.status = msg.result.ObjectID
	return m, nil
}

func (m Model) onPreviewRead(msg readMsg) (Model, tea.Cmd) {
	if msg.id != "" && msg.id != m.previewID {
		return m, nil
	}
	m.previewLoading = false
	if msg.err != nil {
		m.preview = nil
		m.previewBody = styleErr.Render(msg.err.Error())
		return m, nil
	}
	if msg.result != nil {
		if m.readCache != nil {
			m.readCache[msg.result.ObjectID] = msg.result
		}
		m.setPreview(msg.result)
		if msg.vault != "" {
			m.vaultPath = msg.vault
		}
	}
	return m, nil
}

func (m *Model) setPreview(rd *api.ReadResult) {
	m.preview = rd
	m.previewDoc = frontmatter.Split(rd.Content)
	w := m.previewWidth
	if w < 20 {
		w = 36
	}
	m.previewBody = md.Render(m.previewDoc.Body, w-2)
	m.previewScroll = 0
}

func (m Model) onSearch(msg searchMsg) (Model, tea.Cmd) {
	if m.overlay == overlayPicker && m.pickerKind == pickerNotes {
		if msg.query != strings.TrimSpace(m.pickerQ) {
			return m, nil
		}
	} else if msg.gen != m.gen {
		return m, nil
	}
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if m.overlay != overlayPicker || m.pickerKind != pickerNotes || msg.result == nil {
		return m, nil
	}
	for _, hit := range msg.result.Items {
		id := hit.ObjectID
		if hit.IsSection && hit.FileObjectID != "" {
			id = hit.FileObjectID
		}
		if id == "" {
			continue
		}
		obj := api.Object{ID: id, Title: hit.Title, FilePath: hit.FilePath}
		if existing, ok := m.noteIndex[id]; ok {
			obj = existing
			if obj.Title == "" {
				obj.Title = hit.Title
			}
		}
		m.noteIndex[id] = obj
	}
	m.rebuildNotePicks()
	m.refilterPicker()
	return m, nil
}

func (m Model) onEditorDone(msg editorDoneMsg) (Model, tea.Cmd) {
	if msg.config {
		return m.onConfigEditorDone(msg)
	}
	if msg.err != nil {
		m.errMsg = msg.err.Error()
	}
	if m.screen == kindNote && msg.objectID != "" {
		gen := m.nextGen()
		m.loading = true
		m.status = "refreshing…"
		return m, fetchRead(m.client, gen, msg.objectID, "note")
	}
	if m.screen == "browse" {
		return m.refresh()
	}
	return m, nil
}

func (m Model) onKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.overlay {
	case overlayHelp:
		if key == "?" || key == "esc" || key == "q" {
			m.overlay = overlayNone
		}
		return m, nil
	case overlaySearch:
		return m.keysSearch(msg)
	case overlayPicker:
		return m.keysPicker(msg)
	case overlayLeader:
		return m.keysLeader(msg)
	case overlayRQL:
		return m.keysRQL(msg)
	case overlayArgs:
		return m.keysArgs(msg)
	case overlaySave:
		return m.keysSave(msg)
	}
	if key == "e" {
		return m.openEditor()
	}
	if key == "?" {
		m.overlay = overlayHelp
		return m, nil
	}
	if isSlash(msg) {
		return m.startSearch()
	}
	if m.screen == kindNote {
		return m.keysNote(msg)
	}
	return m.keysBrowse(msg)
}

func (m Model) keysLeader(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	m.overlay = overlayNone
	if isSlash(msg) {
		return m.startSearch()
	}
	switch key {
	case "t":
		return m.openTypePicker()
	case "q":
		return m.openQueryPicker()
	case "f":
		return m.openNotesPicker()
	case "d":
		return m.openDaily()
	case "l":
		return m.openViewsPicker()
	case "c":
		return m.openConfigAtView()
	case "esc", " ":
		return m, nil
	}
	return m, nil
}

func (m Model) keysRQL(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		m.overlay = overlayNone
		m.rql = ""
		return m, nil
	case "enter":
		q := strings.TrimSpace(m.rql)
		m.overlay = overlayNone
		if q == "" {
			return m, nil
		}
		return m.loadRQL(q)
	case "backspace":
		m.rql = dropLastRune(m.rql)
		return m, nil
	}
	if text := typedText(msg); text != "" {
		m.rql += text
	}
	return m, nil
}

func (m Model) keysSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		m.searchQ = ""
		m.searchHits = nil
		m.overlay = overlayNone
		m.applyFilterAndGroup()
		return m, nil
	case "enter":
		m.overlay = overlayNone
		m.jumpSearchFirst()
		return m, nil
	case "backspace":
		m.searchQ = dropLastRune(m.searchQ)
		m.jumpSearchFirst()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	if text := typedText(msg); text != "" {
		m.searchQ += text
		m.jumpSearchFirst()
	}
	return m, nil
}

func (m Model) keysBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	if m.pending == "ctrlw" {
		return m.keysCtrlW(msg)
	}
	if m.pending == "g" {
		m.pending = ""
		switch key {
		case "g":
			m.moveToStart()
			return m.afterMove()
		case "l":
			return m.openViewsPicker()
		}
	}
	if isSlash(msg) {
		return m.startSearch()
	}
	switch key {
	case "q":
		return m.openQueryPicker()
	case "esc":
		if m.searchQ != "" {
			m.searchQ = ""
			m.searchHits = nil
			m.applyFilterAndGroup()
		}
		return m, nil
	case "t":
		return m.openTypePicker()
	case "ctrl+p":
		return m.openNotesPicker()
	case " ":
		m.overlay = overlayLeader
		return m, nil
	case ",":
		return m.openConfig(0)
	case "S":
		return m.startSaveView()
	case "n":
		m.jumpSearch(1)
		return m.afterMove()
	case "N":
		m.jumpSearch(-1)
		return m.afterMove()
	case "d":
		return m.openDaily()
	case ".":
		return m.cycleGroup()
	case "p":
		return m.togglePreview()
	case ":":
		m.overlay = overlayRQL
		m.rql = ""
		return m, nil
	case "r":
		return m.refresh()
	case "enter":
		return m.enterBrowse()
	case "tab":
		return m.cycleFocus(1), nil
	case "shift+tab":
		return m.cycleFocus(-1), nil
	case "ctrl+b":
		return m.toggleSidebar(), nil
	case "ctrl+e":
		return m.toggleSidebarFocus(), nil
	case "ctrl+w":
		m.pending = "ctrlw"
		return m, nil
	case "ctrl+h":
		return m.moveFocus(-1, 0), nil
	case "ctrl+j":
		return m.moveFocus(0, 1), nil
	case "ctrl+k":
		return m.moveFocus(0, -1), nil
	case "ctrl+l":
		return m.moveFocus(1, 0), nil
	case "j", "down":
		if m.focus == focusPreview {
			m.previewScroll++
			return m, nil
		}
		m.move(1)
		return m.afterMove()
	case "k", "up":
		if m.focus == focusPreview {
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		}
		m.move(-1)
		return m.afterMove()
	case "g":
		m.pending = "g"
		return m, nil
	case "G":
		if m.focus == focusPreview {
			m.previewScroll = 1 << 20
			return m, nil
		}
		m.moveToEnd()
		return m.afterMove()
	case "ctrl+d":
		m.move(m.pageSize())
		return m.afterMove()
	case "ctrl+u":
		m.move(-m.pageSize())
		return m.afterMove()
	case "h", "left":
		return m.keyH()
	case "l", "right":
		return m.keyL()
	}
	return m, nil
}

func (m Model) enterBrowse() (Model, tea.Cmd) {
	if m.focus == focusSidebar {
		return m.activateNav()
	}
	if tgt := m.boundTarget(m.focus); tgt != "" {
		m.setFocus(tgt)
		return m, nil
	}
	return m.openSelectedNote()
}

func (m Model) keyH() (Model, tea.Cmd) {
	if m.focus == focusPreview {
		if len(m.panes) > 0 {
			m.setFocus(m.panes[len(m.panes)-1].id)
		}
		return m, nil
	}
	if m.focus == focusSidebar {
		if m.navCur >= 0 && m.navCur < len(m.nav) && m.nav[m.navCur].Kind == navCat {
			return m.expandCategory(m.nav[m.navCur].Category, false), nil
		}
		return m, nil
	}
	p := m.p()
	if p.spec.Kind == kindBoard {
		p.moveBoardCol(-1)
		return m.afterMove()
	}
	return m, nil
}

func (m Model) keyL() (Model, tea.Cmd) {
	if m.focus == focusSidebar {
		if m.navCur >= 0 && m.navCur < len(m.nav) && m.nav[m.navCur].Kind == navCat {
			return m.expandCategory(m.nav[m.navCur].Category, true), nil
		}
		return m, nil
	}
	p := m.focusedView()
	if p != nil && p.spec.Kind == kindBoard && m.focus == p.id {
		p.moveBoardCol(1)
		return m.afterMove()
	}
	if m.previewOn {
		m.setFocus(focusPreview)
	}
	return m, nil
}

func (m Model) afterMove() (Model, tea.Cmd) {
	cmds := []tea.Cmd{m.previewCmd()}
	if p := m.focusedView(); p != nil {
		var c tea.Cmd
		m, c = m.refreshBound(p.id)
		cmds = append(cmds, c)
	}
	return m, batchCmds(cmds)
}

func (m Model) startSearch() (Model, tea.Cmd) {
	m.pending = ""
	if m.screen == kindNote {
		m.screen = "browse"
		m.note = nil
		m.status = m.p().sourceLabel
	}
	m.overlay = overlaySearch
	return m, nil
}

func (m Model) keysNote(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	if m.pending == "g" {
		m.pending = ""
		if key == "g" {
			m.noteScroll = 0
			return m, nil
		}
	}
	if isSlash(msg) {
		return m.startSearch()
	}
	switch key {
	case "q", "esc", "backspace":
		m.screen = "browse"
		m.note = nil
		m.status = m.p().sourceLabel
		return m, m.previewCmd()
	case "t":
		return m.openTypePicker()
	case "Q":
		return m.openQueryPicker()
	case "ctrl+p":
		return m.openNotesPicker()
	case "d":
		return m.openDaily()
	case "r":
		if m.note != nil {
			gen := m.nextGen()
			m.loading = true
			return m, fetchRead(m.client, gen, m.note.ObjectID, "note")
		}
		return m, nil
	case "tab":
		m.cycleLink(1)
		return m, nil
	case "shift+tab":
		m.cycleLink(-1)
		return m, nil
	case "h", "left":
		m.pane = paneMain
		return m, nil
	case "l", "right":
		m.pane = paneBacklinks
		return m, nil
	case "[":
		return m.historyGo(-1)
	case "]":
		return m.historyGo(1)
	case "enter":
		return m.followOrOpenBacklink()
	case "j", "down":
		if m.pane == paneBacklinks {
			m.moveBack(1)
		} else {
			m.noteScroll++
		}
		return m, nil
	case "k", "up":
		if m.pane == paneBacklinks {
			m.moveBack(-1)
		} else if m.noteScroll > 0 {
			m.noteScroll--
		}
		return m, nil
	case "g":
		m.pending = "g"
		return m, nil
	case "G":
		m.noteScroll = 1 << 20
		return m, nil
	case "ctrl+d":
		m.noteScroll += m.pageSize()
		return m, nil
	case "ctrl+u":
		m.noteScroll = max(0, m.noteScroll-m.pageSize())
		return m, nil
	}
	return m, nil
}

func (m Model) cycleGroup() (Model, tea.Cmd) {
	p := m.p()
	if p.spec.Kind != layout.KindDate {
		m.status = p.spec.DisplayName()
		return m, nil
	}
	next := layout.NextGranularity(p.spec)
	p.applySpec(next, m.typeSchemaFor(p))
	m.status = next.DisplayName() + " / " + next.Granularity
	return m, m.previewCmd()
}

func (m Model) togglePreview() (Model, tea.Cmd) {
	m.previewOn = !m.previewOn
	if !m.previewOn {
		if m.focus == focusPreview {
			m.focusMainArea()
		}
		return m, nil
	}
	return m, m.previewCmd()
}

func (m Model) previewCmd() tea.Cmd {
	if !m.previewOn || m.screen == kindNote {
		return nil
	}
	obj := m.selectedObject()
	if obj == nil || obj.ID == "" {
		m.preview = nil
		m.previewID = ""
		m.previewBody = ""
		return nil
	}
	if m.previewID == obj.ID && m.preview != nil {
		return nil
	}
	m.previewID = obj.ID
	if rd, ok := m.readCache[obj.ID]; ok {
		m.setPreview(rd)
		return nil
	}
	m.previewLoading = true
	return fetchRead(m.client, m.gen, obj.ID, "preview")
}

func (m Model) openDaily() (Model, tea.Cmd) {
	m.overlay = overlayNone
	if m.daily != nil && strings.TrimSpace(m.daily.TodayID) != "" {
		if m.daily.TodayExists {
			m.status = "daily " + m.daily.TodayID
			return m.openNote(m.daily.TodayID)
		}
		m.status = "no daily note for " + m.daily.TodayID
		return m, nil
	}
	m.status = "no daily note in session"
	return m, nil
}

func (m *Model) move(delta int) {
	if m.focus == focusSidebar {
		m.navCur = clamp(m.navCur+delta, 0, max(0, len(m.nav)-1))
		return
	}
	if p := m.focusedView(); p != nil {
		p.move(delta)
	}
}

func (m *Model) moveToStart() {
	if m.focus == focusSidebar {
		m.navCur = 0
		return
	}
	if m.focus == focusPreview {
		m.previewScroll = 0
		return
	}
	if p := m.focusedView(); p != nil {
		p.moveToStart()
	}
}

func (m *Model) moveToEnd() {
	if m.focus == focusSidebar {
		m.navCur = max(0, len(m.nav)-1)
		return
	}
	if p := m.focusedView(); p != nil {
		p.moveToEnd()
	}
}

func (m *Model) moveBack(delta int) {
	if m.note == nil {
		return
	}
	m.backIdx = clamp(m.backIdx+delta, 0, max(0, len(m.note.Backlinks)-1))
}

func (m *Model) cycleLink(delta int) {
	if m.note == nil || len(m.note.Links) == 0 {
		return
	}
	n := len(m.note.Links)
	m.linkIdx = (m.linkIdx + delta + n) % n
}

func (m Model) refresh() (Model, tea.Cmd) {
	if m.cfgPath != "" {
		m2, err := m.reloadConfig()
		m = m2
		if err != nil {
			m.errMsg = err.Error()
			m.status = "config error; keeping last good"
		}
	}
	if m.screen == kindNote && m.note != nil {
		if m.readCache != nil {
			delete(m.readCache, m.note.ObjectID)
		}
		gen := m.nextGen()
		m.loading = true
		return m, fetchRead(m.client, gen, m.note.ObjectID, "note")
	}
	if len(m.panes) == 0 {
		return m, fetchSchema(m.client)
	}
	var cmds []tea.Cmd
	for _, p := range m.panes {
		if m.queryCache != nil {
			delete(m.queryCache, cacheKey(p.sourceQuery, p.sourceInputs))
		}
		sel := ""
		if p.bind != "" {
			if d := m.viewByID(p.bind); d != nil {
				sel = d.selectedID()
			}
		}
		var cmd tea.Cmd
		m, cmd = m.startPaneQuery(p, sel)
		cmds = append(cmds, cmd)
	}
	return m, batchCmds(cmds)
}

func (m Model) openSelectedNote() (Model, tea.Cmd) {
	obj := m.selectedObject()
	if obj == nil || obj.ID == "" {
		return m, nil
	}
	return m.openNote(obj.ID)
}

func (m Model) openNote(id string) (Model, tea.Cmd) {
	if id == "" {
		return m, nil
	}
	if m.notePtr < len(m.noteStack)-1 {
		m.noteStack = m.noteStack[:m.notePtr+1]
	}
	m.noteStack = append(m.noteStack, noteFrame{ID: id})
	m.notePtr = len(m.noteStack) - 1
	gen := m.nextGen()
	m.loading = true
	m.status = "reading " + id + "…"
	m.overlay = overlayNone
	return m, fetchRead(m.client, gen, id, "note")
}

func (m Model) historyGo(delta int) (Model, tea.Cmd) {
	next := m.notePtr + delta
	if next < 0 {
		m.screen = "browse"
		m.note = nil
		return m, nil
	}
	if next >= len(m.noteStack) {
		return m, nil
	}
	m.notePtr = next
	id := m.noteStack[m.notePtr].ID
	gen := m.nextGen()
	m.loading = true
	return m, fetchRead(m.client, gen, id, "note")
}

func (m Model) followOrOpenBacklink() (Model, tea.Cmd) {
	if m.note == nil {
		return m, nil
	}
	if m.pane == paneBacklinks {
		if m.backIdx >= 0 && m.backIdx < len(m.note.Backlinks) {
			return m.openNote(m.note.Backlinks[m.backIdx].Source)
		}
		return m, nil
	}
	if len(m.note.Links) == 0 {
		return m, nil
	}
	link := m.note.Links[m.linkIdx]
	if tgt := link.FollowTarget(); tgt != "" {
		return m.openNote(tgt)
	}
	if link.Status == api.LinkAmbiguous {
		m.errMsg = "ambiguous: " + strings.Join(link.Matches, ", ")
		return m, nil
	}
	m.errMsg = "unresolved: " + link.Target
	return m, nil
}

func (m *Model) applySession(sess *api.Session) {
	if sess != nil {
		if strings.TrimSpace(sess.VaultPath) != "" {
			m.vaultPath = sess.VaultPath
		}
		if strings.TrimSpace(sess.Editor) != "" {
			m.editor = sess.Editor
		}
		if strings.TrimSpace(sess.EditorMode) != "" {
			m.editorMode = sess.EditorMode
		}
		if sess.Daily != nil && strings.TrimSpace(sess.Daily.TodayID) != "" {
			m.daily = sess.Daily
		}
	}
	if strings.TrimSpace(m.editor) == "" {
		m.editor = editor.Resolve("")
	}
}

func (m Model) openEditor() (Model, tea.Cmd) {
	path := ""
	objectID := ""
	if m.screen == kindNote && m.note != nil {
		path = editor.FilePath(m.vaultPath, m.note.Path)
		objectID = m.note.ObjectID
	} else if m.previewOn && m.focus == focusPreview && m.preview != nil {
		path = editor.FilePath(m.vaultPath, m.preview.Path)
		objectID = m.preview.ObjectID
	} else if obj := m.selectedObject(); obj != nil {
		path = editor.FilePath(m.vaultPath, obj.FilePath)
		objectID = obj.ID
		if path == "" {
			if rd, ok := m.readCache[obj.ID]; ok {
				path = editor.FilePath(m.vaultPath, rd.Path)
			}
		}
	}
	ed := editor.Resolve(m.editor)
	cmd, err := editor.Command(ed, path)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	if editor.ShouldSuspend(ed, m.editorMode) {
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return editorDoneMsg{err: err, objectID: objectID}
		})
	}
	return m, func() tea.Msg {
		if err := cmd.Start(); err != nil {
			return editorDoneMsg{err: err, objectID: objectID}
		}
		return editorDoneMsg{objectID: objectID}
	}
}

func (m Model) noteBodyWidth() int {
	w := m.width - sidebarWidth - 4
	if m.note != nil && len(m.note.Backlinks) > 0 {
		w -= 22
	}
	if w < 24 {
		w = 24
	}
	return w
}
