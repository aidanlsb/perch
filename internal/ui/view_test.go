package ui

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func TestViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 80
	m.height = 24
	out := m.View()
	if out == "" || len(out) < 20 {
		t.Fatalf("empty view %q", out)
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
