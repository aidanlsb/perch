package frontmatter

import (
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	doc := Split("---\nname: Freya\ntype: person\n---\n\n# Hello [[x]]\n")
	if doc.Fields["name"] != "Freya" || doc.Fields["type"] != "person" {
		t.Fatalf("%+v", doc.Fields)
	}
	if !strings.Contains(doc.Body, "# Hello [[x]]") {
		t.Fatalf("body %q", doc.Body)
	}
}

func TestSplitNoFrontmatter(t *testing.T) {
	doc := Split("# just a note\n")
	if doc.Body != "# just a note\n" || len(doc.Fields) != 0 {
		t.Fatalf("%+v", doc)
	}
}
