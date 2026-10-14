package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolve picks an editor command. Raven's editor setting is not on rvn api
// (see README). Fall back to $VISUAL, then $EDITOR.
func Resolve(fromAPI string) string {
	if s := strings.TrimSpace(fromAPI); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("VISUAL")); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("EDITOR"))
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
func Command(editor, path string) (*exec.Cmd, error) {
	editor = strings.TrimSpace(editor)
	path = strings.TrimSpace(path)
	if editor == "" {
		return nil, fmt.Errorf("no editor configured (set $VISUAL or $EDITOR)")
	}
	if path == "" {
		return nil, fmt.Errorf("no file path to open")
	}
	if strings.Contains(editor, " ") {
		return exec.Command("sh", "-c", editor+" "+shellQuote(path)), nil
	}
	return exec.Command(editor, path), nil
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
