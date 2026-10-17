package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
)

func nestedLayoutCfg() *config.File {
	return &config.File{
		Views: map[string]config.View{
			"projects": {Source: "project", Layout: config.Present{Kind: kindList}},
			"tasks": {
				Source: "type:project .id=={{selected}}",
				Layout: config.Present{Kind: kindList},
			},
			"notes": {Source: "person", Layout: config.Present{Kind: kindList}},
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
	if m.focus == "projects" || m.focus == focusSidebar {
		t.Fatalf("ctrl-l should leave projects, got %s", m.focus)
	}
	m.setFocus("projects")
	m, _ = m.update(key("tab"))
	if m.focus == "projects" {
		t.Fatal("tab should cycle")
	}
	m.setFocus("projects")
	m, _ = m.update(key("ctrl+w"))
	m, _ = m.update(key("l"))
	if m.focus == "projects" || m.focus == focusSidebar {
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
source = "item"
layout = { kind = "list" }
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
	if m.cfg.Views["ok"].Source != "item" {
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
	if m2.cfg == nil || m2.cfg.Views["ok"].Source != "item" {
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
	if !strings.Contains(s, "source") {
		t.Fatalf("missing source:\n%s", s)
	}
	if m.cfg == nil || m.cfg.Views["my-board"].Source == "" {
		t.Fatalf("cfg not reloaded %+v", m.cfg)
	}
}

func TestSidebarCategoriesCollapsed(t *testing.T) {
	m := loadBrowse(t)
	if m.browseOpen || m.viewsOpen {
		t.Fatal("categories should start collapsed")
	}
	out := m.viewSidebar(12)
	if !strings.Contains(out, "Browse") || !strings.Contains(out, "Views") {
		t.Fatalf("%s", out)
	}
	if strings.Contains(out, "project") {
		t.Fatalf("types should be hidden until expanded: %s", out)
	}
	m.setFocus(focusSidebar)
	m, _ = m.update(key("enter"))
	if !m.browseOpen {
		t.Fatal("enter expands Browse")
	}
}
