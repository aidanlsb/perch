package editor

import (
	"path/filepath"
	"testing"
)

func TestResolveFallback(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")
	if got := Resolve(""); got != "vim" {
		t.Fatalf("%q", got)
	}
	if got := Resolve("helix"); got != "helix" {
		t.Fatalf("%q", got)
	}
	t.Setenv("VISUAL", "code")
	if got := Resolve(""); got != "code" {
		t.Fatalf("%q", got)
	}
}

func TestFilePath(t *testing.T) {
	got := FilePath("/vault", "projects/a.md")
	if got != filepath.Join("/vault", "projects/a.md") {
		t.Fatalf("%s", got)
	}
	if FilePath("/vault", "/abs/x.md") != "/abs/x.md" {
		t.Fatal("abs")
	}
}

func TestCommand(t *testing.T) {
	cmd, err := Command("vim", "/tmp/note.md")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path == "" || len(cmd.Args) != 2 {
		t.Fatalf("%+v", cmd)
	}
	_, err = Command("", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
