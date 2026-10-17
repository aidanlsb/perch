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
	if got := Resolve("   "); got != "code" {
		t.Fatalf("empty session editor should fall back, got %q", got)
	}
}

func TestShouldSuspend(t *testing.T) {
	if !ShouldSuspend("vim", ModeTerminal) {
		t.Fatal("terminal always suspends")
	}
	if ShouldSuspend("vim", ModeGUI) {
		t.Fatal("gui never suspends")
	}
	if !ShouldSuspend("nvim", ModeAuto) {
		t.Fatal("auto nvim is terminal")
	}
	if ShouldSuspend("code", ModeAuto) {
		t.Fatal("auto code is gui")
	}
	if !ShouldSuspend("tui", "tui") {
		t.Fatal("tui alias is terminal")
	}
	if ShouldSuspend("/usr/bin/code", "auto") {
		t.Fatal("path to code")
	}
	if !ShouldSuspend(`"/usr/bin/nvim"`, ModeAuto) {
		t.Fatal("quoted nvim")
	}
}

func TestNormalizeMode(t *testing.T) {
	if NormalizeMode("") != ModeAuto {
		t.Fatal("empty")
	}
	if NormalizeMode("TTY") != ModeTerminal {
		t.Fatal("tty")
	}
	if NormalizeMode("non-blocking") != ModeGUI {
		t.Fatal("gui alias")
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

func TestCommandLine(t *testing.T) {
	cmd, err := CommandLine("nvim", "/tmp/config.toml", 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "+12" || cmd.Args[2] != "/tmp/config.toml" {
		t.Fatalf("%+v", cmd.Args)
	}
	args := LineArgs("hx", "/tmp/x.toml", 4)
	if len(args) != 1 || args[0] != "/tmp/x.toml:4" {
		t.Fatalf("%v", args)
	}
	args = LineArgs("code", "/tmp/x.toml", 4)
	if len(args) != 2 || args[0] != "-g" {
		t.Fatalf("%v", args)
	}
	args = LineArgs("unknown", "/tmp/x.toml", 4)
	if len(args) != 1 || args[0] != "/tmp/x.toml" {
		t.Fatalf("unsupported editors skip the jump: %v", args)
	}
}
