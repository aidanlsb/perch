package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
)

func TestViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 80
	m.height = 24
	out := m.View()
	if out == "" || len(out) < 20 {
		t.Fatalf("empty view %q", out)
	}
	if strings.Contains(out, "Browse") {
		t.Fatalf("sidebar chrome: %s", out)
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

func TestDatePickerRendersMonth(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m.daily = &api.DailyInfo{TodayID: "2026-10-15"}
	m, _ = leader(m, "sd")
	out := m.View()
	if !strings.Contains(out, "2026-10-15") {
		t.Fatalf("missing selected date: %s", out)
	}
	if !strings.Contains(out, "October") && !strings.Contains(out, "Su Mo") {
		t.Fatalf("missing calendar: %s", out)
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
