package group

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func sampleProjects() []api.Object {
	return []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"status": "active", "due": "2026-10-14", "owner": "people/x"}},
		{ID: "b", Title: "B", Fields: map[string]any{"status": "idea", "due": "2026-10-01"}},
		{ID: "c", Title: "C", Fields: map[string]any{"status": "done", "due": "2026-09-20", "owner": "people/y"}},
		{ID: "d", Title: "D", Fields: map[string]any{}},
		{ID: "e", Title: "E", Fields: map[string]any{"status": "active", "owner": []any{"people/x", "people/z"}}},
	}
}

func TestBoardEnumOrderPlusEmpty(t *testing.T) {
	schema := api.TypeSchema{Fields: map[string]api.FieldSchema{
		"status": {Type: "enum", Values: []string{"idea", "active", "done"}},
	}}
	cols := Board(sampleProjects(), "status", schema)
	if len(cols) != 4 {
		t.Fatalf("len %d", len(cols))
	}
	if cols[0].Label != "idea" || cols[1].Label != "active" || cols[2].Label != "done" || cols[3].Label != emptyLabel {
		t.Fatalf("labels %+v", labels(cols))
	}
	if len(cols[1].Items) != 2 || len(cols[3].Items) != 1 {
		t.Fatalf("counts %+v", cols)
	}
}

func TestBoardGroupsObservedValuesWithoutSchema(t *testing.T) {
	objs := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"value": "high"}},
		{ID: "b", Title: "B", Fields: map[string]any{"value": "low"}},
		{ID: "c", Title: "C", Fields: map[string]any{"value": "high"}},
		{ID: "d", Title: "D"},
	}
	cols := Board(objs, "value", api.TypeSchema{})
	if len(cols) != 3 {
		t.Fatalf("labels %v", labels(cols))
	}
	if cols[0].Key != "high" || len(cols[0].Items) != 2 {
		t.Fatalf("high %+v", cols[0])
	}
	if cols[1].Key != "low" || len(cols[1].Items) != 1 {
		t.Fatalf("low %+v", cols[1])
	}
	if cols[2].Label != emptyLabel || len(cols[2].Items) != 1 {
		t.Fatalf("empty %+v", cols[2])
	}
}

func TestBoardRefAndArray(t *testing.T) {
	schema := api.TypeSchema{Fields: map[string]api.FieldSchema{
		"owner": {Type: "ref[]", Target: "person"},
	}}
	cols := Board(sampleProjects(), "owner", schema)
	// people/x, people/y, people/z, empty
	if len(cols) != 4 {
		t.Fatalf("labels %v", labels(cols))
	}
	if cols[0].Key != "people/x" || len(cols[0].Items) != 2 {
		t.Fatalf("x %+v", cols[0])
	}
}

func TestDateBucketsNewestFirst(t *testing.T) {
	cols := Date(sampleProjects(), "due", "month")
	if cols[0].Key != "2026-10" {
		t.Fatalf("first %s", cols[0].Key)
	}
	if cols[len(cols)-1].Label != emptyLabel {
		t.Fatalf("empty last %s", cols[len(cols)-1].Label)
	}
	days := Date(sampleProjects(), "due", "day")
	if days[0].Key != "2026-10-14" {
		t.Fatalf("day %s", days[0].Key)
	}
}

func TestFilterAndSort(t *testing.T) {
	objs := Filter(sampleProjects(), "active")
	if len(objs) != 2 {
		t.Fatalf("%d", len(objs))
	}
	sorted := SortObjects(sampleProjects(), "due")
	if sorted[0].ID != "c" {
		t.Fatalf("sort by due %+v", sorted)
	}
}

func TestSortByArbitraryFieldName(t *testing.T) {
	objs := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"xyz": "m"}},
		{ID: "b", Title: "B", Fields: map[string]any{"xyz": "a"}},
	}
	sorted := SortObjects(objs, "xyz")
	if sorted[0].ID != "b" {
		t.Fatalf("%v", sorted)
	}
	named := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"name": "z"}},
		{ID: "b", Title: "B", Fields: map[string]any{"name": "a"}},
	}
	sorted = SortObjects(named, "name")
	if sorted[0].ID != "b" {
		t.Fatalf("name is a field, not Title: %+v", sorted)
	}
}

func labels(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Label
	}
	return out
}
