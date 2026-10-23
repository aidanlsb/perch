package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
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
// Absolute paths are allowed only when they stay inside the vault.
func FilePath(vaultPath, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	vaultPath = strings.TrimSpace(vaultPath)
	if rel == "" {
		return "", fmt.Errorf("no file path to open")
	}
	if vaultPath == "" {
		return "", fmt.Errorf("no vault path")
	}
	vaultAbs, err := filepath.Abs(vaultPath)
	if err != nil {
		return "", err
	}
	var candidate string
	if filepath.IsAbs(rel) {
		candidate = filepath.Clean(rel)
	} else {
		candidate = filepath.Join(vaultAbs, rel)
	}
	candAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if err := confinedTo(vaultAbs, candAbs); err != nil {
		return "", err
	}
	if vEval, err1 := filepath.EvalSymlinks(vaultAbs); err1 == nil {
		if cEval, err2 := filepath.EvalSymlinks(candAbs); err2 == nil {
			if err := confinedTo(vEval, cEval); err != nil {
				return "", err
			}
			return cEval, nil
		}
	}
	return candAbs, nil
}

func confinedTo(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("path %q is outside the vault", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside the vault", path)
	}
	return nil
}

// Command builds the process that opens path in editor.
func Command(editorCmd, path string) (*exec.Cmd, error) {
	return CommandLine(editorCmd, path, 0)
}

// CommandLine opens path, jumping to line when the editor supports it.
// The editor string is split on quotes/whitespace with no shell expansion.
func CommandLine(editorCmd, path string, line int) (*exec.Cmd, error) {
	editorCmd = strings.TrimSpace(editorCmd)
	path = strings.TrimSpace(path)
	if editorCmd == "" {
		return nil, fmt.Errorf("no editor configured (set $VISUAL or $EDITOR)")
	}
	if path == "" {
		return nil, fmt.Errorf("no file path to open")
	}
	parts, err := splitCommand(editorCmd)
	if err != nil {
		return nil, fmt.Errorf("editor command: %w", err)
	}
	args := LineArgs(commandName(editorCmd), path, line)
	cmdArgs := append(append([]string{}, parts[1:]...), args...)
	return exec.Command(parts[0], cmdArgs...), nil
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
	case "quill":
		return []string{fmt.Sprintf("%s:%d", path, line)}
	case "code", "code-insiders", "codium", "cursor":
		return []string{"-g", fmt.Sprintf("%s:%d", path, line)}
	default:
		return []string{path}
	}
}

func commandName(editorCmd string) string {
	parts, err := splitCommand(editorCmd)
	if err != nil || len(parts) == 0 {
		return ""
	}
	base := filepath.Base(parts[0])
	base = strings.TrimSuffix(base, ".exe")
	return strings.ToLower(base)
}

// splitCommand tokenizes an editor string with shell-style quotes but
// without $ ` or other expansion, and never invokes a shell.
func splitCommand(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty command")
	}
	var parts []string
	var cur strings.Builder
	quote := rune(0)
	escaped := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		if quote == '"' {
			if escaped {
				cur.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				quote = 0
				continue
			}
			cur.WriteRune(r)
			continue
		}
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if escaped {
		return nil, fmt.Errorf("dangling backslash in editor command")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in editor command")
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("empty command")
	}
	return parts, nil
}
