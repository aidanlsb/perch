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
	return atomicWrite(path, []byte(stubHeader))
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
	block, err := EncodeView(name, v)
	if err != nil {
		return err
	}
	src, err := readOrEmpty(path)
	if err != nil {
		return err
	}
	return commit(path, replaceOrAppend(src, "views", name, block))
}

// AddView writes a new [views.name] table. It errors if the name already exists.
func AddView(path, name string, v View) error {
	name = strings.TrimSpace(name)
	f, err := loadForWrite(path)
	if err != nil {
		return err
	}
	if _, ok := f.Views[name]; ok {
		return fmt.Errorf("view %q already exists", name)
	}
	return UpsertView(path, name, v)
}

// RemoveView deletes [views.name]. If layouts reference the view, it errors
// unless force is true. Force drops those panels (and layouts left with none).
func RemoveView(path, name string, force bool) error {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		return fmt.Errorf("no config path")
	}
	if name == "" {
		return fmt.Errorf("view name is required")
	}
	src, err := readOrEmpty(path)
	if err != nil {
		return err
	}
	f, err := Parse(path, src)
	if err != nil {
		return err
	}
	if _, ok := f.Views[name]; !ok {
		return fmt.Errorf("view %q not found", name)
	}
	refs := f.ReferencingLayouts(name)
	if len(refs) > 0 && !force {
		return &InUseError{View: name, Layouts: refs}
	}
	next := stripTables(src, "views", name)
	if force {
		for _, ln := range refs {
			l := stripViewFromLayout(f.Layouts[ln], name)
			if len(l.Leaves()) == 0 {
				next = stripTables(next, "layouts", ln)
				continue
			}
			block, encErr := EncodeLayout(ln, l)
			if encErr != nil {
				return encErr
			}
			next = replaceOrAppend(next, "layouts", ln, block)
		}
	}
	return commit(path, next)
}

// UpsertLayout writes [layouts.name], replacing a previous table of that name.
func UpsertLayout(path, name string, l Layout) error {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		return fmt.Errorf("no config path")
	}
	if name == "" {
		return fmt.Errorf("layout name is required")
	}
	block, err := EncodeLayout(name, l)
	if err != nil {
		return err
	}
	src, err := readOrEmpty(path)
	if err != nil {
		return err
	}
	return commit(path, replaceOrAppend(src, "layouts", name, block))
}

// AddLayout writes a new [layouts.name] table. It errors if the name exists.
func AddLayout(path, name string, l Layout) error {
	name = strings.TrimSpace(name)
	f, err := loadForWrite(path)
	if err != nil {
		return err
	}
	if _, ok := f.Layouts[name]; ok {
		return fmt.Errorf("layout %q already exists", name)
	}
	return UpsertLayout(path, name, l)
}

// RemoveLayout deletes [layouts.name].
func RemoveLayout(path, name string) error {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		return fmt.Errorf("no config path")
	}
	if name == "" {
		return fmt.Errorf("layout name is required")
	}
	src, err := readOrEmpty(path)
	if err != nil {
		return err
	}
	f, err := Parse(path, src)
	if err != nil {
		return err
	}
	if _, ok := f.Layouts[name]; !ok {
		return fmt.Errorf("layout %q not found", name)
	}
	return commit(path, stripTables(src, "layouts", name))
}

func loadForWrite(path string) (*File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("no config path")
	}
	src, err := readOrEmpty(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(src) == "" {
		return &File{Path: path, Views: map[string]View{}, Layouts: map[string]Layout{}}, nil
	}
	return Parse(path, src)
}

func readOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if os.IsNotExist(err) {
		return "", nil
	}
	return "", err
}

// commit validates next with the existing loader and writes it atomically.
func commit(path, next string) error {
	if next != "" && !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	if _, err := Parse(path, next); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWrite(path, []byte(next))
}

func atomicWrite(path string, data []byte) error {
	perm := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	_ = tmp.Chmod(perm)
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	ok = true
	return nil
}

// EncodeView renders a [views.name] table using the documented inline layout form.
func EncodeView(name string, v View) (string, error) {
	nv, err := v.normalize()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[views.%s]\n", tomlKey(name))
	if nv.Query != "" {
		fmt.Fprintf(&b, "query = %s\n", tomlString(nv.Query))
	} else {
		fmt.Fprintf(&b, "type = %s\n", tomlString(nv.Type))
	}
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

// EncodeLayout renders a [layouts.name] table.
func EncodeLayout(name string, l Layout) (string, error) {
	nl, err := l.normalize()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[layouts.%s]\n", tomlKey(name))
	fmt.Fprintf(&b, "split = %s\n", tomlString(nl.Split))
	b.WriteString("panels = [\n")
	b.WriteString(encodePanels(nl.Panels, "  "))
	b.WriteString("]\n")
	return b.String(), nil
}

func encodePanels(ps []Panel, indent string) string {
	var b strings.Builder
	for i, p := range ps {
		b.WriteString(indent)
		b.WriteString(encodePanel(p, indent))
		if i+1 < len(ps) {
			b.WriteString(",")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func encodePanel(p Panel, indent string) string {
	if p.View != "" {
		s := "{ view = " + tomlString(p.View)
		if p.Bind != "" {
			s += ", bind = " + tomlString(p.Bind)
		}
		return s + " }"
	}
	var b strings.Builder
	b.WriteString("{ split = " + tomlString(p.Split) + ", panels = [\n")
	inner := indent + "    "
	b.WriteString(encodePanels(p.Panels, inner))
	b.WriteString(indent)
	b.WriteString("  ] }")
	return b.String()
}

func stripViewFromLayout(l Layout, view string) Layout {
	l.Panels = stripViewFromPanels(l.Panels, view)
	return l
}

func stripViewFromPanels(ps []Panel, view string) []Panel {
	var out []Panel
	for _, p := range ps {
		if p.View == view {
			continue
		}
		if p.View == "" && len(p.Panels) > 0 {
			p.Panels = stripViewFromPanels(p.Panels, view)
			if len(p.Panels) == 0 {
				continue
			}
		}
		if p.Bind == view {
			p.Bind = ""
		}
		out = append(out, p)
	}
	return out
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

func replaceOrAppend(src, kind, name, block string) string {
	body := strings.TrimRight(stripTables(src, kind, name), "\n")
	if body == "" {
		return block
	}
	return body + "\n\n" + block
}

func stripTables(src, kind, name string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			if isKindTableHeader(trim, kind, name) {
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

func isKindTableHeader(trim, kind, name string) bool {
	prefix := "[" + kind + "."
	if !strings.HasPrefix(trim, prefix) || !strings.HasSuffix(trim, "]") {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(trim, prefix), "]")
	if inner == name || strings.HasPrefix(inner, name+".") {
		return true
	}
	q := tomlString(name)
	return inner == q || strings.HasPrefix(inner, q+".")
}
