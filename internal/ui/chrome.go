package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

const (
	crumbSep   = " › "
	sightArrow = "⟵"
	liveMark   = "◉"
	brandMark  = "◈ perch"
	thumbGlyph = "█"
	trackGlyph = "│"
)

type scrollState struct {
	offset int
	total  int
	view   int
}

func (s scrollState) active() bool {
	return s.view > 0 && s.total > s.view
}

func frameBox(body string, width, height int, title string, focused bool, scroll scrollState) string {
	if width < 2 || height < 1 {
		return truncate(body, max(0, width))
	}
	fg := colBorder
	if focused {
		fg = colAccent
	}
	edge := lipgloss.NewStyle().Foreground(fg)
	top := borderTitleLine(title, width, focused)
	if height == 1 {
		return top
	}
	innerW := max(1, width-2)
	bot := edge.Render("╰" + strings.Repeat("─", innerW) + "╯")
	if height == 2 {
		return top + "\n" + bot
	}
	innerH := height - 2
	lines := fitLines(body, innerW, innerH)
	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for i, line := range lines {
		right := trackGlyph
		st := edge
		if scroll.active() && thumbAt(i, innerH, scroll.offset, scroll.total) {
			right = thumbGlyph
			st = styleThumb
		}
		b.WriteString(edge.Render(trackGlyph))
		b.WriteString(line)
		b.WriteString(st.Render(right))
		b.WriteByte('\n')
	}
	b.WriteString(bot)
	return b.String()
}

func borderTitleLine(title string, width int, focused bool) string {
	fg := colBorder
	if focused {
		fg = colAccent
	}
	st := lipgloss.NewStyle().Foreground(fg)
	if width < 2 {
		return st.Render(truncate("╭╮", width))
	}
	inner := width - 2
	if strings.TrimSpace(ansi.Strip(title)) == "" {
		return st.Render("╭" + strings.Repeat("─", inner) + "╮")
	}
	label := " " + title + " "
	maxLabel := max(0, inner-2)
	if lipgloss.Width(label) > maxLabel {
		plain := strings.TrimSpace(ansi.Strip(title))
		label = " " + truncate(plain, max(0, maxLabel-2)) + " "
	}
	rest := inner - lipgloss.Width(label)
	left := 1
	if rest < 1 {
		left = max(0, rest)
	}
	right := max(0, rest-left)
	return st.Render("╭"+strings.Repeat("─", left)) + label + st.Render(strings.Repeat("─", right)+"╮")
}

func modalFrame(title, body string, width int) string {
	if width < 8 {
		width = 8
	}
	innerW := max(1, width-2)
	inner := fitWidth(body, innerW)
	h := lipgloss.Height(inner) + 2
	return frameBox(inner, width, h, title, true, scrollState{})
}

func fitWidth(s string, width int) string {
	raw := strings.Split(s, "\n")
	for i, line := range raw {
		raw[i] = padToWidth(line, width)
	}
	return strings.Join(raw, "\n")
}

func fitLines(s string, width, height int) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		out[i] = padToWidth(line, width)
	}
	return out
}

func padToWidth(s string, width int) string {
	s = truncate(s, width)
	pad := width - lipgloss.Width(s)
	if pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func thumbAt(row, viewH, offset, total int) bool {
	if viewH <= 0 || total <= viewH {
		return false
	}
	thumb := max(1, viewH*viewH/total)
	if thumb > viewH {
		thumb = viewH
	}
	maxOff := total - viewH
	if maxOff < 1 {
		return false
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	start := offset * (viewH - thumb) / maxOff
	return row >= start && row < start+thumb
}

func withRightThumb(s string, width, height, offset, total int) string {
	if width < 2 || height < 1 || total <= height {
		return fitWidth(s, width)
	}
	innerW := width - 1
	lines := fitLines(s, innerW, height)
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line)
		if thumbAt(i, height, offset, total) {
			b.WriteString(styleThumb.Render(thumbGlyph))
		} else {
			b.WriteString(styleDim.Render(trackGlyph))
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func dimFrame(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if height < 1 {
		height = len(lines)
	}
	out := make([]string, max(height, len(lines)))
	for i := range out {
		plain := ""
		if i < len(lines) {
			plain = ansi.Strip(lines[i])
		}
		if w := ansi.StringWidth(plain); width > 0 && w < width {
			plain += strings.Repeat(" ", width-w)
		} else if width > 0 {
			plain = ansi.Truncate(plain, width, "")
		}
		out[i] = styleDim.Render(plain)
	}
	return strings.Join(out, "\n")
}

func overlayOn(base string, width, height int, modal string) string {
	base = dimFrame(base, width, height)
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
	need := y + len(mods)
	for len(bases) < need {
		bases = append(bases, "")
	}
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
	if x < 0 {
		x = 0
	}
	mw := lipgloss.Width(overlay)
	plain := ansi.Strip(base)
	left := cellsPrefix(plain, x)
	if w := ansi.StringWidth(left); w < x {
		left += strings.Repeat(" ", x-w)
	}
	right := cellsFrom(plain, x+mw)
	out := styleDim.Render(left) + overlay
	if right != "" {
		out += styleDim.Render(right)
	}
	return out
}

func cellsPrefix(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "")
}

func cellsFrom(s string, start int) string {
	if start <= 0 {
		return s
	}
	if s == "" {
		return ""
	}
	w := 0
	i := 0
	for i < len(s) {
		cluster, _, cw, _ := uniseg.FirstGraphemeClusterInString(s[i:], -1)
		if cluster == "" {
			_, sz := utf8.DecodeRuneInString(s[i:])
			if sz <= 0 {
				break
			}
			cluster = s[i : i+sz]
			cw = 1
		}
		if w >= start {
			return s[i:]
		}
		if w+cw > start {
			return s[i:]
		}
		w += cw
		i += len(cluster)
	}
	return ""
}

func formatBreadcrumb(crumbs []string, width int) string {
	if width <= 0 || len(crumbs) == 0 {
		return ""
	}
	join := func(cs []string) string {
		return strings.Join(cs, crumbSep)
	}
	s := join(crumbs)
	if lipgloss.Width(s) <= width {
		return s
	}
	for len(crumbs) > 1 {
		crumbs = crumbs[1:]
		s = "… › " + join(crumbs)
		if lipgloss.Width(s) <= width {
			return s
		}
	}
	return truncate("… › "+crumbs[0], width)
}

func styleHintLine(s string, width int) string {
	groups := strings.Split(s, "  ")
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		key, desc, ok := strings.Cut(g, " ")
		if !ok {
			parts = append(parts, styleKey.Render(g))
			continue
		}
		parts = append(parts, styleKey.Render(key)+" "+styleDim.Render(desc))
	}
	return truncate(strings.Join(parts, "  "), width)
}

func wrapJoined(parts []string, sep string, width int) string {
	if len(parts) == 0 || width <= 0 {
		return ""
	}
	var lines []string
	var cur string
	sepW := lipgloss.Width(sep)
	for _, p := range parts {
		if cur == "" {
			if lipgloss.Width(p) > width {
				cur = truncate(ansi.Strip(p), width)
			} else {
				cur = p
			}
			continue
		}
		if lipgloss.Width(cur)+sepW+lipgloss.Width(p) <= width {
			cur += sep + p
			continue
		}
		lines = append(lines, cur)
		if lipgloss.Width(p) > width {
			cur = truncate(ansi.Strip(p), width)
		} else {
			cur = p
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func rowCursor(selected, focused bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

func selectStyle(selected, focused bool) lipgloss.Style {
	switch {
	case selected && focused:
		return styleRowSel
	case selected:
		return styleRowIdle
	default:
		return styleRow
	}
}
