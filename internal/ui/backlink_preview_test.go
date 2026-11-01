package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/charmbracelet/x/ansi"
	"strconv"
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

func TestTraitPreviewHighlightCoversRenderedBlock(t *testing.T) {
	content := "---\nname: Raven\n---\n\n# Heading\n\nIntro paragraph with enough words to wrap in a narrow preview pane.\n\n- short item\n- this is a long list item containing Ship it @priority(high) and extra words so wrapping is guaranteed in the preview\n- another item\n"
	line := sourceLineContaining(t, content, "Ship it @priority(high)")
	m := loadBrowse(t)
	m.width, m.height = 100, 28
	m.previewOn = true
	m.previewWidth = 40
	m.scrollNeedle = "Ship it"
	m.previewLine = line
	m.p().objects = []api.Object{{
		ID: "projects/raven.md:trait:0", Kind: "trait", Title: "Ship it",
		Fields:   map[string]any{"content": "Ship it @priority(high)", "trait_type": "priority", "value": "high"},
		FilePath: "projects/raven.md", Line: line, ObjectID: "projects/raven",
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	m.setPreview(&api.ReadResult{
		ObjectID: "projects/raven",
		Path:     "projects/raven.md",
		Content:  content,
	})
	out := ansi.Strip(m.viewPreview(48, 22))
	if !strings.Contains(out, "projects/raven.md:"+strconv.Itoa(line)) {
		t.Fatalf("missing path:line:\n%s", out)
	}
	marked := markedPreviewLines(out)
	if len(marked) == 0 {
		t.Fatalf("no marked lines:\n%s", out)
	}
	joined := strings.Join(marked, "\n")
	if !strings.Contains(joined, "Ship it") {
		t.Fatalf("highlight missed trait text:\n%s\nfull:\n%s", joined, out)
	}
	if !strings.Contains(joined, "long list item") {
		t.Fatalf("highlight missed wrapped list item start:\n%s", joined)
	}
	if strings.Contains(joined, "short item") || strings.Contains(joined, "another item") {
		t.Fatalf("highlight leaked to neighboring items:\n%s", joined)
	}
	if !strings.Contains(out, "Ship it") {
		t.Fatalf("scroll missed trait:\n%s", out)
	}
}

func TestBacklinkPreviewHighlightsSnippetNotHeading(t *testing.T) {
	content := "---\nname: Odin\n---\n\n# Odin\n\nA long intro paragraph that reflows under Glamour so source line 2 is not rendered line 2.\n\nSee [[projects/a]] on this line of the note.\n"
	src := &api.ReadResult{
		ObjectID: "people/odin",
		Path:     "people/odin.md",
		Content:  content,
	}
	m := loadBrowse(t)
	m.width, m.height = 120, 36
	m.readCache["people/odin"] = src
	note := &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  "Alpha\n",
		Backlinks: []api.BacklinkGroup{{
			Source:  "people/odin",
			Title:   "Odin",
			Line:    sourceLineContaining(t, content, "[[projects/a]]"),
			Snippet: "See [[projects/a]] on this line of the note.",
			Lines:   []string{"See [[projects/a]] on this line of the note."},
		}},
	}
	m = applyNote(t, m, note)
	m.pane = paneBacklinks
	m.backIdx = 0
	out := ansi.Strip(m.viewBacklinkPreview(40, 16))
	marked := markedPreviewLines(out)
	if len(marked) == 0 {
		t.Fatalf("no marked backlink lines:\n%s", out)
	}
	joined := strings.Join(marked, "\n")
	if !strings.Contains(strings.ToLower(joined), "projects/a") && !strings.Contains(joined, "this line") {
		t.Fatalf("highlight missed snippet:\n%s\nfull:\n%s", joined, out)
	}
	if strings.Contains(joined, "long intro paragraph") {
		t.Fatalf("highlight landed on intro, not the backlink:\n%s", joined)
	}
}

func markedPreviewLines(out string) []string {
	var marked []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "▸") {
			marked = append(marked, line)
		}
	}
	return marked
}
