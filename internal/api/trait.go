package api

import (
	"strconv"
	"strings"
)

const (
	colContent   = "content"
	colTraitType = "trait_type"
	colValue     = "value"
	colSource    = "source"
)

// TraitListColumns is the default list layout for query_kind=trait rows.
// Content is the primary readable text; the rest is source context.
var TraitListColumns = []string{colContent, colTraitType, colValue, colSource}

// IsTrait reports whether this row came from a trait query.
func (o Object) IsTrait() bool {
	return strings.EqualFold(strings.TrimSpace(o.Kind), "trait")
}

// TraitTitle is the primary label for a trait row: cleaned content when
// present, otherwise a short parent object label. Value and the raw trait
// id are never the title — they belong in secondary columns.
func TraitTitle(raw map[string]any, obj Object) string {
	content := traitContent(raw, obj)
	if content != "" {
		return content
	}
	if label := ParentLabel(obj.ObjectID); label != "" {
		return label
	}
	return ""
}

func traitContent(raw map[string]any, obj Object) string {
	s := firstString(raw, "content")
	if s == "" {
		s = ScalarString(obj.Fields["content"])
	}
	return firstLine(PlainWikilinks(s))
}

// HasTraitContent reports whether the trait row's content field is non-empty.
func (o Object) HasTraitContent() bool {
	return firstLine(ScalarString(o.Fields["content"])) != ""
}

// ParentLabel is a short fallback title from a containing object_id
// (file object, last path segment, section suffix dropped).
func ParentLabel(objectID string) string {
	id := fileObjectRef(objectID)
	if id == "" {
		return ""
	}
	if i := strings.LastIndex(id, "/"); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

// TraitSource is where the trait lives: file_path:line, else object_id.
func TraitSource(obj Object) string {
	path := strings.TrimSpace(obj.FilePath)
	if path != "" && obj.Line > 0 {
		return path + ":" + strconv.Itoa(obj.Line)
	}
	if id := strings.TrimSpace(obj.ObjectID); id != "" {
		return id
	}
	return path
}

// PlainWikilinks replaces [[target|alias]] with alias and [[target]] with
// target so trait content is readable in list/board/date labels. Trait tags
// such as @name(value) are left as written.
func PlainWikilinks(s string) string {
	if !strings.Contains(s, "[[") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "[[")
		if start < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+start])
		rest := s[i+start+2:]
		end := strings.Index(rest, "]]")
		if end < 0 {
			b.WriteString(s[i+start:])
			break
		}
		inner := strings.TrimSpace(rest[:end])
		if pipe := strings.LastIndex(inner, "|"); pipe >= 0 {
			inner = strings.TrimSpace(inner[pipe+1:])
		}
		b.WriteString(inner)
		i = i + start + 2 + end + 2
	}
	return b.String()
}
