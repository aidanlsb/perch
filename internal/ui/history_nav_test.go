package ui

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func TestUnifiedHistoryHomeBrowseNote(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	if m.screen != kindBrowse {
		t.Fatal(m.screen)
	}
	m, _ = m.update(key("enter"))
	m = applyNote(t, m, &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "A\n"})
	m, _ = m.update(key("esc"))
	if m.screen != kindBrowse {
		t.Fatalf("note -> browse, %s", m.screen)
	}
	m, _ = m.update(key("esc"))
	if m.screen != kindHome {
		t.Fatalf("browse -> home, %s", m.screen)
	}
	m, cmd := m.update(key("]"))
	if m.screen != kindBrowse && cmd == nil {
		// forward should restore browse
		if m.screen != kindBrowse {
			t.Fatalf("forward to browse, %s", m.screen)
		}
	}
	if m.screen == kindHome {
		m, _ = m.historyGo(1)
	}
	if m.screen != kindBrowse {
		t.Fatalf("want browse after forward, %s", m.screen)
	}
}

func TestHistoryAcrossTypeQueryRQL(t *testing.T) {
	m := loadBrowse(t)
	m, cmd := m.loadType("person")
	if cmd == nil {
		t.Fatal("type")
	}
	m, _ = m.update(queryMsg{gen: m.gen, pane: m.p().id, typeName: "person", objects: []api.Object{
		{ID: "people/x", Title: "X", Type: "person"},
	}})
	if m.p().typeName != "person" {
		t.Fatal(m.p().typeName)
	}
	q := m.schema.Queries["open"]
	m, cmd = m.loadQuery(&q)
	if cmd == nil {
		t.Fatal("query")
	}
	m, _ = m.loadRQL("type:project")
	m, _ = m.update(key("["))
	if m.p().sourceLabel != "open" && m.p().sourceBase != "open" {
		t.Fatalf("back to query, label=%s base=%s", m.p().sourceLabel, m.p().sourceBase)
	}
	m, _ = m.update(key("["))
	if m.p().typeName != "person" {
		t.Fatalf("back to type person, %s", m.p().typeName)
	}
}

func TestBrowseEscUnwindsLocalStateThenHistory(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m.searchQ = "Be"
	m.recomputeSearchHits()
	m, _ = m.update(key("esc"))
	if m.searchQ != "" {
		t.Fatalf("esc clears search, %q", m.searchQ)
	}
	if m.screen != kindBrowse {
		t.Fatal("still browse")
	}

	m.previewOn = true
	m.setFocus(focusPreview)
	m, _ = m.update(key("esc"))
	if m.focus == focusPreview {
		t.Fatal("esc leaves preview")
	}

	m.zoomed = true
	m, _ = m.update(key("backspace"))
	if m.zoomed {
		t.Fatal("backspace unzooms")
	}

	m, _ = m.update(key("esc"))
	if m.screen != kindHome {
		t.Fatalf("then history to home, %s", m.screen)
	}
}

func TestBrowseEscReturnsBoundChildToDriver(t *testing.T) {
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	m.setFocus("tasks")
	if m.focusedView() == nil || m.focusedView().bind != "projects" {
		t.Fatalf("want bound child, %+v", m.focusedView())
	}
	m, _ = m.update(key("esc"))
	if m.focus != "projects" {
		t.Fatalf("esc to driver, focus=%s", m.focus)
	}
}

func TestPickerQTypesInsteadOfClosing(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "t")
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatal("types")
	}
	m, _ = m.update(runeKey('q'))
	if m.overlay != overlayPicker {
		t.Fatal("q must not close picker")
	}
	if m.pickerQ != "q" {
		t.Fatalf("pickerQ=%q", m.pickerQ)
	}
}

func TestEscFromNavChildReturnsToNav(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, " ")
	for i, it := range m.pickerHit {
		if it.ID == "nav:queries" {
			m.pickerCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	if m.pickerKind != pickerQueries {
		t.Fatalf("kind %v", m.pickerKind)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayPicker || m.pickerKind != pickerNav {
		t.Fatalf("esc to nav, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func TestArgsEscStepsBackThenQueries(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "q")
	for i, it := range m.pickerHit {
		if it.Label == "need" {
			m.pickerCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	if m.overlay != overlayArgs {
		t.Fatal("args")
	}
	m, _ = m.update(runeKey('x'))
	m, _ = m.update(key("esc"))
	if m.overlay != overlayPicker || m.pickerKind != pickerQueries {
		t.Fatalf("esc at idx0 -> queries, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func TestSetEscStepsBackToFieldPicker(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("s"))
	if m.pickerKind != pickerFields {
		t.Fatal("fields")
	}
	for i, it := range m.pickerHit {
		if it.ID == "status" {
			m.pickerCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	if m.overlay != overlayPicker || m.pickerKind != pickerValues {
		t.Fatalf("value picker overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayPicker || m.pickerKind != pickerFields {
		t.Fatalf("esc back to fields, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayNone || m.mut.kind != mutNone {
		t.Fatalf("esc on first step aborts, overlay=%v mut=%s", m.overlay, m.mut.kind)
	}
}

func TestNewObjectEscStepsBack(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("c"))
	if m.overlay != overlayPrompt {
		t.Fatalf("title prompt, overlay=%v step=%s", m.overlay, m.mut.step)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayNone {
		t.Fatalf("prefilled type: title is first step, esc aborts, overlay=%v", m.overlay)
	}

	m = loadBrowse(t)
	m.p().typeName = ""
	m, _ = m.update(key("c"))
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatalf("no pane type: type picker, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	m, _ = m.update(key("enter"))
	if m.overlay != overlayPrompt {
		t.Fatalf("title after type, overlay=%v", m.overlay)
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayPicker || m.pickerKind != pickerTypes {
		t.Fatalf("esc from title -> type picker, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func TestActionsEscReturnsToActions(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "a")
	for i, it := range m.pickerHit {
		if it.ID == "capture" {
			m.pickerCur = i
			break
		}
	}
	m, _ = m.update(key("enter"))
	if m.overlay != overlayPrompt {
		t.Fatalf("capture prompt %v", m.overlay)
	}
	if !m.mut.fromActions {
		t.Fatal("fromActions")
	}
	m, _ = m.update(key("esc"))
	if m.overlay != overlayPicker || m.pickerKind != pickerActions {
		t.Fatalf("esc to actions, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}

func TestBracketHistoryOnBrowseAndHome(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.loadType("person")
	m, _ = m.update(key("["))
	if m.p().typeName != "project" && m.screen != kindBrowse && m.screen != kindHome {
		t.Fatalf("back screen=%s type=%s", m.screen, m.p().typeName)
	}
}
