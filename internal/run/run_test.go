package run

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/cli"
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
	if !strings.Contains(out.String(), "poll-interval") {
		t.Fatalf("missing poll-interval flag:\n%s", out.String())
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	var out, errb bytes.Buffer
	err := Main([]string{"nope", "--json"}, &out, &errb)
	if err == nil {
		t.Fatal("expected error")
	}
	var env cli.Response
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatal(jerr)
	}
	if env.OK || env.Error == nil || env.Error.Code != cli.CodeCommandNotFound {
		t.Fatalf("%+v", env)
	}
}

func TestOpenMissingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`[views.a]
source = "item"
layout = { kind = "list" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	err := Main([]string{"open", "ghost", "--config", path, "--json"}, &out, &errb)
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
source = "item"
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
