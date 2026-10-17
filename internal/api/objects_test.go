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
}
