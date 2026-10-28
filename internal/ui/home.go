package ui

import (
	"fmt"
	"path"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	homeViews homeSection = iota
	homeTypes
	homeQueries
	homeSectionCount
)

type homeSection int

type homeItem struct {
	Kind  string
	ID    string
	Label string
	Sub   string
	Badge string
}

func (m Model) keysHome(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	if m.pending == "g" {
		m.pending = ""
		if key == "g" {
			m.homeCur[m.homeSec] = 0
			return m, nil
		}
		if key == "h" {
			return m.goHome()
		}
	}
	if m.pending == "ctrlw" {
		return m.keysCtrlW(msg)
	}
	if isSlash(msg) {
		return m.openSearchPicker()
	}
	if isSpace(msg) {
		return m.startLeader()
	}
	switch key {
	case "esc", "backspace", "[", "ctrl+o":
		return m.historyGo(-1)
	case "]", "ctrl+i":
		return m.historyGo(1)
	case "d":
		return m.openDailyDate(m.todayID())
	case "f", "ctrl+p":
		return m.openNotesPicker()
	case "s":
		return m.openSearchPicker()
	case ":":
		m.overlay = overlayRQL
		m.rql = ""
		return m, nil
	case "c":
		return m.startNewObject()
	case "a":
		return m.openActionsPicker()
	case ",":
		return m.openConfigAtView()
	case "r":
		return m.refresh()
	case "tab", "l", "right":
		m.moveHomeSec(1)
		return m, nil
	case "shift+tab", "h", "left":
		m.moveHomeSec(-1)
		return m, nil
	case "j", "down":
		m.moveHome(1)
		return m, nil
	case "k", "up":
		m.moveHome(-1)
		return m, nil
	case "g":
		m.pending = "g"
		return m, nil
	case "G":
		items := m.homeItems(homeSection(m.homeSec))
		if n := len(items); n > 0 {
			m.homeCur[m.homeSec] = n - 1
		}
		return m, nil
	case "enter":
		return m.enterHome()
	}
	return m, nil
}

func (m *Model) moveHomeSec(delta int) {
	m.homeSec = (int(m.homeSec) + delta + int(homeSectionCount)) % int(homeSectionCount)
}

func (m *Model) moveHome(delta int) {
	items := m.homeItems(homeSection(m.homeSec))
	if len(items) == 0 {
		m.homeCur[m.homeSec] = 0
		return
	}
	m.homeCur[m.homeSec] = clamp(m.homeCur[m.homeSec]+delta, 0, len(items)-1)
}

func (m Model) enterHome() (Model, tea.Cmd) {
	items := m.homeItems(homeSection(m.homeSec))
	idx := m.homeCur[m.homeSec]
	if idx < 0 || idx >= len(items) {
		return m, nil
	}
	it := items[idx]
	switch it.Kind {
	case "view":
		return m.loadNamedView(it.ID)
	case "layout":
		return m.loadNamedLayout(it.ID)
	case "type":
		return m.loadType(it.ID)
	case "query":
		if q, ok := m.schema.Queries[it.ID]; ok {
			if q.Name == "" {
				q.Name = it.ID
			}
			cp := q
			return m.loadQuery(&cp)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) homeItems(sec homeSection) []homeItem {
	switch sec {
	case homeViews:
		return m.homeViewItems()
	case homeTypes:
		return m.homeTypeItems()
	case homeQueries:
		return m.homeQueryItems()
	}
	return nil
}

func (m Model) homeViewItems() []homeItem {
	if m.cfg == nil {
		return nil
	}
	var out []homeItem
	for _, name := range m.cfg.ViewsInOrder() {
		v := m.cfg.Views[name]
		out = append(out, homeItem{
			Kind:  "view",
			ID:    name,
			Label: name,
			Badge: viewKindBadge(v),
			Sub:   viewSourceSummary(v),
		})
	}
	for _, name := range m.cfg.LayoutsInOrder() {
		out = append(out, homeItem{
			Kind:  "layout",
			ID:    name,
			Label: name,
			Badge: "layout",
			Sub:   layoutSummary(m.cfg.Layouts[name]),
		})
	}
	return out
}

func viewKindBadge(v config.View) string {
	k := strings.ToLower(strings.TrimSpace(v.Layout.Kind))
	switch k {
	case kindBoard, kindDate, kindList:
		return k
	}
	return kindList
}

func viewSourceSummary(v config.View) string {
	if q := strings.TrimSpace(v.Query); q != "" {
		return "query " + q
	}
	if t := strings.TrimSpace(v.Type); t != "" {
		return "type " + t
	}
	return ""
}

func layoutSummary(l config.Layout) string {
	n := 0
	var walk func(config.Panel)
	walk = func(p config.Panel) {
		if p.IsLeaf() {
			n++
			return
		}
		for _, k := range p.Panels {
			walk(k)
		}
	}
	for _, p := range l.Panels {
		walk(p)
	}
	if n == 1 {
		return "1 panel"
	}
	return fmt.Sprintf("%d panels", n)
}

func (m Model) homeTypeItems() []homeItem {
	var out []homeItem
	for _, name := range m.schema.SortedTypeNames() {
		ts := m.schema.Types[name]
		out = append(out, homeItem{
			Kind:  "type",
			ID:    name,
			Label: name,
			Badge: "type",
			Sub:   typeHomeSub(ts, m.typeObjectCount(name)),
		})
	}
	return out
}

func typeHomeSub(ts api.TypeSchema, n int) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d", n))
	if p := strings.TrimSpace(ts.DefaultPath); p != "" {
		parts = append(parts, p)
	}
	return strings.Join(parts, " · ")
}

func (m Model) typeObjectCount(typeName string) int {
	n := 0
	for _, o := range m.noteIndex {
		if o.Type == typeName {
			n++
		}
	}
	return n
}

func (m Model) homeQueryItems() []homeItem {
	var out []homeItem
	for _, q := range m.schema.SortedSavedQueries() {
		sub := strings.TrimSpace(q.Description)
		if sub == "" {
			sub = q.Query
			if len(q.Args) > 0 {
				sub = "args " + strings.Join(q.Args, ", ")
				if q.Query != "" {
					sub += " · " + q.Query
				}
			}
		}
		out = append(out, homeItem{
			Kind:  "query",
			ID:    q.Name,
			Label: q.Name,
			Badge: "query",
			Sub:   sub,
		})
	}
	return out
}

func (m Model) viewHome(height int) string {
	banner := m.viewHomeBanner()
	used := lipgloss.Height(banner)
	strip := m.viewHomeStrip()
	used += lipgloss.Height(strip)
	innerH := max(6, height-used)
	cols := m.viewHomeColumns(max(20, m.width), innerH)
	return lipgloss.JoinVertical(lipgloss.Left, banner, cols, strip)
}

func (m Model) viewHomeBanner() string {
	name := m.vaultDisplayName()
	p := strings.TrimSpace(m.vaultPath)
	head := styleHeader.Render(name)
	if p != "" && p != name {
		head += "  " + styleMuted.Render(p)
	}
	nTypes := len(m.schema.SortedTypeNames())
	nTraits := len(m.schema.Traits)
	nQueries := len(m.schema.Queries)
	nObjs := len(m.noteIndex)
	stats := fmt.Sprintf("%d objects · %d types · %d traits · %d saved queries", nObjs, nTypes, nTraits, nQueries)
	daily := m.homeDailyStatus()
	w := max(1, m.width)
	lines := []string{
		styleHeader.Width(w).Render(truncate(head, w)),
		styleMuted.Width(w).Render(truncate(stats, w)),
	}
	if daily != "" {
		lines = append(lines, styleDim.Width(w).Render(truncate(daily, w)))
	}
	lines = append(lines, styleDim.Render(strings.Repeat("─", min(w, 48))))
	return strings.Join(lines, "\n")
}

func (m Model) vaultDisplayName() string {
	if n := strings.TrimSpace(m.vaultName); n != "" {
		return n
	}
	if m.session != nil {
		if n := m.session.DisplayName(); n != "" {
			return n
		}
	}
	p := strings.TrimRight(strings.TrimSpace(m.vaultPath), "/")
	if p == "" {
		return "vault"
	}
	return path.Base(p)
}

func (m Model) homeDailyStatus() string {
	if m.daily == nil || strings.TrimSpace(m.daily.TodayID) == "" {
		return ""
	}
	id := m.daily.TodayID
	if m.daily.TodayExists {
		return "daily " + id + " exists"
	}
	return "daily " + id + " will be created on open"
}

func (m Model) viewHomeStrip() string {
	return styleDim.Render(truncate("d today   f find   s search   : rql   c new   a actions   , config   Space h home", m.width))
}

func (m Model) viewHomeColumns(width, height int) string {
	titles := []string{"views & layouts", "types", "saved queries"}
	n := int(homeSectionCount)
	colW := max(18, (width-n+1)/n)
	if colW*n > width {
		colW = max(16, width/n)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		focused := m.homeSec == i
		parts = append(parts, m.viewHomeSection(homeSection(i), titles[i], colW, height, focused))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) viewHomeSection(sec homeSection, title string, width, height int, focused bool) string {
	fg := colBorder
	if focused {
		fg = colAccent
	}
	box := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.NormalBorder()).
		BorderForeground(fg)
	innerW := max(1, width-4)
	innerH := max(1, height-2)
	var b strings.Builder
	head := title
	if focused {
		head = "▸ " + title
	}
	b.WriteString(styleSectionTitle.Render(head))
	b.WriteByte('\n')
	items := m.homeItems(sec)
	if len(items) == 0 {
		b.WriteString(styleDim.Render(m.homeEmptyHint(sec)))
		return box.Render(b.String())
	}
	cur := clamp(m.homeCur[sec], 0, len(items)-1)
	off := 0
	rows := max(1, innerH-1)
	if cur >= rows {
		off = cur - rows + 1
	}
	end := min(len(items), off+rows)
	for i := off; i < end; i++ {
		it := items[i]
		line := it.Label
		if it.Badge != "" {
			line += "  " + styleDim.Render(it.Badge)
		}
		if it.Sub != "" {
			line += "  " + styleDim.Render(truncate(it.Sub, max(6, innerW/2)))
		}
		line = truncate(line, innerW)
		if i == cur {
			st := styleNavSel
			if !focused {
				st = styleNav
			}
			b.WriteString(st.Width(innerW).Render(line))
		} else {
			b.WriteString(styleNav.Render(line))
		}
		b.WriteByte('\n')
	}
	return box.Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) homeEmptyHint(sec homeSection) string {
	switch sec {
	case homeViews:
		return "no views yet — S saves a query, or add [views.*] / [layouts.*]"
	case homeTypes:
		return "no types in schema"
	case homeQueries:
		return "no saved queries"
	}
	return "empty"
}
