package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/termesc"
)

// sourceSpan is a range of rendered Markdown lines that correspond to a
// 1-based source file line (and optional search needles such as trait
// content or a backlink snippet).
type sourceSpan struct {
	start  int // inclusive rendered line index
	end    int // inclusive; end < start means no highlight
	scroll int
}

func (s sourceSpan) ok() bool {
	return s.end >= s.start && s.start >= 0
}

// sourceLineScroll maps a 1-based source file line onto a viewport offset in
// rendered Markdown. Prefer mapSourceSpan when a content needle is available:
// Glamour reflows headings, lists, and wrapped paragraphs so source line
// numbers are not 1:1 with rendered lines.
func sourceLineScroll(content, rendered string, line int) int {
	return mapSourceSpan(content, rendered, line).scroll
}

func mapSourceSpan(content, rendered string, line int, needles ...string) sourceSpan {
	renderedLines := strings.Split(rendered, "\n")
	nLines := len(renderedLines)
	if nLines == 0 {
		return sourceSpan{start: 0, end: -1, scroll: 0}
	}
	block := sourceLineBlock(content, line)
	search := make([]string, 0, len(needles)+1)
	search = append(search, needles...)
	if block != "" {
		search = append(search, block)
	}
	if start, end, ok := findRenderedRange(renderedLines, search); ok {
		return clampSpan(start, end, nLines)
	}
	return legacySourceSpan(content, rendered, line, nLines)
}

func clampSpan(start, end, n int) sourceSpan {
	if start < 0 {
		start = 0
	}
	if end >= n {
		end = n - 1
	}
	if end < start {
		return sourceSpan{start: 0, end: -1, scroll: 0}
	}
	return sourceSpan{start: start, end: end, scroll: start}
}

func legacySourceSpan(content, rendered string, line, nRendered int) sourceSpan {
	if line <= 1 {
		return sourceSpan{start: 0, end: -1, scroll: 0}
	}
	off := legacyBodyOffset(content, line)
	if off <= 0 {
		return sourceSpan{start: 0, end: -1, scroll: 0}
	}
	if off >= nRendered {
		off = nRendered - 1
	}
	return sourceSpan{start: off, end: off, scroll: off}
}

func legacyBodyOffset(content string, line int) int {
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
	return bodyLine
}

func sourceLineBlock(content string, line int) string {
	if content == "" || line <= 0 {
		return ""
	}
	doc := frontmatter.Split(content)
	prefixLen := 0
	if doc.Body != "" {
		if strings.HasSuffix(content, doc.Body) {
			prefixLen = len(content) - len(doc.Body)
		} else if i := strings.Index(content, doc.Body); i >= 0 {
			prefixLen = i
		}
	}
	fmLines := 0
	if prefixLen > 0 && prefixLen <= len(content) {
		fmLines = strings.Count(content[:prefixLen], "\n")
	}
	if line <= fmLines {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		line = len(lines)
	}
	idx := line - 1
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	start, end := blockRange(lines, idx)
	return strings.Join(lines[start:end+1], "\n")
}

func blockRange(lines []string, idx int) (start, end int) {
	start, end = idx, idx
	if isFence(lines[idx]) {
		end = idx
		for end+1 < len(lines) && !isFence(lines[end+1]) {
			end++
		}
		if end+1 < len(lines) {
			end++
		}
		return start, end
	}
	if item := listItemStart(lines, idx); item >= 0 {
		indent := leadingIndent(lines[item])
		end = item
		for end+1 < len(lines) {
			n := lines[end+1]
			if strings.TrimSpace(n) == "" {
				break
			}
			if isListStart(n) && leadingIndent(n) <= indent {
				break
			}
			if isHeading(n) || isFence(n) {
				break
			}
			end++
		}
		return item, end
	}
	if isHeading(lines[idx]) {
		return idx, idx
	}
	for start > 0 {
		prev := lines[start-1]
		if strings.TrimSpace(prev) == "" || isHeading(prev) || isFence(prev) || isListStart(prev) {
			break
		}
		start--
	}
	for end+1 < len(lines) {
		next := lines[end+1]
		if strings.TrimSpace(next) == "" || isHeading(next) || isFence(next) || isListStart(next) {
			break
		}
		end++
	}
	return start, end
}

func listItemStart(lines []string, idx int) int {
	if isListStart(lines[idx]) {
		return idx
	}
	indent := leadingIndent(lines[idx])
	for i := idx; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			return -1
		}
		if isListStart(lines[i]) && leadingIndent(lines[i]) <= indent {
			return i
		}
		if isHeading(lines[i]) || isFence(lines[i]) {
			return -1
		}
	}
	return -1
}

func isListStart(s string) bool {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ ") ||
		strings.HasPrefix(s, "-\t") || strings.HasPrefix(s, "*\t") || strings.HasPrefix(s, "+\t") {
		return true
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') && i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\t')
}

func isHeading(s string) bool {
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "#") {
		return false
	}
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	return i > 0 && i <= 6 && (i == len(s) || s[i] == ' ' || s[i] == '\t')
}

func isFence(s string) bool {
	s = strings.TrimLeft(s, " \t")
	return strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~")
}

func leadingIndent(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
			continue
		}
		if r == '\t' {
			n += 4
			continue
		}
		break
	}
	return n
}

func findRenderedRange(lines []string, needles []string) (start, end int, ok bool) {
	plains := make([]string, len(lines))
	for i, l := range lines {
		plains[i] = foldPreviewText(l)
	}
	for _, needle := range needles {
		folded := foldPreviewText(needle)
		if utf8.RuneCountInString(folded) < 3 {
			continue
		}
		if s, e, found := matchFoldedRange(plains, folded); found {
			s, e = expandRenderedBlock(lines, s, e)
			return s, e, true
		}
	}
	return 0, 0, false
}

func matchFoldedRange(plains []string, needle string) (start, end int, ok bool) {
	for i, p := range plains {
		if p == "" {
			continue
		}
		if strings.Contains(p, needle) {
			return i, i, true
		}
	}
	bestStart, bestEnd := -1, -1
	for i := 0; i < len(plains); i++ {
		var b strings.Builder
		for j := i; j < len(plains); j++ {
			if plains[j] == "" {
				if b.Len() == 0 {
					break
				}
				break
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(plains[j])
			cat := b.String()
			if strings.Contains(cat, needle) {
				if bestStart < 0 || j-i < bestEnd-bestStart {
					bestStart, bestEnd = i, j
				}
				break
			}
			if len(cat) > len(needle)+64 {
				break
			}
		}
	}
	if bestStart < 0 {
		return 0, 0, false
	}
	return bestStart, bestEnd, true
}

func expandRenderedBlock(lines []string, start, end int) (int, int) {
	plain := func(i int) string {
		if i < 0 || i >= len(lines) {
			return ""
		}
		return strings.TrimSpace(termesc.Plain(lines[i]))
	}
	if start > 0 && !renderedBlockStart(plain(start)) {
		for start > 0 {
			prev := plain(start - 1)
			if prev == "" {
				break
			}
			start--
			if renderedBlockStart(prev) {
				break
			}
		}
	}
	for end+1 < len(lines) {
		next := plain(end + 1)
		if next == "" || renderedBlockStart(next) {
			break
		}
		end++
	}
	return start, end
}

func renderedBlockStart(s string) bool {
	if s == "" {
		return true
	}
	switch {
	case strings.HasPrefix(s, "#"):
		return true
	case strings.HasPrefix(s, "•"), strings.HasPrefix(s, "‣"),
		strings.HasPrefix(s, "●"), strings.HasPrefix(s, "○"),
		strings.HasPrefix(s, "–"), strings.HasPrefix(s, "—"):
		return true
	}
	return isListStart(s)
}

func foldPreviewText(s string) string {
	s = termesc.Plain(s)
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	trimLead := true
	for _, r := range s {
		if trimLead {
			if unicode.IsSpace(r) {
				continue
			}
			switch r {
			case '•', '‣', '●', '○', '–', '—', '-', '*', '+', '>', '|', '#':
				continue
			}
			if r >= '0' && r <= '9' {
				continue
			}
			if r == '.' || r == ')' {
				continue
			}
			trimLead = false
		}
		if unicode.IsSpace(r) {
			if lastSpace {
				continue
			}
			b.WriteByte(' ')
			lastSpace = true
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
