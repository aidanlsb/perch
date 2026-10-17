package ui

import (
	"fmt"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "perch"
	}
	header := m.viewHeader()
	footer := m.viewFooter()
	bodyH := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))
	var body string
	if m.screen == kindNote && m.note != nil {
		body = m.viewNote(bodyH)
	} else {
		body = m.viewBrowse(bodyH)
	}
	frame := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	switch m.overlay {
	case overlayHelp:
		return overlayOn(frame, m.width, m.height, m.viewHelp())
	case overlayPicker:
		return overlayOn(frame, m.width, m.height, m.viewPicker())
	case overlayLeader:
		return overlayOn(frame, m.width, m.height, m.viewLeader())
	case overlayRQL:
		return overlayOn(frame, m.width, m.height, m.viewRQL())
	case overlayArgs:
		return overlayOn(frame, m.width, m.height, m.viewArgs())
	case overlaySave:
		return overlayOn(frame, m.width, m.height, m.viewSave())
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
		if m.focus != focusSidebar && m.focus != focusPreview && m.focus != "" {
			label += " · " + m.focus
		}
	}
	mid := styleMuted.Render(" · " + label)
	if p.spec.Kind != "" && m.screen != kindNote {
		name := m.currentLayoutName()
		mid += styleMuted.Render("  " + name)
		if p.spec.Kind == kindDate && p.granularity != "" {
			mid += styleDim.Render(" / " + p.granularity)
		}
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
		case focusSidebar:
			if !m.sidebarHidden {
				return m.viewSidebar(height)
			}
			return m.viewPaneBox(m.viewByID(m.mainFocusID()), m.width, height, true)
		case focusPreview:
			return m.viewPreview(m.width, height)
		default:
			return m.viewPaneBox(m.viewByID(m.zoomTarget()), m.width, height, true)
		}
	}
	rest := m.width
	var side string
	if !m.sidebarHidden {
		side = m.viewSidebar(height)
		rest = max(10, m.width-lipgloss.Width(side))
	}
	if m.previewOn {
		pw := max(previewMinW, rest*2/5)
		if pw > rest-16 {
			pw = max(previewMinW, rest/3)
		}
		mainW := max(12, rest-pw)
		main := m.viewTree(m.tree, mainW, height)
		prev := m.viewPreview(pw, height)
		if side == "" {
			return lipgloss.JoinHorizontal(lipgloss.Top, main, prev)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, side, main, prev)
	}
	main := m.viewTree(m.tree, rest, height)
	if side == "" {
		return main
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, main)
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

type sideRow struct {
	title bool
	nav   int
	text  string
}

func (m Model) sidebarRows() []sideRow {
	var rows []sideRow
	for i, item := range m.nav {
		text := item.Label
		title := item.Kind == navCat
		if title {
			mark := "▸ "
			if (item.Category == catBrowse && m.browseOpen) || (item.Category == catViews && m.viewsOpen) {
				mark = "▾ "
			}
			text = mark + item.Label
		}
		rows = append(rows, sideRow{title: title, nav: i, text: text})
	}
	return rows
}

// sidebarClip keeps navCur visible in a viewport of `height` lines, reserving
// a line each for ▲ / ▼ when more items exist above or below.
func sidebarClip(n, cur, height int) (start, end int, moreAbove, moreBelow bool) {
	if n <= 0 || height <= 0 {
		return 0, 0, false, false
	}
	if cur < 0 {
		cur = 0
	}
	if cur >= n {
		cur = n - 1
	}
	start = 0
	for pass := 0; pass < 6; pass++ {
		inner := height
		if start > 0 {
			inner--
		}
		if start+inner < n {
			inner--
		}
		if inner < 1 {
			inner = 1
		}
		if cur < start {
			start = cur
			continue
		}
		if cur >= start+inner {
			start = cur - inner + 1
			if start < 0 {
				start = 0
			}
			continue
		}
		end = start + inner
		if end > n {
			end = n
		}
		return start, end, start > 0, end < n
	}
	end = min(n, start+max(1, height))
	return start, end, start > 0, end < n
}

func (m Model) viewSidebar(height int) string {
	w := min(sidebarWidth, max(12, m.width/4))
	rows := m.sidebarRows()
	curRow := 0
	for i, r := range rows {
		if !r.title && r.nav == m.navCur {
			curRow = i
			break
		}
	}
	start, end, moreAbove, moreBelow := sidebarClip(len(rows), curRow, height)
	var b strings.Builder
	if moreAbove {
		b.WriteString(styleDim.Width(w).Render(" ▲"))
		b.WriteByte('\n')
	}
	sideFocused := m.focus == focusSidebar
	fg := colBorder
	if sideFocused {
		fg = colAccent
	}
	for i := start; i < end; i++ {
		r := rows[i]
		label := r.text
		if lipgloss.Width(label) > w-2 {
			label = truncate(label, w-2)
		}
		if r.title {
			st := styleSidebarTitle
			if r.nav == m.navCur {
				st = styleNavSel
			}
			b.WriteString(st.Width(w).Render(label))
			b.WriteByte('\n')
			continue
		}
		line := styleNav.Width(w).Render("  " + label)
		if r.nav == m.navCur {
			prefix := "▸ "
			if !sideFocused {
				prefix = "  "
			}
			line = styleNavSel.Width(w).Render(prefix + label)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if moreBelow {
		b.WriteString(styleDim.Width(w).Render(" ▼"))
		b.WriteByte('\n')
	}
	out := strings.TrimRight(b.String(), "\n")
	box := lipgloss.NewStyle().
		Width(w).
		Height(height).
		Border(lipgloss.NormalBorder()).
		BorderForeground(fg)
	return box.Render(out)
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
	fields := v.listFields
	if len(fields) == 0 {
		fields = defaultFields(m.typeSchemaFor(v))
	}
	cols := append([]string{"id"}, fields...)
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

	widths := columnWidths(v.filtered, cols, width)
	var b strings.Builder
	b.WriteString(styleColHead.Render(formatRow(cols, widths)))
	b.WriteByte('\n')
	rows := height - 1
	end := min(len(v.filtered), off+rows)
	for i := off; i < end; i++ {
		obj := v.filtered[i]
		cells := rowCells(obj, cols)
		line := formatRow(cells, widths)
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
			label := truncate(c.Items[j].Title, colW)
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
			label := truncate("  "+obj.Title, width)
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
	var b strings.Builder
	title := "backlinks"
	if m.pane == paneBacklinks {
		title = "▸ backlinks"
	}
	b.WriteString(styleSidebarTitle.Render(title))
	b.WriteByte('\n')
	if m.note == nil || len(m.note.Backlinks) == 0 {
		b.WriteString(styleDim.Render("none"))
		return style.Render(b.String())
	}
	for i, g := range m.note.Backlinks {
		label := truncate(g.Source, width-3)
		if i == m.backIdx {
			st := styleNavSel
			if m.pane != paneBacklinks {
				st = styleNav
			}
			b.WriteString(st.Width(width - 2).Render(label))
		} else {
			b.WriteString(styleNav.Render(label))
		}
		b.WriteByte('\n')
	}
	return style.Render(strings.TrimRight(b.String(), "\n"))
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
			b.WriteString("\n" + styleSidebarTitle.Render(section) + "\n")
		}
		keys := padTrunc(bind.Keys, 26)
		b.WriteString(styleMuted.Render(keys) + "  " + bind.Desc + "\n")
	}
	w := min(84, max(40, m.width-2))
	return styleHelpBox.Width(w).Padding(0, 2).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) viewPicker() string {
	title := "types"
	switch m.pickerKind {
	case pickerQueries:
		title = "saved queries"
	case pickerNotes:
		title = "notes"
	case pickerViews:
		title = "views"
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

func (m Model) viewLeader() string {
	text := strings.TrimSpace(`
t  types
q  saved queries
f  notes
d  daily
l  views
c  config
`)
	return styleModal.Width(min(28, m.width-4)).Render("space\n" + text)
}

func (m Model) viewSave() string {
	return styleModal.Width(min(48, m.width-4)).Render(
		styleMuted.Render("save view as  ") + m.saveName + "█",
	)
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
		head := m.headingFromDoc(m.preview.ObjectID, m.previewDoc)
		if head == "" {
			head = m.preview.ObjectID
		}
		body = styleHeader.Render(truncate(head, width-3)) + "\n"
		if m.preview.Path != "" {
			body += styleDim.Render(truncate(m.preview.Path, width-3)) + "\n"
		}
		body += windowLines(m.previewBody, m.previewScroll, max(1, height-3), width-3)
	}
	return box.Render(styleSidebarTitle.Render(title) + "\n" + body)
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

func rowCells(obj api.Object, cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		if c == "id" {
			out[i] = obj.ID
			continue
		}
		if c == "" {
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
	parts := make([]string, len(cells))
	for i, c := range cells {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		parts[i] = padTrunc(c, w)
	}
	return strings.Join(parts, "  ")
}

func columnWidths(objects []api.Object, cols []string, total int) []int {
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
			if l := lipgloss.Width(cell); l > w[j] {
				w[j] = min(36, l)
			}
		}
	}
	sum := 0
	for _, x := range w {
		sum += x
	}
	sum += 2 * (n - 1)
	if sum > total && total > 0 {
		scale := float64(total-2*(n-1)) / float64(sum-2*(n-1))
		for i := range w {
			w[i] = max(4, int(float64(w[i])*scale))
		}
	}
	return w
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
		slice[i] = truncate(slice[i], width)
	}
	for len(slice) < height {
		slice = append(slice, "")
	}
	return strings.Join(slice, "\n")
}
