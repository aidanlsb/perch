package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/md"
	"github.com/charmbracelet/x/ansi"
)

func TestSourceLineScrollFrontmatter(t *testing.T) {
	content := "---\nname: Raven\n---\n\n# Heading\n\nbody line\n"
	rendered := "# Heading\n\nbody line"
	if got := sourceLineScroll(content, rendered, 1); got != 0 {
		t.Fatalf("frontmatter line: %d", got)
	}
	if got := sourceLineScroll(content, rendered, 7); got <= 0 {
		t.Fatalf("body line should scroll, got %d", got)
	}
}

func TestSourceLineScrollNoFrontmatter(t *testing.T) {
	content := "alpha\n\nbravo\n\ncharlie\n\ndelta\n"
	rendered := content
	if got := sourceLineScroll(content, rendered, 5); got != 4 {
		t.Fatalf("got %d", got)
	}
	span := mapSourceSpan(content, rendered, 5)
	if !span.ok() || span.start != 4 || span.end != 4 {
		t.Fatalf("span %+v", span)
	}
}

func TestMapSourceSpanFindsWrappedListItem(t *testing.T) {
	content := "---\nname: Raven\n---\n\n# Heading\n\nIntro paragraph with enough words to wrap in a narrow preview pane.\n\n- short item\n- this is a long list item containing Ship it @priority(high) and extra words so wrapping is guaranteed in the preview\n- another item\n"
	line := sourceLineContaining(t, content, "Ship it @priority(high)")
	rendered := md.Render(mustBody(content), 36)
	span := mapSourceSpan(content, rendered, line, "Ship it")
	if !span.ok() {
		t.Fatalf("no span for line %d\nrendered:\n%s", line, ansi.Strip(rendered))
	}
	lines := strings.Split(ansi.Strip(rendered), "\n")
	block := strings.Join(lines[span.start:span.end+1], "\n")
	if !strings.Contains(block, "Ship it") {
		t.Fatalf("span %d-%d missed trait text:\n%s\nfull:\n%s", span.start, span.end, block, ansi.Strip(rendered))
	}
	if !strings.Contains(block, "long list item") {
		t.Fatalf("span missed the start of the wrapped item:\n%s", block)
	}
	if strings.Contains(block, "short item") {
		t.Fatalf("span leaked onto previous item:\n%s", block)
	}
	if strings.Contains(block, "another item") {
		t.Fatalf("span leaked onto next item:\n%s", block)
	}
	legacy := legacyBodyOffset(content, line)
	if span.start == legacy && strings.Count(ansi.Strip(rendered), "\n") != strings.Count(mustBody(content), "\n") {
		t.Logf("rendered reflowed; span.start=%d legacy=%d (expected to differ)", span.start, legacy)
	}
	if span.scroll != span.start {
		t.Fatalf("scroll %d want start %d", span.scroll, span.start)
	}
}

func TestMapSourceSpanUsesNeedleOverWrongLine(t *testing.T) {
	content := traitNoteContent()
	rendered := md.Render(mustBody(content), 40)
	// API line 18 is in the filler paragraphs; content is at the end.
	span := mapSourceSpan(content, rendered, 18, "Ship it")
	if !span.ok() {
		t.Fatal("expected needle match")
	}
	lines := strings.Split(ansi.Strip(rendered), "\n")
	block := strings.Join(lines[span.start:span.end+1], "\n")
	if !strings.Contains(block, "Ship it") {
		t.Fatalf("needle should win over source line 18:\n%s", block)
	}
}

func sourceLineContaining(t *testing.T, content, needle string) int {
	t.Helper()
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("needle %q not in content", needle)
	return 0
}

func mustBody(content string) string {
	return frontmatter.Split(content).Body
}
