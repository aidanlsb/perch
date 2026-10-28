package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/layout"
)

func nestedLayoutCfg() *config.File {
	return &config.File{
		Views: map[string]config.View{
			"projects": {Type: "project", Present: config.Present{Kind: kindList}},
			"tasks": {
				Type:    "project",
				Present: config.Present{Kind: kindList, Filter: ".id=={{selected}}"},
			},
			"notes": {Type: "person", Present: config.Present{Kind: kindList}},
		},
		Layouts: map[string]config.Layout{
			"workspace": {
				Split: "row",
				Panels: []config.Panel{
					{View: "projects"},
					{
						Split: "col",
						Panels: []config.Panel{
							{View: "tasks", Bind: "projects"},
							{View: "notes"},
						},
					},
				},
			},
		},
	}
}

func TestOpenOptionLoadsView(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg(), Open: "projects"})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if m.currentView != "projects" {
		t.Fatalf("view %s", m.currentView)
	}
}

func TestOpenOptionLoadsLayout(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg(), Open: "workspace"})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if m.currentLayout != "workspace" || len(m.panes) != 3 {
		t.Fatalf("layout %s panes %d", m.currentLayout, len(m.panes))
	}
	if len(m.hist) < 2 || m.hist[0].Screen != kindHome {
		t.Fatalf("home not under layout: %+v", m.hist)
	}
}

func TestOpenOptionLoadsHome(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg(), Open: "home"})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if m.screen != kindHome {
		t.Fatalf("screen %s", m.screen)
	}
	if m.currentView != "" || m.currentLayout != "" {
		t.Fatalf("view=%s layout=%s", m.currentView, m.currentLayout)
	}
	if len(m.hist) < 1 || m.hist[0].Screen != kindHome {
		t.Fatalf("hist %+v", m.hist)
	}
}

func TestOpenOptionLoadsQuery(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg(), Open: "open"})
	m, cmd := m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if cmd == nil {
		t.Fatal("expected query")
	}
	p := m.p()
	if !p.sourceIsQuery || p.sourceBase != "open" {
		t.Fatalf("pane %+v", p)
	}
	if len(m.hist) < 2 || m.hist[0].Screen != kindHome {
		t.Fatalf("home not under query: %+v", m.hist)
	}
}

func TestOpenOptionLoadsType(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg(), Open: "person"})
	m, cmd := m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if cmd == nil {
		t.Fatal("expected type query")
	}
	if m.p().typeName != "person" || m.p().sourceIsQuery {
		t.Fatalf("pane %+v", m.p())
	}
	if len(m.hist) < 2 || m.hist[0].Screen != kindHome {
		t.Fatalf("home not under type: %+v", m.hist)
	}
}

func TestOpenOptionQueryArgsPromptAndComplete(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}, Open: "need"})
	m, cmd := m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if cmd != nil {
		t.Fatal("need should prompt")
	}
	if m.overlay != overlayArgs || len(m.argsNames) != 1 || m.argsNames[0] != "x" {
		t.Fatalf("overlay=%v names=%v", m.overlay, m.argsNames)
	}

	m = NewModel(Options{Config: &config.File{}, Open: "need", OpenInputs: map[string]string{"x": "p"}})
	m, cmd = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if cmd == nil {
		t.Fatal("complete args should query")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay %v", m.overlay)
	}
	if m.p().rawInputs["x"] != "p" {
		t.Fatalf("inputs %v", m.p().rawInputs)
	}
}

func TestOpenOptionUnknownGoesHome(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}, Open: "ghost"})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if m.screen != kindHome {
		t.Fatalf("screen %s", m.screen)
	}
	if !strings.Contains(m.errMsg, "ghost") {
		t.Fatalf("err %q", m.errMsg)
	}
}

func TestLayoutNestingLoadsLeaves(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m.width, m.height = 80, 24
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	if len(m.panes) != 3 {
		t.Fatalf("leaves %d", len(m.panes))
	}
	ids := []string{m.panes[0].id, m.panes[1].id, m.panes[2].id}
	if ids[0] != "projects" || ids[1] != "tasks" || ids[2] != "notes" {
		t.Fatalf("%v", ids)
	}
	if m.panes[1].bind != "projects" {
		t.Fatalf("bind %s", m.panes[1].bind)
	}
	if m.tree.split != "row" || len(m.tree.kids) != 2 || m.tree.kids[1].split != "col" {
		t.Fatalf("tree %+v", m.tree)
	}
}

func TestBindUpdatesLiveFromSelection(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, cmd := m.loadNamedLayout("workspace")
	if cmd == nil {
		t.Fatal("expected queries for unbound panes")
	}
	left := m.viewByID("projects")
	m, _ = m.update(queryMsg{
		gen: left.gen, pane: left.id, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project"},
			{ID: "projects/b", Title: "Beta", Type: "project"},
		},
	})
	tasks := m.viewByID("tasks")
	if !strings.Contains(tasks.sourceQuery, "projects/a") {
		t.Fatalf("bound query should use first selection, got %s", tasks.sourceQuery)
	}
	m.setFocus("projects")
	m, _ = m.update(key("j"))
	tasks = m.viewByID("tasks")
	if !strings.Contains(tasks.sourceQuery, "projects/b") {
		t.Fatalf("live bind after move, got %s", tasks.sourceQuery)
	}
	m.width, m.height = 100, 24
	plain := strings.ReplaceAll(m.View(), "\x1b", "")
	// Sightline should name the driving selection even after ANSI stripping of CSI guts.
	head := m.paneHeader(tasks, false)
	if !strings.Contains(head, sightArrow) || !strings.Contains(head, "Beta") {
		t.Fatalf("binding sightline: %q", head)
	}
	if !strings.Contains(plain, "Beta") {
		t.Fatalf("driver title missing from view")
	}
}

func TestEnterOnDriverFocusesBound(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	left := m.viewByID("projects")
	m, _ = m.update(queryMsg{
		gen: left.gen, pane: left.id, typeName: "project",
		objects: []api.Object{{ID: "projects/a", Title: "Alpha", Type: "project"}},
	})
	m.setFocus("projects")
	m, cmd := m.update(key("enter"))
	if cmd != nil {
		t.Fatal("enter on driver should move focus, not open a note")
	}
	if m.focus != "tasks" {
		t.Fatalf("focus %s", m.focus)
	}
}

func TestFocusMovementAndZoom(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m.width, m.height = 80, 24
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	m.setFocus("projects")
	m.recomputeRegions()
	m, _ = m.update(key("ctrl+l"))
	if m.focus == "projects" {
		t.Fatalf("ctrl-l should leave projects, got %s", m.focus)
	}
	m.setFocus("projects")
	m, _ = m.update(key("tab"))
	if m.focus == "projects" {
		t.Fatalf("tab should cycle within main, got %s", m.focus)
	}
	m.setFocus("projects")
	m, _ = m.update(key("ctrl+w"))
	m, _ = m.update(key("l"))
	if m.focus == "projects" {
		t.Fatalf("ctrl-w l should leave projects, got %s", m.focus)
	}
	m.setFocus("projects")
	m, _ = m.update(key("ctrl+w"))
	m, _ = m.update(key("o"))
	if !m.zoomed {
		t.Fatal("zoom")
	}
	m, _ = m.update(key("ctrl+w"))
	m, _ = m.update(key("="))
	if m.zoomed {
		t.Fatal("restore")
	}
}

func TestReloadKeepsLastGoodOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	good := `
[views.ok]
type = "item"
kind = "list"
`
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	m := loadBrowse(t)
	m.cfgPath = path
	m, err := m.reloadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if m.cfg.Views["ok"].Type != "item" {
		t.Fatalf("%+v", m.cfg.Views["ok"])
	}
	bad := `[[types.item.layouts]]
name = "Board"
kind = "board"
group_by = "state"
`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	m2, _ := m.onConfigEditorDone(editorDoneMsg{config: true})
	if m2.cfg == nil || m2.cfg.Views["ok"].Type != "item" {
		t.Fatalf("should keep last good, got %+v", m2.cfg)
	}
	if !strings.Contains(m2.status, "keeping last good") {
		t.Fatalf("status %q", m2.status)
	}
	if !strings.Contains(m2.errMsg, ":") {
		t.Fatalf("want line number, got %q", m2.errMsg)
	}
}

func TestSaveViewWritesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	m := loadBrowse(t)
	m.cfgPath = path
	focusMain(&m)
	m, _ = m.update(key("S"))
	if m.overlay != overlaySave {
		t.Fatal("save overlay")
	}
	for _, r := range "my-board" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "[views.my-board]") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, `type = "project"`) || !strings.Contains(s, `kind = "`) {
		t.Fatalf("want type and kind, got:\n%s", s)
	}
	if strings.Contains(s, "source") || strings.Contains(s, "layout =") {
		t.Fatalf("want flat renderer keys, got:\n%s", s)
	}
	if m.cfg == nil || m.cfg.Views["my-board"].Type != "project" {
		t.Fatalf("cfg not reloaded %+v", m.cfg)
	}
}

func TestNamedQueryBoardAndDateViews(t *testing.T) {
	cfg := &config.File{
		Views: map[string]config.View{
			"board": {Query: "open", Present: config.Present{Kind: kindBoard, GroupBy: "status"}},
			"when":  {Query: "open", Present: config.Present{Kind: kindDate, GroupBy: "due", Granularity: layout.GranWeek}},
		},
	}
	m := loadNamedView(t, cfg, "board")
	if !m.p().sourceIsQuery || m.p().spec.Kind != kindBoard || m.p().groupBy != "status" {
		t.Fatalf("query board pane %+v spec %+v", m.p(), m.p().spec)
	}
	m, _ = m.loadNamedView("when")
	if !m.p().sourceIsQuery || m.p().spec.Kind != kindDate || m.p().granularity != layout.GranWeek {
		t.Fatalf("query date pane %+v spec %+v", m.p(), m.p().spec)
	}
}

func TestInlineLayoutPanels(t *testing.T) {
	cfg := &config.File{
		Layouts: map[string]config.Layout{
			"ws": {
				Split: "row",
				Panels: []config.Panel{
					{Type: "project", ID: "projects", Present: config.Present{Kind: kindBoard, GroupBy: "status"}},
					{Query: "open", Bind: "projects", Present: config.Present{Kind: kindList}},
				},
			},
		},
	}
	m := NewModel(Options{Config: cfg})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("ws")
	if len(m.panes) != 2 {
		t.Fatalf("panes %d", len(m.panes))
	}
	if m.panes[0].id != "projects" || m.panes[0].spec.Kind != kindBoard || m.panes[0].typeName != "project" {
		t.Fatalf("left id=%s kind=%s type=%s", m.panes[0].id, m.panes[0].spec.Kind, m.panes[0].typeName)
	}
	if m.panes[1].bind != "projects" || !m.panes[1].sourceIsQuery {
		t.Fatalf("right bind=%s query=%v", m.panes[1].bind, m.panes[1].sourceIsQuery)
	}
	if m.tree.split != "row" || m.tree.kids[0].id != "projects" {
		t.Fatalf("tree %+v", m.tree)
	}
}

func TestTypePickerListsSchemaTypes(t *testing.T) {
	m := loadBrowse(t)
	out := m.View()
	if strings.Contains(out, "Browse") {
		t.Fatalf("sidebar chrome: %s", out)
	}
	m, _ = pressLeader(m, "t")
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatal("type picker")
	}
	if len(m.pickerHit) == 0 {
		t.Fatal("expected types")
	}
	name := m.pickerHit[0].Type
	m, _ = m.update(key("enter"))
	p := m.p()
	if p.sourceIsQuery || p.typeName != name || p.sourceBase != "type:"+name {
		t.Fatalf("type picker should open as Type, got type=%s base=%s query=%v", p.typeName, p.sourceBase, p.sourceIsQuery)
	}
	if p.sourceQuery != "type:"+name {
		t.Fatalf("sourceQuery %s", p.sourceQuery)
	}
}
