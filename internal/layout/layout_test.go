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
			"name":     {Type: "string"},
			"status":   {Type: "enum", Values: []string{"idea", "active", "done"}},
			"priority": {Type: "enum", Values: []string{"low", "high"}},
			"due":      {Type: "date"},
			"owner":    {Type: "ref", Target: "person"},
		},
	}
}

func TestSynthesizeProject(t *testing.T) {
	ls := Synthesize(projectSchema())
	if len(ls) != 3 {
		t.Fatalf("%d %+v", len(ls), ls)
	}
	if ls[0].Name != "List" || ls[0].Kind != KindList {
		t.Fatalf("list %+v", ls[0])
	}
	if ls[1].Name != "Board" || ls[1].Kind != KindBoard || ls[1].GroupBy != "status" {
		t.Fatalf("board prefers status, got %+v", ls[1])
	}
	if ls[2].Name != "By due" || ls[2].Kind != KindDate || ls[2].GroupBy != "due" || ls[2].Granularity != GranDay {
		t.Fatalf("date %+v", ls[2])
	}
}

func TestSynthesizePrefersDateField(t *testing.T) {
	ts := api.TypeSchema{
		Name: "date",
		Fields: map[string]api.FieldSchema{
			"date": {Type: "date"},
			"name": {Type: "string"},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 2 {
		t.Fatalf("%d", len(ls))
	}
	if ls[1].Name != "Date" || ls[1].Kind != KindDate || ls[1].GroupBy != "date" {
		t.Fatalf("%+v", ls[1])
	}
}

func TestSynthesizeListOnly(t *testing.T) {
	ts := api.TypeSchema{
		Name: "person",
		Fields: map[string]api.FieldSchema{
			"name":  {Type: "string"},
			"email": {Type: "string"},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 1 || ls[0].Kind != KindList {
		t.Fatalf("%+v", ls)
	}
}

func TestSynthesizeBoardWithoutStatus(t *testing.T) {
	ts := api.TypeSchema{
		Name: "task",
		Fields: map[string]api.FieldSchema{
			"name": {Type: "string"},
			"kind": {Type: "enum", Values: []string{"a", "b"}},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 2 || ls[1].Name != "By kind" || ls[1].GroupBy != "kind" {
		t.Fatalf("%+v", ls)
	}
}

func TestCycleForwardAndBack(t *testing.T) {
	ls := Synthesize(projectSchema())
	s, i := Cycle(ls, 0, 1)
	if s.Kind != KindBoard || i != 1 {
		t.Fatalf("fwd %+v %d", s, i)
	}
	s, i = Cycle(ls, i, 1)
	if s.Kind != KindDate || i != 2 {
		t.Fatalf("date %+v %d", s, i)
	}
	s, i = Cycle(ls, i, 1)
	if s.Kind != KindList || i != 0 {
		t.Fatalf("wrap %+v %d", s, i)
	}
	s, i = Cycle(ls, 0, -1)
	if s.Kind != KindDate || i != 2 {
		t.Fatalf("back %+v %d", s, i)
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

func TestAppendFilter(t *testing.T) {
	if got := AppendFilter("type:project", ".status==active"); got != "type:project .status==active" {
		t.Fatal(got)
	}
	if got := AppendFilter("type:project", ""); got != "type:project" {
		t.Fatal(got)
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
