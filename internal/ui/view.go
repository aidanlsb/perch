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
	brand := styleHeader.Render(brandMark)
	right := m.vantageStatus()
	rightW := lipgloss.Width(right)
	trailBudget := max(8, m.width-lipgloss.Width(brand)-rightW-4)
	trail := styleMuted.Render(formatBreadcrumb(m.breadcrumbs(), trailBudget))
	left := brand + "  " + trail
	gap := max(1, m.width-lipgloss.Width(left)-rightW)
	line := left + strings.Repeat(" ", gap) + right
	if lipgloss.Width(line) < m.width {
		line += strings.Repeat(" ", max(0, m.width-lipgloss.Width(line)))
	}
	return truncate(line, m.width)
}

func (m Model) vantageStatus() string {
	var parts []string
	if m.overlay == overlaySearch || m.searchQ != "" {
		s := "/" + m.searchQ
		if m.overlay == overlaySearch {
			s += "█"
		}
		if n := len(m.searchHits); n > 0 && m.searchQ != "" {
			s += fmt.Sprintf("  %d/%d", m.searchAt+1, n)
		}
		parts = append(parts, styleHeader.Render(s))
	}
	if m.loading {
		parts = append(parts, styleMuted.Render("loading…"))
	}
	if live := m.liveBadge(); live != "" {
		parts = append(parts, live)
	}
	if m.status != "" && !m.loading && m.overlay != overlaySearch && m.searchQ == "" {
		parts = append(parts, styleStatus.Render(truncate(m.status, max(8, m.width/4))))
	}
	return strings.Join(parts, "  ")
}

func (m Model) liveBadge() string {
	if m.pollInterval <= 0 {
		return ""
	}
	if m.liveErr != "" {
		return styleWarn.Render(liveMark + " " + m.liveErr)
	}
	if m.revGeneration != "" {
		return styleOk.Render(liveMark + " live")
	}
	return styleDim.Render(liveMark)
}

func (m Model) viewFooter() string {
	if m.errMsg != "" {
		return styleErr.Render(truncate(m.errMsg, m.width))
	}
	if len(m.cfgWarnings) > 0 {
		return styleWarn.Render(truncate(strings.Join(m.cfgWarnings, "; "), m.width))
	}
	return styleHintLine(footerKeys(m), m.width)
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
	innerW := max(1, width-2)
	innerH := max(1, height-2)
	return frameBox(m.viewPane(v, innerW, innerH, focused), width, height, m.paneHeader(v, focused), focused, m.paneScroll(v, innerH))
}

func (m Model) viewPane(v *viewState, width, height int, focused bool) string {
	if v == nil {
		return styleMuted.Render("empty")
	}
	if v.loading && len(v.objects) == 0 {
		return styleMuted.Render("loading…")
	}
	switch v.spec.Kind {
	case kindBoard:
		return m.viewBoard(v, width, height, focused)
	case kindDate:
		return m.viewDate(v, width, height, focused)
	default:
		return m.viewList(v, width, height, focused)
	}
}

func (m Model) paneHeader(v *viewState, focused bool) string {
	if v == nil {
		return "empty"
	}
	name := v.sourceLabel
	if v.viewName != "" {
		name = v.viewName
	}
	if name == "" {
		name = v.id
	}
	if focused {
		name = "▸ " + name
	}
	var parts []string
	parts = append(parts, name)
	if v.bind != "" {
		parts = append(parts, m.bindingSightline(v))
	}
	kind := v.spec.Kind
	if kind == "" {
		kind = kindList
	}
	badge := kind
	if kind == kindDate && v.granularity != "" {
		badge += " / " + v.granularity
	}
	if strings.TrimSpace(v.spec.Filter) != "" || (m.searchQ != "" && focused) {
		badge += " *"
	}
	parts = append(parts, badge)
	parts = append(parts, m.paneCount(v))
	return strings.Join(parts, " · ")
}

func (m Model) bindingSightline(v *viewState) string {
	if v == nil || v.bind == "" {
		return ""
	}
	d := m.viewByID(v.bind)
	if d == nil {
		return sightArrow + " " + v.bind
	}
	if sel := d.selectedObject(); sel != nil {
		title := strings.TrimSpace(m.displayTitle(*sel))
		if title == "" {
			title = sel.ID
		}
		if title != "" {
			return sightArrow + " " + title
		}
	}
	label := d.sourceLabel
	if d.viewName != "" {
		label = d.viewName
	}
	if label == "" {
		label = d.id
	}
	return sightArrow + " " + label
}

func (m Model) paneCount(v *viewState) string {
	if v == nil {
		return "0"
	}
	switch v.spec.Kind {
	case kindBoard:
		n := 0
		for _, c := range v.board {
			n += len(c.Items)
		}
		return fmt.Sprintf("%d", n)
	case kindDate:
		n := 0
		for _, c := range v.dateCols {
			n += len(c.Items)
		}
		if n == 0 {
			return "0"
		}
		cur := 0
		for i := 0; i < v.dateSec && i < len(v.dateCols); i++ {
			cur += len(v.dateCols[i].Items)
		}
		cur += v.dateRow + 1
		return fmt.Sprintf("%d/%d", clamp(cur, 1, n), n)
	default:
		n := len(v.filtered)
		if n == 0 {
			return "0"
		}
		return fmt.Sprintf("%d/%d", v.listCur+1, n)
	}
}

func (m Model) paneScroll(v *viewState, innerH int) scrollState {
	if v == nil || innerH < 1 {
		return scrollState{}
	}
	switch v.spec.Kind {
	case kindBoard:
		return scrollState{}
	case kindDate:
		total := 0
		cur := 0
		for si, sec := range v.dateCols {
			if si == v.dateSec {
				cur = total + 1 + v.dateRow
			}
			total += 1 + len(sec.Items)
		}
		off := 0
		if cur >= innerH {
			off = cur - innerH + 1
		}
		return scrollState{offset: off, total: total, view: innerH}
	default:
		n := len(v.filtered) + 1
		off := v.listOff
		if v.listCur < off {
			off = v.listCur
		}
		if v.listCur >= off+innerH-1 {
			off = v.listCur - (innerH - 2)
		}
		if off < 0 {
			off = 0
		}
		return scrollState{offset: off, total: n, view: innerH}
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

	innerW := max(1, width-2)
	widths := columnWidths(v.filtered, cols, innerW, flex)
	heads := make([]string, len(cols))
	for i, c := range cols {
		heads[i] = columnHeading(c)
	}
	var b strings.Builder
	b.WriteString(styleColHead.Render(formatRow(heads, widths)))
	b.WriteByte('\n')
	rows := height - 1
	end := min(len(v.filtered), off+rows)
	for i := off; i < end; i++ {
		obj := m.labeled(v.filtered[i])
		cells := rowCells(obj, cols)
		sel := i == v.listCur
		line := rowCursor(sel, focused) + formatRowFlex(cells, widths, flex)
		st := selectStyle(sel, focused)
		if sel {
			b.WriteString(st.Width(width).Render(truncate(line, width)))
		} else {
			b.WriteString(st.Render(truncate(line, width)))
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
	gutter := 1
	maxCols := max(1, (width+gutter)/(12+gutter))
	start := 0
	if v.boardCol >= maxCols {
		start = v.boardCol - maxCols + 1
	}
	end := min(n, start+maxCols)
	shown := end - start
	gutterW := gutter * max(0, shown-1)
	colW := max(10, (width-gutterW)/max(1, shown))

	var cols []string
	for i := start; i < end; i++ {
		c := v.board[i]
		active := i == v.boardCol
		headTxt := truncate(fmt.Sprintf("%s  %d", c.Label, len(c.Items)), colW)
		var head string
		if active {
			head = styleBoardHead.Width(colW).Render(headTxt)
		} else {
			head = styleBoardHeadIdle.Width(colW).Render(headTxt)
		}
		var rows []string
		rows = append(rows, head)
		ruleN := min(colW, max(4, lipgloss.Width(c.Label)+2))
		rows = append(rows, styleDim.Render(strings.Repeat("─", ruleN)))
		visible := height - 2
		off := 0
		if active && v.boardRow >= visible {
			off = v.boardRow - visible + 1
		}
		cardW := max(1, colW-1)
		showMeta := colW >= 16 && len(v.listFields) > 0
		for j := off; j < len(c.Items) && j-off < visible; j++ {
			sel := active && j == v.boardRow
			mark := " "
			if sel {
				mark = "▌"
			}
			title := truncate(m.displayTitle(c.Items[j]), cardW)
			line := mark + title
			st := selectStyle(sel, focused)
			rows = append(rows, st.Width(colW).Render(truncate(line, colW)))
			if showMeta && j-off+1 < visible {
				if meta := cardMeta(c.Items[j], v.listFields, v.groupBy, m.displayTitle(c.Items[j])); meta != "" {
					rows = append(rows, styleDim.Width(colW).Render(truncate(" "+meta, colW)))
				}
			}
		}
		if len(c.Items) == 0 {
			rows = append(rows, styleDim.Render("empty"))
		}
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, rows...))
		if i < end-1 {
			div := styleDim.Render("│")
			cols = append(cols, lipgloss.NewStyle().Height(height).Render(div))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func cardMeta(obj api.Object, fields []string, skip, title string) string {
	var parts []string
	for _, f := range fields {
		if f == "" || f == skip || f == colLabel {
			continue
		}
		s := api.ScalarString(obj.Fields[f])
		if s == "" || s == title {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

func (m Model) viewDate(v *viewState, width, height int, focused bool) string {
	if len(v.dateCols) == 0 {
		return styleMuted.Render("no dates")
	}
	var lines []string
	for si, sec := range v.dateCols {
		head := fmt.Sprintf("── %s · %d ", sec.Label, len(sec.Items))
		if w := lipgloss.Width(head); w < width {
			head += strings.Repeat("─", width-w)
		}
		lines = append(lines, styleDateHead.Width(width).Render(truncate(head, width)))
		for ri, obj := range sec.Items {
			sel := si == v.dateSec && ri == v.dateRow
			label := rowCursor(sel, focused) + m.displayTitle(obj)
			st := selectStyle(sel, focused)
			if sel {
				lines = append(lines, st.Width(width).Render(truncate(label, width)))
			} else {
				lines = append(lines, st.Render(truncate(label, width)))
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
	nLines := strings.Count(m.noteBody, "\n") + 1
	body = withRightThumb(body, mainW, bodyH, m.noteScroll, nLines)
	main := lipgloss.JoinVertical(lipgloss.Left, header, body, linkBar)
	if backW == 0 {
		return main
	}
	back := m.viewBacklinks(backW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, main, back)
}

func (m Model) viewNoteHeader(width int) string {
	id := m.note.ObjectID
	heading := m.headingFromDoc(id, m.noteDoc)
	typ := m.objectType(id)
	path := m.note.Path
	if path == "" {
		path = id
	}
	title := heading
	if title == "" {
		title = id
	}
	var topParts []string
	topParts = append(topParts, styleHeader.Render(title))
	if typ != "" {
		topParts = append(topParts, styleDim.Render("["+typ+"]"))
	}
	if path != "" && path != title {
		topParts = append(topParts, styleDim.Render(path))
	} else if id != "" && id != title {
		topParts = append(topParts, styleDim.Render(id))
	}
	top := wrapJoined(topParts, "  ", width)
	skip := m.schemaNameField(typ)
	var pills []string
	for _, k := range m.noteDoc.Order {
		if skip != "" && k == skip {
			continue
		}
		val := m.noteDoc.Fields[k]
		if strings.TrimSpace(val) == "" {
			continue
		}
		pills = append(pills, styleFMKey.Render(k+": ")+styleFMVal.Render(val))
	}
	meta := wrapJoined(pills, styleDim.Render("  ·  "), width)
	if meta == "" {
		return top
	}
	return top + "\n" + meta
}

func (m Model) viewLinkBar(width int) string {
	if m.note == nil || len(m.note.Links) == 0 {
		return styleDim.Render("links 0")
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
	label := fmt.Sprintf("links %d/%d  %s  ● %s", m.linkIdx+1, len(m.note.Links), text, link.Status)
	return st.Render(truncate(label, width))
}

func (m Model) viewBacklinks(width, height int) string {
	focused := m.pane == paneBacklinks
	n := 0
	if m.note != nil {
		n = len(m.note.Backlinks)
	}
	title := "backlinks · " + fmt.Sprintf("%d", n)
	if focused {
		title = "▸ " + title
	}
	previewH := 0
	if m.note != nil && n > 0 && (focused || height >= 14) {
		previewH = height / 2
		if previewH < 6 {
			previewH = min(8, height/3)
		}
		if previewH > height-6 {
			previewH = max(0, height-6)
		}
	}
	innerW := max(1, width-2)
	innerH := max(1, height-2)
	listH := innerH
	if previewH > 0 {
		listH = max(1, innerH-previewH)
	}
	list := m.viewBacklinkList(innerW, listH)
	body := list
	if previewH > 0 {
		prev := m.viewBacklinkPreview(innerW, previewH)
		body = lipgloss.JoinVertical(lipgloss.Left, list, prev)
	}
	return frameBox(body, width, height, title, focused, scrollState{})
}

func (m Model) viewBacklinkList(width, height int) string {
	if m.note == nil || len(m.note.Backlinks) == 0 {
		return styleDim.Render("none")
	}
	var b strings.Builder
	inner := max(8, width)
	focused := m.pane == paneBacklinks
	for i, g := range m.note.Backlinks {
		label := m.backlinkLabel(g)
		sel := i == m.backIdx
		line := rowCursor(sel, focused) + truncate(label, max(1, inner-2))
		st := selectStyle(sel, focused)
		if sel {
			b.WriteString(st.Width(inner).Render(truncate(line, inner)))
		} else {
			b.WriteString(st.Render(truncate(line, inner)))
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
	mark := -1
	if line > 0 {
		mark = off
	}
	b.WriteString(windowLinesMark(body, off, max(1, height-used), max(8, width-1), mark, line))
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
	section := ""
	for _, bind := range Keymap {
		if bind.Section != section {
			section = bind.Section
			b.WriteString("\n" + styleSectionTitle.Render(section) + "\n")
		}
		keys := padTrunc(bind.Keys, 26)
		b.WriteString(styleKey.Render(keys) + "  " + styleDim.Render(bind.Desc) + "\n")
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	h := max(8, m.height-4)
	off := m.helpOff
	if off > len(lines)-h {
		off = max(0, len(lines)-h)
	}
	if off < 0 {
		off = 0
	}
	end := min(len(lines), off+h)
	body := strings.Join(lines[off:end], "\n")
	w := min(84, max(40, m.width-4))
	focus := m.focus
	switch m.screen {
	case kindNote:
		focus = "note"
	case kindHome:
		focus = "home"
	}
	title := "keys · " + focus
	return modalFrame(title, body, w)
}

func (m Model) pickerTitle() string {
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
	nHit := len(m.pickerHit)
	nAll := len(m.pickerAll)
	if nAll > 0 {
		title += fmt.Sprintf("  %d/%d", nHit, nAll)
	} else if nHit > 0 {
		title += fmt.Sprintf("  %d", nHit)
	}
	return title
}

func (m Model) viewPicker() string {
	rowW := min(64, max(36, m.width-8))
	var b strings.Builder
	b.WriteString(styleHeader.Render("▸ ") + m.pickerQ + "█\n")
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
		sel := i == m.pickerCur
		line := pickerRowLine(it, rowW, sel)
		if sel {
			b.WriteString(styleNavSel.Width(rowW).Render(line))
		} else {
			b.WriteString(styleNav.Width(rowW).Render(line))
		}
		b.WriteByte('\n')
	}
	return modalFrame(m.pickerTitle(), strings.TrimRight(b.String(), "\n"), min(68, m.width-4))
}

func pickerRowLine(it pickItem, width int, selected bool) string {
	label := it.Label
	if selected {
		label = "▸ " + label
	} else {
		label = "  " + label
	}
	badge := it.Sub
	if badge == "" || badge == it.Label {
		return truncate(label, width)
	}
	bw := lipgloss.Width(badge)
	avail := width - bw - 1
	if avail < 8 {
		return truncate(label+"  "+badge, width)
	}
	label = truncate(label, avail)
	gap := max(1, avail-lipgloss.Width(label))
	return label + strings.Repeat(" ", gap) + styleDim.Render(badge)
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
	b.WriteString(styleHeader.Render("▸ ") + cur.Format("Monday 2006-01-02") + "\n")
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
	title := strings.TrimSpace(label)
	if title == "" {
		title = "date"
	}
	return modalFrame(title, strings.TrimRight(b.String(), "\n"), min(36, m.width-4))
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
	body := styleHeader.Render("▸ ") + m.saveName + "█"
	return modalFrame("save view as", body, min(48, m.width-4))
}

func (m Model) viewPrompt() string {
	label := m.mut.promptLabel
	if label == "" {
		label = "value"
	}
	body := styleHeader.Render("▸ ") + m.mut.promptBuf + "█"
	return modalFrame(label, body, min(56, m.width-4))
}

func (m Model) viewConfirm() string {
	return modalFrame("confirm", m.mut.confirmText, min(64, m.width-4))
}

func (m Model) viewRQL() string {
	body := styleHeader.Render("▸ ") + m.rql + "█"
	return modalFrame("query", body, min(64, m.width-4))
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
	for i, n := range m.argsNames {
		val := m.argsVals[n]
		cur := i == m.argsIdx
		if cur {
			val = m.argsBuf + "█"
		}
		line := n + ": " + val
		if cur {
			line = "▸ " + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	if arg != "" {
		b.WriteString(styleDim.Render(fmt.Sprintf("%d/%d  %s", m.argsIdx+1, len(m.argsNames), arg)))
	}
	title := "args"
	if name != "" {
		title = "args · " + name
	}
	return modalFrame(title, strings.TrimRight(b.String(), "\n"), min(64, m.width-4))
}

func (m Model) viewPreview(width, height int) string {
	focused := m.focus == focusPreview
	title := "preview"
	if focused {
		title = "▸ preview"
	}
	innerW := max(1, width-2)
	innerH := max(1, height-2)
	var body string
	scroll := scrollState{view: innerH}
	switch {
	case !m.previewOn:
		body = ""
	case m.previewLoading && m.preview == nil:
		body = styleMuted.Render("loading…")
	case m.preview == nil:
		body = styleDim.Render("no selection")
	default:
		used := 0
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
			mark = sourceLineScroll(m.preview.Content, m.previewBody, m.previewLine)
		}
		winH := max(1, innerH-used)
		body += windowLinesMark(m.previewBody, m.previewScroll, winH, innerW, mark, m.previewLine)
		nLines := strings.Count(m.previewBody, "\n") + 1
		scroll = scrollState{offset: m.previewScroll, total: nLines, view: winH}
	}
	return frameBox(body, width, height, title, focused, scroll)
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

// colLabel is a synthetic list column for obj.Title. It is not a vault field.
const colLabel = "\x00"

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
	if !objectRowsNeedLabel(objects) {
		return append([]string{"id"}, fields...), -1
	}
	rest := make([]string, 0, 1+len(fields))
	rest = append(rest, "id")
	skip := ts.NameField
	for _, f := range fields {
		if skip != "" && f == skip {
			continue
		}
		rest = append(rest, f)
	}
	return append([]string{colLabel}, rest...), 0
}

func objectRowsNeedLabel(objects []api.Object) bool {
	n := min(len(objects), 24)
	for i := 0; i < n; i++ {
		t := strings.TrimSpace(objects[i].Title)
		if t != "" && t != objects[i].ID {
			return true
		}
	}
	return false
}

func columnHeading(c string) string {
	if c == colLabel {
		return ""
	}
	return c
}

func traitRows(objects []api.Object) bool {
	return len(objects) > 0 && objects[0].IsTrait()
}

func flexColumnIndex(cols []string) int {
	for i, c := range cols {
		switch c {
		case colLabel, "content", "title":
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
		if c == colLabel {
			if obj.Title != "" {
				out[i] = obj.Title
			} else {
				out[i] = obj.ID
			}
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
	return windowLinesMark(s, offset, height, width, -1, 0)
}

func windowLinesMark(s string, offset, height, width, mark, srcLine int) string {
	lines := strings.Split(s, "\n")
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines) {
		offset = max(0, len(lines)-1)
	}
	end := min(len(lines), offset+height)
	slice := lines[offset:end]
	gutter := 0
	if srcLine > 0 && mark >= 0 {
		maxNum := srcLine + (offset + height - 1 - mark)
		if maxNum < srcLine {
			maxNum = srcLine
		}
		gutter = len(fmt.Sprintf("%d", max(1, maxNum))) + 1
		if gutter < 3 {
			gutter = 3
		}
	}
	innerW := width
	if gutter > 0 {
		innerW = max(1, width-gutter-1)
	}
	for i := range slice {
		body := truncate(slice[i], innerW)
		hit := mark >= 0 && offset+i == mark
		if gutter > 0 {
			n := srcLine + (offset + i - mark)
			if n < 1 {
				n = 1
			}
			num := styleDim.Render(fmt.Sprintf("%*d", gutter-1, n))
			if hit {
				slice[i] = num + styleHeader.Render("▸") + styleRowSel.Width(innerW).Render(body)
			} else {
				slice[i] = num + " " + body
			}
			continue
		}
		if hit {
			if !strings.HasPrefix(strings.TrimSpace(body), "▸") {
				body = truncate("▸ "+strings.TrimLeft(body, " "), width)
			}
			slice[i] = styleRowSel.Width(width).Render(body)
		} else {
			slice[i] = body
		}
	}
	for len(slice) < height {
		slice = append(slice, "")
	}
	return strings.Join(slice, "\n")
}
