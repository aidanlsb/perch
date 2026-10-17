package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const stubHeader = `# Perch config. Raven does not read this file.
# Type, query, and field names are yours; Perch does not treat any of them as special.
# See examples/config.toml.
`

// Ensure writes a comment-only file if path does not exist.
func Ensure(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("no config path")
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(stubHeader), 0o644)
}

// UpsertView writes [views.name] into path, replacing a previous table of that
// name. It never writes outside path (Perch's config, not the vault).
func UpsertView(path, name string, v View) error {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		return fmt.Errorf("no config path")
	}
	if name == "" {
		return fmt.Errorf("view name is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing := ""
	data, err := os.ReadFile(path)
	if err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	block, err := EncodeView(name, v)
	if err != nil {
		return err
	}
	next := replaceOrAppendView(existing, name, block)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(next), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// EncodeView renders a [views.name] table using the documented inline layout form.
func EncodeView(name string, v View) (string, error) {
	nv, err := v.normalize()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[views.%s]\n", tomlKey(name))
	fmt.Fprintf(&b, "source = %s\n", tomlString(nv.Source))
	if len(nv.Inputs) > 0 {
		keys := make([]string, 0, len(nv.Inputs))
		for k := range nv.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("inputs = { ")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s = %s", tomlKey(k), tomlString(nv.Inputs[k]))
		}
		b.WriteString(" }\n")
	}
	b.WriteString("layout = {")
	fmt.Fprintf(&b, " kind = %s", tomlString(nv.Layout.Kind))
	if nv.Layout.GroupBy != "" {
		fmt.Fprintf(&b, ", group_by = %s", tomlString(nv.Layout.GroupBy))
	}
	if len(nv.Layout.Fields) > 0 {
		b.WriteString(", fields = [")
		for i, f := range nv.Layout.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tomlString(f))
		}
		b.WriteString("]")
	}
	if nv.Layout.Sort != "" {
		fmt.Fprintf(&b, ", sort = %s", tomlString(nv.Layout.Sort))
	}
	if nv.Layout.Filter != "" {
		fmt.Fprintf(&b, ", filter = %s", tomlString(nv.Layout.Filter))
	}
	if nv.Layout.Granularity != "" && nv.Layout.Kind == KindDate {
		fmt.Fprintf(&b, ", granularity = %s", tomlString(nv.Layout.Granularity))
	}
	b.WriteString(" }\n")
	return b.String(), nil
}

func tomlKey(name string) string {
	if isBareTOMLKey(name) {
		return name
	}
	return tomlString(name)
}

func isBareTOMLKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func tomlString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

func replaceOrAppendView(src, name, block string) string {
	body := stripViewTables(src, name)
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return block
	}
	return body + "\n\n" + block
}

func stripViewTables(src, name string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			if isViewTableHeader(trim, name) {
				skip = true
				continue
			}
			skip = false
		}
		if skip {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isViewTableHeader(trim, name string) bool {
	if !strings.HasPrefix(trim, "[views.") || !strings.HasSuffix(trim, "]") {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(trim, "[views."), "]")
	if inner == name || strings.HasPrefix(inner, name+".") {
		return true
	}
	q := `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
	return inner == q || strings.HasPrefix(inner, q+".")
}
