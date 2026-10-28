package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
)

func TestStartupHomeWhenNoViewsOrLayouts(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m.width, m.height = 100, 28
	m, _ = m.update(schemaMsg{
		schema: ptrSchema(sampleSchema()),
		session: &api.Session{
			VaultPath: "/vaults/aidan", VaultName: "aidan",
			Daily: &api.DailyInfo{TodayID: "2026-10-28", TodayExists: false},
		},
	})
	if m.screen != kindHome || m.overlay != overlayNone {
		t.Fatalf("screen=%s overlay=%v", m.screen, m.overlay)
	}
	out := m.View()
	for _, want := range []string{"aidan", "/vaults/aidan", "views & layouts", "types", "saved queries", "will be created"} {
		if !strings.Contains(out, want) {
			t.Fatalf("home missing %q in\n%s", want, out)
		}
	}
}

func TestStartupSeedsHomeUnderFirstView(t *testing.T) {
	f, err := config.Parse("config.toml", `
[views.zebra]
type = "person"
kind = "list"
`)
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel(Options{Config: f})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if m.screen != kindBrowse || m.currentView != "zebra" {
		t.Fatalf("screen=%s view=%s", m.screen, m.currentView)
	}
	if len(m.hist) < 2 || m.hist[0].Screen != kindHome {
		t.Fatalf("home not under view: %+v", m.hist)
	}
	m, _ = m.update(key("esc"))
	if m.screen != kindHome {
		t.Fatalf("back from first view should be home, screen=%s", m.screen)
	}
}

func TestHomeSectionNavAndEnter(t *testing.T) {
	m := NewModel(Options{Config: namedViewCfg()})
	m.width, m.height = 120, 30
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.goHome()
	if m.screen != kindHome {
		t.Fatal(m.screen)
	}
	if m.homeSec != int(homeViews) {
		t.Fatalf("sec %d", m.homeSec)
	}
	m, _ = m.update(key("l"))
	if m.homeSec != int(homeTypes) {
		t.Fatalf("l -> types, sec=%d", m.homeSec)
	}
	m, _ = m.update(key("tab"))
	if m.homeSec != int(homeQueries) {
		t.Fatalf("tab -> queries, sec=%d", m.homeSec)
	}
	m, _ = m.update(key("h"))
	if m.homeSec != int(homeTypes) {
		t.Fatalf("h -> types, sec=%d", m.homeSec)
	}
	// types are sorted: person, project (section omitted)
	if got := m.homeItems(homeTypes); len(got) < 2 || got[0].Label != "person" {
		t.Fatalf("types %+v", got)
	}
	m, cmd := m.update(key("enter"))
	if cmd == nil {
		t.Fatal("enter type should query")
	}
	if m.screen != kindBrowse || m.p().typeName != "person" {
		t.Fatalf("screen=%s type=%s", m.screen, m.p().typeName)
	}
}

func TestHomeEnterLayout(t *testing.T) {
	f, err := config.Parse("config.toml", `
[views.zebra]
type = "person"
kind = "list"

[layouts.workspace]
split = "row"
panels = [ { view = "zebra" } ]
`)
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel(Options{Config: f})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.goHome()
	m.homeSec = int(homeViews)
	items := m.homeItems(homeViews)
	idx := -1
	for i, it := range items {
		if it.Kind == "layout" && it.ID == "workspace" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("layout missing from home: %+v", items)
	}
	m.homeCur[homeViews] = idx
	m, cmd := m.update(key("enter"))
	if m.currentLayout != "workspace" {
		t.Fatalf("enter layout, layout=%q view=%q cmd=%v", m.currentLayout, m.currentView, cmd)
	}
}

func TestHomeEnterQueryAndView(t *testing.T) {
	m := NewModel(Options{Config: namedViewCfg()})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.goHome()
	m.homeSec = int(homeQueries)
	m.homeCur[homeQueries] = 0
	items := m.homeItems(homeQueries)
	if len(items) == 0 {
		t.Fatal("queries")
	}
	m, cmd := m.update(key("enter"))
	if items[0].ID == "need" {
		if m.overlay != overlayArgs {
			t.Fatalf("need should prompt args, overlay=%v cmd=%v", m.overlay, cmd)
		}
	} else if cmd == nil && m.overlay == overlayNone {
		if m.p().sourceBase != items[0].ID && m.p().sourceLabel != items[0].ID {
			t.Fatalf("query pane %+v item=%+v", m.p(), items[0])
		}
	}

	m, _ = m.goHome()
	m.homeSec = int(homeViews)
	m.homeCur[homeViews] = 0
	views := m.homeItems(homeViews)
	if len(views) == 0 {
		t.Fatal("views")
	}
	m, cmd = m.update(key("enter"))
	if cmd == nil && m.currentView == "" && m.currentLayout == "" {
		t.Fatalf("enter view did nothing, item=%+v", views[0])
	}
}

func TestSpaceHAndGHJumpHome(t *testing.T) {
	m := loadBrowse(t)
	if m.screen != kindBrowse {
		t.Fatal(m.screen)
	}
	m, _ = pressLeader(m, "h")
	if m.screen != kindHome {
		t.Fatalf("space h -> %s", m.screen)
	}
	m, _ = m.loadType("project")
	m, _ = m.update(key("g"))
	m, _ = m.update(key("h"))
	if m.screen != kindHome {
		t.Fatalf("gh -> %s", m.screen)
	}
}

func TestNavPickerListsHomeFirst(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, " ")
	if len(m.pickerHit) == 0 || m.pickerHit[0].ID != "nav:home" {
		t.Fatalf("home first: %v", labelsOf(m.pickerHit))
	}
}

func TestHomeShortcuts(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m, _ = m.update(schemaMsg{
		schema:  ptrSchema(sampleSchema()),
		session: &api.Session{Daily: &api.DailyInfo{TodayID: "2026-10-28", TodayExists: true}},
	})
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("d should open today")
	}
	m, _ = m.goHome()
	m, _ = m.update(key("f"))
	if m.overlay != overlayPicker || m.pickerKind != pickerNotes {
		t.Fatalf("f notes overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.goHome()
	m, _ = m.update(key("s"))
	if m.overlay != overlayPicker || m.pickerKind != pickerSearch {
		t.Fatalf("s search")
	}
	m, _ = m.goHome()
	m, _ = m.update(key("/"))
	if m.overlay != overlayPicker || m.pickerKind != pickerSearch {
		t.Fatalf("/ search on home, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.goHome()
	m, _ = m.update(key(":"))
	if m.overlay != overlayRQL {
		t.Fatal(":")
	}
	m, _ = m.goHome()
	m, _ = m.update(key("c"))
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatalf("c new overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.goHome()
	m, _ = m.update(key("a"))
	if m.overlay != overlayPicker || m.pickerKind != pickerActions {
		t.Fatal("a")
	}
}

func TestHomeRendersTypeCountsAfterCorpus(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m.width, m.height = 100, 28
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.update(objectsMsg{objects: []api.Object{
		{ID: "projects/a", Type: "project", Title: "A"},
		{ID: "projects/b", Type: "project", Title: "B"},
		{ID: "people/x", Type: "person", Title: "X"},
	}})
	m.homeSec = int(homeTypes)
	out := m.View()
	if !strings.Contains(out, "project") {
		t.Fatalf("types card missing project:\n%s", out)
	}
}
