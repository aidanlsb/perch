package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/frontmatter"
)

// sourceLineScroll maps a 1-based source file line onto a viewport offset in
// rendered Markdown. Frontmatter is shown as a header, so lines inside it
// stay at offset 0. The body offset is the source line minus the frontmatter
// line count, clamped to the rendered body.
func sourceLineScroll(content, rendered string, line int) int {
	if line <= 1 {
		return 0
	}
	doc := frontmatter.Split(content)
	prefixLen := 0
	if doc.Body != "" {
		if strings.HasSuffix(content, doc.Body) {
			prefixLen = len(content) - len(doc.Body)
		} else if i := strings.Index(content, doc.Body); i >= 0 {
			prefixLen = i
		}
	} else {
		prefixLen = len(content)
	}
	if prefixLen < 0 {
		prefixLen = 0
	}
	if prefixLen > len(content) {
		prefixLen = len(content)
	}
	fmLines := strings.Count(content[:prefixLen], "\n")
	bodyLine := line - fmLines - 1
	if bodyLine <= 0 {
		return 0
	}
	n := strings.Count(rendered, "\n")
	if bodyLine > n {
		return n
	}
	return bodyLine
}
