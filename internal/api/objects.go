package api

import "strings"

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
		obj.Title = DisplayTitle(obj, typeSchema)
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
	if id, ok := raw["id"].(string); ok {
		obj.ID = id
	}
	if obj.ID == "" {
		if id, ok := raw["object_id"].(string); ok {
			obj.ID = id
		}
	}
	if obj.ID == "" {
		if id, ok := raw["source_id"].(string); ok {
			obj.ID = id
		}
	}
	if t, ok := raw["type"].(string); ok && t != "" {
		obj.Type = t
	}
	if p, ok := raw["file_path"].(string); ok {
		obj.FilePath = p
	}
	if fields, ok := raw["fields"].(map[string]any); ok {
		obj.Fields = fields
	} else {
		for k, v := range raw {
			switch k {
			case "id", "num", "type", "file_path", "line", "fields":
				continue
			default:
				obj.Fields[k] = v
			}
		}
	}
	if title, ok := raw["title"].(string); ok {
		obj.Title = title
	}
	if obj.ID == "" {
		obj.ID = ScalarString(raw["id"])
	}
	return obj
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
