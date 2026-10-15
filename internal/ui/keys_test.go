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
	for _, want := range []string{"ctrl+c", "cycle layout", "daily"} {
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
	if m.sourceQuery != "type:project" || m.viewKind != kindBoard {
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
	if m.viewKind != kindBoard {
		t.Fatalf("infer board, got %s", m.viewKind)
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindBoard || m.groupBy != "owner" {
		t.Fatalf("next group %+v", m.spec)
	}
	m, _ = m.update(key("L"))
	if m.viewKind != kindDate {
		t.Fatalf("date %s", m.viewKind)
	}
	m, _ = m.update(key("."))
	if m.granularity != layout.GranWeek {
		t.Fatalf("gran %s", m.granularity)
	}
	m, _ = m.update(key("p"))
	if !m.previewOn {
		t.Fatal("preview")
	}
}

func TestDailyHotkey(t *testing.T) {
	m := loadBrowse(t)
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("date cmd")
	}
	m, cmd = m.update(dateMsg{
		gen: m.gen,
		result: &api.DateResult{
			Date: "2026-10-15", DailyExists: true, DailyNoteID: "2026-10-15",
		},
	})
	if cmd == nil {
		t.Fatal("read daily")
	}
	m, _ = m.update(readMsg{
		gen:     m.gen,
		purpose: "note",
		result:  &api.ReadResult{ObjectID: "2026-10-15", Path: "daily/2026-10-15.md", Content: "# today\n"},
	})
	if m.screen != kindNote || m.note.ObjectID != "2026-10-15" {
		t.Fatalf("screen %s note %+v", m.screen, m.note)
	}
}

func TestDailyUsesSessionWhenPresent(t *testing.T) {
	m := loadBrowse(t)
	m.daily = &api.DailyInfo{Directory: "daily", TodayID: "2026-10-15", TodayPath: "daily/2026-10-15.md", TodayExists: true}
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("read today")
	}
	if !strings.Contains(m.status, "2026-10-15") {
		t.Fatalf("status %s", m.status)
	}
}

func TestDailySessionMissingOpensDateList(t *testing.T) {
	m := loadBrowse(t)
	m.daily = &api.DailyInfo{TodayID: "2026-10-15", TodayExists: false}
	m, cmd := m.update(key("d"))
	if cmd == nil {
		t.Fatal("load date type")
	}
	if m.sourceQuery != "type:date" {
		t.Fatalf("source %s", m.sourceQuery)
	}
}

func TestDailyMissingFallsBackToDateType(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("d"))
	m, cmd := m.update(dateMsg{
		gen:    m.gen,
		result: &api.DateResult{Date: "2026-10-15", DailyExists: false},
	})
	if cmd == nil {
		t.Fatal("load date type")
	}
	if m.sourceQuery != "type:date" {
		t.Fatalf("source %s", m.sourceQuery)
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

func TestConfigTypeLayoutUsed(t *testing.T) {
	m := NewModel(Options{Config: &config.File{
		Types: map[string]config.Layout{
			"person": {Layout: "list", Sort: "name"},
		},
	}})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.update(queryMsg{gen: m.gen, typeName: "person", objects: nil})
	// first nav is person; list not board
	if m.viewKind != kindList {
		t.Fatalf("%s", m.viewKind)
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
