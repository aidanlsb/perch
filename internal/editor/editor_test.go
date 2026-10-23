package editor

import (
	"os"
	"os/exec"
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
	vault := t.TempDir()
	got, err := FilePath(vault, "projects/a.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(vault, "projects/a.md")
	if got != want {
		t.Fatalf("%s want %s", got, want)
	}
	inside := filepath.Join(vault, "notes/x.md")
	got, err = FilePath(vault, inside)
	if err != nil {
		t.Fatal(err)
	}
	if got != inside {
		t.Fatalf("abs inside vault: %s", got)
	}
}

func TestFilePathRejectsTraversal(t *testing.T) {
	vault := t.TempDir()
	outside := filepath.Join(filepath.Dir(vault), "secret.md")
	if _, err := FilePath(vault, "../secret.md"); err == nil {
		t.Fatal("relative escape")
	}
	if _, err := FilePath(vault, outside); err == nil {
		t.Fatal("absolute outside vault")
	}
	if _, err := FilePath(vault, "/etc/passwd"); err == nil {
		t.Fatal("abs passwd")
	}
	if _, err := FilePath("", "a.md"); err == nil {
		t.Fatal("empty vault")
	}
}

func TestFilePathRejectsSymlinkEscape(t *testing.T) {
	vault := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(target, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(vault, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := FilePath(vault, "link.md"); err == nil {
		t.Fatal("symlink escape")
	}
}

func TestCommand(t *testing.T) {
	cmd, err := Command("vim", "/tmp/note.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "vim" || cmd.Args[1] != "/tmp/note.md" {
		t.Fatalf("%+v", cmd.Args)
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

func TestCommandLineDoesNotUseShell(t *testing.T) {
	assertNoShell := func(t *testing.T, cmd *exec.Cmd) {
		t.Helper()
		base := filepath.Base(cmd.Path)
		if base == "sh" || base == "bash" || (len(cmd.Args) > 0 && (cmd.Args[0] == "sh" || cmd.Args[0] == "-c")) {
			t.Fatalf("must not invoke a shell: path=%q args=%+v", cmd.Path, cmd.Args)
		}
	}

	cmd, err := CommandLine("vim; touch /tmp/pwned", "/tmp/note.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	assertNoShell(t, cmd)
	if cmd.Args[0] != "vim;" {
		t.Fatalf("semicolon must not start a new shell command: %+v", cmd.Args)
	}

	cmd, err = CommandLine("vim $(uname)", "/tmp/note.md", 3)
	if err != nil {
		t.Fatal(err)
	}
	assertNoShell(t, cmd)
	if len(cmd.Args) < 2 || cmd.Args[1] != "$(uname)" {
		t.Fatalf("expansion must not run: %+v", cmd.Args)
	}
}

func TestCommandLineSplitsFlagsWithoutShell(t *testing.T) {
	cmd, err := CommandLine("code --wait", "/tmp/note.md", 4)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code", "--wait", "-g", "/tmp/note.md:4"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("%+v", cmd.Args)
	}
	for i, w := range want {
		if cmd.Args[i] != w {
			t.Fatalf("%+v", cmd.Args)
		}
	}

	cmd, err = CommandLine(`"/usr/bin/nvim" -u NONE`, "/tmp/note.md", 2)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "/usr/bin/nvim" || cmd.Args[1] != "-u" || cmd.Args[2] != "NONE" || cmd.Args[3] != "+2" {
		t.Fatalf("%+v", cmd.Args)
	}
}

func TestSplitCommandRejectsUnclosedQuote(t *testing.T) {
	if _, err := CommandLine(`nvim "oops`, "/tmp/x", 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestLineArgsQuill(t *testing.T) {
	tests := []struct {
		name string
		line int
		want []string
	}{
		{name: "with line", line: 12, want: []string{"/tmp/note.md:12"}},
		{name: "without line", line: 0, want: []string{"/tmp/note.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LineArgs("quill", "/tmp/note.md", tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("%v", got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("%v", got)
				}
			}
		})
	}
}

func TestIsTerminalEditorQuill(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{name: "quill is gui", command: "quill", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTerminalEditor(tt.command); got != tt.want {
				t.Fatalf("IsTerminalEditor(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
