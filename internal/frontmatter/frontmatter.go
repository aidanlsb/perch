package frontmatter

import (
	"strings"
)

// Doc is a markdown file split into YAML-ish frontmatter and body.
// This is display-only. Perch does not parse wiki links from either part.
type Doc struct {
	Fields map[string]string
	Order  []string
	Body   string
	Raw    string
}

// Split pulls a leading --- ... --- block into key: value pairs.
func Split(content string) Doc {
	doc := Doc{Raw: content, Body: content, Fields: map[string]string{}}
	if !strings.HasPrefix(content, "---") {
		return doc
	}
	rest := content[3:]
	if len(rest) > 0 && (rest[0] == '\n' || rest[0] == '\r') {
		if rest[0] == '\r' && len(rest) > 1 && rest[1] == '\n' {
			rest = rest[2:]
		} else {
			rest = rest[1:]
		}
	} else {
		return doc
	}
	end := indexFence(rest)
	if end < 0 {
		return doc
	}
	block := rest[:end]
	body := rest[end:]
	body = strings.TrimPrefix(body, "---")
	body = strings.TrimPrefix(body, "\r\n")
	body = strings.TrimPrefix(body, "\n")
	doc.Body = body
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if _, exists := doc.Fields[key]; !exists {
			doc.Order = append(doc.Order, key)
		}
		doc.Fields[key] = val
	}
	return doc
}

func indexFence(s string) int {
	if strings.HasPrefix(s, "---") {
		return 0
	}
	idx := strings.Index(s, "\n---")
	if idx >= 0 {
		return idx + 1
	}
	idx = strings.Index(s, "\r\n---")
	if idx >= 0 {
		return idx + 2
	}
	return -1
}
