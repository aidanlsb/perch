package api

import (
	"strconv"
	"strings"
)

// ObjectsFromCorpus flattens `objects` summaries into view rows (id, name, path).
func ObjectsFromCorpus(items []ObjectSummary) []Object {
	out := make([]Object, 0, len(items))
	for _, it := range items {
		obj := Object{
			ID:       it.ID,
			Type:     it.Type,
			Title:    it.Name,
			FilePath: it.FilePath,
			Kind:     "object",
			Fields:   map[string]any{},
		}
		if obj.Title == "" {
			obj.Title = obj.ID
		}
		out = append(out, obj)
	}
	return out
}

// ObjectsFromQuery flattens a query payload into view rows.
func ObjectsFromQuery(q *QueryResult, typeSchema TypeSchema) []Object {
	if q == nil {
		return nil
	}
	out := make([]Object, 0, len(q.Items))
	kind := strings.TrimSpace(q.QueryKind)
	if kind == "" {
		kind = "object"
	}
	for _, raw := range q.Items {
		obj := objectFromItem(raw, kind, q.Type)
		if obj.IsTrait() {
			if obj.Title == "" {
				obj.Title = obj.ID
			}
		} else {
			obj.Title = DisplayTitle(obj, typeSchema)
		}
		out = append(out, obj)
	}
	return out
}

func objectFromItem(raw map[string]any, kind, typeName string) Object {
	obj := Object{
		Kind:   kind,
		Type:   typeName,
		Fields: map[string]any{},
	}
	obj.ID = stringField(raw, "id")
	obj.ObjectID = firstString(raw, "file_object_id", "object_id", "source_id")
	if t := stringField(raw, "type"); t != "" {
		obj.Type = t
	}
	obj.FilePath = stringField(raw, "file_path")
	obj.Line = intFromAny(raw["line"])
	if obj.Line == 0 {
		obj.Line = intFromAny(raw["line_start"])
	}
	if fields, ok := raw["fields"].(map[string]any); ok {
		obj.Fields = fields
	} else {
		for k, v := range raw {
			switch k {
			case "id", "num", "type", "file_path", "line", "line_start", "fields",
				"object_id", "file_object_id", "source_id":
				continue
			default:
				obj.Fields[k] = v
			}
		}
	}
	if obj.IsTrait() {
		obj.Title = TraitTitle(raw, obj)
	} else {
		if obj.Title == "" {
			obj.Title = firstLine(firstString(raw, "title", "content", "display"))
		}
		if obj.Title == "" {
			obj.Title = firstLine(ScalarString(raw["value"]))
		}
	}
	if obj.ID == "" {
		obj.ID = obj.ObjectID
	}
	return obj
}

// OpenRef is the object id to pass to `read`. Nested query rows (traits,
// sections, links) carry a containing object separately from the row id.
// Section-scoped ids in Raven are `fileObject#slug`; OpenRef returns the
// file object so the note view is the whole file, scrolled via Line.
func (o Object) OpenRef() string {
	id := strings.TrimSpace(o.ObjectID)
	if id == "" {
		id = strings.TrimSpace(o.ID)
	}
	return fileObjectRef(id)
}

// BindID is substituted for {{selected}}. Same as OpenRef: the containing
// file object, not a trait or section id.
func (o Object) BindID() string {
	return o.OpenRef()
}

// IsFileObject reports whether this row is itself a file-backed object.
func (o Object) IsFileObject() bool {
	switch strings.ToLower(strings.TrimSpace(o.Kind)) {
	case "trait", "section", "link":
		return false
	}
	if strings.TrimSpace(o.ID) == "" {
		return false
	}
	return o.OpenRef() == o.ID
}

func fileObjectRef(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.IndexByte(id, '#'); i > 0 {
		return id[:i]
	}
	return id
}

func stringField(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(ScalarString(raw[key]))
}

func firstString(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringField(raw, k); s != "" {
			return s
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		n, _ := strconv.Atoi(strings.TrimSpace(ScalarString(v)))
		return n
	}
}

// FollowTarget returns the object id to open for a wiki link, if any.
func (l ReadLink) FollowTarget() string {
	switch l.Status {
	case LinkResolved:
		if l.ObjectID != "" {
			return l.ObjectID
		}
	}
	if l.ObjectID != "" {
		return l.ObjectID
	}
	return ""
}

// SpanOK reports whether byte offsets slice content to the recorded text.
func (l ReadLink) SpanOK(content string) bool {
	if l.ByteStart < 0 || l.ByteEnd > len(content) || l.ByteStart >= l.ByteEnd {
		return false
	}
	return content[l.ByteStart:l.ByteEnd] == l.Text
}
