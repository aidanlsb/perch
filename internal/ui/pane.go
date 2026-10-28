package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/group"
	"github.com/aidanlsb/perch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	focusPreview = "preview"
)

type viewState struct {
	id            string
	viewName      string
	bind          string
	spec          layout.Spec
	sourceQuery   string
	sourceBase    string
	sourceRQL     string
	sourceLabel   string
	sourceIsQuery bool
	typeName      string
	groupBy       string
	granularity   string
	listFields    []string
	sortField     string
	objects       []api.Object
	filtered      []api.Object
	hasMore       bool
	total         int
	listCur       int
	listOff       int
	board         []group.Column
	boardCol      int
	boardRow      int
	dateCols      []group.Column
	dateSec       int
	dateRow       int
	rawInputs     map[string]string
	sourceInputs  map[string]string
	savedQuery    *api.SavedQuery
	gen           int
	loading       bool
	keepCursor    bool
	restoreID     string
	restoreOff    int
	live          bool
	queryKind     string
	previewOff    bool
}

type layoutTree struct {
	split string
	id    string
	kids  []layoutTree
}

func (m *Model) p() *viewState {
	if v := m.viewByID(m.focus); v != nil {
		return v
	}
	if len(m.panes) > 0 {
		return m.panes[0]
	}
	if m.emptyPane == nil {
		m.emptyPane = &viewState{spec: layout.Spec{Kind: kindList}.Normalize()}
	}
	return m.emptyPane
}

func (m *Model) viewByID(id string) *viewState {
	for _, p := range m.panes {
		if p.id == id {
			return p
		}
	}
	return nil
}

func (m Model) typeSchemaFor(v *viewState) api.TypeSchema {
	if v == nil || v.typeName == "" {
		return api.TypeSchema{}
	}
	return m.schema.Types[v.typeName]
}

func (v *viewState) applySpec(s layout.Spec, ts api.TypeSchema) {
	s = s.Normalize()
	v.spec = s
	v.groupBy = s.GroupBy
	v.granularity = s.Granularity
	if len(s.Fields) > 0 {
		v.listFields = s.Fields
	} else if ts.Name != "" || len(ts.Fields) > 0 {
		v.listFields = defaultFields(ts)
		v.spec.Fields = v.listFields
	}
	v.sortField = s.Sort
	v.applyFilterAndGroup(ts)
}

func (v *viewState) applyFilterAndGroup(ts api.TypeSchema) {
	r := layout.Apply(v.objects, v.spec, ts, "", false)
	v.filtered = r.Objects
	v.board = r.Board
	v.dateCols = r.Date
	if v.boardCol >= len(v.board) {
		v.boardCol = max(0, len(v.board)-1)
	}
	if len(v.board) > 0 && v.boardRow >= len(v.board[v.boardCol].Items) {
		v.boardRow = max(0, len(v.board[v.boardCol].Items)-1)
	}
	if v.dateSec >= len(v.dateCols) {
		v.dateSec = max(0, len(v.dateCols)-1)
	}
	if len(v.dateCols) > 0 && v.dateRow >= len(v.dateCols[v.dateSec].Items) {
		v.dateRow = max(0, len(v.dateCols[v.dateSec].Items)-1)
	}
	if v.listCur >= len(v.filtered) {
		v.listCur = max(0, len(v.filtered)-1)
	}
}

func (v *viewState) resetCursor() {
	v.listCur = 0
	v.listOff = 0
	v.boardCol = 0
	v.boardRow = 0
	v.dateSec = 0
	v.dateRow = 0
}

func (v *viewState) selectedObject() *api.Object {
	if v == nil {
		return nil
	}
	switch v.spec.Kind {
	case kindBoard:
		if v.boardCol < 0 || v.boardCol >= len(v.board) {
			return nil
		}
		items := v.board[v.boardCol].Items
		if v.boardRow < 0 || v.boardRow >= len(items) {
			return nil
		}
		o := items[v.boardRow]
		return &o
	case kindDate:
		if v.dateSec < 0 || v.dateSec >= len(v.dateCols) {
			return nil
		}
		items := v.dateCols[v.dateSec].Items
		if v.dateRow < 0 || v.dateRow >= len(items) {
			return nil
		}
		o := items[v.dateRow]
		return &o
	default:
		if v.listCur < 0 || v.listCur >= len(v.filtered) {
			return nil
		}
		o := v.filtered[v.listCur]
		return &o
	}
}

func (v *viewState) selectID(id string) bool {
	if v == nil || id == "" {
		return false
	}
	switch v.spec.Kind {
	case kindBoard:
		for c := range v.board {
			for r, obj := range v.board[c].Items {
				if obj.ID == id {
					v.boardCol = c
					v.boardRow = r
					return true
				}
			}
		}
	case kindDate:
		for s := range v.dateCols {
			for r, obj := range v.dateCols[s].Items {
				if obj.ID == id {
					v.dateSec = s
					v.dateRow = r
					return true
				}
			}
		}
	default:
		for i, obj := range v.filtered {
			if obj.ID == id {
				v.listCur = i
				return true
			}
		}
	}
	return false
}

func (v *viewState) move(delta int) {
	switch v.spec.Kind {
	case kindBoard:
		if len(v.board) == 0 {
			return
		}
		col := &v.board[v.boardCol]
		v.boardRow = clamp(v.boardRow+delta, 0, max(0, len(col.Items)-1))
	case kindDate:
		v.moveDate(delta)
	default:
		v.listCur = clamp(v.listCur+delta, 0, max(0, len(v.filtered)-1))
	}
}

func (v *viewState) moveDate(delta int) {
	if len(v.dateCols) == 0 {
		return
	}
	v.dateRow += delta
	for v.dateRow < 0 && v.dateSec > 0 {
		v.dateSec--
		v.dateRow = len(v.dateCols[v.dateSec].Items)
		if v.dateRow > 0 {
			v.dateRow--
		}
	}
	for v.dateSec < len(v.dateCols) && v.dateRow >= len(v.dateCols[v.dateSec].Items) {
		overflow := v.dateRow - len(v.dateCols[v.dateSec].Items)
		if v.dateSec+1 >= len(v.dateCols) {
			v.dateRow = max(0, len(v.dateCols[v.dateSec].Items)-1)
			break
		}
		v.dateSec++
		v.dateRow = overflow
	}
	if v.dateRow < 0 {
		v.dateRow = 0
	}
}

func (v *viewState) moveBoardCol(delta int) {
	if len(v.board) == 0 {
		return
	}
	v.boardCol = clamp(v.boardCol+delta, 0, len(v.board)-1)
	v.boardRow = clamp(v.boardRow, 0, max(0, len(v.board[v.boardCol].Items)-1))
}

func (v *viewState) moveToStart() {
	v.listCur = 0
	v.boardRow = 0
	v.dateSec = 0
	v.dateRow = 0
}

func (v *viewState) moveToEnd() {
	v.listCur = max(0, len(v.filtered)-1)
	if len(v.board) > 0 {
		v.boardRow = max(0, len(v.board[v.boardCol].Items)-1)
	}
	if len(v.dateCols) > 0 {
		v.dateSec = len(v.dateCols) - 1
		v.dateRow = max(0, len(v.dateCols[v.dateSec].Items)-1)
	}
}

func (v *viewState) resolveWithSelected(selected string) (query string, inputs map[string]string) {
	sub := func(s string) string { return config.SubstituteSelected(s, selected) }
	base := sub(v.sourceBase)
	rql := sub(v.sourceRQL)
	filter := sub(v.spec.Filter)
	if strings.TrimSpace(filter) == "" {
		query = base
	} else {
		b := base
		if v.sourceIsQuery && strings.TrimSpace(rql) != "" {
			b = rql
		}
		query = layout.AppendFilter(b, filter)
	}
	if len(v.rawInputs) > 0 {
		inputs = make(map[string]string, len(v.rawInputs))
		for k, val := range v.rawInputs {
			inputs[k] = sub(val)
		}
	}
	return query, inputs
}

func (v *viewState) selectedID() string {
	if o := v.selectedObject(); o != nil {
		return o.BindID()
	}
	return ""
}

func (v *viewState) selectedRowID() string {
	if o := v.selectedObject(); o != nil {
		return o.ID
	}
	return ""
}

func (m Model) selectedObject() *api.Object {
	if m.overlay == overlayPicker && m.pickerKind == pickerNotes {
		if m.pickerCur >= 0 && m.pickerCur < len(m.pickerHit) {
			it := m.pickerHit[m.pickerCur]
			if it.Object != nil {
				o := *it.Object
				return &o
			}
			if o, ok := m.noteIndex[it.ID]; ok {
				return &o
			}
		}
	}
	return m.p().selectedObject()
}

func (m *Model) selectID(id string) bool {
	if m.p().selectID(id) {
		m.setFocus(m.p().id)
		return true
	}
	return false
}

func applyViewSource(p *viewState, schema api.Schema, v config.View) {
	if p == nil {
		return
	}
	p.rawInputs = v.Inputs
	p.savedQuery = nil
	p.sourceIsQuery = false
	p.sourceRQL = ""
	p.typeName = ""
	if qname := strings.TrimSpace(v.Query); qname != "" {
		p.sourceIsQuery = true
		p.sourceBase = qname
		if sq, ok := schema.Queries[qname]; ok {
			if sq.Name == "" {
				sq.Name = qname
			}
			cp := sq
			p.savedQuery = &cp
			p.sourceRQL = sq.Query
			p.sourceBase = sq.Name
			p.typeName = typeFromRQL(sq.Query)
		}
		return
	}
	t := strings.TrimSpace(v.Type)
	p.sourceBase = "type:" + t
	p.typeName = t
}

func typeFromRQL(q string) string {
	body := strings.TrimSpace(q)
	if !strings.HasPrefix(body, "type:") {
		return ""
	}
	rest := strings.TrimPrefix(body, "type:")
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func treeFromLayout(l config.Layout) layoutTree {
	return nodeFromPanel(config.Panel{Split: l.Split, Panels: l.Panels})
}

func nodeFromPanel(p config.Panel) layoutTree {
	if p.IsLeaf() {
		return layoutTree{id: p.LeafID()}
	}
	n := layoutTree{split: p.Split}
	for _, k := range p.Panels {
		n.kids = append(n.kids, nodeFromPanel(k))
	}
	return n
}

func (m *Model) setSinglePane(p *viewState) {
	if p.id == "" {
		p.id = "main"
	}
	m.panes = []*viewState{p}
	m.tree = layoutTree{id: p.id}
	m.currentView = p.viewName
	m.currentLayout = ""
	if m.focus != focusPreview {
		m.setFocus(p.id)
	}
}

func (m Model) startPaneQuery(p *viewState, selected string) (Model, tea.Cmd) {
	return m.queryPane(p, selected, false, false)
}

func (m Model) queryPane(p *viewState, selected string, keep, live bool) (Model, tea.Cmd) {
	if p == nil {
		return m, nil
	}
	if keep {
		p.restoreID = p.selectedRowID()
		p.restoreOff = p.listOff
		p.keepCursor = true
	} else {
		p.restoreID = ""
		p.restoreOff = 0
		p.keepCursor = false
		p.resetCursor()
	}
	p.live = live
	query, inputs := p.resolveWithSelected(selected)
	p.sourceQuery = query
	p.sourceInputs = inputs
	ts := m.typeSchemaFor(p)
	if ent, ok := m.queryCache[cacheKey(query, inputs)]; ok {
		p.objects = ent.objects
		if p.typeName == "" {
			p.typeName = ent.typeName
			ts = m.typeSchemaFor(p)
		}
		p.hasMore = ent.hasMore
		p.total = ent.total
		p.applyFilterAndGroup(ts)
		if keep && p.restoreID != "" {
			p.selectID(p.restoreID)
		}
	}
	p.gen = m.nextGen()
	p.loading = true
	return m, fetchQuery(m.client, p.id, p.gen, query, p.typeName, m.queryLimit, ts, inputs)
}

func (m Model) refreshBound(driver string) (Model, tea.Cmd) {
	return m.refreshBoundOpts(driver, false, false)
}

func (m Model) refreshBoundOpts(driver string, keep, live bool) (Model, tea.Cmd) {
	sel := ""
	if d := m.viewByID(driver); d != nil {
		sel = d.selectedID()
	}
	var cmds []tea.Cmd
	for _, p := range m.panes {
		if p.bind != driver {
			continue
		}
		var cmd tea.Cmd
		m, cmd = m.queryPane(p, sel, keep, live)
		cmds = append(cmds, cmd)
	}
	return m, batchCmds(cmds)
}

func (m Model) boundTarget(driver string) string {
	for _, p := range m.panes {
		if p.bind == driver {
			return p.id
		}
	}
	return ""
}

func batchCmds(cmds []tea.Cmd) tea.Cmd {
	var out []tea.Cmd
	for _, c := range cmds {
		if c != nil {
			out = append(out, c)
		}
	}
	switch len(out) {
	case 0:
		return nil
	case 1:
		return out[0]
	default:
		return tea.Batch(out...)
	}
}
