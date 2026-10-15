package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/aidanlsb/perch/internal/layout"
)

// File is Perch's own config. Raven does not read it.
//
// Layout defaults live under [types.<name>] and [queries.<name>]. They set the
// starting Spec for that source; the TUI cycles layouts at runtime with L.
//
// [[views]] remains for extra named sources (a query plus a layout that should
// show up in the sidebar as its own row).
type File struct {
	Types   map[string]Layout `toml:"types"`
	Queries map[string]Layout `toml:"queries"`
	Views   []View            `toml:"views"`
}

// Layout is a default Spec for a type or saved query.
type Layout struct {
	Layout      string   `toml:"layout"` // list, board, date (preferred)
	Kind        string   `toml:"kind"`   // alias of layout
	GroupBy     string   `toml:"group_by"`
	Granularity string   `toml:"granularity"`
	Fields      []string `toml:"fields"`
	Sort        string   `toml:"sort"`
}

// View is an extra named source shown in the sidebar.
type View struct {
	Name        string   `toml:"name"`
	Source      string   `toml:"source"`
	Kind        string   `toml:"kind"`
	Layout      string   `toml:"layout"`
	GroupBy     string   `toml:"group_by"`
	Granularity string   `toml:"granularity"`
	Fields      []string `toml:"fields"`
	Sort        string   `toml:"sort"`
}

const (
	KindList  = layout.KindList
	KindBoard = layout.KindBoard
	KindDate  = layout.KindDate
	GranDay   = layout.GranDay
	GranWeek  = layout.GranWeek
	GranMonth = layout.GranMonth
)

// Load reads a TOML config. Missing file yields an empty File, not an error.
func Load(path string) (*File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &File{}, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &File{}, nil
	}
	var f File
	if _, err := toml.DecodeFile(path, &f); err != nil {
		return nil, fmt.Errorf("perch config %s: %w", path, err)
	}
	if f.Types == nil {
		f.Types = map[string]Layout{}
	}
	if f.Queries == nil {
		f.Queries = map[string]Layout{}
	}
	for name, l := range f.Types {
		nl, err := l.normalize()
		if err != nil {
			return nil, fmt.Errorf("types.%s: %w", name, err)
		}
		f.Types[name] = nl
	}
	for name, l := range f.Queries {
		nl, err := l.normalize()
		if err != nil {
			return nil, fmt.Errorf("queries.%s: %w", name, err)
		}
		f.Queries[name] = nl
	}
	for i := range f.Views {
		if err := f.Views[i].normalize(); err != nil {
			return nil, fmt.Errorf("views[%d]: %w", i, err)
		}
	}
	return &f, nil
}

func (l Layout) normalize() (Layout, error) {
	l.Layout = strings.ToLower(strings.TrimSpace(l.Layout))
	l.Kind = strings.ToLower(strings.TrimSpace(l.Kind))
	l.GroupBy = strings.TrimSpace(l.GroupBy)
	l.Granularity = strings.ToLower(strings.TrimSpace(l.Granularity))
	l.Sort = strings.TrimSpace(l.Sort)
	kind := l.Layout
	if kind == "" {
		kind = l.Kind
	}
	if kind != "" {
		switch kind {
		case KindList, KindBoard, KindDate:
		default:
			return l, fmt.Errorf("unknown layout %q (list, board, date)", kind)
		}
	}
	spec := layout.Spec{
		Kind:        kind,
		GroupBy:     l.GroupBy,
		Granularity: l.Granularity,
		Fields:      l.Fields,
		Sort:        l.Sort,
	}.Normalize()
	if l.Granularity != "" {
		switch l.Granularity {
		case GranDay, GranWeek, GranMonth:
		default:
			return l, fmt.Errorf("unknown granularity %q (day, week, month)", l.Granularity)
		}
	}
	if spec.Kind == KindBoard && spec.GroupBy == "" {
		return l, fmt.Errorf("board layouts require group_by")
	}
	if spec.Kind == KindDate && spec.GroupBy == "" {
		return l, fmt.Errorf("date layouts require group_by")
	}
	l.Layout = spec.Kind
	l.Kind = spec.Kind
	l.GroupBy = spec.GroupBy
	l.Granularity = spec.Granularity
	l.Sort = spec.Sort
	return l, nil
}

func (v *View) normalize() error {
	v.Name = strings.TrimSpace(v.Name)
	v.Source = strings.TrimSpace(v.Source)
	if v.Source == "" {
		return fmt.Errorf("source is required")
	}
	l, err := Layout{
		Layout:      v.Layout,
		Kind:        v.Kind,
		GroupBy:     v.GroupBy,
		Granularity: v.Granularity,
		Fields:      v.Fields,
		Sort:        v.Sort,
	}.normalize()
	if err != nil {
		return err
	}
	v.Layout = l.Layout
	v.Kind = l.Kind
	v.GroupBy = l.GroupBy
	v.Granularity = l.Granularity
	v.Sort = l.Sort
	if v.Name == "" {
		v.Name = v.Source
	}
	return nil
}

// Spec converts a type/query layout into a layout.Spec.
func (l Layout) Spec() layout.Spec {
	return layout.Spec{
		Kind:        firstNonEmpty(l.Layout, l.Kind),
		GroupBy:     l.GroupBy,
		Granularity: l.Granularity,
		Fields:      l.Fields,
		Sort:        l.Sort,
	}.Normalize()
}

// Spec converts a named view into a layout.Spec.
func (v View) Spec() layout.Spec {
	return layout.Spec{
		Kind:        firstNonEmpty(v.Layout, v.Kind),
		GroupBy:     v.GroupBy,
		Granularity: v.Granularity,
		Fields:      v.Fields,
		Sort:        v.Sort,
	}.Normalize()
}

// TypeSpec returns a configured type default, or ok=false.
func (f *File) TypeSpec(name string) (layout.Spec, bool) {
	if f == nil || f.Types == nil {
		return layout.Spec{}, false
	}
	l, ok := f.Types[name]
	if !ok {
		return layout.Spec{}, false
	}
	return l.Spec(), true
}

// QuerySpec returns a configured saved-query default, or ok=false.
func (f *File) QuerySpec(name string) (layout.Spec, bool) {
	if f == nil || f.Queries == nil {
		return layout.Spec{}, false
	}
	l, ok := f.Queries[name]
	if !ok {
		return layout.Spec{}, false
	}
	return l.Spec(), true
}

// DefaultPath is ~/.config/perch/config.toml (XDG).
func DefaultPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "perch", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "perch.toml"
	}
	return filepath.Join(home, ".config", "perch", "config.toml")
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
