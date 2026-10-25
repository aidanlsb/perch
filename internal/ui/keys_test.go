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
	for _, want := range []string{"ctrl+c", "ctrl-h/j/k/l", "space t", "type picker", "daily-note date picker", "full-text content search", "fuzzy search", "navigation menu"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q in %s", want, out)
		}
	}
	for _, old := range []string{"ctrl-b", "ctrl-e", "space st", "space sd", "space sg", "space l", "space c"} {
		if strings.Contains(out, old) {
			t.Fatalf("help still lists removed key %q", old)
		}
	}
}

func TestTypeAndQueryPickers(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "t")
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
	p := m.p()
	if p.sourceIsQuery || p.typeName != "project" || p.sourceBase != "type:project" {
		t.Fatalf("t should open as Type, got type=%s base=%s query=%v", p.typeName, p.sourceBase, p.sourceIsQuery)
	}
	if p.sourceQuery != "type:project" || p.spec.Kind != kindList {
		t.Fatalf("source=%s kind=%s group=%s", p.sourceQuery, p.spec.Kind, p.groupBy)
	}

	m, _ = pressLeader(m, "q")
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
	m, _ = pressLeader(m, "v")
	if m.overlay != overlayPicker || m.pickerKind != pickerViews {
		t.Fatalf("sv picker overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	if len(m.pickerHit) < 2 {
		t.Fatalf("views %v", labelsOf(m.pickerHit))
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

func TestDailyPickerOpensOnToday(t *testing.T) {
	m := loadBrowse(t)
	m.daily = &api.DailyInfo{TodayID: "2026-10-15", TodayExists: true}
	m, cmd := pressLeader(m, "d")
	if cmd != nil {
		t.Fatal("picker is local")
	}
	if m.overlay != overlayDate {
		t.Fatalf("overlay %v", m.overlay)
	}
	if m.dateCur != "2026-10-15" {
		t.Fatalf("date %s", m.dateCur)
	}
	m, _ = m.update(key("h"))
	if m.dateCur != "2026-10-14" {
		t.Fatalf("h %s", m.dateCur)
	}
	m, _ = m.update(key("l"))
	if m.dateCur != "2026-10-15" {
		t.Fatalf("l %s", m.dateCur)
	}
	m, _ = m.update(key("k"))
	if m.dateCur != "2026-10-08" {
		t.Fatalf("k week %s", m.dateCur)
	}
	m, _ = m.update(key("j"))
	if m.dateCur != "2026-10-15" {
		t.Fatalf("j week %s", m.dateCur)
	}
	m, _ = m.update(key("h"))
	m, _ = m.update(key("t"))
	if m.dateCur != "2026-10-15" {
		t.Fatalf("t %s", m.dateCur)
	}
	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("enter should invoke date")
	}
}

func TestDailyMissingDoesNotOpenType(t *testing.T) {
	m := loadBrowse(t)
	src := m.p().sourceQuery
	m.gen = 4
	m, _ = m.update(dateMsg{
		gen:    4,
		date:   "2026-10-15",
		result: &api.DateResult{Date: "2026-10-15", DailyExists: false},
	})
	if m.p().sourceQuery != src {
		t.Fatalf("source changed to %s", m.p().sourceQuery)
	}
	if !strings.Contains(m.status, "2026-10-15") {
		t.Fatalf("status %s", m.status)
	}
}

func TestDailyExistsOpensNote(t *testing.T) {
	m := loadBrowse(t)
	m.gen = 3
	m, cmd := m.update(dateMsg{
		gen:    3,
		date:   "2026-10-15",
		result: &api.DateResult{Date: "2026-10-15", DailyNoteID: "2026-10-15", DailyExists: true},
	})
	if cmd == nil {
		t.Fatal("read daily note")
	}
	if !strings.Contains(m.status, "2026-10-15") {
		t.Fatalf("status %s", m.status)
	}
}

func TestDailyPrevNextFromNote(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "2026-10-15", Path: "journal/2026-10-15.md", Content: "today\n"}})
	if m.screen != kindNote {
		t.Fatal("note")
	}
	m, cmd := m.update(key("["))
	if cmd != nil {
		t.Fatal("pending [d")
	}
	m, cmd = m.update(key("d"))
	if cmd == nil {
		t.Fatal("prev day")
	}
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "2026-10-15", Path: "journal/2026-10-15.md", Content: "today\n"}})
	m, _ = m.update(key("]"))
	m, cmd = m.update(key("d"))
	if cmd == nil {
		t.Fatal("next day")
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

func TestLeaderPickers(t *testing.T) {
	m := loadBrowse(t)
	cases := []struct {
		key  string
		kind pickerKind
		date bool
	}{
		{"t", pickerTypes, false},
		{"q", pickerQueries, false},
		{"v", pickerViews, false},
		{"f", pickerNotes, false},
		{"s", pickerSearch, false},
		{"d", 0, true},
	}
	for _, tc := range cases {
		m, _ = m.update(key("esc"))
		m, _ = pressLeader(m, tc.key)
		if tc.date {
			if m.overlay != overlayDate {
				t.Fatalf("space %s overlay %v, want daily", tc.key, m.overlay)
			}
			continue
		}
		if m.overlay != overlayPicker || m.pickerKind != tc.kind {
			t.Fatalf("space %s overlay=%v kind=%v", tc.key, m.overlay, m.pickerKind)
		}
	}
}

func TestBareNavKeysDoNothing(t *testing.T) {
	m := loadBrowse(t)
	src := m.p().sourceQuery
	for _, k := range []string{"t", "d", "q", "v", "f", "s"} {
		m, cmd := m.update(key(k))
		if cmd != nil {
			t.Fatalf("bare %s ran a command", k)
		}
		if m.overlay != overlayNone {
			t.Fatalf("bare %s overlay %v", k, m.overlay)
		}
		if m.p().sourceQuery != src {
			t.Fatalf("bare %s changed source to %s", k, m.p().sourceQuery)
		}
	}
}

func TestSpaceLeader(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 120, 24
	m, _ = m.update(key(" "))
	if m.pending != "space" {
		t.Fatalf("pending %q", m.pending)
	}
	if m.overlay != overlayNone {
		t.Fatalf("space overlay %v", m.overlay)
	}
	out := m.View()
	for _, want := range []string{"space", "views", "types", "queries", "daily", "notes", "search", "config"} {
		if !strings.Contains(out, want) {
			t.Fatalf("leader footer missing %q in %s", want, out)
		}
	}
	m, _ = m.update(key("esc"))
	if m.pending != "" || m.overlay != overlayNone {
		t.Fatalf("esc should cancel leader, pending=%q overlay=%v", m.pending, m.overlay)
	}
	m, _ = m.update(key(" "))
	m, _ = m.update(key("c"))
	if m.pending != "" || m.overlay != overlayNone {
		t.Fatalf("unknown leader key should cancel, pending=%q overlay=%v", m.pending, m.overlay)
	}
	m, _ = pressLeader(m, " ")
	if m.overlay != overlayPicker || m.pickerKind != pickerNav {
		t.Fatalf("space space overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayNone {
		t.Fatalf("esc from nav with a view loaded should close, overlay=%v", m.overlay)
	}
}

func TestNavIncludesViewsWhenConfigured(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "projects")
	m, _ = pressLeader(m, " ")
	found := false
	for _, it := range m.pickerHit {
		if it.ID == "nav:views" {
			found = true
		}
	}
	if !found {
		t.Fatalf("nav missing views item: %v", labelsOf(m.pickerHit))
	}
}

func TestCommaOpensConfigAtView(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "projects")
	m, _ = m.update(key(","))
	if m.overlay != overlayNone {
		t.Fatalf("overlay %v", m.overlay)
	}
	m, _ = pressLeader(m, ",")
	if m.overlay != overlayNone {
		t.Fatalf("space , overlay %v", m.overlay)
	}
}

func TestContentSearchPicker(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "s")
	if m.overlay != overlayPicker || m.pickerKind != pickerSearch {
		t.Fatalf("s overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, cmd := m.update(runeKey('r'))
	if cmd == nil {
		t.Fatal("search fetch")
	}
	m, _ = m.update(searchMsg{
		query: "r",
		result: &api.SearchResult{Query: "r", Items: []api.SearchMatch{
			{ObjectID: "projects/a", Title: "Alpha", Snippet: "hello raven"},
		}},
	})
	if len(m.pickerHit) != 1 || m.pickerHit[0].Label != "Alpha" {
		t.Fatalf("hits %v", labelsOf(m.pickerHit))
	}
}

func TestNotesPickerFromF(t *testing.T) {
	m := loadBrowse(t)
	m.mergeNotes([]api.Object{{ID: "people/freya", Title: "Freya"}})
	m, _ = pressLeader(m, "f")
	if m.overlay != overlayPicker || m.pickerKind != pickerNotes {
		t.Fatal("space f notes picker")
	}
}

func TestQOpensQueriesNotQuit(t *testing.T) {
	m := loadBrowse(t)
	m, cmd := m.update(key("q"))
	if cmd != nil {
		t.Fatal("bare q must not quit")
	}
	if m.overlay != overlayNone {
		t.Fatalf("bare q overlay=%v", m.overlay)
	}
	m, cmd = pressLeader(m, "q")
	if cmd != nil {
		t.Fatal("space q must not quit")
	}
	if m.overlay != overlayPicker || m.pickerKind != pickerQueries {
		t.Fatalf("space q overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func TestNoteQOpensQueriesEscGoesBack(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "hi\n"}})
	if m.screen != kindNote {
		t.Fatal("note")
	}
	m, cmd := pressLeader(m, "q")
	if cmd != nil {
		t.Fatal("space q must not quit")
	}
	if m.overlay != overlayPicker || m.pickerKind != pickerQueries {
		t.Fatalf("note space q overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayNone || m.screen != kindNote {
		t.Fatalf("esc closes picker, screen=%s overlay=%v", m.screen, m.overlay)
	}
	m, _ = m.update(key("esc"))
	if m.screen != "browse" {
		t.Fatalf("esc back to browse, screen=%s", m.screen)
	}
}

func TestBracketDStillDailyFromNote(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "2026-10-15", Path: "journal/2026-10-15.md", Content: "today\n"}})
	m, _ = m.update(key("["))
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("[d should still shift daily")
	}
	if m.overlay == overlayDate {
		t.Fatal("[d must not open the date picker")
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
			"ghost": {Type: "ghost", Layout: config.Present{Kind: kindList}},
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
			"projects": {Type: "project", Layout: config.Present{Kind: kindBoard, GroupBy: "status"}},
			"open":     {Type: "project", Layout: config.Present{Kind: kindList, Filter: ".status==active"}},
			"week":     {Type: "project", Layout: config.Present{Kind: kindDate, GroupBy: "due", Granularity: layout.GranDay}},
			"people":   {Type: "person", Layout: config.Present{Kind: kindList, Sort: "name"}},
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
