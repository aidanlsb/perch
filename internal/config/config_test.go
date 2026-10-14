package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || len(f.Views) != 0 {
		t.Fatalf("%v %+v", err, f)
	}
}

func TestLoadViews(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[[views]]
name = "Projects"
source = "type:project"
kind = "board"
group_by = "status"
fields = ["status", "due"]
sort = "name"

[[views]]
name = "Daily"
source = "type:date"
kind = "date"
group_by = "date"
granularity = "week"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Views) != 2 {
		t.Fatalf("%d", len(f.Views))
	}
	if f.Views[0].Kind != KindBoard || f.Views[0].GroupBy != "status" {
		t.Fatalf("%+v", f.Views[0])
	}
	if f.Views[1].Granularity != GranWeek {
		t.Fatalf("%+v", f.Views[1])
	}
}

func TestLoadRejectsBadKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte(`[[views]]
name = "x"
source = "type:project"
kind = "kanban"
`), 0o644)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}
