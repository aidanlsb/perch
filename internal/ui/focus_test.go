package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func workspaceFocusModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(Options{Config: nestedLayoutCfg()})
	m.width, m.height = 80, 24
	m, _ = m.update(schemaMsg{schema: ptrSchema(sampleSchema())})
	m, _ = m.loadNamedLayout("workspace")
	m.recomputeRegions()
	return m
}

func TestPanelMotionsStayInMain(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("projects")
	motions := []string{"ctrl+h", "ctrl+j", "ctrl+k", "ctrl+l", "tab", "shift+tab"}
	for _, k := range motions {
		m.setFocus("projects")
		m, _ = m.update(key(k))
		if m.focus == focusSidebar {
			t.Fatalf("%s entered sidebar", k)
		}
	}
	m.setFocus("projects")
	m, _ = m.update(key("ctrl+h"))
	if m.focus != "projects" {
		t.Fatalf("ctrl-h left leftmost panel: %s", m.focus)
	}
	m.setFocus("projects")
	m, _ = m.update(key("ctrl+w"))
	m, _ = m.update(key("h"))
	if m.focus == focusSidebar {
		t.Fatal("ctrl-w h entered sidebar")
	}
	if m.focus != "projects" {
		t.Fatalf("ctrl-w h left leftmost panel: %s", m.focus)
	}
	m.setFocus("projects")
	seen := map[string]bool{}
	for i := 0; i < 9; i++ {
		m, _ = m.update(key("tab"))
		if m.focus == focusSidebar {
			t.Fatal("tab entered sidebar")
		}
		seen[m.focus] = true
	}
	for _, id := range []string{"projects", "tasks", "notes"} {
		if !seen[id] {
			t.Fatalf("tab missed %s in %v", id, seen)
		}
	}
	m.setFocus("projects")
	m, _ = m.update(key("h"))
	if m.focus == focusSidebar {
		t.Fatal("h entered sidebar")
	}
}

func TestCtrlETogglesSidebarAndRestoresPanel(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("projects")
	m.setFocus("notes")
	m, _ = m.update(key("ctrl+e"))
	if m.focus != focusSidebar {
		t.Fatalf("ctrl-e from main: %s", m.focus)
	}
	if m.sidebarHidden {
		t.Fatal("ctrl-e must not hide the sidebar")
	}
	m, _ = m.update(key("ctrl+e"))
	if m.focus != "notes" {
		t.Fatalf("ctrl-e should restore last panel, got %s", m.focus)
	}
}

func TestCtrlBHidesAndShowsSidebar(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("projects")
	m.recomputeRegions()
	if len(m.regs) == 0 || m.regs[0].x == 0 {
		t.Fatalf("visible sidebar should offset main, regs=%+v", m.regs)
	}
	shownW := m.regs[0].w
	out := m.View()
	if !strings.Contains(out, "Browse") {
		t.Fatalf("visible sidebar missing Browse: %s", out)
	}

	m, _ = m.update(key("ctrl+b"))
	if !m.sidebarHidden {
		t.Fatal("ctrl-b should hide")
	}
	if m.focus != "projects" {
		t.Fatalf("hide must keep main focus, got %s", m.focus)
	}
	for _, r := range m.regs {
		if r.id == focusSidebar {
			t.Fatal("hidden sidebar still a region")
		}
	}
	if len(m.regs) == 0 || m.regs[0].x != 0 {
		t.Fatalf("hidden sidebar should expand main to x=0, regs=%+v", m.regs)
	}
	if m.regs[0].w <= shownW {
		t.Fatalf("main did not expand: before=%d after=%d", shownW, m.regs[0].w)
	}
	out = m.View()
	if strings.Contains(out, "Browse") {
		t.Fatalf("hidden sidebar still in view: %s", out)
	}

	m, _ = m.update(key("ctrl+b"))
	if m.sidebarHidden {
		t.Fatal("ctrl-b should show")
	}
	if m.focus != "projects" {
		t.Fatalf("show must not steal focus, got %s", m.focus)
	}
	if len(m.regs) == 0 || m.regs[0].x == 0 {
		t.Fatalf("shown sidebar should offset main, regs=%+v", m.regs)
	}
}

func TestCtrlBMovesFocusOffHiddenSidebar(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("tasks")
	m, _ = m.update(key("ctrl+e"))
	if m.focus != focusSidebar {
		t.Fatalf("focus %s", m.focus)
	}
	m, _ = m.update(key("ctrl+b"))
	if !m.sidebarHidden {
		t.Fatal("hidden")
	}
	if m.focus != "tasks" {
		t.Fatalf("hide while focused should restore last panel, got %s", m.focus)
	}
}

func TestCtrlEShowsHiddenSidebarAndFocusesIt(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("notes")
	m, _ = m.update(key("ctrl+b"))
	if !m.sidebarHidden || m.focus != "notes" {
		t.Fatalf("hidden=%v focus=%s", m.sidebarHidden, m.focus)
	}
	m, _ = m.update(key("ctrl+e"))
	if m.sidebarHidden {
		t.Fatal("ctrl-e should show hidden sidebar")
	}
	if m.focus != focusSidebar {
		t.Fatalf("ctrl-e should focus sidebar, got %s", m.focus)
	}
	m, _ = m.update(key("ctrl+e"))
	if m.focus != "notes" {
		t.Fatalf("restore %s", m.focus)
	}
}

func TestTabIncludesPreviewNotSidebar(t *testing.T) {
	m := workspaceFocusModel(t)
	m.previewOn = true
	m.setFocus("projects")
	m.recomputeRegions()
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		m, _ = m.update(key("tab"))
		if m.focus == focusSidebar {
			t.Fatal("tab entered sidebar")
		}
		seen[m.focus] = true
	}
	if !seen[focusPreview] {
		t.Fatal("tab should reach preview")
	}
}

func TestCtrlEBDoNotScrollLists(t *testing.T) {
	m := loadBrowse(t)
	m.height = 24
	focusMain(&m)
	p := m.p()
	p.objects = nil
	for i := 0; i < 40; i++ {
		p.objects = append(p.objects, api.Object{ID: string(rune('a'+(i%26))) + string(rune('0'+i/26)), Title: "N"})
	}
	m.applyFilterAndGroup()
	before := m.p().listCur
	m, _ = m.update(key("ctrl+e"))
	if m.focus != focusSidebar {
		t.Fatalf("ctrl-e %s", m.focus)
	}
	if m.p().listCur != before {
		t.Fatalf("ctrl-e scrolled list %d -> %d", before, m.p().listCur)
	}
	m, _ = m.update(key("ctrl+e"))
	m, _ = m.update(key("ctrl+b"))
	if m.p().listCur != before {
		t.Fatalf("ctrl-b scrolled list %d -> %d", before, m.p().listCur)
	}
	m, _ = m.update(key("ctrl+d"))
	if m.p().listCur == before {
		t.Fatal("ctrl-d should page")
	}
}
