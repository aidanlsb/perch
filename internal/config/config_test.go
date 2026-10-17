package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || len(f.Views) != 0 || len(f.Layouts) != 0 {
		t.Fatalf("%v %+v", err, f)
	}
}

func TestLoadViewsAndLayouts(t *testing.T) {
	path := writeTOML(t, `
[views.projects]
source = "item"
layout = { kind = "board", group_by = "state", fields = ["state", "when"] }

[views.by-when]
source = "item"
layout = { kind = "list", fields = ["state", "when"], sort = "when" }

[views.open]
source = "item"
layout = { kind = "list", filter = ".state==open" }

[views.week]
source = "event"
layout = { kind = "date", group_by = "when", granularity = "week" }

[views.inbox]
source = "inbox"
layout = { kind = "list", sort = "label" }

[views.tasks]
source = "task"
inputs = { item = "{{selected}}" }
layout = { kind = "list" }

[layouts.workspace]
split = "row"
panels = [
  { view = "projects" },
  { view = "tasks", bind = "projects" },
]
`)
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v := f.Views["projects"]
	if v.Source != "item" || v.Layout.Kind != KindBoard || v.Layout.GroupBy != "state" {
		t.Fatalf("%+v", v)
	}
	spec := v.Spec("projects")
	if spec.Name != "projects" || spec.Kind != KindBoard {
		t.Fatalf("spec %+v", spec)
	}
	if f.Views["open"].Layout.Filter != ".state==open" {
		t.Fatalf("filter %+v", f.Views["open"])
	}
	if f.Views["week"].Layout.Granularity != GranWeek {
		t.Fatalf("%+v", f.Views["week"])
	}
	if f.Views["tasks"].Inputs["item"] != SelectedToken {
		t.Fatalf("inputs %+v", f.Views["tasks"].Inputs)
	}
	if f.ViewLine("projects") < 2 {
		t.Fatalf("line %d", f.ViewLine("projects"))
	}
	ls := f.Layouts["workspace"]
	if ls.Split != SplitRow || len(ls.Leaves()) != 2 {
		t.Fatalf("%+v", ls)
	}
	if ls.Leaves()[1].Bind != "projects" {
		t.Fatalf("bind %+v", ls.Leaves())
	}
}

func TestLoadNestedPanels(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[views.b]
source = "item"
layout = { kind = "list" }
[views.c]
source = "item"
layout = { kind = "list" }

[layouts.nested]
split = "row"
panels = [
  { view = "a" },
  { split = "col", panels = [
      { view = "b", bind = "a" },
      { view = "c", bind = "b" },
  ] },
]
`)
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	leaves := f.Layouts["nested"].Leaves()
	if len(leaves) != 3 || leaves[0].View != "a" || leaves[1].View != "b" || leaves[2].View != "c" {
		t.Fatalf("%+v", leaves)
	}
	if leaves[1].Bind != "a" || leaves[2].Bind != "b" {
		t.Fatalf("binds %+v", leaves)
	}
}

func TestLoadRejectsLegacyTypeLayouts(t *testing.T) {
	path := writeTOML(t, `
[[types.item.layouts]]
name = "Board"
kind = "board"
group_by = "state"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "[views.") || !strings.Contains(msg, "[layouts.") {
		t.Fatalf("want pointer at new shape, got %v", err)
	}
	if !strings.Contains(msg, ":") {
		t.Fatalf("want line number, got %v", err)
	}
}

func TestLoadRejectsLegacyQueryLayouts(t *testing.T) {
	path := writeTOML(t, `
[[queries.inbox.layouts]]
name = "Inbox"
kind = "list"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[views.") {
		t.Fatalf("want pointer at new shape, got %v", err)
	}
}

func TestLoadRejectsLegacyViewsArray(t *testing.T) {
	path := writeTOML(t, `
[[views]]
name = "Projects"
source = "type:item"
kind = "board"
group_by = "state"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[[views]]") || !strings.Contains(err.Error(), "[views.") {
		t.Fatalf("want views-removed pointer, got %v", err)
	}
}

func TestLoadRejectsLegacyLayoutEquals(t *testing.T) {
	path := writeTOML(t, `
[types.item]
layout = "board"
group_by = "state"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "[views.") {
		t.Fatalf("want pointer at new shape, got %v", err)
	}
}

func TestLoadRejectsBadKind(t *testing.T) {
	path := writeTOML(t, `
[views.x]
source = "item"
layout = { kind = "kanban" }
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsBoardWithoutGroupBy(t *testing.T) {
	path := writeTOML(t, `
[views.board]
source = "item"
layout = { kind = "board" }
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsUnknownViewInLayout(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[layouts.x]
split = "row"
panels = [ { view = "a" }, { view = "ghost" } ]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `unknown view "ghost"`) {
		t.Fatalf("%v", err)
	}
}

func TestLoadRejectsBindToMissingPanel(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[views.b]
source = "item"
layout = { kind = "list" }
[layouts.x]
split = "row"
panels = [ { view = "a" }, { view = "b", bind = "nope" } ]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a panel") {
		t.Fatalf("%v", err)
	}
}

func TestLoadRejectsBindCycle(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[views.b]
source = "item"
layout = { kind = "list" }
[layouts.x]
split = "row"
panels = [ { view = "a", bind = "b" }, { view = "b", bind = "a" } ]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("%v", err)
	}
}

func TestLoadRejectsDuplicateView(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[layouts.x]
split = "col"
panels = [ { view = "a" }, { view = "a" } ]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("%v", err)
	}
}

func TestLoadTOMLErrorHasLine(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ":") {
		t.Fatalf("want path:line, got %v", err)
	}
}

func TestValidateUnknownFields(t *testing.T) {
	f := &File{
		Views: map[string]View{
			"board": {Source: "item", Layout: Present{Kind: KindBoard, GroupBy: "nope", Fields: []string{"state", "missing"}, Sort: "zzz"}},
			"ghost": {Source: "ghost", Layout: Present{Kind: KindList}},
			"rql":   {Source: "type:item .state==open", Layout: Present{Kind: KindList, Fields: []string{"missing"}}},
		},
	}
	schema := api.Schema{
		Types: map[string]api.TypeSchema{
			"item": {
				Name: "item",
				Fields: map[string]api.FieldSchema{
					"state": {Type: "enum", Values: []string{"a"}},
					"when":  {Type: "date"},
				},
			},
		},
		Queries: map[string]api.SavedQuery{
			"inbox": {Name: "inbox", Query: "type:item"},
		},
	}
	warns := f.Validate(schema)
	joined := strings.Join(warns, "\n")
	for _, want := range []string{
		`views.ghost: unknown source "ghost"`,
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
	f := &File{Views: map[string]View{
		"board": {Source: "item", Layout: Present{Kind: KindBoard, GroupBy: "when"}},
	}}
	schema := api.Schema{Types: map[string]api.TypeSchema{
		"item": {Name: "item", Fields: map[string]api.FieldSchema{
			"when": {Type: "date"},
		}},
	}}
	warns := f.Validate(schema)
	if len(warns) != 1 || !strings.Contains(warns[0], "not an enum or ref") {
		t.Fatalf("%v", warns)
	}
}

func TestUpsertViewWritesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# keep me\n[views.old]\nsource = \"item\"\nlayout = { kind = \"list\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := View{Source: "item", Layout: Present{Kind: KindList, Fields: []string{"state"}, Sort: "when"}}
	if err := UpsertView(path, "saved", v); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "# keep me") || !strings.Contains(s, "[views.saved]") || !strings.Contains(s, "[views.old]") {
		t.Fatalf("%s", s)
	}
	v.Layout.Kind = KindBoard
	v.Layout.GroupBy = "state"
	if err := UpsertView(path, "saved", v); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(path)
	s = string(body)
	if strings.Count(s, "[views.saved]") != 1 {
		t.Fatalf("duplicate section:\n%s", s)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Views["saved"].Layout.Kind != KindBoard || f.Views["saved"].Layout.GroupBy != "state" {
		t.Fatalf("%+v", f.Views["saved"])
	}
}

func TestAddViewRefusesDuplicate(t *testing.T) {
	path := writeTOML(t, `[views.old]
source = "item"
layout = { kind = "list" }
`)
	err := AddView(path, "old", View{Source: "item", Layout: Present{Kind: KindList}})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("%v", err)
	}
}

func TestRemoveViewRefusesInUse(t *testing.T) {
	path := writeTOML(t, `
# keep
[views.a]
source = "item"
layout = { kind = "list" }
[views.b]
source = "item"
layout = { kind = "list" }
[layouts.ws]
split = "row"
panels = [ { view = "a" }, { view = "b" } ]
`)
	err := RemoveView(path, "a", false)
	var inUse *InUseError
	if err == nil || !errors.As(err, &inUse) {
		t.Fatalf("%v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "# keep") || !strings.Contains(string(body), "[views.a]") {
		t.Fatalf("file changed:\n%s", body)
	}
	if err := RemoveView(path, "a", true); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Views["a"]; ok {
		t.Fatal("view still present")
	}
	if !strings.Contains(string(mustRead(t, path)), "# keep") {
		t.Fatal("lost comment")
	}
	leaves := f.Layouts["ws"].Leaves()
	if len(leaves) != 1 || leaves[0].View != "b" {
		t.Fatalf("%+v", leaves)
	}
}

func TestRemoveViewForceDropsEmptyLayout(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[layouts.only]
split = "row"
panels = [ { view = "a" } ]
`)
	if err := RemoveView(path, "a", true); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Layouts["only"]; ok {
		t.Fatal("empty layout should be removed")
	}
}

func TestAddLayoutAndRemove(t *testing.T) {
	path := writeTOML(t, `
[views.a]
source = "item"
layout = { kind = "list" }
`)
	if err := AddLayout(path, "ws", Layout{Split: SplitRow, Panels: []Panel{{View: "a"}}}); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Layouts["ws"].Split != SplitRow || f.Layouts["ws"].Leaves()[0].View != "a" {
		t.Fatalf("%+v", f.Layouts["ws"])
	}
	if err := RemoveLayout(path, "ws"); err != nil {
		t.Fatal(err)
	}
	f, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Layouts["ws"]; ok {
		t.Fatal("layout still present")
	}
}

func TestCommitRefusesInvalid(t *testing.T) {
	path := writeTOML(t, `[views.a]
source = "item"
layout = { kind = "list" }
`)
	err := AddLayout(path, "bad", Layout{Split: "row", Panels: []Panel{{View: "ghost"}}})
	if err == nil {
		t.Fatal("expected validation failure")
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Layouts["bad"]; ok {
		t.Fatal("wrote invalid layout")
	}
}

func TestCheckReportsLineNumbers(t *testing.T) {
	path := writeTOML(t, `
[views.board]
source = "item"
layout = { kind = "board" }
[layouts.x]
split = "row"
panels = [ { view = "ghost" } ]
`)
	issues := Check(path)
	if len(issues) < 2 {
		t.Fatalf("%+v", issues)
	}
	joined := ""
	for _, is := range issues {
		if is.Line < 2 {
			t.Fatalf("want line, got %+v", is)
		}
		joined += is.Message
	}
	if !strings.Contains(joined, "group_by") || !strings.Contains(joined, "unknown view") {
		t.Fatalf("%s", joined)
	}
}

func TestCheckMissingIsOK(t *testing.T) {
	if issues := Check(filepath.Join(t.TempDir(), "nope.toml")); len(issues) != 0 {
		t.Fatalf("%+v", issues)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSubstituteSelected(t *testing.T) {
	if got := SubstituteSelected("type:task .item=="+SelectedToken, "items/a"); got != "type:task .item==items/a" {
		t.Fatal(got)
	}
}

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
