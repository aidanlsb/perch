package api

import "testing"

func TestObjectsFromCorpus(t *testing.T) {
	got := ObjectsFromCorpus([]ObjectSummary{
		{ID: "people/freya", Type: "person", Name: "Freya", FilePath: "people/freya.md"},
		{ID: "2026-10-15", Type: "date", FilePath: "daily/2026-10-15.md"},
	})
	if len(got) != 2 || got[0].Title != "Freya" || got[0].FilePath != "people/freya.md" {
		t.Fatalf("%+v", got)
	}
	if got[1].Title != "2026-10-15" {
		t.Fatalf("id fallback title %q", got[1].Title)
	}
	if _, ok := got[0].Fields["name"]; ok {
		t.Fatalf("corpus must not invent a name field: %+v", got[0].Fields)
	}
	if !got[0].IsFileObject() || got[0].OpenRef() != "people/freya" {
		t.Fatalf("corpus row should be a file object: %+v", got[0])
	}
}

func TestObjectsFromQueryTraitRows(t *testing.T) {
	qr := &QueryResult{
		QueryKind: "trait",
		Trait:     "priority",
		Items: []map[string]any{
			{
				"num":        1.0,
				"id":         "projects/raven.md:trait:0",
				"trait_type": "priority",
				"value":      "high",
				"content":    "Ship it",
				"file_path":  "projects/raven.md",
				"line":       12.0, // JSON numbers decode as float64
				"object_id":  "projects/raven",
			},
			{
				"id":        "projects/raven.md:trait:1",
				"content":   "Nested under a heading",
				"file_path": "projects/raven.md",
				"line":      28.0,
				"object_id": "projects/raven#tasks",
			},
		},
	}
	got := ObjectsFromQuery(qr, TypeSchema{})
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	row := got[0]
	if row.ID != "projects/raven.md:trait:0" {
		t.Fatalf("row id %q", row.ID)
	}
	if row.ObjectID != "projects/raven" {
		t.Fatalf("object_id %q", row.ObjectID)
	}
	if row.OpenRef() != "projects/raven" {
		t.Fatalf("OpenRef %q", row.OpenRef())
	}
	if row.BindID() != "projects/raven" {
		t.Fatalf("BindID %q", row.BindID())
	}
	if row.Line != 12 || row.FilePath != "projects/raven.md" {
		t.Fatalf("line/path %+v", row)
	}
	if row.Kind != "trait" || row.IsFileObject() {
		t.Fatalf("kind/nested %+v", row)
	}
	if row.Title != "Ship it" {
		t.Fatalf("title %q", row.Title)
	}
	if got[1].OpenRef() != "projects/raven" {
		t.Fatalf("section-scoped object_id should bind the file object, got %q", got[1].OpenRef())
	}
	if got[1].Line != 28 {
		t.Fatalf("line %d", got[1].Line)
	}
}

func TestObjectsFromQuerySectionAndLink(t *testing.T) {
	sec := ObjectsFromQuery(&QueryResult{
		QueryKind: "section",
		Items: []map[string]any{
			{
				"id":             "projects/raven#tasks",
				"file_object_id": "projects/raven",
				"file_path":      "projects/raven.md",
				"title":          "Tasks",
				"line_start":     20.0,
			},
		},
	}, TypeSchema{})
	if len(sec) != 1 || sec[0].OpenRef() != "projects/raven" || sec[0].Line != 20 || sec[0].Title != "Tasks" {
		t.Fatalf("%+v", sec)
	}
	if sec[0].IsFileObject() {
		t.Fatal("section row is not a file object")
	}

	link := ObjectsFromQuery(&QueryResult{
		QueryKind: "link",
		Items: []map[string]any{
			{
				"source_id": "projects/raven",
				"file_path": "projects/raven.md",
				"line":      9.0,
				"display":   "the spec",
			},
		},
	}, TypeSchema{})
	if len(link) != 1 || link[0].OpenRef() != "projects/raven" || link[0].Line != 9 {
		t.Fatalf("%+v", link)
	}
	if link[0].Title != "the spec" {
		t.Fatalf("title %q", link[0].Title)
	}
}

func TestObjectsFromQueryObjectRowUnchanged(t *testing.T) {
	got := ObjectsFromQuery(&QueryResult{
		QueryKind: "type",
		Type:      "project",
		Items: []map[string]any{
			{
				"id": "projects/raven", "type": "project",
				"fields":    map[string]any{"name": "Raven"},
				"file_path": "projects/raven.md",
				"line":      1.0,
			},
		},
	}, TypeSchema{NameField: "name", Fields: map[string]FieldSchema{"name": {Type: "string"}}})
	if len(got) != 1 || got[0].Title != "Raven" || got[0].OpenRef() != "projects/raven" {
		t.Fatalf("%+v", got)
	}
	if !got[0].IsFileObject() {
		t.Fatal("type query row should be a file object")
	}
}
