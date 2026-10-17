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
		if m.focus == "" {
			t.Fatalf("%s cleared focus", k)
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
	if m.focus != "projects" {
		t.Fatalf("ctrl-w h left leftmost panel: %s", m.focus)
	}
	m.setFocus("projects")
	seen := map[string]bool{}
	for i := 0; i < 9; i++ {
		m, _ = m.update(key("tab"))
		seen[m.focus] = true
	}
	for _, id := range []string{"projects", "tasks", "notes"} {
		if !seen[id] {
			t.Fatalf("tab missed %s in %v", id, seen)
		}
	}
}

func TestMainRegionsStartAtOrigin(t *testing.T) {
	m := workspaceFocusModel(t)
	m.setFocus("projects")
	m.recomputeRegions()
	if len(m.regs) == 0 || m.regs[0].x != 0 {
		t.Fatalf("main should start at x=0, regs=%+v", m.regs)
	}
	out := m.View()
	if strings.Contains(out, "Browse") {
		t.Fatalf("sidebar chrome still in view: %s", out)
	}
}

func TestTabIncludesPreview(t *testing.T) {
	m := workspaceFocusModel(t)
	m.previewOn = true
	m.setFocus("projects")
	m.recomputeRegions()
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		m, _ = m.update(key("tab"))
		seen[m.focus] = true
	}
	if !seen[focusPreview] {
		t.Fatal("tab should reach preview")
	}
}

func TestCtrlDMovesListNotSwallowed(t *testing.T) {
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
	m, _ = m.update(key("ctrl+d"))
	if m.p().listCur == before {
		t.Fatal("ctrl-d should page")
	}
}
