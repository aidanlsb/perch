package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/md"
	"github.com/aidanlsb/perch/internal/termesc"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	return termesc.ForTUI(m.view())
}

func (m Model) view() string {
	if m.width == 0 {
		return "perch"
	}
	header := m.viewHeader()
	footer := m.viewFooter()
	bodyH := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))
	var body string
	switch {
	case m.screen == kindNote && m.note != nil:
		body = m.viewNote(bodyH)
	case m.screen == kindHome:
		body = m.viewHome(bodyH)
	default:
		body = m.viewBrowse(bodyH)
	}
	frame := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	switch m.overlay {
	case overlayHelp:
		return overlayOn(frame, m.width, m.height, m.viewHelp())
	case overlayPicker:
		return overlayOn(frame, m.width, m.height, m.viewPicker())
	case overlayDate:
		return overlayOn(frame, m.width, m.height, m.viewDatePicker())
	case overlayRQL:
		return overlayOn(frame, m.width, m.height, m.viewRQL())
	case overlayArgs:
		return overlayOn(frame, m.width, m.height, m.viewArgs())
	case overlaySave:
		return overlayOn(frame, m.width, m.height, m.viewSave())
	case overlayPrompt:
		return overlayOn(frame, m.width, m.height, m.viewPrompt())
	case overlayConfirm:
		return overlayOn(frame, m.width, m.height, m.viewConfirm())
	case overlaySearch:
		return frame
	}
	return frame
}

func (m Model) viewHeader() string {
	left := styleHeader.Render("perch")
	p := m.p()
	label := p.sourceLabel
	if m.currentLayout != "" {
		label = m.currentLayout
		if m.focus != focusPreview && m.focus != "" {
			label += " · " + m.focus
		}
	}
	mid := styleMuted.Render(" · " + label)
	if p.spec.Kind != "" && m.screen == kindBrowse {
		name := m.currentLayoutName()
		mid += styleMuted.Render("  " + name)
		if p.spec.Kind == kindDate && p.granularity != "" {
			mid += styleDim.Render(" / " + p.granularity)
		}
	}
	if m.screen == kindHome {
		mid = styleMuted.Render(" · home")
	}
	if m.screen == kindNote && m.note != nil {
		mid = styleMuted.Render(" · " + m.note.ObjectID)
	}
	right := m.status
	if m.loading {
		right = "loading…"
	}
	if m.overlay == overlaySearch || m.searchQ != "" {
		right = "/" + m.searchQ
		if m.overlay == overlaySearch {
			right += "█"
		}
		if n := len(m.searchHits); n > 0 && m.searchQ != "" {
			right += fmt.Sprintf("  %d/%d", m.searchAt+1, n)
		}
	}
	live := m.liveErr
	rightW := lipgloss.Width(right)
	if live != "" {
		rightW += lipgloss.Width("  " + live)
	}
	gap := max(0, m.width-lipgloss.Width(left+mid)-rightW-1)
	line := left + mid + strings.Repeat(" ", gap) + styleStatus.Render(right)
	if live != "" {
		line += "  " + styleWarn.Render(live)
	}
	if lipgloss.Width(line) < m.width {
		line += strings.Repeat(" ", max(0, m.width-lipgloss.Width(line)))
	}
	return line
}

func (m Model) viewFooter() string {
	if m.errMsg != "" {
		return styleErr.Render(truncate(m.errMsg, m.width))
	}
	if len(m.cfgWarnings) > 0 {
		return styleWarn.Render(truncate(strings.Join(m.cfgWarnings, "; "), m.width))
	}
	keys := footerKeys(m)
	return styleFooter.Render(truncate(keys, m.width))
}

func (m Model) viewBrowse(height int) string {
	if m.zoomed {
		switch m.zoomTarget() {
		case focusPreview:
			return m.viewPreview(m.width, height)
		default:
			return m.viewPaneBox(m.viewByID(m.zoomTarget()), m.width, height, true)
		}
	}
	if m.previewOn {
		pw := max(previewMinW, m.width*2/5)
		if pw > m.width-16 {
			pw = max(previewMinW, m.width/3)
		}
		mainW := max(12, m.width-pw)
		main := m.viewTree(m.tree, mainW, height)
		prev := m.viewPreview(pw, height)
		return lipgloss.JoinHorizontal(lipgloss.Top, main, prev)
	}
	return m.viewTree(m.tree, m.width, height)
}

func (m Model) viewTree(n layoutTree, width, height int) string {
	if n.split == "" || len(n.kids) == 0 {
		id := n.id
		if id == "" && len(m.panes) == 1 {
			id = m.panes[0].id
		}
		return m.viewPaneBox(m.viewByID(id), width, height, m.focus == id)
	}
	sizes := splitSizes(splitAxis(n.split, width, height), len(n.kids))
	parts := make([]string, 0, len(n.kids))
	for i, kid := range n.kids {
		if n.split == "col" {
			parts = append(parts, m.viewTree(kid, width, sizes[i]))
		} else {
			parts = append(parts, m.viewTree(kid, sizes[i], height))
		}
	}
	if n.split == "col" {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) viewPaneBox(v *viewState, width, height int, focused bool) string {
	fg := colBorder
	if focused {
		fg = colAccent
	}
	box := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.NormalBorder()).
		BorderForeground(fg)
	innerW := max(1, width-2)
	innerH := max(1, height-2)
	return box.Render(m.viewPane(v, innerW, innerH, focused))
}

func (m Model) viewPane(v *viewState, width, height int, focused bool) string {
	pad := lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1)
	if v == nil {
		return pad.Render(styleMuted.Render("empty"))
	}
	if v.loading && len(v.objects) == 0 {
		return pad.Render(styleMuted.Render("loading…"))
	}
	switch v.spec.Kind {
	case kindBoard:
		return pad.Render(m.viewBoard(v, width-1, height, focused))
	case kindDate:
		return pad.Render(m.viewDate(v, width-1, height, focused))
	default:
		return pad.Render(m.viewList(v, width-1, height, focused))
	}
}

func (m Model) viewList(v *viewState, width, height int, focused bool) string {
	if len(v.filtered) == 0 {
		return styleMuted.Render("no objects")
	}
	cols, flex := listColumns(v.filtered, v.listFields, m.typeSchemaFor(v))
	off := v.listOff
	if v.listCur < off {
		off = v.listCur
	}
	if v.listCur >= off+height-1 {
		off = v.listCur - (height - 2)
	}
	if off < 0 {
		off = 0
	}

	widths := columnWidths(v.filtered, cols, width, flex)
	var b strings.Builder
	b.WriteString(styleColHead.Render(formatRow(cols, widths)))
	b.WriteByte('\n')
	rows := height - 1
	end := min(len(v.filtered), off+rows)
	for i := off; i < end; i++ {
		obj := m.labeled(v.filtered[i])
		cells := rowCells(obj, cols)
		line := formatRowFlex(cells, widths, flex)
		if i == v.listCur {
			st := styleRowSel
			if !focused {
				st = styleRow
			}
			b.WriteString(st.Width(width).Render(line))
		} else {
			b.WriteString(styleRow.Render(line))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) viewBoard(v *viewState, width, height int, focused bool) string {
	if len(v.board) == 0 {
		return styleMuted.Render("no columns")
	}
	n := len(v.board)
	colW := max(12, (width-n+1)/max(1, n))
	if colW*n > width {
		colW = max(12, width/max(1, min(n, 4)))
	}
	maxCols := max(1, width/max(colW, 12))
	start := 0
	if v.boardCol >= maxCols {
		start = v.boardCol - maxCols + 1
	}
	end := min(n, start+maxCols)
	colW = max(12, (width-(end-start)+1)/max(1, end-start))

	var cols []string
	for i := start; i < end; i++ {
		c := v.board[i]
		head := styleBoardHead.Width(colW).Render(truncate(fmt.Sprintf("%s (%d)", c.Label, len(c.Items)), colW))
		var rows []string
		rows = append(rows, head)
		rows = append(rows, styleDim.Render(strings.Repeat("─", min(colW, 8))))
		visible := height - 2
		off := 0
		if i == v.boardCol && v.boardRow >= visible {
			off = v.boardRow - visible + 1
		}
		for j := off; j < len(c.Items) && j-off < visible; j++ {
			label := truncate(m.displayTitle(c.Items[j]), colW)
			if i == v.boardCol && j == v.boardRow {
				st := styleRowSel
				if !focused {
					st = styleRow
				}
				rows = append(rows, st.Width(colW).Render(label))
			} else {
				rows = append(rows, styleRow.Width(colW).Render(label))
			}
		}
		if len(c.Items) == 0 {
			rows = append(rows, styleDim.Render("empty"))
		}
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, rows...))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) viewDate(v *viewState, width, height int, focused bool) string {
	if len(v.dateCols) == 0 {
		return styleMuted.Render("no dates")
	}
	var lines []string
	for si, sec := range v.dateCols {
		lines = append(lines, styleDateHead.Width(width).Render(fmt.Sprintf("%s  %d", sec.Label, len(sec.Items))))
		for ri, obj := range sec.Items {
			label := truncate("  "+m.displayTitle(obj), width)
			if si == v.dateSec && ri == v.dateRow {
				st := styleRowSel
				if !focused {
					st = styleRow
				}
				lines = append(lines, st.Width(width).Render(label))
			} else {
				lines = append(lines, styleRow.Render(label))
			}
		}
	}
	total := len(lines)
	curLine := 0
	n := 0
	for si, sec := range v.dateCols {
		n++ // header
		if si == v.dateSec {
			curLine = n + v.dateRow
			break
		}
		n += len(sec.Items)
	}
	off := 0
	if curLine >= height {
		off = curLine - height + 1
	}
	end := min(total, off+height)
	if off > end {
		off = 0
	}
	return strings.Join(lines[off:end], "\n")
}

func (m Model) viewNote(height int) string {
	if m.note == nil {
		return styleMuted.Render("no note")
	}
	backW := 0
	if len(m.note.Backlinks) > 0 {
		backW = min(28, max(16, m.width/5))
	}
	mainW := max(20, m.width-backW)
	header := m.viewNoteHeader(mainW)
	linkBar := m.viewLinkBar(mainW)
	used := lipgloss.Height(header) + lipgloss.Height(linkBar)
	bodyH := max(1, height-used)
	body := windowLines(m.noteBody, m.noteScroll, bodyH, mainW)
	main := lipgloss.JoinVertical(lipgloss.Left, header, body, linkBar)
	if backW == 0 {
		return main
	}
	back := m.viewBacklinks(backW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, main, back)
}

func (m Model) viewNoteHeader(width int) string {
	var lines []string
	id := m.note.ObjectID
	heading := m.headingFromDoc(id, m.noteDoc)
	title := id
	if heading != "" && heading != id {
		title = heading + "  " + styleDim.Render(id)
	}
	skip := m.schemaNameField(m.objectType(id))
	lines = append(lines, styleHeader.Render(truncate(title, width)))
	if m.note.Path != "" {
		lines = append(lines, styleDim.Render(truncate(m.note.Path, width)))
	}
	for _, k := range m.noteDoc.Order {
		if skip != "" && k == skip {
			continue
		}
		line := styleFMKey.Render(k+": ") + styleFMVal.Render(m.noteDoc.Fields[k])
		lines = append(lines, truncate(line, width))
	}
	lines = append(lines, styleDim.Render(strings.Repeat("─", min(width, 40))))
	return strings.Join(lines, "\n")
}

func (m Model) viewLinkBar(width int) string {
	if m.note == nil || len(m.note.Links) == 0 {
		return styleDim.Render("no wiki links")
	}
	link := m.note.Links[m.linkIdx]
	st := styleOk
	switch link.Status {
	case api.LinkUnresolved:
		st = styleLinkBad
	case api.LinkAmbiguous:
		st = styleWarn
	}
	if m.pane == paneMain {
		st = styleLinkSel
	}
	text := link.Text
	if link.SpanOK(m.note.Content) {
		text = m.note.Content[link.ByteStart:link.ByteEnd]
	}
	label := fmt.Sprintf("%d/%d  %s  %s", m.linkIdx+1, len(m.note.Links), text, link.Status)
	return st.Render(truncate(label, width))
}

func (m Model) viewBacklinks(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colBorder).
		PaddingLeft(1)
	previewH := 0
	if m.note != nil && len(m.note.Backlinks) > 0 && (m.pane == paneBacklinks || height >= 14) {
		previewH = height / 2
		if previewH < 6 {
			previewH = min(8, height/3)
		}
		if previewH > height-6 {
			previewH = max(0, height-6)
		}
	}
	listH := height
	if previewH > 0 {
		listH = height - previewH
	}
	list := m.viewBacklinkList(width-2, max(3, listH))
	if previewH <= 0 {
		return style.Height(height).Render(list)
	}
	prev := m.viewBacklinkPreview(width-2, previewH)
	return style.Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, list, prev))
}

func (m Model) viewBacklinkList(width, height int) string {
	var b strings.Builder
	title := "backlinks"
	if m.pane == paneBacklinks {
		title = "▸ backlinks"
	}
	b.WriteString(styleSectionTitle.Render(title))
	b.WriteByte('\n')
	if m.note == nil || len(m.note.Backlinks) == 0 {
		b.WriteString(styleDim.Render("none"))
		return b.String()
	}
	inner := max(8, width-1)
	for i, g := range m.note.Backlinks {
		label := m.backlinkLabel(g)
		if i == m.backIdx {
			st := styleNavSel
			if m.pane != paneBacklinks {
				st = styleNav
			}
			b.WriteString(st.Width(inner).Render(truncate(label, inner)))
		} else {
			b.WriteString(styleNav.Render(truncate(label, inner)))
		}
		b.WriteByte('\n')
		if snip := g.SnippetText(); snip != "" {
			b.WriteString(styleDim.Render(truncate("  "+snip, inner)))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) backlinkLabel(g api.BacklinkGroup) string {
	label := g.DisplayTitle()
	if obj, ok := m.noteIndex[g.Source]; ok && obj.Title != "" && obj.Title != obj.ID {
		label = obj.Title
	}
	if n := g.SourceLine(); n > 0 {
		label += fmt.Sprintf(":%d", n)
	}
	return label
}

func (m Model) viewBacklinkPreview(width, height int) string {
	sep := styleDim.Render(strings.Repeat("─", min(width, 24)))
	if m.note == nil || m.backIdx < 0 || m.backIdx >= len(m.note.Backlinks) {
		return sep
	}
	g := m.note.Backlinks[m.backIdx]
	rd, ok := m.readCache[g.Source]
	var b strings.Builder
	b.WriteString(sep)
	b.WriteByte('\n')
	if !ok || rd == nil {
		b.WriteString(styleMuted.Render("preview…"))
		return b.String()
	}
	doc := frontmatter.Split(rd.Content)
	body := md.Render(doc.Body, max(12, width-1))
	line := backlinkAnchorLine(g, rd.Content, m.note.ObjectID)
	head := m.headingFromDoc(rd.ObjectID, doc)
	if head == "" {
		head = rd.ObjectID
	}
	if line > 0 && rd.Path != "" {
		head = fmt.Sprintf("%s:%d", rd.Path, line)
	} else if rd.Path != "" {
		head = rd.Path
	}
	b.WriteString(styleHeader.Render(truncate(head, width)))
	b.WriteByte('\n')
	off := sourceLineScroll(rd.Content, body, line)
	used := 3
	b.WriteString(windowLinesMark(body, off, max(1, height-used), max(8, width-1), off))
	return strings.TrimRight(b.String(), "\n")
}

func backlinkAnchorLine(g api.BacklinkGroup, content, targetID string) int {
	if n := g.SourceLine(); n > 0 {
		return n
	}
	needles := make([]string, 0, 2+len(g.Lines))
	if t := strings.TrimSpace(targetID); t != "" {
		needles = append(needles, t)
	}
	needles = append(needles, g.Lines...)
	if content == "" {
		return 0
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, n := range needles {
			n = strings.TrimSpace(n)
			if n != "" && strings.Contains(line, n) {
				return i + 1
			}
		}
	}
	return 0
}

func (m Model) viewHelp() string {
	var b strings.Builder
	focus := m.focus
	if m.screen == kindNote {
		focus = "note"
	}
	b.WriteString(styleHeader.Render("perch") + styleMuted.Render("  keys  focused: "+focus) + "\n")
	section := ""
	for _, bind := range Keymap {
		if bind.Section != section {
			section = bind.Section
			b.WriteString("\n" + styleSectionTitle.Render(section) + "\n")
		}
		keys := padTrunc(bind.Keys, 26)
		b.WriteString(styleMuted.Render(keys) + "  " + bind.Desc + "\n")
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	h := max(8, m.height-2)
	off := m.helpOff
	if off > len(lines)-h {
		off = max(0, len(lines)-h)
	}
	if off < 0 {
		off = 0
	}
	end := min(len(lines), off+h)
	body := strings.Join(lines[off:end], "\n")
	w := min(84, max(40, m.width-2))
	return styleHelpBox.Width(w).Height(h).Padding(0, 1).Render(body)
}

func (m Model) viewPicker() string {
	title := "types"
	switch m.pickerKind {
	case pickerNav:
		title = "navigate"
	case pickerQueries:
		title = "saved queries"
	case pickerNotes:
		title = "notes"
	case pickerViews:
		title = "views & layouts"
	case pickerSearch:
		title = "search"
	case pickerActions:
		title = "actions"
	case pickerFields:
		title = "fields"
	case pickerValues:
		title = "value"
	case pickerRefs:
		title = "ref"
	}
	var b strings.Builder
	b.WriteString(styleMuted.Render(title+"  ") + m.pickerQ + "█\n")
	if len(m.pickerHit) == 0 {
		b.WriteString(styleDim.Render("no matches"))
	}
	maxRows := max(6, min(16, m.height-8))
	start := 0
	if m.pickerCur >= maxRows {
		start = m.pickerCur - maxRows + 1
	}
	end := min(len(m.pickerHit), start+maxRows)
	for i := start; i < end; i++ {
		it := m.pickerHit[i]
		line := it.Label
		if it.Sub != "" && it.Sub != it.Label {
			line += "  " + styleDim.Render(truncate(it.Sub, 32))
		}
		if i == m.pickerCur {
			b.WriteString(styleNavSel.Render(truncate(line, 60)))
		} else {
			b.WriteString(styleNav.Render(truncate(line, 60)))
		}
		b.WriteByte('\n')
	}
	return styleModal.Width(min(68, m.width-4)).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) viewDatePicker() string {
	cur, ok := parseDailyID(m.dateCur)
	if !ok {
		cur = time.Now()
	}
	today, todayOK := parseDailyID(m.todayID())
	var b strings.Builder
	label := "daily  "
	if m.datePurpose == "value" {
		label = "date  "
	}
	b.WriteString(styleMuted.Render(label) + cur.Format("Monday 2006-01-02") + "\n")
	b.WriteString(styleDim.Render(cur.Format("January 2006")) + "\n")
	b.WriteString(styleDim.Render("Su Mo Tu We Th Fr Sa") + "\n")
	first := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC)
	startPad := int(first.Weekday())
	days := daysInMonth(cur)
	col := 0
	for i := 0; i < startPad; i++ {
		b.WriteString("   ")
		col++
	}
	for d := 1; d <= days; d++ {
		day := time.Date(cur.Year(), cur.Month(), d, 0, 0, 0, 0, time.UTC)
		label := fmt.Sprintf("%2d", d)
		switch {
		case sameDay(day, cur):
			label = styleRowSel.Render(label)
		case todayOK && sameDay(day, today):
			label = styleOk.Render(label)
		}
		b.WriteString(label)
		col++
		if col == 7 {
			b.WriteByte('\n')
			col = 0
		} else {
			b.WriteByte(' ')
		}
	}
	if col != 0 {
		b.WriteByte('\n')
	}
	hint := "h/l day  j/k week  t today  enter"
	if m.datePurpose == "value" && m.mut.optional {
		hint += "  ctrl-d unset"
	}
	b.WriteString(styleDim.Render(hint))
	return styleModal.Width(min(36, m.width-4)).Render(strings.TrimRight(b.String(), "\n"))
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (m Model) viewSave() string {
	return styleModal.Width(min(48, m.width-4)).Render(
		styleMuted.Render("save view as  ") + m.saveName + "█",
	)
}

func (m Model) viewPrompt() string {
	return styleModal.Width(min(56, m.width-4)).Render(
		styleMuted.Render(m.mut.promptLabel+"  ") + m.mut.promptBuf + "█",
	)
}

func (m Model) viewConfirm() string {
	return styleModal.Width(min(64, m.width-4)).Render(m.mut.confirmText)
}

func (m Model) viewRQL() string {
	return styleModal.Width(min(64, m.width-4)).Render(
		styleMuted.Render("query  ") + m.rql + "█",
	)
}

func (m Model) viewArgs() string {
	name := ""
	if m.argsQuery != nil {
		name = m.argsQuery.Name
	}
	arg := ""
	if m.argsIdx >= 0 && m.argsIdx < len(m.argsNames) {
		arg = m.argsNames[m.argsIdx]
	}
	var b strings.Builder
	b.WriteString(styleMuted.Render("saved query  ") + name + "\n")
	for i, n := range m.argsNames {
		val := m.argsVals[n]
		if i == m.argsIdx {
			val = m.argsBuf + "█"
		}
		b.WriteString(n + ": " + val + "\n")
	}
	if arg != "" {
		b.WriteString(styleDim.Render(fmt.Sprintf("%d/%d  %s", m.argsIdx+1, len(m.argsNames), arg)))
	}
	return styleModal.Width(min(64, m.width-4)).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) viewPreview(width, height int) string {
	title := "preview"
	fg := colBorder
	if m.focus == focusPreview {
		title = "▸ preview"
		fg = colAccent
	}
	box := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder()).
		BorderForeground(fg).
		PaddingLeft(1)
	var body string
	switch {
	case !m.previewOn:
		body = ""
	case m.previewLoading && m.preview == nil:
		body = styleMuted.Render("loading…")
	case m.preview == nil:
		body = styleDim.Render("no selection")
	default:
		innerW := max(1, width-3)
		used := 1 // pane title
		if sel := m.p().selectedObject(); sel != nil && !sel.IsFileObject() {
			if caption := strings.TrimSpace(m.displayTitle(*sel)); caption != "" {
				body = styleHeader.Render(truncate(caption, innerW)) + "\n"
				used++
			}
		}
		head := m.headingFromDoc(m.preview.ObjectID, m.previewDoc)
		if head == "" {
			head = m.preview.ObjectID
		}
		body += styleHeader.Render(truncate(head, innerW)) + "\n"
		used++
		loc := m.preview.Path
		if m.previewLine > 0 {
			if loc == "" {
				loc = m.preview.ObjectID
			}
			loc = fmt.Sprintf("%s:%d", loc, m.previewLine)
		}
		if loc != "" {
			st := styleDim
			if m.previewLine > 0 {
				st = styleHeader
			}
			body += st.Render(truncate(loc, innerW)) + "\n"
			used++
		}
		mark := -1
		if m.previewLine > 0 {
			mark = m.previewScroll
		}
		body += windowLinesMark(m.previewBody, m.previewScroll, max(1, height-used), innerW, mark)
	}
	return box.Render(styleSectionTitle.Render(title) + "\n" + body)
}

func overlayOn(base string, width, height int, modal string) string {
	mw := lipgloss.Width(modal)
	mh := lipgloss.Height(modal)
	x := max(0, (width-mw)/2)
	y := max(0, (height-mh)/3)
	if y+mh > height {
		y = max(0, height-mh)
	}
	return placeOverlay(base, modal, x, y)
}

func placeOverlay(base, modal string, x, y int) string {
	bases := strings.Split(base, "\n")
	mods := strings.Split(modal, "\n")
	for i, line := range mods {
		row := y + i
		if row < 0 || row >= len(bases) {
			continue
		}
		bases[row] = overlayLine(bases[row], line, x)
	}
	return strings.Join(bases, "\n")
}

func overlayLine(base, overlay string, x int) string {
	if x <= 0 {
		return overlay
	}
	// crude: pad then splice by display width isn't perfect with ANSI; good enough
	pad := x
	if pad > len(base) {
		base += strings.Repeat(" ", pad-len(base))
	}
	return base[:min(x, len(base))] + overlay
}

func (m Model) objectType(id string) string {
	if obj, ok := m.noteIndex[id]; ok && obj.Type != "" {
		return obj.Type
	}
	return m.p().typeName
}

func (m Model) schemaNameField(typeName string) string {
	if typeName != "" {
		if ts, ok := m.schema.Types[typeName]; ok {
			return ts.NameField
		}
	}
	if tn := m.p().typeName; tn != "" {
		return m.schema.Types[tn].NameField
	}
	return ""
}

func (m Model) headingFromDoc(id string, doc frontmatter.Doc) string {
	if nf := m.schemaNameField(m.objectType(id)); nf != "" {
		if name := doc.Fields[nf]; name != "" {
			return name
		}
	}
	if obj, ok := m.noteIndex[id]; ok && obj.Title != "" {
		return obj.Title
	}
	if id != "" {
		return id
	}
	return ""
}

func (m Model) displayTitle(obj api.Object) string {
	if !obj.IsTrait() || obj.HasTraitContent() {
		return obj.Title
	}
	if parent, ok := m.noteIndex[obj.OpenRef()]; ok {
		if t := strings.TrimSpace(parent.Title); t != "" && t != parent.ID {
			return t
		}
	}
	return obj.Title
}

func (m Model) labeled(obj api.Object) api.Object {
	if t := m.displayTitle(obj); t != obj.Title {
		obj.Title = t
	}
	return obj
}

func listColumns(objects []api.Object, fields []string, ts api.TypeSchema) ([]string, int) {
	if traitRows(objects) {
		var cols []string
		if len(fields) == 0 {
			cols = append([]string{}, api.TraitListColumns...)
		} else {
			cols = append([]string{}, fields...)
		}
		return cols, flexColumnIndex(cols)
	}
	if len(fields) == 0 {
		fields = defaultFields(ts)
	}
	return append([]string{"id"}, fields...), -1
}

func traitRows(objects []api.Object) bool {
	return len(objects) > 0 && objects[0].IsTrait()
}

func flexColumnIndex(cols []string) int {
	for i, c := range cols {
		switch c {
		case "content", "title":
			return i
		}
	}
	if len(cols) == 0 {
		return -1
	}
	return 0
}

func rowCells(obj api.Object, cols []string) []string {
	out := make([]string, len(cols))
	trait := obj.IsTrait()
	for i, c := range cols {
		if c == "" {
			continue
		}
		if trait {
			switch c {
			case "content", "title":
				out[i] = obj.Title
				continue
			case "source":
				out[i] = api.TraitSource(obj)
				continue
			}
		}
		if c == "id" {
			out[i] = obj.ID
			continue
		}
		if s := api.ScalarString(obj.Fields[c]); s != "" {
			out[i] = s
			continue
		}
		if i == 1 {
			out[i] = obj.Title
		}
	}
	return out
}

func formatRow(cells []string, widths []int) string {
	return formatRowFlex(cells, widths, -1)
}

func formatRowFlex(cells []string, widths []int, flex int) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		p := padTrunc(c, w)
		if flex >= 0 && i != flex {
			p = styleDim.Render(p)
		}
		parts[i] = p
	}
	return strings.Join(parts, "  ")
}

func columnWidths(objects []api.Object, cols []string, total, flex int) []int {
	n := len(cols)
	if n == 0 {
		return nil
	}
	w := make([]int, n)
	for i, c := range cols {
		w[i] = min(24, max(len(c), 4))
	}
	limit := min(len(objects), 40)
	for i := 0; i < limit; i++ {
		cells := rowCells(objects[i], cols)
		for j, cell := range cells {
			if j == flex {
				continue
			}
			if l := lipgloss.Width(cell); l > w[j] {
				w[j] = min(columnCap(cols[j]), l)
			}
		}
	}
	gaps := 2 * (n - 1)
	if flex >= 0 && flex < n {
		others := 0
		for i, x := range w {
			if i != flex {
				others += x
			}
		}
		remain := total - gaps - others
		const minFlex = 12
		if total > 0 && remain < minFlex {
			avail := max(0, total-gaps-minFlex)
			if others > 0 && avail < others {
				scale := float64(avail) / float64(others)
				for i := range w {
					if i == flex {
						continue
					}
					w[i] = max(4, int(float64(w[i])*scale))
				}
			}
			others = 0
			for i, x := range w {
				if i != flex {
					others += x
				}
			}
			remain = total - gaps - others
		}
		if total <= 0 {
			w[flex] = minFlex
		} else {
			w[flex] = max(4, remain)
		}
		return w
	}
	sum := 0
	for _, x := range w {
		sum += x
	}
	sum += gaps
	if sum > total && total > 0 {
		scale := float64(total-gaps) / float64(sum-gaps)
		for i := range w {
			w[i] = max(4, int(float64(w[i])*scale))
		}
	}
	return w
}

func columnCap(col string) int {
	switch col {
	case "trait_type", "value":
		return 14
	case "source":
		return 22
	default:
		return 36
	}
}

func padTrunc(s string, w int) string {
	s = truncate(s, w)
	pad := w - lipgloss.Width(s)
	if pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > w-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func windowLines(s string, offset, height, width int) string {
	return windowLinesMark(s, offset, height, width, -1)
}

func windowLinesMark(s string, offset, height, width, mark int) string {
	lines := strings.Split(s, "\n")
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines) {
		offset = max(0, len(lines)-1)
	}
	end := min(len(lines), offset+height)
	slice := lines[offset:end]
	for i := range slice {
		line := truncate(slice[i], width)
		if mark >= 0 && offset+i == mark {
			if !strings.HasPrefix(strings.TrimSpace(line), "▸") {
				line = truncate("▸ "+strings.TrimLeft(line, " "), width)
			}
			slice[i] = styleRowSel.Width(width).Render(line)
		} else {
			slice[i] = line
		}
	}
	for len(slice) < height {
		slice = append(slice, "")
	}
	return strings.Join(slice, "\n")
}
