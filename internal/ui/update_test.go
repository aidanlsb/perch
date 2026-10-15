package ui

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func sampleSchema() api.Schema {
	return api.Schema{
		Version: 1,
		Types: map[string]api.TypeSchema{
			"project": {
				Name:      "project",
				NameField: "name",
				Fields: map[string]api.FieldSchema{
					"name":   {Type: "string"},
					"status": {Type: "enum", Values: []string{"idea", "active", "done"}},
					"due":    {Type: "date"},
					"owner":  {Type: "ref", Target: "person"},
				},
			},
			"person":  {Name: "person", NameField: "name", Fields: map[string]api.FieldSchema{"name": {Type: "string"}}},
			"section": {Name: "section", Builtin: true},
		},
		Queries: map[string]api.SavedQuery{
			"open": {Name: "open", Query: "type:project .status==active"},
			"need": {Name: "need", Query: "type:project refs([[{{args.x}}]])", Args: []string{"x"}},
		},
	}
}

func loadBrowse(t *testing.T) Model {
	t.Helper()
	m := NewModel(Options{Config: &config.File{Views: []config.View{{
		Name: "Kanban", Source: "type:project", Kind: "board", GroupBy: "status",
	}}}})
	m, cmd := m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	if cmd == nil {
		t.Fatal("expected query cmd")
	}
	m, _ = m.update(queryMsg{
		gen: m.gen, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project", Fields: map[string]any{"name": "Alpha", "status": "active", "due": "2026-10-01"}, FilePath: "projects/a.md"},
			{ID: "projects/b", Title: "Beta", Type: "project", Fields: map[string]any{"name": "Beta", "status": "idea", "due": "2026-09-01"}, FilePath: "projects/b.md"},
		},
	})
	return m
}

func ptrSchema(s api.Schema) *api.Schema { return &s }

func TestSchemaBuildsSidebar(t *testing.T) {
	m := loadBrowse(t)
	var labels []string
	for _, n := range m.nav {
		labels = append(labels, n.Label)
	}
	if len(labels) < 4 {
		t.Fatalf("%v", labels)
	}
	if labels[0] != "person" && labels[0] != "project" {
		t.Fatalf("types first %v", labels)
	}
	for _, l := range labels {
		if l == "section" {
			t.Fatal("section should be omitted")
		}
	}
}

func TestMoveAndSearch(t *testing.T) {
	m := loadBrowse(t)
	m.pane = paneMain
	m, _ = m.update(key("j"))
	if m.listCur != 1 {
		t.Fatalf("cursor %d", m.listCur)
	}
	m, _ = m.update(key("g"))
	m, _ = m.update(key("g"))
	if m.listCur != 0 {
		t.Fatalf("gg %d", m.listCur)
	}
	m, _ = m.update(key("/"))
	if m.overlay != overlaySearch {
		t.Fatal("search overlay")
	}
	m, _ = m.update(runeKey('B'))
	m, _ = m.update(runeKey('e'))
	obj := m.selectedObject()
	if obj == nil || obj.Title != "Beta" {
		t.Fatalf("%v", obj)
	}
	m, _ = m.update(key("enter"))
	if m.overlay != overlayNone {
		t.Fatal("enter keeps search")
	}
}

func TestOpenNoteTabFollowHistory(t *testing.T) {
	m := loadBrowse(t)
	m.pane = paneMain
	m, cmd := m.update(key("enter"))
	if cmd == nil {
		t.Fatal("expected read")
	}
	content := "See [[people/freya]] and [[missing]].\n"
	m, _ = m.update(readMsg{
		gen: m.gen,
		result: &api.ReadResult{
			ObjectID: "projects/a",
			Path:     "projects/a.md",
			Content:  content,
			Links: []api.ReadLink{
				{ByteStart: 4, ByteEnd: 20, Text: "[[people/freya]]", Target: "people/freya", Status: api.LinkResolved, ObjectID: "people/freya"},
				{ByteStart: 25, ByteEnd: 36, Text: "[[missing]]", Target: "missing", Status: api.LinkUnresolved},
			},
			Backlinks: []api.BacklinkGroup{{Source: "people/odin", Lines: []string{"hi"}}},
		},
	})
	if m.screen != kindNote || m.note == nil {
		t.Fatal("not in note")
	}
	if !m.note.Links[0].SpanOK(content) {
		t.Fatal("byte offsets")
	}
	if m.linkIdx != 0 {
		t.Fatal(m.linkIdx)
	}
	m, _ = m.update(key("tab"))
	if m.linkIdx != 1 {
		t.Fatalf("tab %d", m.linkIdx)
	}
	m, _ = m.update(key("shift+tab"))
	if m.linkIdx != 0 {
		t.Fatal("shift-tab")
	}
	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("follow")
	}
	m, _ = m.update(readMsg{
		gen: m.gen,
		result: &api.ReadResult{
			ObjectID: "people/freya",
			Path:     "people/freya.md",
			Content:  "Freya\n",
		},
	})
	if m.note.ObjectID != "people/freya" {
		t.Fatal(m.note.ObjectID)
	}
	m, cmd = m.update(key("["))
	if cmd == nil {
		t.Fatal("back")
	}
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: content}})
	if m.note.ObjectID != "projects/a" {
		t.Fatal("history back")
	}
	m, _ = m.update(key("q"))
	if m.screen != "browse" {
		t.Fatal("q back to browse")
	}
}

func TestBoardFromSavedView(t *testing.T) {
	m := loadBrowse(t)
	// last nav item is Kanban
	m.navCur = len(m.nav) - 1
	m, _ = m.update(key("enter"))
	m, _ = m.update(queryMsg{
		gen: m.gen, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Fields: map[string]any{"status": "active"}},
			{ID: "projects/b", Title: "Beta", Fields: map[string]any{"status": "idea"}},
		},
	})
	if m.viewKind != kindBoard {
		t.Fatalf("kind %s", m.viewKind)
	}
	if len(m.board) < 3 {
		t.Fatalf("cols %d", len(m.board))
	}
	m.pane = paneMain
	m, _ = m.update(key("l"))
	if m.boardCol == 0 {
		t.Fatal("expected column move")
	}
}

func TestSavedQueryArgsPrompt(t *testing.T) {
	m := loadBrowse(t)
	for i, n := range m.nav {
		if n.Label == "need" {
			m.navCur = i
			break
		}
	}
	m, cmd := m.update(key("enter"))
	if cmd != nil {
		t.Fatal("should prompt, not query")
	}
	if m.overlay != overlayArgs || len(m.argsNames) != 1 || m.argsNames[0] != "x" {
		t.Fatalf("overlay=%v names=%v", m.overlay, m.argsNames)
	}
	m, _ = m.update(runeKey('p'))
	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("expected query with inputs")
	}
	if m.sourceInputs["x"] != "p" || m.sourceQuery != "need" {
		t.Fatalf("inputs=%v source=%s", m.sourceInputs, m.sourceQuery)
	}
}

func TestRefusesNilClientWritesStillReadonly(t *testing.T) {
	if !api.IsForbidden("set") || !api.IsWrite("move") {
		t.Fatal("readonly helpers")
	}
}

func TestSessionAppliedFromSchemaMsg(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m, _ = m.update(schemaMsg{
		schema: ptrSchema(sampleSchema()),
		session: &api.Session{
			VaultPath: "/from-session", Editor: "hx", EditorMode: "terminal",
			Daily: &api.DailyInfo{TodayID: "2026-10-15", TodayExists: false, Directory: "daily"},
		},
	})
	if m.vaultPath != "/from-session" || m.editor != "hx" || m.editorMode != "terminal" {
		t.Fatalf("vault=%q editor=%q mode=%q", m.vaultPath, m.editor, m.editorMode)
	}
	if m.daily == nil || m.daily.TodayID != "2026-10-15" {
		t.Fatalf("daily %+v", m.daily)
	}
}

func TestQuit(t *testing.T) {
	m := loadBrowse(t)
	_, cmd := m.update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("quit")
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case " ":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
