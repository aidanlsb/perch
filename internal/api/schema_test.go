package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFieldType(t *testing.T) {
	base, arr := ParseFieldType("ref[]")
	if base != "ref" || !arr {
		t.Fatalf("%s %v", base, arr)
	}
}

func TestSortedTypeNamesSkipsSection(t *testing.T) {
	s := Schema{Types: map[string]TypeSchema{
		"project": {Name: "project"},
		"section": {Name: "section", Builtin: true},
		"date":    {Name: "date", Builtin: true},
	}}
	names := s.SortedTypeNames()
	if len(names) != 2 || names[0] != "date" || names[1] != "project" {
		t.Fatalf("%v", names)
	}
}

func TestDisplayTitleUsesNameFieldOnly(t *testing.T) {
	o := Object{ID: "p/x", Title: "FromAPI", Fields: map[string]any{
		"name": "NameFieldVal", "title": "TitleField", "alias": "AliasField", "xyz": "XYZ",
	}}
	if got := DisplayTitle(o, TypeSchema{NameField: "xyz"}); got != "XYZ" {
		t.Fatalf("name_field xyz: %q", got)
	}
	if got := DisplayTitle(o, TypeSchema{NameField: "name"}); got != "NameFieldVal" {
		t.Fatalf("name_field name: %q", got)
	}
	if got := DisplayTitle(o, TypeSchema{}); got != "FromAPI" {
		t.Fatalf("no name_field should use Title, not name/title/alias: %q", got)
	}
	if got := DisplayTitle(Object{ID: "p/x"}, TypeSchema{}); got != "p/x" {
		t.Fatalf("id fallback %q", got)
	}
}

func TestTypeSchemaFieldOrderFromJSON(t *testing.T) {
	raw := []byte(`{
		"name": "item",
		"name_field": "label",
		"fields": {
			"label": {"type": "string"},
			"xyz": {"type": "enum", "values": ["a", "b"]},
			"when": {"type": "date"}
		}
	}`)
	var ts TypeSchema
	if err := json.Unmarshal(raw, &ts); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ts.FieldOrder, ","); got != "label,xyz,when" {
		t.Fatalf("order %v", ts.FieldOrder)
	}
	if enums := ts.EnumFields(); len(enums) != 1 || enums[0] != "xyz" {
		t.Fatalf("enums %v", enums)
	}
	if dates := ts.DateFields(); len(dates) != 1 || dates[0] != "when" {
		t.Fatalf("dates %v", dates)
	}
	names := ts.FieldNames()
	if len(names) != 3 || names[0] != "label" || names[1] != "xyz" || names[2] != "when" {
		t.Fatalf("field names %v", names)
	}
}

func TestEnumFieldsNameAgnostic(t *testing.T) {
	raw := func(enum, date string) []byte {
		return []byte(`{"fields":{"` + enum + `":{"type":"enum","values":["a"]},"` + date + `":{"type":"date"}}}`)
	}
	var a, b TypeSchema
	if err := json.Unmarshal(raw("xyz", "when"), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw("status", "date"), &b); err != nil {
		t.Fatal(err)
	}
	if a.EnumFields()[0] != "xyz" || b.EnumFields()[0] != "status" {
		t.Fatalf("enums %v %v", a.EnumFields(), b.EnumFields())
	}
	if a.DateFields()[0] != "when" || b.DateFields()[0] != "date" {
		t.Fatalf("dates %v %v", a.DateFields(), b.DateFields())
	}
}
