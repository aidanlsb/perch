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
	m.width, m.height = 100, 48
	m, _ = m.update(key("?"))
	if m.overlay != overlayHelp {
		t.Fatal("help")
	}
	out := m.View()
	for _, want := range []string{"ctrl+c", "ctrl-h/j/k/l", "daily", "fuzzy search"} {
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
	if m.p().sourceQuery != "type:project" || m.p().spec.Kind != kindList {
		t.Fatalf("source=%s kind=%s group=%s", m.p().sourceQuery, m.p().spec.Kind, m.p().groupBy)
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

func TestBrowseTypeIsPlainList(t *testing.T) {
	m := loadProjectBoard(t)
	if m.p().spec.Kind != kindList || m.p().groupBy != "" {
		t.Fatalf("browse must be a plain list, got %+v", m.p().spec)
	}
	m, _ = m.update(key("L"))
	if m.p().spec.Kind != kindList {
		t.Fatalf("L must not synthesize a board, got %s", m.p().spec.Kind)
	}
	m, _ = m.update(key("p"))
	if !m.previewOn {
		t.Fatal("preview")
	}
}

func TestDateGranularityOnNamedView(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "week")
	if m.p().spec.Kind != kindDate {
		t.Fatalf("%s", m.p().spec.Kind)
	}
	m.setFocus(m.p().id)
	m, _ = m.update(key("."))
	if m.p().granularity != layout.GranWeek {
		t.Fatalf("gran %s", m.p().granularity)
	}
}

func TestViewsPicker(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "projects")
	m, _ = m.update(key("g"))
	m, _ = m.update(key("l"))
	if m.overlay != overlayPicker || m.pickerKind != pickerViews {
		t.Fatalf("gl picker overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	if len(m.pickerHit) < 2 {
		t.Fatalf("views %v", labelsOf(m.pickerHit))
	}
	m, _ = m.update(key("esc"))
	m, _ = m.update(key(" "))
	m, _ = m.update(key("l"))
	if m.overlay != overlayPicker || m.pickerKind != pickerViews {
		t.Fatal("space l picker")
	}
}

func TestNamedViewFilter(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "open")
	if m.p().sourceQuery != "type:project .status==active" && !strings.Contains(m.p().spec.Filter, "status") {
		if m.p().spec.Filter != ".status==active" {
			t.Fatalf("filter %q query %s", m.p().spec.Filter, m.p().sourceQuery)
		}
	}
}

func TestDailyHotkeyWithoutSession(t *testing.T) {
	m := loadBrowse(t)
	src := m.p().sourceQuery
	m, cmd := m.update(key("d"))
	if cmd != nil {
		t.Fatal("no session daily: must not fetch or load a type")
	}
	if m.p().sourceQuery != src {
		t.Fatalf("source changed to %s", m.p().sourceQuery)
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
	src := m.p().sourceQuery
	m.daily = &api.DailyInfo{TodayID: "2026-10-15", TodayExists: false}
	m, cmd := m.update(key("d"))
	if cmd != nil {
		t.Fatal("missing file must not load a type")
	}
	if m.p().sourceQuery != src {
		t.Fatalf("source changed to %s", m.p().sourceQuery)
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
	focusMain(&m)
	p := m.p()
	p.objects = nil
	for i := 0; i < 40; i++ {
		p.objects = append(p.objects, api.Object{ID: string(rune('a'+(i%26))) + string(rune('0'+i/26)), Title: "N"})
	}
	m.applyFilterAndGroup()
	m, _ = m.update(key("ctrl+d"))
	if m.p().listCur == 0 {
		t.Fatal("expected page down")
	}
}

func TestEditorFromBrowseUsesSelectedRow(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m.vaultPath = "/vault"
	m.editor = "true"
	m.editorMode = "gui"
	_, cmd := m.update(key("e"))
	if cmd == nil {
		t.Fatal("editor cmd")
	}
}

func TestConfigNamedViewUsed(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "people")
	if m.p().spec.Kind != kindList || m.currentLayoutName() != "people" {
		t.Fatalf("%s %s", m.p().spec.Kind, m.currentLayoutName())
	}
	if m.p().spec.Sort != "name" {
		t.Fatalf("sort %s", m.p().spec.Sort)
	}
}

func TestConfigWarningsOnSchema(t *testing.T) {
	m := NewModel(Options{Config: &config.File{
		Views: map[string]config.View{
			"ghost": {Source: "ghost", Layout: config.Present{Kind: kindList}},
		},
	}})
	m.width, m.height = 80, 24
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if len(m.cfgWarnings) == 0 {
		t.Fatal("expected warnings")
	}
	out := m.View()
	if !strings.Contains(out, "unknown source") {
		t.Fatalf("footer missing warning: %s", out)
	}
}

func loadProjectBoard(t *testing.T) Model {
	t.Helper()
	m := loadBrowse(t)
	m, _ = m.loadType("project")
	m, _ = m.update(queryMsg{
		gen: m.p().gen, pane: m.p().id, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project", Fields: map[string]any{"status": "active", "due": "2026-10-01", "owner": "people/x"}},
			{ID: "projects/b", Title: "Beta", Type: "project", Fields: map[string]any{"status": "idea", "due": "2026-09-01"}},
		},
	})
	return m
}

func namedViewCfg() *config.File {
	return &config.File{
		Views: map[string]config.View{
			"projects": {Source: "project", Layout: config.Present{Kind: kindBoard, GroupBy: "status"}},
			"open":     {Source: "project", Layout: config.Present{Kind: kindList, Filter: ".status==active"}},
			"week":     {Source: "project", Layout: config.Present{Kind: kindDate, GroupBy: "due", Granularity: layout.GranDay}},
			"people":   {Source: "person", Layout: config.Present{Kind: kindList, Sort: "name"}},
		},
	}
}

func loadNamedView(t *testing.T, cfg *config.File, name string) Model {
	t.Helper()
	m := NewModel(Options{Config: cfg})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedView(name)
	m, _ = m.update(queryMsg{
		gen: m.p().gen, pane: m.p().id, typeName: m.p().typeName,
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project", Fields: map[string]any{"status": "active", "due": "2026-10-01"}},
			{ID: "projects/b", Title: "Beta", Type: "project", Fields: map[string]any{"status": "idea", "due": "2026-09-01"}},
		},
	})
	return m
}

func focusMain(m *Model) {
	if len(m.panes) > 0 {
		m.setFocus(m.panes[0].id)
	}
}

func labelsOf(items []pickItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}
