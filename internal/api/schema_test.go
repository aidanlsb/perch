package api

import "testing"

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

func TestDisplayTitle(t *testing.T) {
	ts := TypeSchema{NameField: "name"}
	o := Object{ID: "p/x", Fields: map[string]any{"name": "Hello"}}
	if got := DisplayTitle(o, ts); got != "Hello" {
		t.Fatalf("%q", got)
	}
}
