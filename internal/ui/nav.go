package ui

import (
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) loadType(name string) (Model, tea.Cmd) {
	ts := m.schema.Types[name]
	p := &viewState{
		id:          "type:" + name,
		spec:        layout.DefaultList(ts),
		sourceLabel: name,
	}
	applyViewSource(p, m.schema, config.View{Type: name})
	p.applySpec(p.spec, ts)
	m.beginNav()
	m.screen = kindBrowse
	m.clearNoteDisplay()
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.errMsg = ""
	m.setSinglePane(p)
	m.commitNav()
	m.status = "loading " + name + "…"
	return m.startPaneQuery(p, "")
}

func (m Model) loadNamedView(name string) (Model, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}
	v, ok := m.cfg.Views[name]
	if !ok {
		m.status = "no view " + name
		return m, nil
	}
	p := m.paneFromView(name, v, "")
	m.beginNav()
	m.screen = kindBrowse
	m.clearNoteDisplay()
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.errMsg = ""
	m.setSinglePane(p)
	m.commitNav()
	if p.savedQuery != nil && len(p.savedQuery.Args) > 0 && !inputsComplete(p.savedQuery.Args, p.rawInputs) && !v.NeedsSelected() {
		return m.startArgsPrompt(p.savedQuery)
	}
	m.status = "loading " + name + "…"
	return m.startPaneQuery(p, "")
}

func (m Model) loadNamedLayout(name string) (Model, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}
	l, ok := m.cfg.Layouts[name]
	if !ok {
		m.status = "no layout " + name
		return m, nil
	}
	m.beginNav()
	m.screen = kindBrowse
	m.clearNoteDisplay()
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.errMsg = ""
	m.currentLayout = name
	m.currentView = ""
	m.tree = treeFromLayout(l)
	m.panes = nil
	var cmds []tea.Cmd
	for _, leaf := range l.Leaves() {
		p := m.paneFromPanel(leaf)
		m.panes = append(m.panes, p)
		if p.bind == "" {
			var cmd tea.Cmd
			m, cmd = m.startPaneQuery(p, "")
			cmds = append(cmds, cmd)
		}
	}
	if m.focus != focusPreview && len(m.panes) > 0 {
		m.setFocus(m.panes[0].id)
	}
	m.commitNav()
	m.status = "layout " + name
	return m, batchCmds(cmds)
}

func (m *Model) paneFromView(name string, v config.View, bind string) *viewState {
	spec := v.Spec(name)
	p := &viewState{
		id:          name,
		viewName:    name,
		bind:        bind,
		spec:        spec,
		sourceLabel: name,
	}
	applyViewSource(p, m.schema, v)
	p.applySpec(spec, m.typeSchemaFor(p))
	return p
}

func (m *Model) paneFromPanel(leaf config.Panel) *viewState {
	id := leaf.LeafID()
	v := leaf.AsView(nil)
	viewName := ""
	if m.cfg != nil && leaf.View != "" {
		v = leaf.AsView(m.cfg.Views)
		viewName = leaf.View
	}
	spec := v.Spec(id)
	label := id
	if viewName != "" {
		label = viewName
	} else if v.Query != "" {
		label = v.Query
	} else if v.Type != "" {
		label = v.Type
	}
	p := &viewState{
		id:          id,
		viewName:    viewName,
		bind:        leaf.Bind,
		spec:        spec,
		sourceLabel: label,
	}
	applyViewSource(p, m.schema, v)
	p.applySpec(spec, m.typeSchemaFor(p))
	return p
}

func (m Model) loadQuery(q *api.SavedQuery) (Model, tea.Cmd) {
	return m.loadQueryWithInputs(q, nil)
}

func (m Model) loadQueryWithInputs(q *api.SavedQuery, inputs map[string]string) (Model, tea.Cmd) {
	if q == nil {
		return m, nil
	}
	p := &viewState{
		id:            "query:" + q.Name,
		spec:          layout.Spec{Name: q.Name, Kind: kindList}.Normalize(),
		sourceBase:    q.Name,
		sourceRQL:     q.Query,
		sourceLabel:   q.Name,
		sourceIsQuery: true,
		typeName:      typeFromRQL(q.Query),
		savedQuery:    q,
		rawInputs:     inputs,
	}
	if p.typeName != "" {
		p.applySpec(layout.DefaultList(m.schema.Types[p.typeName]), m.schema.Types[p.typeName])
	} else {
		p.applySpec(p.spec, api.TypeSchema{})
	}
	m.beginNav()
	m.screen = kindBrowse
	m.clearNoteDisplay()
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.setSinglePane(p)
	m.commitNav()
	if len(q.Args) > 0 && !inputsComplete(q.Args, p.rawInputs) {
		return m.startArgsPrompt(q)
	}
	m.status = "loading " + q.Name + "…"
	return m.startPaneQuery(p, "")
}

func (m Model) loadRQL(q string) (Model, tea.Cmd) {
	p := &viewState{
		id:            "rql",
		spec:          layout.Spec{Name: "RQL", Kind: kindList}.Normalize(),
		sourceBase:    q,
		sourceQuery:   q,
		sourceRQL:     q,
		sourceLabel:   q,
		sourceIsQuery: true,
		typeName:      typeFromRQL(q),
	}
	if p.typeName != "" {
		p.applySpec(layout.DefaultList(m.schema.Types[p.typeName]), m.schema.Types[p.typeName])
	} else {
		p.applySpec(p.spec, api.TypeSchema{})
	}
	m.beginNav()
	m.screen = kindBrowse
	m.clearNoteDisplay()
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.setSinglePane(p)
	m.commitNav()
	m.status = "loading…"
	return m.startPaneQuery(p, "")
}
