package api

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// ParseFieldType splits a Raven field type spec such as "ref[]" into base and array.
func ParseFieldType(spec string) (base string, array bool) {
	spec = strings.TrimSpace(spec)
	if strings.HasSuffix(spec, "[]") {
		return strings.TrimSuffix(spec, "[]"), true
	}
	return spec, false
}

// IsEnum reports whether the field is a single or array enum.
func (f FieldSchema) IsEnum() bool {
	base, _ := ParseFieldType(f.Type)
	return base == "enum"
}

// IsDate reports whether the field is date or datetime (scalar or array).
func (f FieldSchema) IsDate() bool {
	base, _ := ParseFieldType(f.Type)
	return base == "date" || base == "datetime"
}

// IsRef reports whether the field is a ref or ref[].
func (f FieldSchema) IsRef() bool {
	base, _ := ParseFieldType(f.Type)
	return base == "ref"
}

// IsArray reports whether the field type has a [] suffix.
func (f FieldSchema) IsArray() bool {
	_, array := ParseFieldType(f.Type)
	return array
}

// SortedTypeNames returns type names for the sidebar. "section" is omitted
// because Raven rejects type:section queries.
func (s Schema) SortedTypeNames() []string {
	names := make([]string, 0, len(s.Types))
	for name := range s.Types {
		if name == "section" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SortedSavedQueries returns saved queries from the schema payload, sorted by name.
func (s Schema) SortedSavedQueries() []SavedQuery {
	if len(s.Queries) == 0 {
		return nil
	}
	names := make([]string, 0, len(s.Queries))
	for name := range s.Queries {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]SavedQuery, 0, len(names))
	for _, name := range names {
		q := s.Queries[name]
		if q.Name == "" {
			q.Name = name
		}
		out = append(out, q)
	}
	return out
}

// UnmarshalJSON records field definition order from the JSON object keys.
func (t *TypeSchema) UnmarshalJSON(data []byte) error {
	type alias TypeSchema
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = TypeSchema(raw)
	t.FieldOrder = jsonObjectKeys(data, "fields")
	return nil
}

// FieldNames returns field names for list columns: name_field first (from
// the schema), then the remaining fields in schema definition order.
func (t TypeSchema) FieldNames() []string {
	order := t.schemaFieldOrder()
	if t.NameField == "" {
		return order
	}
	if _, ok := t.Fields[t.NameField]; !ok {
		return order
	}
	out := make([]string, 0, len(order))
	out = append(out, t.NameField)
	for _, name := range order {
		if name != t.NameField {
			out = append(out, name)
		}
	}
	return out
}

// schemaFieldOrder is definition order when known, else alphabetical.
// Names are not preferred.
func (t TypeSchema) schemaFieldOrder() []string {
	if len(t.Fields) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(t.Fields))
	out := make([]string, 0, len(t.Fields))
	for _, name := range t.FieldOrder {
		if _, ok := t.Fields[name]; ok && !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}
	rest := make([]string, 0, len(t.Fields)-len(out))
	for name := range t.Fields {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// EnumFields returns scalar enum fields in schema definition order.
func (t TypeSchema) EnumFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsEnum() && !f.IsArray() })
}

// DateFields returns scalar date/datetime fields in schema definition order.
func (t TypeSchema) DateFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsDate() && !f.IsArray() })
}

// RefFields returns ref and ref[] fields (board group-by allows both).
func (t TypeSchema) RefFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsRef() })
}

func (t TypeSchema) fieldsMatching(ok func(FieldSchema) bool) []string {
	var names []string
	for _, name := range t.schemaFieldOrder() {
		if ok(t.Fields[name]) {
			names = append(names, name)
		}
	}
	return names
}

func jsonObjectKeys(data []byte, field string) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil
		}
		name, _ := key.(string)
		if name == field {
			return decodeObjectKeyOrder(dec)
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil
		}
	}
	return nil
}

func decodeObjectKeyOrder(dec *json.Decoder) []string {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return nil
		}
		name, ok := k.(string)
		if !ok {
			return nil
		}
		keys = append(keys, name)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil
		}
	}
	return keys
}

// DisplayTitle uses the schema name_field, then the query row's Title, then id.
// It does not guess vault field names.
func DisplayTitle(obj Object, typeSchema TypeSchema) string {
	if typeSchema.NameField != "" {
		if s := ScalarString(obj.Fields[typeSchema.NameField]); s != "" {
			return s
		}
	}
	if obj.Title != "" {
		return obj.Title
	}
	return obj.ID
}

// ScalarString renders a JSON field value as a single display string.
func ScalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := ScalarString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		if s, ok := t["id"].(string); ok {
			return s
		}
		if s, ok := t["path"].(string); ok {
			return s
		}
	}
	return strings.TrimSpace(strings.Trim(strings.ReplaceAll(fmtSprint(v), "\n", " "), "[]"))
}

func fmtSprint(v any) string {
	return strings.TrimSpace(stringify(v))
}

func stringify(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := strings.Trim(string(b), `"`)
	return s
}

// ScalarStrings extracts zero or more string tokens from a field (arrays included).
func ScalarStrings(v any) []string {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s := ScalarString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s := ScalarString(v); s != "" {
		return []string{s}
	}
	return nil
}
