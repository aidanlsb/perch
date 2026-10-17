package layout

import (
	"encoding/json"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func itemSchema(enumName, dateName string, order []string) api.TypeSchema {
	return api.TypeSchema{
		Name:       "item",
		NameField:  "label",
		FieldOrder: order,
		Fields: map[string]api.FieldSchema{
			"label":  {Type: "string"},
			enumName: {Type: "enum", Values: []string{"a", "b", "c"}},
			dateName: {Type: "date"},
			"owner":  {Type: "ref", Target: "person"},
		},
	}
}

func TestSynthesizeFromJSONSchemaOrder(t *testing.T) {
	raw := []byte(`{
		"name": "item",
		"name_field": "label",
		"fields": {
			"label": {"type": "string"},
			"xyz": {"type": "enum", "values": ["a", "b"]},
			"when": {"type": "date"}
		}
	}`)
	var ts api.TypeSchema
	if err := json.Unmarshal(raw, &ts); err != nil {
		t.Fatal(err)
	}
	ls := Synthesize(ts)
	if len(ls) != 3 || ls[1].GroupBy != "xyz" || ls[2].GroupBy != "when" {
		t.Fatalf("%+v", ls)
	}
	if ls[1].Name != "Board" || ls[2].Name != "Date" {
		t.Fatalf("names %+v", ls)
	}
}

func TestSynthesizeFromSchemaOrder(t *testing.T) {
	ts := itemSchema("xyz", "when", []string{"label", "xyz", "when", "owner"})
	ls := Synthesize(ts)
	if len(ls) != 3 {
		t.Fatalf("%d %+v", len(ls), ls)
	}
	if ls[0].Name != "List" || ls[0].Kind != KindList {
		t.Fatalf("list %+v", ls[0])
	}
	if ls[1].Name != "Board" || ls[1].Kind != KindBoard || ls[1].GroupBy != "xyz" {
		t.Fatalf("board %+v", ls[1])
	}
	if ls[2].Name != "Date" || ls[2].Kind != KindDate || ls[2].GroupBy != "when" || ls[2].Granularity != GranDay {
		t.Fatalf("date %+v", ls[2])
	}
}

func TestSynthesizeIsNameAgnostic(t *testing.T) {
	a := Synthesize(itemSchema("xyz", "when", []string{"label", "xyz", "when", "owner"}))
	b := Synthesize(itemSchema("status", "date", []string{"label", "status", "date", "owner"}))
	if len(a) != 3 || len(b) != 3 {
		t.Fatalf("%+v %+v", a, b)
	}
	if a[1].Kind != KindBoard || b[1].Kind != KindBoard {
		t.Fatal("board")
	}
	if a[1].Name != "Board" || b[1].Name != "Board" {
		t.Fatalf("board names %q %q", a[1].Name, b[1].Name)
	}
	if a[1].GroupBy != "xyz" || b[1].GroupBy != "status" {
		t.Fatalf("board group %q %q", a[1].GroupBy, b[1].GroupBy)
	}
	if a[2].Name != "Date" || b[2].Name != "Date" {
		t.Fatalf("date names %q %q", a[2].Name, b[2].Name)
	}
	if a[2].GroupBy != "when" || b[2].GroupBy != "date" {
		t.Fatalf("date group %q %q", a[2].GroupBy, b[2].GroupBy)
	}
}

func TestSynthesizeUsesFirstEnumInSchemaOrder(t *testing.T) {
	ts := api.TypeSchema{
		FieldOrder: []string{"priority", "status"},
		Fields: map[string]api.FieldSchema{
			"status":   {Type: "enum", Values: []string{"a"}},
			"priority": {Type: "enum", Values: []string{"low"}},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 2 || ls[1].GroupBy != "priority" {
		t.Fatalf("first enum in order is priority, got %+v", ls)
	}
	ts.FieldOrder = []string{"status", "priority"}
	ls = Synthesize(ts)
	if ls[1].GroupBy != "status" {
		t.Fatalf("first enum in order is status, got %+v", ls[1])
	}
}

func TestSynthesizeWithoutFieldOrderIsAlphabetical(t *testing.T) {
	ts := api.TypeSchema{
		Fields: map[string]api.FieldSchema{
			"status":   {Type: "enum", Values: []string{"a"}},
			"priority": {Type: "enum", Values: []string{"low"}},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 2 || ls[1].GroupBy != "priority" {
		t.Fatalf("alpha fallback must not prefer status, got %+v", ls)
	}
}

func TestSynthesizeListOnly(t *testing.T) {
	ts := api.TypeSchema{
		Name: "person",
		Fields: map[string]api.FieldSchema{
			"label": {Type: "string"},
			"email": {Type: "string"},
		},
	}
	ls := Synthesize(ts)
	if len(ls) != 1 || ls[0].Kind != KindList {
		t.Fatalf("%+v", ls)
	}
}

func TestCycleForwardAndBack(t *testing.T) {
	ls := Synthesize(itemSchema("xyz", "when", []string{"label", "xyz", "when", "owner"}))
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
	ts := itemSchema("xyz", "when", []string{"label", "xyz", "when", "owner"})
	objs := []api.Object{
		{ID: "a", Title: "A", Fields: map[string]any{"xyz": "b"}},
		{ID: "b", Title: "B", Fields: map[string]any{"xyz": "a"}},
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
