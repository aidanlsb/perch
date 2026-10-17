package ui

import (
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	catBrowse = "Browse"
	catViews  = "Views"
)

func (m *Model) rebuildNav() {
	cur := ""
	if m.navCur >= 0 && m.navCur < len(m.nav) {
		cur = navKey(m.nav[m.navCur])
	}
	m.nav = nil
	m.nav = append(m.nav, navItem{Kind: navCat, Label: catBrowse, Category: catBrowse})
	if m.browseOpen {
		for _, name := range m.schema.SortedTypeNames() {
			m.nav = append(m.nav, navItem{
				Kind:  navType,
				Label: name,
				Type:  name,
			})
		}
	}
	m.nav = append(m.nav, navItem{Kind: navCat, Label: catViews, Category: catViews})
	if m.viewsOpen && m.cfg != nil {
		for _, name := range m.cfg.ViewNames() {
			m.nav = append(m.nav, navItem{Kind: navView, Label: name, View: name})
		}
		for _, name := range m.cfg.LayoutNames() {
			m.nav = append(m.nav, navItem{Kind: navLayout, Label: name, Layout: name})
		}
	}
	m.navCur = 0
	if cur != "" {
		for i, n := range m.nav {
			if navKey(n) == cur {
				m.navCur = i
				break
			}
		}
	}
}

func navKey(n navItem) string {
	switch n.Kind {
	case navCat:
		return "cat:" + n.Category
	case navType:
		return "type:" + n.Type
	case navView:
		return "view:" + n.View
	case navLayout:
		return "layout:" + n.Layout
	default:
		return n.Label
	}
}

func (m Model) activateNav() (Model, tea.Cmd) {
	if m.navCur < 0 || m.navCur >= len(m.nav) {
		return m, nil
	}
	item := m.nav[m.navCur]
	switch item.Kind {
	case navCat:
		return m.toggleCategory(item.Category), nil
	case navType:
		return m.loadType(item.Type)
	case navView:
		return m.loadNamedView(item.View)
	case navLayout:
		return m.loadNamedLayout(item.Layout)
	default:
		return m, nil
	}
}

func (m Model) toggleCategory(cat string) Model {
	switch cat {
	case catBrowse:
		m.browseOpen = !m.browseOpen
	case catViews:
		m.viewsOpen = !m.viewsOpen
	}
	m.rebuildNav()
	return m
}

func (m Model) expandCategory(cat string, open bool) Model {
	switch cat {
	case catBrowse:
		m.browseOpen = open
	case catViews:
		m.viewsOpen = open
	}
	m.rebuildNav()
	return m
}

func (m Model) loadType(name string) (Model, tea.Cmd) {
	ts := m.schema.Types[name]
	p := &viewState{
		id:          "type:" + name,
		spec:        layout.DefaultList(ts),
		sourceBase:  "type:" + name,
		sourceQuery: "type:" + name,
		sourceLabel: name,
		typeName:    name,
	}
	p.applySpec(p.spec, ts)
	m.screen = "browse"
	m.note = nil
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.errMsg = ""
	m.setSinglePane(p)
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
	m.screen = "browse"
	m.note = nil
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.errMsg = ""
	m.setSinglePane(p)
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
	m.screen = "browse"
	m.note = nil
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
		v := m.cfg.Views[leaf.View]
		p := m.paneFromView(leaf.View, v, leaf.Bind)
		m.panes = append(m.panes, p)
		if p.bind == "" {
			var cmd tea.Cmd
			m, cmd = m.startPaneQuery(p, "")
			cmds = append(cmds, cmd)
		}
	}
	if m.focus != focusSidebar && m.focus != focusPreview && len(m.panes) > 0 {
		m.setFocus(m.panes[0].id)
	}
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
	m.screen = "browse"
	m.note = nil
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.setSinglePane(p)
	if len(q.Args) > 0 && !inputsComplete(q.Args, p.rawInputs) {
		return m.startArgsPrompt(q)
	}
	m.status = "loading " + q.Name + "…"
	return m.startPaneQuery(p, "")
}

func (m Model) loadRQL(q string) (Model, tea.Cmd) {
	p := &viewState{
		id:          "rql",
		spec:        layout.Spec{Name: "RQL", Kind: kindList}.Normalize(),
		sourceBase:  q,
		sourceQuery: q,
		sourceLabel: q,
		typeName:    typeFromRQL(q),
	}
	if p.typeName != "" {
		p.applySpec(layout.DefaultList(m.schema.Types[p.typeName]), m.schema.Types[p.typeName])
	} else {
		p.applySpec(p.spec, api.TypeSchema{})
	}
	m.screen = "browse"
	m.note = nil
	m.searchQ = ""
	m.searchHits = nil
	m.overlay = overlayNone
	m.setSinglePane(p)
	m.status = "loading…"
	return m.startPaneQuery(p, "")
}
