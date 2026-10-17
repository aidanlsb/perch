package layout

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func itemSchema() api.TypeSchema {
	return api.TypeSchema{
		Name:       "item",
		NameField:  "label",
		FieldOrder: []string{"label", "xyz", "when", "owner"},
		Fields: map[string]api.FieldSchema{
			"label": {Type: "string"},
			"xyz":   {Type: "enum", Values: []string{"a", "b", "c"}},
			"when":  {Type: "date"},
			"owner": {Type: "ref", Target: "person"},
		},
	}
}

func TestDefaultListOnly(t *testing.T) {
	ls := DefaultList(itemSchema())
	if ls.Kind != KindList || ls.Name != "List" || ls.Sort != "label" {
		t.Fatalf("%+v", ls)
	}
	empty := DefaultList(api.TypeSchema{
		Fields: map[string]api.FieldSchema{
			"xyz":  {Type: "enum", Values: []string{"a"}},
			"when": {Type: "date"},
		},
	})
	if empty.Kind != KindList || empty.GroupBy != "" {
		t.Fatalf("browse must not invent board/date, got %+v", empty)
	}
}

func TestNextGranularity(t *testing.T) {
	s := Spec{Kind: KindDate, GroupBy: "when", Granularity: GranDay}
	s = NextGranularity(s)
	if s.Granularity != GranWeek {
		t.Fatal(s.Granularity)
	}
	s = NextGranularity(s)
	if s.Granularity != GranMonth {
		t.Fatal(s.Granularity)
	}
	s = NextGranularity(s)
	if s.Granularity != GranDay {
		t.Fatal(s.Granularity)
	}
	list := NextGranularity(Spec{Kind: KindList})
	if list.Kind != KindList {
		t.Fatal(list)
	}
}

func TestAppendFilter(t *testing.T) {
	if got := AppendFilter("type:item", ".xyz==a"); got != "type:item .xyz==a" {
		t.Fatal(got)
	}
	if got := AppendFilter("type:item", ""); got != "type:item" {
		t.Fatal(got)
	}
}

func TestApplyBoardGroups(t *testing.T) {
	ts := itemSchema()
	objs := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"xyz": "b"}},
		{ID: "b", Title: "B", Fields: map[string]any{"xyz": "idea"}},
	}
	r := Apply(objs, Spec{Kind: KindBoard, GroupBy: "xyz"}, ts, "", false)
	if len(r.Board) < 3 {
		t.Fatalf("cols %d", len(r.Board))
	}
	r = Apply(objs, Spec{Kind: KindList}, ts, "ida", true)
	if len(r.Objects) != 1 || r.Objects[0].Title != "B" {
		t.Fatalf("fuzzy %+v", r.Objects)
	}
}

func TestNormalizeInfersKindFromGroupBy(t *testing.T) {
	s := Spec{GroupBy: "xyz"}.Normalize()
	if s.Kind != KindBoard {
		t.Fatal(s.Kind)
	}
	s = Spec{GroupBy: "when", Granularity: GranWeek}.Normalize()
	if s.Kind != KindDate {
		t.Fatal(s.Kind)
	}
}
