package run

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/cli"
	"github.com/aidanlsb/perch/internal/config"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := Main([]string{"--version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("%s", out.String())
	}
}

func TestHelp(t *testing.T) {
	var out bytes.Buffer
	if err := Main([]string{"--help"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "rvn api") {
		t.Fatalf("%s", out.String())
	}
	if !strings.Contains(out.String(), "perch views") {
		t.Fatalf("want CLI usage, got %s", out.String())
	}
	if !strings.Contains(out.String(), "perch skill") {
		t.Fatalf("want skill usage, got %s", out.String())
	}
	if !strings.Contains(out.String(), "perch <target>") {
		t.Fatalf("want target usage, got %s", out.String())
	}
	if !strings.Contains(out.String(), "poll-interval") {
		t.Fatalf("missing poll-interval flag:\n%s", out.String())
	}
}

func TestUnknownTargetJSON(t *testing.T) {
	srv, cfg := mockRaven(t, map[string]any{
		"version": 1,
		"types":   map[string]any{},
		"queries": map[string]any{},
	})
	var out, errb bytes.Buffer
	err := Main([]string{"nope", "--json", "--url", srv.URL, "--token", "t", "--no-start", "--config", cfg}, &out, &errb)
	if err == nil {
		t.Fatal("expected error")
	}
	var env cli.Response
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatalf("%v %s", jerr, out.String())
	}
	if env.OK || env.Error == nil || env.Error.Code != cli.CodeNotFound {
		t.Fatalf("%+v", env)
	}
}

func TestOpenMissingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`[views.a]
type = "item"
layout = { kind = "list" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, _ := mockRaven(t, map[string]any{
		"version": 1,
		"types":   map[string]any{},
		"queries": map[string]any{},
	})
	var out, errb bytes.Buffer
	err := Main([]string{"open", "ghost", "--config", path, "--json", "--url", srv.URL, "--token", "t", "--no-start"}, &out, &errb)
	if err == nil {
		t.Fatal("expected error")
	}
	var env cli.Response
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatalf("%v %s", jerr, out.String())
	}
	if env.Error == nil || env.Error.Code != cli.CodeNotFound {
		t.Fatalf("%+v", env)
	}
}

func TestSkillListThroughMain(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if err := Main([]string{"--json", "skills", "list", "--dest", root}, &out, &errb); err != nil {
		t.Fatalf("%v %s %s", err, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), `"ok": true`) && !strings.Contains(out.String(), `"ok":true`) {
		t.Fatalf("%s", out.String())
	}
	if !strings.Contains(out.String(), `"name": "perch"`) && !strings.Contains(out.String(), `"name":"perch"`) {
		t.Fatalf("%s", out.String())
	}
}

func TestConfigCheckThroughMain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`[views.a]
type = "item"
layout = { kind = "list" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := Main([]string{"config", "check", "--config", path, "--json"}, &out, &errb); err != nil {
		t.Fatalf("%v %s", err, out.String())
	}
	if !strings.Contains(out.String(), `"ok": true`) && !strings.Contains(out.String(), `"ok":true`) {
		t.Fatalf("%s", out.String())
	}
}

func TestMainRequiresRevision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/health" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "data": map[string]any{"status": "ok", "api_version": "1"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Main([]string{
		"--url", srv.URL, "--token", "t", "--no-start", "--config", cfg,
	}, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "revision") {
		t.Fatalf("got %v out=%s", err, out.String())
	}
}

func TestLaunchTargetResolves(t *testing.T) {
	srv, cfgPath := mockRaven(t, map[string]any{
		"version": 1,
		"types": map[string]any{
			"task": map[string]any{"name": "task", "name_field": "title"},
		},
		"queries": map[string]any{
			"todos": map[string]any{"name": "todos", "query": "type:task"},
			"need":  map[string]any{"name": "need", "query": "type:task", "args": []string{"x"}},
		},
	})
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Views = map[string]config.View{"projects": {Type: "task"}}
	cfg.Layouts = map[string]config.Layout{"workspace": {Split: "row"}}
	client := api.NewClient(srv.URL, "t")
	ctx := context.Background()

	cases := []struct {
		name  string
		extra []string
		kind  string
	}{
		{name: "home", kind: cli.TargetHome},
		{name: "projects", kind: cli.TargetView},
		{name: "workspace", kind: cli.TargetLayout},
		{name: "todos", kind: cli.TargetQuery},
		{name: "task", kind: cli.TargetType},
		{name: "need", extra: []string{"x=p"}, kind: cli.TargetQuery},
	}
	for _, tc := range cases {
		got, err := resolveLaunchTarget(ctx, client, cfg, tc.name, tc.extra)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got.Kind != tc.kind || got.Name != tc.name {
			t.Fatalf("%s: %+v", tc.name, got)
		}
		if tc.name == "need" && got.Inputs["x"] != "p" {
			t.Fatalf("need inputs %+v", got.Inputs)
		}
	}
	if _, err := resolveLaunchTarget(ctx, client, cfg, "ghost", nil); err == nil {
		t.Fatal("ghost")
	}
}

func TestBareTargetUnknownBeforeTUI(t *testing.T) {
	srv, cfg := mockRaven(t, map[string]any{
		"version": 1,
		"types":   map[string]any{"task": map[string]any{"name": "task"}},
		"queries": map[string]any{"todos": map[string]any{"name": "todos", "query": "type:task"}},
	})
	var out, errb bytes.Buffer
	err := Main([]string{"ghost", "--json", "--url", srv.URL, "--token", "t", "--no-start", "--config", cfg}, &out, &errb)
	if err == nil {
		t.Fatal("expected error")
	}
	var env cli.Response
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatal(jerr)
	}
	if env.Error == nil || env.Error.Code != cli.CodeNotFound {
		t.Fatalf("%+v", env)
	}
	if !strings.Contains(env.Error.Message, "ghost") {
		t.Fatalf("message %s", env.Error.Message)
	}
}

func TestViewsSubcommandStillWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`[views.a]
type = "item"
layout = { kind = "list" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := Main([]string{"views", "list", "--config", path, "--json"}, &out, &errb); err != nil {
		t.Fatalf("%v %s", err, out.String())
	}
	if !strings.Contains(out.String(), `"ok": true`) && !strings.Contains(out.String(), `"ok":true`) {
		t.Fatalf("%s", out.String())
	}
}

func mockRaven(t *testing.T, schema map[string]any) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "data": map[string]any{"status": "ok", "api_version": "1"},
			})
		case "/v1/revision":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "data": map[string]any{"generation": "abc", "counter": 1},
			})
		case "/v1/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "data": map[string]any{
					"api_version": "1", "vault_path": "/vault", "editor": "hx", "editor_mode": "terminal",
				},
			})
		case "/v1/schema":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": schema})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return srv, cfg
}
