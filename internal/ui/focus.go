package ui

import tea "github.com/charmbracelet/bubbletea"

type region struct {
	id string
	x  int
	y  int
	w  int
	h  int
}

func (m *Model) setFocus(id string) {
	if id == "" {
		return
	}
	if id != focusPreview {
		m.lastMainFocus = id
	}
	m.focus = id
	switch id {
	case focusPreview:
		m.pane = panePreview
	default:
		m.pane = paneMain
	}
}

func (m Model) validMainFocus(id string) string {
	if id == "" {
		return ""
	}
	if id == focusPreview {
		if m.previewOn {
			return focusPreview
		}
		return ""
	}
	if m.viewByID(id) != nil {
		return id
	}
	return ""
}

func (m Model) mainFocusID() string {
	if id := m.validMainFocus(m.lastMainFocus); id != "" {
		return id
	}
	if len(m.panes) > 0 {
		return m.panes[0].id
	}
	if m.previewOn {
		return focusPreview
	}
	return ""
}

func (m *Model) focusMainArea() {
	m.setFocus(m.mainFocusID())
}

func (m *Model) recomputeRegions() {
	m.regs = m.browseRegions(m.width, max(1, m.height-2))
}

func (m Model) browseRegions(width, height int) []region {
	if width <= 0 || height <= 0 {
		return nil
	}
	if m.zoomed {
		return []region{{id: m.zoomTarget(), x: 0, y: 0, w: width, h: height}}
	}
	if m.previewOn {
		pw := max(previewMinW, width*2/5)
		if pw > width-16 {
			pw = max(previewMinW, width/3)
		}
		mainW := max(12, width-pw)
		regs := m.treeRegions(m.tree, 0, 0, mainW, height)
		regs = append(regs, region{id: focusPreview, x: mainW, y: 0, w: pw, h: height})
		return regs
	}
	return m.treeRegions(m.tree, 0, 0, width, height)
}

func (m Model) treeRegions(n layoutTree, x, y, w, h int) []region {
	if n.split == "" || len(n.kids) == 0 {
		id := n.id
		if id == "" && len(m.panes) == 1 {
			id = m.panes[0].id
		}
		if id == "" {
			return nil
		}
		return []region{{id: id, x: x, y: y, w: w, h: h}}
	}
	sizes := splitSizes(splitAxis(n.split, w, h), len(n.kids))
	var out []region
	off := 0
	for i, kid := range n.kids {
		if n.split == "col" {
			out = append(out, m.treeRegions(kid, x, y+off, w, sizes[i])...)
		} else {
			out = append(out, m.treeRegions(kid, x+off, y, sizes[i], h)...)
		}
		off += sizes[i]
	}
	return out
}

func splitAxis(split string, w, h int) int {
	if split == "col" {
		return h
	}
	return w
}

func splitSizes(total, n int) []int {
	if n <= 0 {
		return nil
	}
	base := total / n
	rem := total % n
	out := make([]int, n)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
		if out[i] < 1 {
			out[i] = 1
		}
	}
	return out
}

func (m Model) zoomTarget() string {
	if m.focus != "" {
		return m.focus
	}
	if id := m.mainFocusID(); id != "" {
		return id
	}
	if len(m.panes) > 0 {
		return m.panes[0].id
	}
	return ""
}

func (m Model) focusOrder() []string {
	var order []string
	var walk func(layoutTree)
	walk = func(n layoutTree) {
		if n.split == "" || len(n.kids) == 0 {
			if n.id != "" {
				order = append(order, n.id)
			}
			return
		}
		for _, k := range n.kids {
			walk(k)
		}
	}
	walk(m.tree)
	if len(m.panes) > 0 && len(order) == 0 {
		order = append(order, m.panes[0].id)
	}
	if m.previewOn {
		order = append(order, focusPreview)
	}
	return order
}

func (m Model) cycleFocus(delta int) Model {
	order := m.focusOrder()
	if len(order) == 0 {
		return m
	}
	idx := -1
	for i, id := range order {
		if id == m.focus {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.focusMainArea()
		return m
	}
	m.setFocus(order[(idx+delta+len(order))%len(order)])
	return m
}

func (m Model) moveFocus(dx, dy int) Model {
	m.recomputeRegions()
	cur, ok := m.regionByID(m.focus)
	if !ok {
		return m.cycleFocus(1)
	}
	bestID := ""
	bestScore := 0
	found := false
	for _, r := range m.regs {
		if r.id == cur.id {
			continue
		}
		if !inDirection(cur, r, dx, dy) {
			continue
		}
		score := focusScore(cur, r, dx, dy)
		if !found || score > bestScore {
			found = true
			bestScore = score
			bestID = r.id
		}
	}
	if found {
		m.setFocus(bestID)
	}
	return m
}

func (m Model) regionByID(id string) (region, bool) {
	for _, r := range m.regs {
		if r.id == id {
			return r, true
		}
	}
	return region{}, false
}

func inDirection(cur, other region, dx, dy int) bool {
	switch {
	case dx < 0:
		return other.x+other.w <= cur.x+1
	case dx > 0:
		return other.x >= cur.x+cur.w-1
	case dy < 0:
		return other.y+other.h <= cur.y+1
	case dy > 0:
		return other.y >= cur.y+cur.h-1
	}
	return false
}

func focusScore(cur, other region, dx, dy int) int {
	overlap := 0
	dist := 0
	if dx != 0 {
		overlap = overlapRange(cur.y, cur.y+cur.h, other.y, other.y+other.h)
		if dx < 0 {
			dist = cur.x - (other.x + other.w)
		} else {
			dist = other.x - (cur.x + cur.w)
		}
	} else {
		overlap = overlapRange(cur.x, cur.x+cur.w, other.x, other.x+other.w)
		if dy < 0 {
			dist = cur.y - (other.y + other.h)
		} else {
			dist = other.y - (cur.y + cur.h)
		}
	}
	if dist < 0 {
		dist = 0
	}
	return overlap*1000 - dist
}

func overlapRange(a0, a1, b0, b1 int) int {
	lo := max(a0, b0)
	hi := min(a1, b1)
	if hi <= lo {
		return 0
	}
	return hi - lo
}

func (m Model) keysCtrlW(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	m.pending = ""
	switch key {
	case "h", "left", "ctrl+h":
		return m.moveFocus(-1, 0), nil
	case "j", "down", "ctrl+j":
		return m.moveFocus(0, 1), nil
	case "k", "up", "ctrl+k":
		return m.moveFocus(0, -1), nil
	case "l", "right", "ctrl+l":
		return m.moveFocus(1, 0), nil
	case "o":
		m.zoomed = true
		m.status = "zoom"
		return m, nil
	case "=":
		m.zoomed = false
		m.status = "restore"
		return m, nil
	}
	return m, nil
}
