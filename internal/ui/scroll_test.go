package ui

import "testing"

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
	content := "a\nb\nc\nd\n"
	rendered := content
	if got := sourceLineScroll(content, rendered, 3); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := sourceLineScroll(content, rendered, 99); got != 4 {
		t.Fatalf("clamp %d", got)
	}
}
