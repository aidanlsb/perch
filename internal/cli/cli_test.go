package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aidanlsb/perch/internal/config"
)

func TestViewsListJSON(t *testing.T) {
	path := writeCfg(t, `
[views.projects]
source = "item"
layout = { kind = "board", group_by = "state", fields = ["state"] }
`)
	env := runJSON(t, "views", "list", "--config", path)
	if !env.OK || env.Error != nil {
		t.Fatalf("%+v", env)
	}
	data := asMap(t, env.Data)
	views := asSlice(t, data["views"])
	if len(views) != 1 {
		t.Fatalf("%v", views)
	}
	v := asMap(t, views[0])
	if v["name"] != "projects" || v["source"] != "item" {
		t.Fatalf("%v", v)
	}
	layout := asMap(t, v["layout"])
	if layout["kind"] != "board" || layout["group_by"] != "state" {
		t.Fatalf("%v", layout)
	}
}

func TestViewsShowAndMissing(t *testing.T) {
	path := writeCfg(t, `
[views.projects]
source = "item"
layout = { kind = "list" }
`)
	env := runJSON(t, "views", "show", "projects", "--config", path)
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	if asMap(t, env.Data)["source"] != "item" {
		t.Fatalf("%v", env.Data)
	}
	fail := runJSONErr(t, "views", "show", "ghost", "--config", path)
	if fail.OK || fail.Error == nil || fail.Error.Code != CodeNotFound {
		t.Fatalf("%+v", fail)
	}
}

func TestViewsAddAndRMJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	env := runJSON(t, "views", "add", "projects", "--source", "item",
		"--layout", "kind=board,group_by=state,fields=state,when",
		"--inputs", "q=1", "--config", path)
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data := asMap(t, env.Data)
	if data["name"] != "projects" {
		t.Fatalf("%v", data)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "[views.projects]") {
		t.Fatalf("%s", body)
	}
	dup := runJSONErr(t, "views", "add", "projects", "--source", "item", "--config", path)
	if dup.Error == nil || dup.Error.Code != CodeDuplicateName {
		t.Fatalf("%+v", dup)
	}
	rm := runJSON(t, "views", "rm", "projects", "--config", path)
	if !rm.OK {
		t.Fatalf("%+v", rm)
	}
	f, err := config.Load(path)
	if err != nil || len(f.Views) != 0 {
		t.Fatalf("%v %+v", err, f)
	}
}

func TestViewsAddMissingSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.toml")
	fail := runJSONErr(t, "views", "add", "x", "--config", path)
	if fail.Error == nil || fail.Error.Code != CodeMissingArgument {
		t.Fatalf("%+v", fail)
	}
}

func TestViewsRMRequiresForce(t *testing.T) {
	path := writeCfg(t, `
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
	fail := runJSONErr(t, "views", "rm", "a", "--config", path)
	if fail.Error == nil || fail.Error.Code != CodeConfirmationRequired {
		t.Fatalf("%+v", fail)
	}
	ok := runJSON(t, "views", "rm", "a", "--force", "--config", path)
	if !ok.OK {
		t.Fatalf("%+v", ok)
	}
}

func TestLayoutsListShowAddRM(t *testing.T) {
	path := writeCfg(t, `
[views.a]
source = "item"
layout = { kind = "list" }
[views.b]
source = "item"
layout = { kind = "list" }
`)
	add := runJSON(t, "layouts", "add", "ws", "--split", "row",
		"--panel", "view=a", "--panel", "view=b,bind=a", "--config", path)
	if !add.OK {
		t.Fatalf("%+v", add)
	}
	data := asMap(t, add.Data)
	if data["name"] != "ws" || data["split"] != "row" {
		t.Fatalf("%v", data)
	}
	list := runJSON(t, "layouts", "list", "--config", path)
	layouts := asSlice(t, asMap(t, list.Data)["layouts"])
	if len(layouts) != 1 {
		t.Fatalf("%v", layouts)
	}
	show := runJSON(t, "layouts", "show", "ws", "--config", path)
	if !show.OK {
		t.Fatalf("%+v", show)
	}
	rm := runJSON(t, "layouts", "rm", "ws", "--config", path)
	if !rm.OK {
		t.Fatalf("%+v", rm)
	}
	missing := runJSONErr(t, "layouts", "show", "ws", "--config", path)
	if missing.Error == nil || missing.Error.Code != CodeNotFound {
		t.Fatalf("%+v", missing)
	}
}

func TestLayoutsAddFromTOMLAndJSON(t *testing.T) {
	path := writeCfg(t, `
[views.a]
source = "item"
layout = { kind = "list" }
`)
	tomlAdd := runJSON(t, "layouts", "add", "fromtoml", "--from", `split = "col"
panels = [ { view = "a" } ]`, "--config", path)
	if !tomlAdd.OK {
		t.Fatalf("%+v", tomlAdd)
	}
	if asMap(t, tomlAdd.Data)["split"] != "col" {
		t.Fatalf("%v", tomlAdd.Data)
	}
	jsonAdd := runJSON(t, "layouts", "add", "fromjson", "--from", `{"split":"row","panels":[{"view":"a"}]}`, "--config", path)
	if !jsonAdd.OK {
		t.Fatalf("%+v", jsonAdd)
	}
}

func TestLayoutsAddFromFile(t *testing.T) {
	path := writeCfg(t, `
[views.a]
source = "item"
layout = { kind = "list" }
`)
	snippet := filepath.Join(t.TempDir(), "lay.json")
	if err := os.WriteFile(snippet, []byte(`{"split":"row","panels":[{"view":"a"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := runJSON(t, "layouts", "add", "filelay", "--from", snippet, "--config", path)
	if !env.OK {
		t.Fatalf("%+v", env)
	}
}

func TestConfigPathAndCheck(t *testing.T) {
	path := writeCfg(t, `
[views.a]
source = "item"
layout = { kind = "list" }
`)
	got := runJSON(t, "config", "path", "--config", path)
	if !got.OK || asMap(t, got.Data)["path"] != path {
		t.Fatalf("%+v", got)
	}
	ok := runJSON(t, "config", "check", "--config", path)
	if !ok.OK || asMap(t, ok.Data)["valid"] != true {
		t.Fatalf("%+v", ok)
	}
	bad := writeCfg(t, `
[views.board]
source = "item"
layout = { kind = "board" }
`)
	fail := runJSONErr(t, "config", "check", "--config", bad)
	if fail.OK || fail.Error == nil || fail.Error.Code != CodeValidationFailed {
		t.Fatalf("%+v", fail)
	}
	details := asMap(t, fail.Error.Details)
	errs := asSlice(t, details["errors"])
	if len(errs) == 0 {
		t.Fatalf("%v", details)
	}
	if asMap(t, errs[0])["line"] == nil {
		t.Fatalf("want line in %v", errs[0])
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	fail := runJSONErr(t, "views", "explode")
	if fail.Error == nil || fail.Error.Code != CodeCommandNotFound {
		t.Fatalf("%+v", fail)
	}
}

func TestHumanViewsList(t *testing.T) {
	path := writeCfg(t, `
[views.projects]
source = "item"
layout = { kind = "list" }
`)
	var out, errb bytes.Buffer
	if err := Run([]string{"views", "list", "--config", path}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "projects") {
		t.Fatalf("%s", out.String())
	}
}

func TestErrorEnvelopeShape(t *testing.T) {
	fail := runJSONErr(t, "views", "show", "nope", "--config", filepath.Join(t.TempDir(), "c.toml"))
	if fail.OK || fail.Error == nil || fail.Error.Code == "" || fail.Error.Message == "" {
		t.Fatalf("envelope %+v", fail)
	}
	if fail.Error.Code != CodeNotFound {
		t.Fatalf("code %s", fail.Error.Code)
	}
}

func TestResolveOpen(t *testing.T) {
	path := writeCfg(t, `
[views.projects]
source = "item"
layout = { kind = "list" }
[layouts.ws]
split = "row"
panels = [ { view = "projects" } ]
`)
	kind, err := ResolveOpen(path, "projects")
	if err != nil || kind != "view" {
		t.Fatalf("%s %v", kind, err)
	}
	kind, err = ResolveOpen(path, "ws")
	if err != nil || kind != "layout" {
		t.Fatalf("%s %v", kind, err)
	}
	if _, err := ResolveOpen(path, "ghost"); err == nil {
		t.Fatal("expected not found")
	}
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runJSON(t *testing.T, args ...string) Response {
	t.Helper()
	var out, errb bytes.Buffer
	if err := Run(append(args, "--json"), &out, &errb); err != nil {
		t.Fatalf("err %v out=%s err=%s", err, out.String(), errb.String())
	}
	return decodeResp(t, out.Bytes())
}

func runJSONErr(t *testing.T, args ...string) Response {
	t.Helper()
	var out, errb bytes.Buffer
	err := Run(append(args, "--json"), &out, &errb)
	if err == nil {
		t.Fatalf("expected error out=%s", out.String())
	}
	return decodeResp(t, out.Bytes())
}

func TestParseTUIPollInterval(t *testing.T) {
	flags, err := ParseTUI([]string{"--poll-interval", "2s", "--url", "http://127.0.0.1:9"})
	if err != nil {
		t.Fatal(err)
	}
	if flags.PollInterval != 2*time.Second {
		t.Fatalf("got %v", flags.PollInterval)
	}
	_, err = ParseTUI([]string{"--poll-interval", "nope"})
	if err == nil || !strings.Contains(err.Error(), "poll-interval") {
		t.Fatalf("got %v", err)
	}
}

func decodeResp(t *testing.T, raw []byte) Response {
	t.Helper()
	var env Response
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("json %v: %s", err, raw)
	}
	return env
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not map: %T %v", v, v)
	}
	return m
}

func asSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("not slice: %T %v", v, v)
	}
	return s
}
