package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || len(f.Types) != 0 || len(f.Queries) != 0 {
		t.Fatalf("%v %+v", err, f)
	}
}

func TestLoadNamedLayouts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[[types.project.layouts]]
name = "Board"
kind = "board"
group_by = "status"
fields = ["status", "due"]

[[types.project.layouts]]
name = "By due"
kind = "list"
fields = ["status", "due"]
sort = "due"

[[types.project.layouts]]
name = "Active"
kind = "list"
filter = ".status==active"

[[types.date.layouts]]
name = "Week"
kind = "date"
group_by = "date"
granularity = "week"

[[queries.open.layouts]]
name = "Open"
kind = "list"
sort = "name"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := f.TypeLayouts("project")
	if len(ls) != 3 {
		t.Fatalf("%d %+v", len(ls), ls)
	}
	if ls[0].Name != "Board" || ls[0].Kind != KindBoard || ls[0].GroupBy != "status" {
		t.Fatalf("first %+v", ls[0])
	}
	if ls[1].Name != "By due" || ls[1].Kind != KindList || ls[1].Sort != "due" {
		t.Fatalf("second %+v", ls[1])
	}
	if ls[2].Filter != ".status==active" {
		t.Fatalf("filter %+v", ls[2])
	}
	date := f.TypeLayouts("date")
	if len(date) != 1 || date[0].Granularity != GranWeek {
		t.Fatalf("%+v", date)
	}
	q := f.QueryLayouts("open")
	if len(q) != 1 || q[0].Kind != KindList || q[0].Name != "Open" {
		t.Fatalf("%+v", q)
	}
}

func TestLoadDefaultNameFromKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[types.project.layouts]]
kind = "list"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := f.TypeLayouts("project")
	if len(ls) != 1 || ls[0].Name != "List" {
		t.Fatalf("%+v", ls)
	}
}

func TestLoadRejectsLegacyLayoutEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[types.project]
layout = "board"
group_by = "status"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[[types.") || !strings.Contains(err.Error(), ".layouts]]") {
		t.Fatalf("want pointer at new shape, got %v", err)
	}
}

func TestLoadRejectsLegacyViews(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[views]]
name = "Projects"
source = "type:project"
kind = "board"
group_by = "status"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[[views]]") || !strings.Contains(err.Error(), "[[types.") {
		t.Fatalf("want views-removed pointer, got %v", err)
	}
}

func TestLoadRejectsBadKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[types.project.layouts]]
name = "x"
kind = "kanban"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsBoardWithoutGroupBy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[types.project.layouts]]
name = "Board"
kind = "board"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateUnknownFields(t *testing.T) {
	f := &File{
		Types: map[string]Source{
			"project": {Layouts: []Layout{
				{Name: "Board", Kind: KindBoard, GroupBy: "nope", Fields: []string{"status", "missing"}, Sort: "zzz"},
			}},
			"ghost": {Layouts: []Layout{{Name: "List", Kind: KindList}}},
		},
		Queries: map[string]Source{
			"nope": {Layouts: []Layout{{Name: "List", Kind: KindList}}},
		},
	}
	schema := api.Schema{
		Types: map[string]api.TypeSchema{
			"project": {
				Name: "project",
				Fields: map[string]api.FieldSchema{
					"status": {Type: "enum", Values: []string{"a"}},
					"due":    {Type: "date"},
				},
			},
		},
		Queries: map[string]api.SavedQuery{
			"open": {Name: "open", Query: "type:project"},
		},
	}
	warns := f.Validate(schema)
	joined := strings.Join(warns, "\n")
	for _, want := range []string{
		`types.ghost: unknown type`,
		`queries.nope: unknown saved query`,
		`unknown group_by field "nope"`,
		`unknown fields entry "missing"`,
		`unknown sort field "zzz"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in\n%s", want, joined)
		}
	}
}

func TestValidateBoardGroupByType(t *testing.T) {
	f := &File{Types: map[string]Source{
		"project": {Layouts: []Layout{
			{Name: "Board", Kind: KindBoard, GroupBy: "due"},
		}},
	}}
	schema := api.Schema{Types: map[string]api.TypeSchema{
		"project": {Name: "project", Fields: map[string]api.FieldSchema{
			"due": {Type: "date"},
		}},
	}}
	warns := f.Validate(schema)
	if len(warns) != 1 || !strings.Contains(warns[0], "not an enum or ref") {
		t.Fatalf("%v", warns)
	}
}
