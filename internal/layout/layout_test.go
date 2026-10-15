package layout

import (
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func projectSchema() api.TypeSchema {
	return api.TypeSchema{
		Name:      "project",
		NameField: "name",
		Fields: map[string]api.FieldSchema{
			"name":   {Type: "string"},
			"status": {Type: "enum", Values: []string{"idea", "active", "done"}},
			"due":    {Type: "date"},
			"owner":  {Type: "ref", Target: "person"},
		},
	}
}

func TestInferStatusBoard(t *testing.T) {
	s := Infer(projectSchema())
	if s.Kind != KindBoard || s.GroupBy != "status" {
		t.Fatalf("%+v", s)
	}
}

func TestInferDateType(t *testing.T) {
	ts := api.TypeSchema{
		Name: "date",
		Fields: map[string]api.FieldSchema{
			"date": {Type: "date"},
			"name": {Type: "string"},
		},
	}
	s := Infer(ts)
	if s.Kind != KindDate || s.GroupBy != "date" || s.Granularity != GranDay {
		t.Fatalf("%+v", s)
	}
}

func TestInferListOtherwise(t *testing.T) {
	ts := api.TypeSchema{
		Name: "person",
		Fields: map[string]api.FieldSchema{
			"name":  {Type: "string"},
			"email": {Type: "string"},
		},
	}
	s := Infer(ts)
	if s.Kind != KindList {
		t.Fatalf("%+v", s)
	}
}

func TestNextCyclesListBoardDate(t *testing.T) {
	ts := projectSchema()
	s := Spec{Kind: KindList}
	s = Next(s, ts)
	if s.Kind != KindBoard || s.GroupBy != "status" {
		t.Fatalf("first board %+v", s)
	}
	s = Next(s, ts)
	if s.Kind != KindBoard || s.GroupBy != "owner" {
		t.Fatalf("ref board %+v", s)
	}
	s = Next(s, ts)
	if s.Kind != KindDate || s.GroupBy != "due" {
		t.Fatalf("date %+v", s)
	}
	s = Next(s, ts)
	if s.Kind != KindList {
		t.Fatalf("wrap %+v", s)
	}
}

func TestNextGranularity(t *testing.T) {
	s := Spec{Kind: KindDate, GroupBy: "due", Granularity: GranDay}
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

func TestApplyBoardGroups(t *testing.T) {
	ts := projectSchema()
	objs := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"status": "active"}},
		{ID: "b", Title: "B", Fields: map[string]any{"status": "idea"}},
	}
	r := Apply(objs, Spec{Kind: KindBoard, GroupBy: "status"}, ts, "", false)
	if len(r.Board) < 3 {
		t.Fatalf("cols %d", len(r.Board))
	}
	r = Apply(objs, Spec{Kind: KindList}, ts, "ida", true)
	if len(r.Objects) != 1 || r.Objects[0].Title != "B" {
		t.Fatalf("fuzzy %+v", r.Objects)
	}
}

func TestNormalizeInfersKindFromGroupBy(t *testing.T) {
	s := Spec{GroupBy: "status"}.Normalize()
	if s.Kind != KindBoard {
		t.Fatal(s.Kind)
	}
	s = Spec{GroupBy: "date", Granularity: GranWeek}.Normalize()
	if s.Kind != KindDate {
		t.Fatal(s.Kind)
	}
}
