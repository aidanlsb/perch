package api

import (
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

// FieldNames returns field names in a stable order: name_field first, then alpha.
func (t TypeSchema) FieldNames() []string {
	if len(t.Fields) == 0 {
		return nil
	}
	names := make([]string, 0, len(t.Fields))
	for name := range t.Fields {
		if t.NameField != "" && name == t.NameField {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if t.NameField != "" {
		if _, ok := t.Fields[t.NameField]; ok {
			names = append([]string{t.NameField}, names...)
		}
	}
	return names
}

// EnumFields returns enum fields in FieldNames order.
func (t TypeSchema) EnumFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsEnum() && !f.IsArray() })
}

// DateFields returns date/datetime scalar fields.
func (t TypeSchema) DateFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsDate() && !f.IsArray() })
}

// RefFields returns ref and ref[] fields (board group-by allows both).
func (t TypeSchema) RefFields() []string {
	return t.fieldsMatching(func(f FieldSchema) bool { return f.IsRef() })
}

func (t TypeSchema) fieldsMatching(ok func(FieldSchema) bool) []string {
	var names []string
	for _, name := range t.FieldNames() {
		if ok(t.Fields[name]) {
			names = append(names, name)
		}
	}
	return names
}

// DisplayTitle picks a human title from name_field, then common keys, then id.
func DisplayTitle(obj Object, typeSchema TypeSchema) string {
	if typeSchema.NameField != "" {
		if s := ScalarString(obj.Fields[typeSchema.NameField]); s != "" {
			return s
		}
	}
	for _, key := range []string{"name", "title", "alias"} {
		if s := ScalarString(obj.Fields[key]); s != "" {
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
