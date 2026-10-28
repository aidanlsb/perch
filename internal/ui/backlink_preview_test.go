package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func TestBacklinksShowSnippetAndPreview(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 120, 36
	focusMain(&m)
	m, _ = m.update(key("enter"))
	src := &api.ReadResult{
		ObjectID: "people/odin",
		Path:     "people/odin.md",
		Content:  "intro\nsee [[projects/a]] on this line\nmore\n",
	}
	m.readCache["people/odin"] = src
	note := &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  "Alpha\n",
		Backlinks: []api.BacklinkGroup{{
			Source:  "people/odin",
			Title:   "Odin",
			Line:    2,
			Snippet: "see [[projects/a]] on this line",
			Lines:   []string{"see [[projects/a]] on this line"},
		}},
	}
	m = applyNote(t, m, note)
	m.pane = paneBacklinks
	m.backIdx = 0
	out := m.View()
	for _, want := range []string{"Odin", "see [[projects/a]]", "people/odin.md:2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("backlinks missing %q in\n%s", want, out)
		}
	}
}

func TestBacklinkMoveFetchesPreview(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 100, 30
	note := &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  "Alpha\n",
		Backlinks: []api.BacklinkGroup{
			{Source: "people/odin", Lines: []string{"see a"}},
			{Source: "people/freya", Lines: []string{"also a"}},
		},
	}
	m, _ = m.update(readMsg{gen: m.gen, result: note})
	if m.pane != paneBacklinks {
		m.pane = paneBacklinks
	}
	m.backIdx = 0
	m, cmd := m.update(key("j"))
	if m.backIdx != 1 {
		t.Fatalf("backIdx %d", m.backIdx)
	}
	if cmd == nil {
		t.Fatal("should fetch missing backlink source")
	}
}

func TestTraitViewAutoEnablesPreview(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.loadRQL("trait:todo")
	m, cmd := m.update(queryMsg{
		gen: m.p().gen, pane: m.p().id, queryKind: "trait",
		objects: sampleTraitRows(),
	})
	if m.p().queryKind != "trait" {
		t.Fatalf("queryKind %s", m.p().queryKind)
	}
	if !m.previewOn {
		t.Fatal("preview should auto-enable for trait views")
	}
	if cmd == nil && m.preview == nil {
		t.Fatal("preview fetch")
	}
	m, _ = m.update(key("p"))
	if m.previewOn {
		t.Fatal("p toggles off")
	}
	if p := m.focusedView(); p == nil || !p.previewOff {
		t.Fatal("user off remembered")
	}
}

func TestTraitPreviewMarksSourceLine(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 100, 24
	m.previewOn = true
	body := traitNoteContent()
	m.setPreview(&api.ReadResult{
		ObjectID: "projects/raven",
		Path:     "projects/raven.md",
		Content:  body,
	})
	m.previewLine = 18
	m.applyPreviewScroll()
	m.p().objects = sampleTraitRows()[:1]
	m.p().applyFilterAndGroup(api.TypeSchema{})
	out := m.viewPreview(48, 20)
	if !strings.Contains(out, "projects/raven.md:18") {
		t.Fatalf("missing path:line header:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("missing marked trait line:\n%s", out)
	}
}
