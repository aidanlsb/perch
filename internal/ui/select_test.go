package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

func TestToggleSelectionAdvancesCursor(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	p := m.p()
	if p.selectedRowID() != "projects/a" {
		t.Fatalf("start %s", p.selectedRowID())
	}
	m, _ = m.update(key("v"))
	p = m.p()
	if !p.isMarked("projects/a") {
		t.Fatal("a should be selected")
	}
	if p.selectedRowID() != "projects/b" {
		t.Fatalf("cursor should advance, got %s", p.selectedRowID())
	}
	m, _ = m.update(key("v"))
	p = m.p()
	if !p.isMarked("projects/b") || p.committedCount() != 2 {
		t.Fatalf("both selected, count=%d", p.committedCount())
	}
	m, _ = m.update(key("k"))
	m, _ = m.update(key("v"))
	p = m.p()
	if p.isMarked("projects/a") {
		t.Fatal("toggle off a")
	}
}

func TestRangeSelectVimLinewise(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	addProject(m.p(), "projects/c", "Gamma")
	m.p().applyFilterAndGroup(m.typeSchemaFor(m.p()))
	m, _ = m.update(key("V"))
	if m.p().rangeAnchor != "projects/a" {
		t.Fatalf("anchor %s", m.p().rangeAnchor)
	}
	m, _ = m.update(key("j"))
	m, _ = m.update(key("j"))
	if !m.p().isMarked("projects/a") || !m.p().isMarked("projects/c") {
		t.Fatalf("range should highlight a..c, marked=%v", m.p().markedIDs())
	}
	m, _ = m.update(key("V"))
	p := m.p()
	if p.rangeAnchor != "" {
		t.Fatal("V commits and leaves range mode")
	}
	if p.committedCount() != 3 {
		t.Fatalf("committed %d %v", p.committedCount(), p.markedIDs())
	}
}

func TestRangeCommitWithBareV(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("V"))
	m, _ = m.update(key("j"))
	m, _ = m.update(key("v"))
	p := m.p()
	if p.rangeAnchor != "" {
		t.Fatal("v commits range")
	}
	if !p.isMarked("projects/a") || !p.isMarked("projects/b") {
		t.Fatalf("committed %v", p.markedIDs())
	}
}

func TestSelectAllVisible(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("ctrl+a"))
	p := m.p()
	if p.committedCount() != 2 {
		t.Fatalf("select all %d %v", p.committedCount(), p.markedIDs())
	}
	if !p.isMarked("projects/a") || !p.isMarked("projects/b") {
		t.Fatal("both visible")
	}
}

func TestEscClearsSelectionBeforeSearch(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("v"))
	m.searchQ = "Be"
	m.recomputeSearchHits()
	m, _ = m.update(key("esc"))
	p := m.p()
	if p.hasMarks() {
		t.Fatal("esc clears selection first")
	}
	if m.searchQ != "Be" {
		t.Fatalf("search should remain, got %q", m.searchQ)
	}
	m, _ = m.update(key("esc"))
	if m.searchQ != "" {
		t.Fatalf("second esc clears search, %q", m.searchQ)
	}
}

func TestEscClearsRangeMode(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("V"))
	m, _ = m.update(key("j"))
	m, _ = m.update(key("esc"))
	if m.p().hasMarks() {
		t.Fatal("esc cancels range and selection")
	}
	if m.screen != kindBrowse {
		t.Fatal("still browse")
	}
}

func TestSelectionSurvivesCursorAndIsPerPane(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	left := m.viewByID("projects")
	m, _ = m.update(queryMsg{
		gen: left.gen, pane: left.id, typeName: "project",
		objects: []api.Object{
			{ID: "projects/a", Title: "Alpha", Type: "project"},
			{ID: "projects/b", Title: "Beta", Type: "project"},
		},
	})
	notes := m.viewByID("notes")
	m, _ = m.update(queryMsg{
		gen: notes.gen, pane: notes.id, typeName: "person",
		objects: []api.Object{{ID: "people/x", Title: "X", Type: "person"}},
	})
	m.setFocus("projects")
	m, _ = m.update(key("v"))
	if !m.viewByID("projects").isMarked("projects/a") {
		t.Fatal("projects selected")
	}
	if m.viewByID("notes").hasMarks() {
		t.Fatal("notes must stay empty")
	}
	m, _ = m.update(key("j"))
	if !m.viewByID("projects").isMarked("projects/a") {
		t.Fatal("selection survives cursor move")
	}
	m.setFocus("notes")
	m, _ = m.update(key("ctrl+a"))
	if !m.viewByID("notes").isMarked("people/x") {
		t.Fatal("notes select-all")
	}
	if !m.viewByID("projects").isMarked("projects/a") {
		t.Fatal("projects selection isolated")
	}
}

func TestSelectionClearsOnHistory(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("ctrl+a"))
	if !m.p().hasMarks() {
		t.Fatal("selected")
	}
	m, _ = m.update(key("esc"))
	if m.p().hasMarks() {
		t.Fatal("esc clears selection")
	}
	m, _ = m.update(key("ctrl+a"))
	m, _ = m.goHome()
	if m.screen != kindHome {
		t.Fatalf("home %s", m.screen)
	}
	m, _ = m.update(key("["))
	if m.screen != kindBrowse {
		t.Fatalf("back to browse %s", m.screen)
	}
	if m.p().hasMarks() {
		t.Fatal("history restore clears selection")
	}
}

func TestCommonFieldsIntersection(t *testing.T) {
	schema := api.Schema{
		Types: map[string]api.TypeSchema{
			"project": {
				Name: "project",
				Fields: map[string]api.FieldSchema{
					"name":   {Type: "string"},
					"status": {Type: "enum", Values: []string{"idea", "active", "done"}},
					"due":    {Type: "date"},
				},
				FieldOrder: []string{"name", "status", "due"},
			},
			"task": {
				Name: "task",
				Fields: map[string]api.FieldSchema{
					"name":   {Type: "string"},
					"status": {Type: "enum", Values: []string{"active", "done", "blocked"}},
					"effort": {Type: "number"},
				},
				FieldOrder: []string{"name", "status", "effort"},
			},
			"person": {
				Name: "person",
				Fields: map[string]api.FieldSchema{
					"name": {Type: "string"},
				},
				FieldOrder: []string{"name"},
			},
		},
	}
	got := commonFields(schema, []api.Object{
		{ID: "p", Type: "project"},
		{ID: "t", Type: "task"},
	})
	names := commonFieldNames(got)
	if strings.Join(names, ",") != "name,status" {
		t.Fatalf("project∩task %v", names)
	}
	status := fieldNamed(got, "status")
	if strings.Join(status.Values, ",") != "active,done" {
		t.Fatalf("enum intersection %v", status.Values)
	}
	got = commonFields(schema, []api.Object{
		{ID: "p", Type: "project"},
		{ID: "n", Type: "person"},
	})
	names = commonFieldNames(got)
	if strings.Join(names, ",") != "name" {
		t.Fatalf("project∩person %v", names)
	}
	got = commonFields(schema, []api.Object{
		{ID: "p", Type: "project"},
		{ID: "x", Type: "unknown"},
	})
	if len(got) != 0 {
		t.Fatalf("unknown type should yield none, %v", commonFieldNames(got))
	}
}

func TestTraitUniformTargetSemantics(t *testing.T) {
	schema := api.Schema{
		Traits: map[string]api.TraitSchema{
			"done":     {Name: "done", Type: "bool"},
			"priority": {Name: "priority", Type: "enum", Values: []string{"low", "high"}},
			"mood":     {Name: "mood", Type: "enum", Values: []string{"a", "b", "c"}},
		},
	}
	val, picker, err := traitUniformTarget(schema, []api.Object{
		traitObj("t0", "done", "false"),
		traitObj("t1", "done", "true"),
	})
	if err != "" || picker || val != "true" {
		t.Fatalf("bool from first: val=%s picker=%v err=%s", val, picker, err)
	}
	val, picker, err = traitUniformTarget(schema, []api.Object{
		traitObj("t0", "priority", "low"),
		traitObj("t1", "priority", "high"),
	})
	if err != "" || picker || val != "high" {
		t.Fatalf("two-value enum: val=%s picker=%v err=%s", val, picker, err)
	}
	val, picker, err = traitUniformTarget(schema, []api.Object{
		traitObj("t0", "mood", "a"),
		traitObj("t1", "mood", "b"),
	})
	if err != "" || !picker || val != "" {
		t.Fatalf("larger enum opens picker: val=%s picker=%v err=%s", val, picker, err)
	}
	_, _, err = traitUniformTarget(schema, []api.Object{
		{ID: "projects/a", Type: "project"},
	})
	if err != "not a trait" {
		t.Fatalf("err %q", err)
	}
}

func TestSelectionHeaderAndGutter(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m.width, m.height = 100, 24
	m, _ = m.update(key("ctrl+a"))
	head := m.paneHeader(m.p(), true)
	if !strings.Contains(head, "2 selected") {
		t.Fatalf("header %q", head)
	}
	out := m.View()
	if !strings.Contains(out, markGlyph) {
		t.Fatalf("gutter marker missing:\n%s", out)
	}
	foot := footerKeys(m)
	if !strings.Contains(foot, "esc clear") {
		t.Fatalf("footer %s", foot)
	}
}

func TestSpaceVStillOpensViews(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "v")
	if m.overlay != overlayPicker || m.pickerKind != pickerViews {
		t.Fatalf("space v overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func addProject(p *viewState, id, title string) {
	p.objects = append(p.objects, api.Object{
		ID: id, Title: title, Type: "project",
		Fields: map[string]any{"name": title, "status": "idea"},
	})
}

func traitObj(id, traitType, value string) api.Object {
	return api.Object{
		ID: id, Kind: "trait", Type: traitType,
		Fields: map[string]any{"trait_type": traitType, "value": value},
	}
}

func commonFieldNames(fields []commonField) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Name
	}
	return out
}

func fieldNamed(fields []commonField, name string) api.FieldSchema {
	for _, f := range fields {
		if f.Name == name {
			return f.Schema
		}
	}
	return api.FieldSchema{}
}

func TestKeyCtrlATypeExists(t *testing.T) {
	msg := key("ctrl+a")
	if keyIdent(msg) != "ctrl+a" {
		t.Fatalf("ident %q type %v", keyIdent(msg), msg.Type)
	}
	_ = tea.KeyCtrlA
}
