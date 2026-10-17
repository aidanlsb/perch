package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ModeAuto     = "auto"
	ModeTerminal = "terminal"
	ModeGUI      = "gui"
)

// Resolve picks an editor command. Prefer the value from GET /v1/session.
// Fall back to $VISUAL, then $EDITOR, when the session editor is empty or
// the endpoint is missing (older rvn).
func Resolve(fromAPI string) string {
	if s := strings.TrimSpace(fromAPI); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("VISUAL")); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("EDITOR"))
}

// NormalizeMode maps Raven editor_mode (and its aliases) to auto|terminal|gui.
func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeTerminal, "tty", "tui", "cli":
		return ModeTerminal
	case ModeGUI, "background", "nonblocking", "non-blocking":
		return ModeGUI
	default:
		return ModeAuto
	}
}

// ShouldSuspend reports whether the TUI should yield the tty to the editor.
// terminal: yes. gui: no. auto: same heuristic as rvn open (isTerminalEditor).
func ShouldSuspend(command, mode string) bool {
	switch NormalizeMode(mode) {
	case ModeTerminal:
		return true
	case ModeGUI:
		return false
	default:
		return IsTerminalEditor(command)
	}
}

// IsTerminalEditor matches Raven's rvn open auto-mode list.
func IsTerminalEditor(command string) bool {
	switch commandName(command) {
	case "vi", "vim", "vimdiff",
		"nvim", "nvimdiff",
		"hx", "helix",
		"nano",
		"emacs", "emacsclient",
		"micro",
		"kak", "kakoune":
		return true
	default:
		return false
	}
}

// FilePath joins a vault-relative read path with the vault root.
func FilePath(vaultPath, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	vaultPath = strings.TrimSpace(vaultPath)
	if vaultPath == "" {
		return rel
	}
	return filepath.Join(vaultPath, rel)
}

// Command builds the process that opens path in editor.
func Command(editorCmd, path string) (*exec.Cmd, error) {
	return CommandLine(editorCmd, path, 0)
}

// CommandLine opens path, jumping to line when the editor supports it.
func CommandLine(editorCmd, path string, line int) (*exec.Cmd, error) {
	editorCmd = strings.TrimSpace(editorCmd)
	path = strings.TrimSpace(path)
	if editorCmd == "" {
		return nil, fmt.Errorf("no editor configured (set $VISUAL or $EDITOR)")
	}
	if path == "" {
		return nil, fmt.Errorf("no file path to open")
	}
	args := LineArgs(commandName(editorCmd), path, line)
	if strings.Contains(editorCmd, " ") {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, shellQuote(a))
		}
		return exec.Command("sh", "-c", editorCmd+" "+strings.Join(parts, " ")), nil
	}
	return exec.Command(editorCmd, args...), nil
}

// LineArgs is the editor argument vector for path, plus a line jump when known.
func LineArgs(name, path string, line int) []string {
	if line <= 0 {
		return []string{path}
	}
	switch name {
	case "vi", "vim", "vimdiff", "nvim", "nvimdiff",
		"emacs", "emacsclient", "nano", "micro", "kak", "kakoune":
		return []string{fmt.Sprintf("+%d", line), path}
	case "hx", "helix":
		return []string{fmt.Sprintf("%s:%d", path, line)}
	case "code", "code-insiders", "codium", "cursor":
		return []string{"-g", fmt.Sprintf("%s:%d", path, line)}
	default:
		return []string{path}
	}
}

func commandName(editorCmd string) string {
	trimmed := strings.TrimSpace(editorCmd)
	if trimmed == "" {
		return ""
	}
	var cmd string
	switch trimmed[0] {
	case '"', '\'':
		quote := trimmed[0]
		if end := strings.IndexByte(trimmed[1:], quote); end >= 0 {
			cmd = trimmed[1 : 1+end]
		} else {
			cmd = trimmed[1:]
		}
	default:
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			return ""
		}
		cmd = fields[0]
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	base := filepath.Base(cmd)
	base = strings.TrimSuffix(base, ".exe")
	return strings.ToLower(base)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
