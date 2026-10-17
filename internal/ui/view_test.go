package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
	tea "github.com/charmbracelet/bubbletea"
)

func TestViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 80
	m.height = 24
	out := m.View()
	if out == "" || len(out) < 20 {
		t.Fatalf("empty view %q", out)
	}
	m.overlay = overlayHelp
	if help := m.View(); help == "" {
		t.Fatal("help")
	}
}

func TestNoteViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 100
	m.height = 30
	m, _ = m.update(readMsg{
		gen:    m.gen,
		result: sampleRead(),
	})
	out := m.View()
	if out == "" {
		t.Fatal("empty")
	}
}

func TestSidebarClipKeepsCursorVisible(t *testing.T) {
	start, end, above, below := sidebarClip(20, 0, 8)
	if start != 0 || !below || above {
		t.Fatalf("top start=%d end=%d above=%v below=%v", start, end, above, below)
	}
	if end-start >= 8 {
		t.Fatalf("indicators should shrink inner, got %d", end-start)
	}

	start, end, above, below = sidebarClip(20, 19, 8)
	if end != 20 || !above || below {
		t.Fatalf("bottom start=%d end=%d above=%v below=%v", start, end, above, below)
	}
	if 19 < start || 19 >= end {
		t.Fatalf("cursor 19 not in [%d,%d)", start, end)
	}

	start, end, above, below = sidebarClip(20, 10, 8)
	if !above || !below {
		t.Fatalf("mid above=%v below=%v", above, below)
	}
	if 10 < start || 10 >= end {
		t.Fatalf("cursor 10 not in [%d,%d)", start, end)
	}
}

func manyTypesModel(t *testing.T, n int) Model {
	t.Helper()
	types := map[string]api.TypeSchema{}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("type-%02d", i)
		types[name] = api.TypeSchema{Name: name, NameField: "name"}
	}
	m := NewModel(Options{Config: &config.File{}})
	m, _ = m.update(schemaMsg{schema: &api.Schema{Version: 1, Types: types}})
	m, _ = m.update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return m
}

func TestSidebarScrollsWithMotionAndResize(t *testing.T) {
	m := manyTypesModel(t, 30)
	m.pane = paneSidebar
	if len(m.nav) < 30 {
		t.Fatalf("nav %d", len(m.nav))
	}
	first := m.nav[0].Label
	last := m.nav[len(m.nav)-1].Label
	const sideH = 8

	side := m.viewSidebar(sideH)
	if !strings.Contains(side, first) {
		t.Fatalf("top missing %s in %s", first, side)
	}
	if strings.Contains(side, last) {
		t.Fatalf("top should not show %s in %s", last, side)
	}
	if !strings.Contains(side, "▼") {
		t.Fatalf("expected below indicator: %s", side)
	}

	m, _ = m.update(key("G"))
	if m.navCur != len(m.nav)-1 {
		t.Fatalf("G navCur=%d", m.navCur)
	}
	side = m.viewSidebar(sideH)
	if !strings.Contains(side, last) {
		t.Fatalf("G missing %s in %s", last, side)
	}
	if strings.Contains(side, first) {
		t.Fatalf("G should not show %s in %s", first, side)
	}
	if !strings.Contains(side, "▲") {
		t.Fatalf("expected above indicator: %s", side)
	}

	m, _ = m.update(key("g"))
	m, _ = m.update(key("g"))
	if m.navCur != 0 {
		t.Fatalf("gg %d", m.navCur)
	}
	side = m.viewSidebar(sideH)
	if !strings.Contains(side, first) {
		t.Fatalf("gg missing %s", first)
	}

	m, _ = m.update(key("ctrl+d"))
	sel := m.nav[m.navCur].Label
	side = m.viewSidebar(sideH)
	if !strings.Contains(side, sel) {
		t.Fatalf("ctrl-d cursor %s not visible in %s", sel, side)
	}

	m, _ = m.update(key("j"))
	sel = m.nav[m.navCur].Label
	side = m.viewSidebar(sideH)
	if !strings.Contains(side, sel) {
		t.Fatalf("j cursor %s not visible in %s", sel, side)
	}

	m, _ = m.update(key("G"))
	m, _ = m.update(tea.WindowSizeMsg{Width: 80, Height: 80})
	side = m.viewSidebar(60)
	if !strings.Contains(side, last) {
		t.Fatalf("resize missing last %s", last)
	}
	if !strings.Contains(side, first) {
		t.Fatalf("tall resize should show first %s", first)
	}
	if strings.Contains(side, "▲") || strings.Contains(side, "▼") {
		t.Fatalf("tall viewport should not need arrows: %s", side)
	}
}

func TestRowCellsUsesNamedField(t *testing.T) {
	obj := api.Object{ID: "a", Title: "Title", Type: "item", Fields: map[string]any{"xyz": "X", "name": "N"}}
	cells := rowCells(obj, []string{"id", "xyz", "name"})
	if cells[0] != "a" || cells[1] != "X" || cells[2] != "N" {
		t.Fatalf("%v", cells)
	}
	obj2 := api.Object{ID: "b", Title: "T", Fields: map[string]any{"title": "FieldTitle"}}
	cells = rowCells(obj2, []string{"id", "title"})
	if cells[1] != "FieldTitle" {
		t.Fatalf("title column must read the field, not obj.Title: %v", cells)
	}
}

func TestNoteHeadingUsesSchemaNameField(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m.schema = api.Schema{Types: map[string]api.TypeSchema{
		"item": {Name: "item", NameField: "xyz"},
	}}
	m.noteIndex = map[string]api.Object{
		"items/a": {ID: "items/a", Type: "item", Title: "Fallback"},
	}
	doc := frontmatter.Doc{Fields: map[string]string{"xyz": "FromXYZ", "name": "FromName"}}
	if got := m.headingFromDoc("items/a", doc); got != "FromXYZ" {
		t.Fatalf("got %q", got)
	}
	other := frontmatter.Doc{Fields: map[string]string{"name": "FromName", "status": "open"}}
	if got := m.headingFromDoc("items/a", other); got != "Fallback" {
		t.Fatalf("empty name_field should use Title, not name: %q", got)
	}
}

func sampleRead() *api.ReadResult {
	content := "---\nname: Alpha\nstatus: active\n---\n\nSee [[people/freya]].\n"
	return &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  content,
		Links: []api.ReadLink{{
			ByteStart: 38, ByteEnd: 54, Text: "[[people/freya]]",
			Target: "people/freya", Status: api.LinkResolved, ObjectID: "people/freya",
		}},
		Backlinks: []api.BacklinkGroup{{Source: "people/odin"}},
	}
}
