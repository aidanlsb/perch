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
type = "item"
layout = { kind = "board", group_by = "state", fields = ["state", "when"] }

[views.by-when]
type = "item"
layout = { kind = "list", fields = ["state", "when"], sort = "when" }

[views.open]
type = "item"
layout = { kind = "list", filter = ".state==open" }

[views.week]
type = "event"
layout = { kind = "date", group_by = "when", granularity = "week" }

[views.inbox]
query = "inbox"
layout = { kind = "list", sort = "label" }

[views.tasks]
type = "task"
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
	if v.Type != "item" || v.Query != "" || v.Layout.Kind != KindBoard || v.Layout.GroupBy != "state" {
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
	if f.Views["inbox"].Query != "inbox" || f.Views["inbox"].Type != "" {
		t.Fatalf("inbox %+v", f.Views["inbox"])
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
type = "item"
layout = { kind = "list" }
[views.b]
type = "item"
layout = { kind = "list" }
[views.c]
type = "item"
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
type = "type:item"
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
type = "item"
layout = { kind = "kanban" }
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsBoardWithoutGroupBy(t *testing.T) {
	path := writeTOML(t, `
[views.board]
type = "item"
layout = { kind = "board" }
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsUnknownViewInLayout(t *testing.T) {
	path := writeTOML(t, `
[views.a]
type = "item"
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
type = "item"
layout = { kind = "list" }
[views.b]
type = "item"
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
type = "item"
layout = { kind = "list" }
[views.b]
type = "item"
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
type = "item"
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
type = "item"
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
			"board": {Type: "item", Layout: Present{Kind: KindBoard, GroupBy: "nope", Fields: []string{"state", "missing"}, Sort: "zzz"}},
			"ghost": {Type: "ghost", Layout: Present{Kind: KindList}},
			"open":  {Query: "inbox", Layout: Present{Kind: KindList, Fields: []string{"missing"}}},
			"gone":  {Query: "nope", Layout: Present{Kind: KindList}},
			"trait": {Query: "priorities", Layout: Present{Kind: KindList, Fields: []string{"missing"}}},
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
			"inbox":      {Name: "inbox", Query: "type:item"},
			"priorities": {Name: "priorities", Query: "trait:priority"},
		},
	}
	warns := f.Validate(schema)
	joined := strings.Join(warns, "\n")
	for _, want := range []string{
		`views.ghost: unknown type "ghost"`,
		`views.gone: unknown query "nope"`,
		`unknown group_by field "nope"`,
		`unknown fields entry "missing"`,
		`unknown sort field "zzz"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in\n%s", want, joined)
		}
	}
	if !strings.Contains(joined, `views.open: unknown fields entry "missing"`) {
		t.Fatalf("saved query with derivable type should check fields:\n%s", joined)
	}
	if strings.Contains(joined, "views.trait") {
		t.Fatalf("saved query without derivable type should skip field checks:\n%s", joined)
	}
}

func TestValidateBoardGroupByType(t *testing.T) {
	f := &File{Views: map[string]View{
		"board": {Type: "item", Layout: Present{Kind: KindBoard, GroupBy: "when"}},
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
	if err := os.WriteFile(path, []byte("# keep me\n[views.old]\ntype = \"item\"\nlayout = { kind = \"list\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := View{Type: "item", Layout: Present{Kind: KindList, Fields: []string{"state"}, Sort: "when"}}
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
type = "item"
layout = { kind = "list" }
`)
	err := AddView(path, "old", View{Type: "item", Layout: Present{Kind: KindList}})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("%v", err)
	}
}

func TestRemoveViewRefusesInUse(t *testing.T) {
	path := writeTOML(t, `
# keep
[views.a]
type = "item"
layout = { kind = "list" }
[views.b]
type = "item"
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
type = "item"
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
type = "item"
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
type = "item"
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
type = "item"
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

func TestLoadRejectsViewSource(t *testing.T) {
	path := writeTOML(t, `
[views.projects]
source = "item"
layout = { kind = "list" }
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "source was removed") {
		t.Fatalf("want source-removed, got %v", err)
	}
	if !strings.Contains(msg, "type =") || !strings.Contains(msg, "query =") {
		t.Fatalf("want new format, got %v", err)
	}
	if !strings.Contains(msg, ":") {
		t.Fatalf("want line number, got %v", err)
	}
}

func TestLoadRejectsTypeAndQuery(t *testing.T) {
	path := writeTOML(t, `
[views.x]
type = "item"
query = "inbox"
layout = { kind = "list" }
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("%v", err)
	}
}

func TestLoadRejectsMissingTypeAndQuery(t *testing.T) {
	path := writeTOML(t, `
[views.x]
layout = { kind = "list" }
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exactly one of type or query") {
		t.Fatalf("%v", err)
	}
}

func TestCheckReportsSourceRemoved(t *testing.T) {
	path := writeTOML(t, `
[views.projects]
source = "item"
layout = { kind = "list" }
`)
	issues := Check(path)
	if len(issues) == 0 {
		t.Fatal("expected issue")
	}
	joined := ""
	for _, is := range issues {
		if is.Line < 2 {
			t.Fatalf("want line, got %+v", is)
		}
		joined += is.Message
	}
	if !strings.Contains(joined, "source was removed") {
		t.Fatalf("%s", joined)
	}
	if strings.Contains(joined, "exactly one of type or query") {
		t.Fatalf("source error should replace missing type/query: %s", joined)
	}
}

func TestValidateIncludesLineNumbers(t *testing.T) {
	path := writeTOML(t, `
[views.ghost]
type = "ghost"
layout = { kind = "list" }
`)
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	warns := f.Validate(api.Schema{Types: map[string]api.TypeSchema{}})
	if len(warns) != 1 {
		t.Fatalf("%v", warns)
	}
	if !strings.Contains(warns[0], ":") || !strings.Contains(warns[0], "unknown type") {
		t.Fatalf("want path:line unknown type, got %q", warns[0])
	}
}

func TestEncodeViewWritesTypeOrQuery(t *testing.T) {
	got, err := EncodeView("projects", View{Type: "item", Layout: Present{Kind: KindList}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `type = "item"`) || strings.Contains(got, "source") || strings.Contains(got, "query") {
		t.Fatalf("%s", got)
	}
	got, err = EncodeView("inbox", View{Query: "inbox", Layout: Present{Kind: KindList}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `query = "inbox"`) || strings.Contains(got, "source") || strings.Contains(got, "type =") {
		t.Fatalf("%s", got)
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
