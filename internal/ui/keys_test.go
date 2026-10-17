package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/layout"
)

func TestKeymapHelpRenders(t *testing.T) {
	if len(Keymap) < 10 {
		t.Fatalf("keymap too small: %d", len(Keymap))
	}
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(key("?"))
	if m.overlay != overlayHelp {
		t.Fatal("help")
	}
	out := m.View()
	for _, want := range []string{"ctrl+c", "next named layout", "daily", "fuzzy search"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q in %s", want, out)
		}
	}
}

func TestTypeAndQueryPickers(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("t"))
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatalf("types overlay %+v", m.overlay)
	}
	if len(m.pickerHit) < 2 {
		t.Fatalf("types %v", labelsOf(m.pickerHit))
	}
	m, _ = m.update(runeKey('p'))
	m, _ = m.update(runeKey('r'))
	if len(m.pickerHit) == 0 || m.pickerHit[0].Label != "project" {
		t.Fatalf("fuzzy types %v", labelsOf(m.pickerHit))
	}
	m, cmd := m.update(key("enter"))
	if cmd == nil {
		t.Fatal("load type")
	}
	if m.sourceQuery != "type:project" || m.viewKind != kindList {
		t.Fatalf("source=%s kind=%s group=%s", m.sourceQuery, m.viewKind, m.groupBy)
	}

	m, _ = m.update(key("q"))
	if m.overlay != overlayPicker || m.pickerKind != pickerQueries {
		t.Fatal("query picker")
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayNone {
		t.Fatal("esc closes")
	}
}

func TestLayoutCycleAndPreview(t *testing.T) {
	m := loadProjectBoard(t)
	if m.viewKind != kindList || m.currentLayoutName() != "List" {
		t.Fatalf("zero-config default list, got %s %s", m.viewKind, m.currentLayoutName())
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindBoard || m.groupBy != "status" || m.currentLayoutName() != "Board" {
		t.Fatalf("board %+v name=%s", m.spec, m.currentLayoutName())
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindDate || m.groupBy != "due" {
		t.Fatalf("date %s %+v", m.viewKind, m.spec)
	}
	m, _ = m.update(key("."))
	if m.granularity != layout.GranWeek {
		t.Fatalf("gran %s", m.granularity)
	}
	m, _ = m.update(key("g"))
	m, _ = m.update(key("L"))
	if m.viewKind != kindBoard {
		t.Fatalf("gL previous %s", m.viewKind)
	}
	m, _ = m.update(key("p"))
	if !m.previewOn {
		t.Fatal("preview")
	}
}

func TestLayoutPickerAndJump(t *testing.T) {
	m := loadProjectBoard(t)
	m, _ = m.update(key("g"))
	m, _ = m.update(key("l"))
	if m.overlay != overlayPicker || m.pickerKind != pickerLayouts {
		t.Fatalf("gl picker overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	if len(m.pickerHit) != 3 {
		t.Fatalf("layouts %v", labelsOf(m.pickerHit))
	}
	m.pickerCur = 1
	m, _ = m.update(key("enter"))
	if m.viewKind != kindBoard || m.currentLayoutName() != "Board" {
		t.Fatalf("picked %+v %s", m.spec, m.currentLayoutName())
	}

	m, _ = m.update(key(" "))
	m, _ = m.update(key("l"))
	if m.overlay != overlayPicker || m.pickerKind != pickerLayouts {
		t.Fatal("space l picker")
	}
	m, _ = m.update(key("esc"))

	m, _ = m.update(key("1"))
	if m.currentLayoutName() != "List" || m.layoutIdx != 0 {
		t.Fatalf("jump 1 %s %d", m.currentLayoutName(), m.layoutIdx)
	}
	m, _ = m.update(key("3"))
	if m.currentLayoutName() != "Date" || m.viewKind != kindDate {
		t.Fatalf("jump 3 %s %s", m.currentLayoutName(), m.viewKind)
	}
	m, _ = m.update(key("9"))
	if m.status != "no layout 9" {
		t.Fatalf("status %q", m.status)
	}
}

func TestLayoutFilterRefetch(t *testing.T) {
	m := NewModel(Options{Config: &config.File{
		Types: map[string]config.Source{
			"project": {Layouts: []config.Layout{
				{Name: "All", Kind: kindList},
				{Name: "Active", Kind: kindList, Filter: ".status==active"},
			}},
		},
	}})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	for i, n := range m.nav {
		if n.Type == "project" {
			m.navCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	m, _ = m.update(queryMsg{gen: m.gen, typeName: "project", objects: nil})
	if m.sourceQuery != "type:project" {
		t.Fatalf("base %s", m.sourceQuery)
	}
	m, cmd := m.update(key("L"))
	if m.sourceQuery != "type:project .status==active" {
		t.Fatalf("filtered %s", m.sourceQuery)
	}
	if m.currentLayoutName() != "Active" {
		t.Fatalf("name %s", m.currentLayoutName())
	}
	if cmd == nil {
		t.Fatal("expected refetch")
	}
}

func TestDailyHotkeyWithoutSession(t *testing.T) {
	m := loadBrowse(t)
	src := m.sourceQuery
	m, cmd := m.update(key("d"))
	if cmd != nil {
		t.Fatal("no session daily: must not fetch or load a type")
	}
	if m.sourceQuery != src {
		t.Fatalf("source changed to %s", m.sourceQuery)
	}
	if !strings.Contains(m.status, "no daily note in session") {
		t.Fatalf("status %q", m.status)
	}
}

func TestDailyUsesSessionWhenPresent(t *testing.T) {
	m := loadBrowse(t)
	m.daily = &api.DailyInfo{Directory: "journal", TodayID: "2026-10-15", TodayPath: "journal/2026-10-15.md", TodayExists: true}
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("read today")
	}
	if !strings.Contains(m.status, "2026-10-15") {
		t.Fatalf("status %s", m.status)
	}
}

func TestDailySessionMissingDoesNotOpenType(t *testing.T) {
	m := loadBrowse(t)
	src := m.sourceQuery
	m.daily = &api.DailyInfo{TodayID: "2026-10-15", TodayExists: false}
	m, cmd := m.update(key("d"))
	if cmd != nil {
		t.Fatal("missing file must not load a type")
	}
	if m.sourceQuery != src {
		t.Fatalf("source changed to %s", m.sourceQuery)
	}
	if !strings.Contains(m.status, "2026-10-15") {
		t.Fatalf("status %s", m.status)
	}
}

func TestObjectsCorpusFillsNotesPicker(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("ctrl+p"))
	m, cmd := m.update(objectsMsg{
		objects: []api.Object{
			{ID: "people/freya", Title: "Freya", Type: "person", FilePath: "people/freya.md"},
		},
		offset: 0, nextOffset: 1, hasMore: false, total: 1,
	})
	if cmd != nil {
		t.Fatal("no more pages")
	}
	if _, ok := m.noteIndex["people/freya"]; !ok {
		t.Fatal("index")
	}
	found := false
	for _, it := range m.pickerHit {
		if it.Label == "Freya" {
			found = true
		}
	}
	if !found {
		t.Fatalf("picker %v", labelsOf(m.pickerHit))
	}
}

func TestObjectsFallbackPrefetchesTypes(t *testing.T) {
	m := loadBrowse(t)
	_, cmd := m.update(objectsMsg{fallback: true})
	if cmd == nil {
		t.Fatal("expected type prefetch")
	}
}

func TestNotesPickerFuzzy(t *testing.T) {
	m := loadBrowse(t)
	m.mergeNotes([]api.Object{
		{ID: "projects/a", Title: "Alpha"},
		{ID: "people/freya", Title: "Freya"},
	})
	m, _ = m.update(key("ctrl+p"))
	if m.overlay != overlayPicker || m.pickerKind != pickerNotes {
		t.Fatal("notes picker")
	}
	m, _ = m.update(runeKey('f'))
	m, _ = m.update(runeKey('r'))
	if len(m.pickerHit) == 0 || m.pickerHit[0].Label != "Freya" {
		t.Fatalf("%v", labelsOf(m.pickerHit))
	}
}

func TestSpaceLeader(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key(" "))
	if m.overlay != overlayLeader {
		t.Fatal("leader")
	}
	m, _ = m.update(key("t"))
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatal("space t")
	}
}

func TestCtrlDMovesHalfPage(t *testing.T) {
	m := loadBrowse(t)
	m.height = 24
	m.pane = paneMain
	m.objects = nil
	for i := 0; i < 40; i++ {
		m.objects = append(m.objects, api.Object{ID: string(rune('a'+(i%26))) + string(rune('0'+i/26)), Title: "N"})
	}
	m.applyFilterAndGroup()
	m, _ = m.update(key("ctrl+d"))
	if m.listCur == 0 {
		t.Fatal("expected page down")
	}
}

func TestEditorFromBrowseUsesSelectedRow(t *testing.T) {
	m := loadBrowse(t)
	m.pane = paneMain
	m.vaultPath = "/vault"
	m.editor = "true"
	m.editorMode = "gui"
	_, cmd := m.update(key("e"))
	if cmd == nil {
		t.Fatal("editor cmd")
	}
}

func TestLayoutCycleNameAgnostic(t *testing.T) {
	schema := api.Schema{
		Types: map[string]api.TypeSchema{
			"item": {
				Name:       "item",
				NameField:  "label",
				FieldOrder: []string{"label", "xyz", "when"},
				Fields: map[string]api.FieldSchema{
					"label": {Type: "string"},
					"xyz":   {Type: "enum", Values: []string{"a", "b"}},
					"when":  {Type: "date"},
				},
			},
		},
	}
	m := NewModel(Options{Config: &config.File{}})
	m, _ = m.update(schemaMsg{schema: ptrSchema(schema)})
	m, _ = m.update(queryMsg{
		gen: m.gen, typeName: "item",
		objects: []api.Object{
			{ID: "items/a", Title: "A", Type: "item", Fields: map[string]any{"xyz": "a", "when": "2026-10-01"}},
		},
	})
	if m.viewKind != kindList || m.currentLayoutName() != "List" {
		t.Fatalf("default %s %s", m.viewKind, m.currentLayoutName())
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindBoard || m.groupBy != "xyz" || m.currentLayoutName() != "Board" {
		t.Fatalf("board %+v name=%s", m.spec, m.currentLayoutName())
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindDate || m.groupBy != "when" || m.currentLayoutName() != "Date" {
		t.Fatalf("date %+v name=%s", m.spec, m.currentLayoutName())
	}
}

func TestConfigTypeLayoutUsed(t *testing.T) {
	m := NewModel(Options{Config: &config.File{
		Types: map[string]config.Source{
			"person": {Layouts: []config.Layout{
				{Name: "People", Kind: "list", Sort: "name"},
			}},
		},
	}})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.update(queryMsg{gen: m.gen, typeName: "person", objects: nil})
	if m.viewKind != kindList || m.currentLayoutName() != "People" {
		t.Fatalf("%s %s", m.viewKind, m.currentLayoutName())
	}
}

func TestConfigWarningsOnSchema(t *testing.T) {
	m := NewModel(Options{Config: &config.File{
		Types: map[string]config.Source{
			"ghost": {Layouts: []config.Layout{{Name: "List", Kind: "list"}}},
		},
	}})
	m.width, m.height = 80, 24
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if len(m.cfgWarnings) == 0 {
		t.Fatal("expected warnings")
	}
	out := m.View()
	if !strings.Contains(out, "unknown type") {
		t.Fatalf("footer missing warning: %s", out)
	}
}

func loadProjectBoard(t *testing.T) Model {
	t.Helper()
	m := loadBrowse(t)
	for i, n := range m.nav {
		if n.Type == "project" {
			m.navCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	m, _ = m.update(queryMsg{
		gen: m.gen, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project", Fields: map[string]any{"status": "active", "due": "2026-10-01", "owner": "people/x"}},
			{ID: "projects/b", Title: "Beta", Type: "project", Fields: map[string]any{"status": "idea", "due": "2026-09-01"}},
		},
	})
	return m
}

func labelsOf(items []pickItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}
