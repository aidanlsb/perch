package ui

import (
	"fmt"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
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
	case overlaySwitcher:
		return overlayOn(frame, m.width, m.height, m.viewSwitcher())
	case overlayFilter:
		return frame
	}
	return frame
}

func (m Model) viewHeader() string {
	left := styleHeader.Render("perch")
	mid := styleMuted.Render(" · " + m.sourceLabel)
	if m.viewKind != "" && m.screen != kindNote {
		mid += styleDim.Render("  " + m.viewKind)
		if m.groupBy != "" {
			mid += styleDim.Render(" by " + m.groupBy)
		}
		if m.viewKind == kindDate {
			mid += styleDim.Render(" / " + m.granularity)
		}
	}
	if m.screen == kindNote && m.note != nil {
		mid = styleMuted.Render(" · " + m.note.ObjectID)
	}
	right := m.status
	if m.loading {
		right = "loading…"
	}
	if m.overlay == overlayFilter {
		right = "/" + m.filter
	}
	gap := max(0, m.width-lipgloss.Width(left+mid)-lipgloss.Width(right)-1)
	line := left + mid + strings.Repeat(" ", gap) + styleStatus.Render(right)
	if lipgloss.Width(line) < m.width {
		line += strings.Repeat(" ", max(0, m.width-lipgloss.Width(line)))
	}
	return line
}

func (m Model) viewFooter() string {
	if m.errMsg != "" {
		return styleErr.Render(truncate(m.errMsg, m.width))
	}
	var keys string
	switch {
	case m.overlay == overlayFilter:
		keys = "filter  enter done  esc clear"
	case m.overlay == overlaySwitcher:
		keys = "search  j/k  enter open  esc"
	case m.screen == kindNote:
		keys = "tab link  enter follow  [ ] history  e edit  h/l backlinks  q back"
	default:
		keys = "hjkl  enter open  / filter  s search  e edit  ? help  q quit"
	}
	return styleFooter.Render(truncate(keys, m.width))
}

func (m Model) viewBrowse(height int) string {
	side := m.viewSidebar(height)
	mainW := max(10, m.width-lipgloss.Width(side))
	main := m.viewMain(mainW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, side, main)
}

func (m Model) viewSidebar(height int) string {
	w := min(sidebarWidth, max(12, m.width/4))
	var b strings.Builder
	lastKind := navKind(-1)
	titles := map[navKind]string{
		navType:  "types",
		navQuery: "queries",
		navView:  "views",
	}
	visible := 0
	start := 0
	if m.navCur < start {
		start = m.navCur
	}
	inner := height
	for inner > 0 && start > 0 && m.navCur-start+3 > inner {
		start++
	}
	mNav := m.nav
	for i := 0; i < len(mNav); i++ {
		item := mNav[i]
		if item.Kind != lastKind {
			if visible >= inner {
				break
			}
			if i >= start {
				b.WriteString(styleSidebarTitle.Width(w).Render(titles[item.Kind]))
				b.WriteByte('\n')
				visible++
			}
			lastKind = item.Kind
		}
		if i < start {
			continue
		}
		if visible >= inner {
			break
		}
		label := item.Label
		if lipgloss.Width(label) > w-2 {
			label = truncate(label, w-2)
		}
		line := styleNav.Width(w).Render("  " + label)
		if i == m.navCur {
			prefix := "▸ "
			if m.pane != paneSidebar {
				prefix = "  "
			}
			line = styleNavSel.Width(w).Render(prefix + label)
		}
		b.WriteString(line)
		b.WriteByte('\n')
		visible++
	}
	out := strings.TrimRight(b.String(), "\n")
	box := lipgloss.NewStyle().
		Width(w).
		Height(height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(colBorder)
	return box.Render(out)
}

func (m Model) viewMain(width, height int) string {
	pad := lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1)
	if m.loading && len(m.objects) == 0 {
		return pad.Render(styleMuted.Render("loading…"))
	}
	switch m.viewKind {
	case kindBoard:
		return pad.Render(m.viewBoard(width-1, height))
	case kindDate:
		return pad.Render(m.viewDate(width-1, height))
	default:
		return pad.Render(m.viewList(width-1, height))
	}
}

func (m Model) viewList(width, height int) string {
	if len(m.filtered) == 0 {
		if m.filter != "" {
			return styleMuted.Render("no matches for /" + m.filter)
		}
		return styleMuted.Render("no objects")
	}
	fields := m.listFields
	if len(fields) == 0 {
		fields = defaultFields(m.typeSchema())
	}
	cols := append([]string{"id"}, fields...)
	// keep cursor in view
	off := m.listOff
	if m.listCur < off {
		off = m.listCur
	}
	if m.listCur >= off+height-1 {
		off = m.listCur - (height - 2)
	}
	if off < 0 {
		off = 0
	}

	widths := columnWidths(m.filtered, cols, width)
	var b strings.Builder
	b.WriteString(styleColHead.Render(formatRow(cols, widths)))
	b.WriteByte('\n')
	rows := height - 1
	end := min(len(m.filtered), off+rows)
	for i := off; i < end; i++ {
		obj := m.filtered[i]
		cells := rowCells(obj, cols)
		line := formatRow(cells, widths)
		if i == m.listCur {
			st := styleRowSel
			if m.pane != paneMain {
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

func (m Model) viewBoard(width, height int) string {
	if len(m.board) == 0 {
		return styleMuted.Render("no columns")
	}
	n := len(m.board)
	colW := max(12, (width-n+1)/max(1, n))
	if colW*n > width {
		colW = max(12, width/max(1, min(n, 4)))
	}
	// show a window of columns around the cursor
	maxCols := max(1, width/max(colW, 12))
	start := 0
	if m.boardCol >= maxCols {
		start = m.boardCol - maxCols + 1
	}
	end := min(n, start+maxCols)
	colW = max(12, (width-(end-start)+1)/max(1, end-start))

	var cols []string
	for i := start; i < end; i++ {
		c := m.board[i]
		head := styleBoardHead.Width(colW).Render(truncate(fmt.Sprintf("%s (%d)", c.Label, len(c.Items)), colW))
		var rows []string
		rows = append(rows, head)
		rows = append(rows, styleDim.Render(strings.Repeat("─", min(colW, 8))))
		visible := height - 2
		off := 0
		if i == m.boardCol && m.boardRow >= visible {
			off = m.boardRow - visible + 1
		}
		for j := off; j < len(c.Items) && j-off < visible; j++ {
			label := truncate(c.Items[j].Title, colW)
			if i == m.boardCol && j == m.boardRow {
				st := styleRowSel
				if m.pane != paneMain {
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

func (m Model) viewDate(width, height int) string {
	if len(m.dateCols) == 0 {
		return styleMuted.Render("no dates")
	}
	var lines []string
	for si, sec := range m.dateCols {
		lines = append(lines, styleDateHead.Width(width).Render(fmt.Sprintf("%s  %d", sec.Label, len(sec.Items))))
		for ri, obj := range sec.Items {
			label := truncate("  "+obj.Title, width)
			if si == m.dateSec && ri == m.dateRow {
				st := styleRowSel
				if m.pane != paneMain {
					st = styleRow
				}
				lines = append(lines, st.Width(width).Render(label))
			} else {
				lines = append(lines, styleRow.Render(label))
			}
		}
	}
	// window
	total := len(lines)
	curLine := 0
	n := 0
	for si, sec := range m.dateCols {
		n++ // header
		if si == m.dateSec {
			curLine = n + m.dateRow
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
	title := m.note.ObjectID
	if name := m.noteDoc.Fields["name"]; name != "" {
		title = name + "  " + styleDim.Render(m.note.ObjectID)
	}
	lines = append(lines, styleHeader.Render(truncate(title, width)))
	if m.note.Path != "" {
		lines = append(lines, styleDim.Render(truncate(m.note.Path, width)))
	}
	for _, k := range m.noteDoc.Order {
		if k == "name" || k == "id" {
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
	text := strings.TrimSpace(`
perch  read-only companion for Raven

navigation
  hjkl / arrows   move
  tab             sidebar ↔ list, or cycle wiki links
  enter           open note / follow link
  [ ]             back / forward
  /               filter current view
  s               search (Raven)
  e o             open in $VISUAL / $EDITOR
  r               refresh
  d w m           date grouping (day/week/month)
  q               back / quit

views come from the vault schema. board and date layouts
are saved in perch's config, not in Raven.
`)
	return styleHelpBox.Width(min(60, m.width-4)).Render(text)
}

func (m Model) viewSwitcher() string {
	var b strings.Builder
	b.WriteString(styleMuted.Render("search  ") + m.switcherQ + "█\n")
	if len(m.switcherHit) == 0 {
		b.WriteString(styleDim.Render("type two characters…"))
	}
	for i, hit := range m.switcherHit {
		line := hit.Title
		if line == "" {
			line = hit.ObjectID
		} else {
			line += "  " + styleDim.Render(hit.ObjectID)
		}
		if i == m.switcherCur {
			b.WriteString(styleNavSel.Render(truncate(line, 56)))
		} else {
			b.WriteString(styleNav.Render(truncate(line, 56)))
		}
		b.WriteByte('\n')
	}
	return styleModal.Width(min(64, m.width-4)).Render(strings.TrimRight(b.String(), "\n"))
}

func overlayOn(base string, width, height int, modal string) string {
	mw := lipgloss.Width(modal)
	mh := lipgloss.Height(modal)
	x := max(0, (width-mw)/2)
	y := max(0, (height-mh)/3)
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

func rowCells(obj api.Object, cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		switch c {
		case "id":
			out[i] = obj.ID
		case "title", "name":
			out[i] = obj.Title
		default:
			if c == "" {
				continue
			}
			if s := api.ScalarString(obj.Fields[c]); s != "" {
				out[i] = s
			} else if c == obj.Type {
				out[i] = obj.Title
			}
			if out[i] == "" && c != "id" {
				// prefer title in first data column
				if i == 1 {
					out[i] = obj.Title
				}
			}
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
