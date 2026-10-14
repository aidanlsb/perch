package ui

import (
	"fmt"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/editor"
	"github.com/aidanlsb/perch/internal/frontmatter"
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
		return m, nil
	case schemaMsg:
		return m.onSchema(msg)
	case queryMsg:
		return m.onQuery(msg)
	case readMsg:
		return m.onRead(msg)
	case searchMsg:
		return m.onSearch(msg)
	case editorDoneMsg:
		return m.onEditorDone(msg)
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m Model) onSchema(msg schemaMsg) (Model, tea.Cmd) {
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
	m.status = fmt.Sprintf("%d types", len(m.schema.Types))
	if len(m.nav) == 0 {
		return m, nil
	}
	return m.loadNav(m.navCur)
}

func (m Model) onQuery(msg queryMsg) (Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		m.objects = nil
		m.applyFilterAndGroup()
		return m, nil
	}
	m.errMsg = ""
	m.objects = msg.objects
	if msg.typeName != "" {
		m.typeName = msg.typeName
	}
	m.hasMore = msg.hasMore
	m.total = msg.total
	if msg.vault != "" {
		m.vaultPath = msg.vault
	}
	m.listCur = 0
	m.listOff = 0
	m.boardCol = 0
	m.boardRow = 0
	m.dateSec = 0
	m.dateRow = 0
	m.applyFilterAndGroup()
	n := len(m.filtered)
	m.status = fmt.Sprintf("%d objects", n)
	if m.hasMore {
		m.status = fmt.Sprintf("%d of %d (truncated)", n, m.total)
	}
	return m, nil
}

func (m Model) onRead(msg readMsg) (Model, tea.Cmd) {
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

func (m Model) onSearch(msg searchMsg) (Model, tea.Cmd) {
	if msg.gen != m.gen && m.overlay == overlaySwitcher {
		// still accept if query matches what is typed
		if msg.query != m.switcherQ {
			return m, nil
		}
	}
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if msg.result != nil {
		m.switcherHit = msg.result.Items
	} else {
		m.switcherHit = nil
	}
	m.switcherCur = 0
	return m, nil
}

func (m Model) onEditorDone(msg editorDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
	}
	if m.screen == kindNote && msg.objectID != "" {
		gen := m.nextGen()
		m.loading = true
		m.status = "refreshing…"
		return m, fetchRead(m.client, gen, msg.objectID)
	}
	if m.screen == "browse" {
		return m.refresh()
	}
	return m, nil
}

func (m Model) onKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.overlay {
	case overlayHelp:
		if key == "?" || key == "esc" || key == "q" {
			m.overlay = overlayNone
		}
		return m, nil
	case overlayFilter:
		return m.keysFilter(msg)
	case overlaySwitcher:
		return m.keysSwitcher(msg)
	}
	if m.screen == kindNote {
		return m.keysNote(msg)
	}
	return m.keysBrowse(msg)
}

func (m Model) keysBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q":
		return m, tea.Quit
	case "?":
		m.overlay = overlayHelp
		return m, nil
	case "tab":
		if m.pane == paneSidebar {
			m.pane = paneMain
		} else {
			m.pane = paneSidebar
		}
		return m, nil
	case "/":
		m.overlay = overlayFilter
		return m, nil
	case "s", "ctrl+p":
		m.overlay = overlaySwitcher
		m.switcherQ = ""
		m.switcherHit = nil
		m.switcherCur = 0
		return m, nil
	case "r":
		return m.refresh()
	case "e", "o":
		return m.openEditor()
	case "enter":
		if m.pane == paneSidebar {
			return m.loadNav(m.navCur)
		}
		return m.openSelectedNote()
	case "d":
		if m.viewKind == kindDate {
			m.granularity = config.GranDay
			m.applyFilterAndGroup()
		}
		return m, nil
	case "w":
		if m.viewKind == kindDate {
			m.granularity = config.GranWeek
			m.applyFilterAndGroup()
		}
		return m, nil
	case "m":
		if m.viewKind == kindDate {
			m.granularity = config.GranMonth
			m.applyFilterAndGroup()
		}
		return m, nil
	case "j", "down":
		m.move(1)
		return m, nil
	case "k", "up":
		m.move(-1)
		return m, nil
	case "g":
		m.moveToStart()
		return m, nil
	case "G":
		m.moveToEnd()
		return m, nil
	case "h", "left":
		if m.pane == paneMain && m.viewKind == kindBoard {
			m.moveBoardCol(-1)
			return m, nil
		}
		m.pane = paneSidebar
		return m, nil
	case "l", "right":
		if m.pane == paneSidebar {
			m.pane = paneMain
			return m, nil
		}
		if m.viewKind == kindBoard {
			m.moveBoardCol(1)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) keysNote(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q", "esc", "backspace":
		m.screen = "browse"
		m.note = nil
		m.status = m.sourceLabel
		return m, nil
	case "?":
		m.overlay = overlayHelp
		return m, nil
	case "e", "o":
		return m.openEditor()
	case "s", "ctrl+p":
		m.overlay = overlaySwitcher
		m.switcherQ = ""
		m.switcherHit = nil
		return m, nil
	case "r":
		if m.note != nil {
			gen := m.nextGen()
			m.loading = true
			return m, fetchRead(m.client, gen, m.note.ObjectID)
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
		m.noteScroll = 0
		return m, nil
	}
	return m, nil
}

func (m Model) keysFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.filter = ""
		m.overlay = overlayNone
		m.applyFilterAndGroup()
		return m, nil
	case "enter":
		m.overlay = overlayNone
		return m, nil
	case "backspace":
		m.filter = dropLastRune(m.filter)
		m.applyFilterAndGroup()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	if msg.Type == tea.KeyRunes {
		m.filter += string(msg.Runes)
		m.applyFilterAndGroup()
	}
	return m, nil
}

func (m Model) keysSwitcher(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.switcherCur >= 0 && m.switcherCur < len(m.switcherHit) {
			id := m.switcherHit[m.switcherCur].ObjectID
			if m.switcherHit[m.switcherCur].IsSection && m.switcherHit[m.switcherCur].FileObjectID != "" {
				id = m.switcherHit[m.switcherCur].FileObjectID
			}
			m.overlay = overlayNone
			return m.openNote(id)
		}
		return m, nil
	case "down", "ctrl+n":
		if m.switcherCur+1 < len(m.switcherHit) {
			m.switcherCur++
		}
		return m, nil
	case "up", "ctrl+p":
		if m.switcherCur > 0 {
			m.switcherCur--
		}
		return m, nil
	case "backspace":
		m.switcherQ = dropLastRune(m.switcherQ)
		return m.searchIfReady()
	case "ctrl+c":
		return m, tea.Quit
	}
	if msg.Type == tea.KeyRunes {
		m.switcherQ += string(msg.Runes)
		return m.searchIfReady()
	}
	return m, nil
}

func (m Model) searchIfReady() (Model, tea.Cmd) {
	q := strings.TrimSpace(m.switcherQ)
	if len([]rune(q)) < 2 {
		m.switcherHit = nil
		return m, nil
	}
	gen := m.nextGen()
	return m, fetchSearch(m.client, gen, q)
}

func (m *Model) move(delta int) {
	if m.pane == paneSidebar {
		m.navCur = clamp(m.navCur+delta, 0, max(0, len(m.nav)-1))
		return
	}
	switch m.viewKind {
	case kindBoard:
		if len(m.board) == 0 {
			return
		}
		col := &m.board[m.boardCol]
		m.boardRow = clamp(m.boardRow+delta, 0, max(0, len(col.Items)-1))
	case kindDate:
		m.moveDate(delta)
	default:
		m.listCur = clamp(m.listCur+delta, 0, max(0, len(m.filtered)-1))
	}
}

func (m *Model) moveDate(delta int) {
	if len(m.dateCols) == 0 {
		return
	}
	m.dateRow += delta
	for m.dateRow < 0 && m.dateSec > 0 {
		m.dateSec--
		m.dateRow = len(m.dateCols[m.dateSec].Items)
		if m.dateRow > 0 {
			m.dateRow--
		}
	}
	for m.dateSec < len(m.dateCols) && m.dateRow >= len(m.dateCols[m.dateSec].Items) {
		overflow := m.dateRow - len(m.dateCols[m.dateSec].Items)
		if m.dateSec+1 >= len(m.dateCols) {
			m.dateRow = max(0, len(m.dateCols[m.dateSec].Items)-1)
			break
		}
		m.dateSec++
		m.dateRow = overflow
	}
	if m.dateRow < 0 {
		m.dateRow = 0
	}
}

func (m *Model) moveBoardCol(delta int) {
	if len(m.board) == 0 {
		return
	}
	m.boardCol = clamp(m.boardCol+delta, 0, len(m.board)-1)
	m.boardRow = clamp(m.boardRow, 0, max(0, len(m.board[m.boardCol].Items)-1))
}

func (m *Model) moveToStart() {
	if m.pane == paneSidebar {
		m.navCur = 0
		return
	}
	m.listCur = 0
	m.boardRow = 0
	m.dateSec = 0
	m.dateRow = 0
}

func (m *Model) moveToEnd() {
	if m.pane == paneSidebar {
		m.navCur = max(0, len(m.nav)-1)
		return
	}
	m.listCur = max(0, len(m.filtered)-1)
	if len(m.board) > 0 {
		m.boardRow = max(0, len(m.board[m.boardCol].Items)-1)
	}
	if len(m.dateCols) > 0 {
		m.dateSec = len(m.dateCols) - 1
		m.dateRow = max(0, len(m.dateCols[m.dateSec].Items)-1)
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

func (m Model) loadNav(index int) (Model, tea.Cmd) {
	if index < 0 || index >= len(m.nav) {
		return m, nil
	}
	item := m.nav[index]
	m.navCur = index
	m.screen = "browse"
	m.note = nil
	m.filter = ""
	m.sourceLabel = item.Label
	m.sourceQuery = item.Source
	m.typeName = item.Type
	m.viewKind = kindList
	m.groupBy = ""
	m.sortField = ""
	m.listFields = nil
	m.granularity = config.GranDay

	if item.Kind == navQuery && item.Query != nil && len(item.Query.Args) > 0 {
		m.objects = nil
		m.applyFilterAndGroup()
		m.errMsg = fmt.Sprintf("saved query %q needs arguments: %s", item.Query.Name, strings.Join(item.Query.Args, ", "))
		m.status = item.Label
		return m, nil
	}
	if item.View != nil {
		m.viewKind = item.View.Kind
		m.sourceQuery = item.View.Source
		m.groupBy = item.View.GroupBy
		m.sortField = item.View.Sort
		m.listFields = item.View.Fields
		if item.View.Granularity != "" {
			m.granularity = item.View.Granularity
		}
		if strings.HasPrefix(item.View.Source, "type:") {
			m.typeName = strings.TrimPrefix(item.View.Source, "type:")
		}
	} else if item.Kind == navType {
		m.typeName = item.Type
		ts := m.typeSchema()
		m.listFields = defaultFields(ts)
		if ts.NameField != "" {
			m.sortField = ts.NameField
		}
	}
	gen := m.nextGen()
	m.loading = true
	m.errMsg = ""
	m.status = "loading " + m.sourceLabel + "…"
	ts := m.typeSchema()
	return m, fetchQuery(m.client, gen, m.sourceQuery, m.typeName, m.queryLimit, ts)
}

func defaultFields(ts api.TypeSchema) []string {
	names := ts.FieldNames()
	if len(names) > 4 {
		return names[:4]
	}
	return names
}

func (m Model) refresh() (Model, tea.Cmd) {
	if m.screen == kindNote && m.note != nil {
		gen := m.nextGen()
		m.loading = true
		return m, fetchRead(m.client, gen, m.note.ObjectID)
	}
	if m.sourceQuery == "" {
		return m, fetchSchema(m.client)
	}
	gen := m.nextGen()
	m.loading = true
	return m, fetchQuery(m.client, gen, m.sourceQuery, m.typeName, m.queryLimit, m.typeSchema())
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
	return m, fetchRead(m.client, gen, id)
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
	return m, fetchRead(m.client, gen, id)
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

func (m Model) openEditor() (Model, tea.Cmd) {
	path := ""
	objectID := ""
	if m.screen == kindNote && m.note != nil {
		path = editor.FilePath(m.vaultPath, m.note.Path)
		objectID = m.note.ObjectID
	} else if obj := m.selectedObject(); obj != nil {
		path = editor.FilePath(m.vaultPath, obj.FilePath)
		objectID = obj.ID
	}
	ed := editor.Resolve(m.editor)
	cmd, err := editor.Command(ed, path)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err, objectID: objectID}
	})
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
