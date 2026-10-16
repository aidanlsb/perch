package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/layout"
)

const (
	legacyRemoved = `the old single layout = form and [[views]] table were removed.

Use an ordered list of named layouts on each type (and each saved query):

[[types.project.layouts]]
name = "Board"
kind = "board"
group_by = "status"
fields = ["status", "due"]

[[types.project.layouts]]
name = "By due"
kind = "list"
fields = ["status", "due"]
sort = "due"

kind is list, board, or date. Optional filter is an RQL predicate appended to
the type source (e.g. filter = ".status==active"). The first layout is the
default. See examples/config.toml.`
)

// File is Perch's own config. Raven does not read it.
//
// Each type and saved query has an ordered list of named layouts. The first
// is the default. L / gL cycle that list; gl opens the layout picker.
type File struct {
	Types   map[string]Source `toml:"types"`
	Queries map[string]Source `toml:"queries"`
	Views   any               `toml:"views"` // present only to reject the old table
}

// Source is [types.<name>] or [queries.<name>]: a list of named layouts.
type Source struct {
	Layouts []Layout `toml:"layouts"`

	// Legacy single-table keys. Any of these set is a hard error.
	Layout      string   `toml:"layout"`
	Kind        string   `toml:"kind"`
	GroupBy     string   `toml:"group_by"`
	Granularity string   `toml:"granularity"`
	Fields      []string `toml:"fields"`
	Sort        string   `toml:"sort"`
}

// Layout is one named presentation of a type or saved query.
type Layout struct {
	Name        string   `toml:"name"`
	Kind        string   `toml:"kind"` // list, board, date
	GroupBy     string   `toml:"group_by"`
	Granularity string   `toml:"granularity"`
	Fields      []string `toml:"fields"`
	Sort        string   `toml:"sort"`
	Filter      string   `toml:"filter"`
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
	if f.Views != nil {
		return nil, fmt.Errorf("perch config %s: [[views]] was removed.\n\n%s", path, legacyRemoved)
	}
	if f.Types == nil {
		f.Types = map[string]Source{}
	}
	if f.Queries == nil {
		f.Queries = map[string]Source{}
	}
	for name, src := range f.Types {
		if src.hasLegacyKeys() {
			return nil, fmt.Errorf("perch config %s: types.%s: %s", path, name, legacyRemoved)
		}
		layouts, err := normalizeLayouts(src.Layouts)
		if err != nil {
			return nil, fmt.Errorf("types.%s: %w", name, err)
		}
		src.Layouts = layouts
		f.Types[name] = src
	}
	for name, src := range f.Queries {
		if src.hasLegacyKeys() {
			return nil, fmt.Errorf("perch config %s: queries.%s: %s", path, name, legacyRemoved)
		}
		layouts, err := normalizeLayouts(src.Layouts)
		if err != nil {
			return nil, fmt.Errorf("queries.%s: %w", name, err)
		}
		src.Layouts = layouts
		f.Queries[name] = src
	}
	return &f, nil
}

func (s Source) hasLegacyKeys() bool {
	return strings.TrimSpace(s.Layout) != "" ||
		strings.TrimSpace(s.Kind) != "" ||
		strings.TrimSpace(s.GroupBy) != "" ||
		strings.TrimSpace(s.Granularity) != "" ||
		len(s.Fields) > 0 ||
		strings.TrimSpace(s.Sort) != ""
}

func normalizeLayouts(in []Layout) ([]Layout, error) {
	out := make([]Layout, 0, len(in))
	for i, l := range in {
		nl, err := l.normalize()
		if err != nil {
			return nil, fmt.Errorf("layouts[%d]: %w", i, err)
		}
		out = append(out, nl)
	}
	return out, nil
}

func (l Layout) normalize() (Layout, error) {
	l.Name = strings.TrimSpace(l.Name)
	l.Kind = strings.ToLower(strings.TrimSpace(l.Kind))
	l.GroupBy = strings.TrimSpace(l.GroupBy)
	l.Granularity = strings.ToLower(strings.TrimSpace(l.Granularity))
	l.Sort = strings.TrimSpace(l.Sort)
	l.Filter = strings.TrimSpace(l.Filter)
	if l.Kind == "" {
		l.Kind = KindList
	}
	switch l.Kind {
	case KindList, KindBoard, KindDate:
	default:
		return l, fmt.Errorf("unknown kind %q (list, board, date)", l.Kind)
	}
	if l.Name == "" {
		l.Name = layout.DefaultName(l.Kind)
	}
	spec := layout.Spec{
		Name:        l.Name,
		Kind:        l.Kind,
		GroupBy:     l.GroupBy,
		Granularity: l.Granularity,
		Fields:      l.Fields,
		Sort:        l.Sort,
		Filter:      l.Filter,
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
	l.Kind = spec.Kind
	l.GroupBy = spec.GroupBy
	l.Granularity = spec.Granularity
	l.Sort = spec.Sort
	l.Filter = spec.Filter
	return l, nil
}

// Spec converts a named layout into a layout.Spec.
func (l Layout) Spec() layout.Spec {
	return layout.Spec{
		Name:        l.Name,
		Kind:        l.Kind,
		GroupBy:     l.GroupBy,
		Granularity: l.Granularity,
		Fields:      l.Fields,
		Sort:        l.Sort,
		Filter:      l.Filter,
	}.Normalize()
}

// TypeLayouts returns configured layouts for a type, or nil if none.
func (f *File) TypeLayouts(name string) []layout.Spec {
	if f == nil || f.Types == nil {
		return nil
	}
	src, ok := f.Types[name]
	if !ok || len(src.Layouts) == 0 {
		return nil
	}
	out := make([]layout.Spec, 0, len(src.Layouts))
	for _, l := range src.Layouts {
		out = append(out, l.Spec())
	}
	return out
}

// QueryLayouts returns configured layouts for a saved query, or nil if none.
func (f *File) QueryLayouts(name string) []layout.Spec {
	if f == nil || f.Queries == nil {
		return nil
	}
	src, ok := f.Queries[name]
	if !ok || len(src.Layouts) == 0 {
		return nil
	}
	out := make([]layout.Spec, 0, len(src.Layouts))
	for _, l := range src.Layouts {
		out = append(out, l.Spec())
	}
	return out
}

// Validate checks configured layouts against a live schema. Unknown types,
// queries, and fields become warnings (not load errors).
func (f *File) Validate(schema api.Schema) []string {
	if f == nil {
		return nil
	}
	var warns []string
	typeNames := make([]string, 0, len(f.Types))
	for name := range f.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)
	for _, name := range typeNames {
		src := f.Types[name]
		ts, ok := schema.Types[name]
		if !ok {
			warns = append(warns, fmt.Sprintf("types.%s: unknown type", name))
			continue
		}
		for i, l := range src.Layouts {
			warns = append(warns, validateLayout("types."+name, i, l, ts)...)
		}
	}
	queryNames := make([]string, 0, len(f.Queries))
	for name := range f.Queries {
		queryNames = append(queryNames, name)
	}
	sort.Strings(queryNames)
	for _, name := range queryNames {
		src := f.Queries[name]
		q, ok := schema.Queries[name]
		if !ok {
			warns = append(warns, fmt.Sprintf("queries.%s: unknown saved query", name))
		}
		ts := typeSchemaForQuery(schema, q)
		if ts.Name == "" && len(ts.Fields) == 0 {
			continue
		}
		for i, l := range src.Layouts {
			warns = append(warns, validateLayout("queries."+name, i, l, ts)...)
		}
	}
	return warns
}

func typeSchemaForQuery(schema api.Schema, q api.SavedQuery) api.TypeSchema {
	body := strings.TrimSpace(q.Query)
	if !strings.HasPrefix(body, "type:") {
		return api.TypeSchema{}
	}
	rest := strings.TrimPrefix(body, "type:")
	name := strings.Fields(rest)
	if len(name) == 0 {
		return api.TypeSchema{}
	}
	return schema.Types[name[0]]
}

func validateLayout(prefix string, i int, l Layout, ts api.TypeSchema) []string {
	label := l.Name
	if label == "" {
		label = l.Kind
	}
	head := fmt.Sprintf("%s layouts[%d] %q", prefix, i, label)
	var warns []string
	checkField := func(kind, field string) {
		if field == "" {
			return
		}
		f, ok := ts.Fields[field]
		if !ok {
			warns = append(warns, fmt.Sprintf("%s: unknown %s field %q", head, kind, field))
			return
		}
		switch kind {
		case "group_by":
			switch l.Kind {
			case KindBoard:
				if !f.IsEnum() && !f.IsRef() {
					warns = append(warns, fmt.Sprintf("%s: group_by %q is not an enum or ref field", head, field))
				}
			case KindDate:
				if !f.IsDate() || f.IsArray() {
					warns = append(warns, fmt.Sprintf("%s: group_by %q is not a date/datetime field", head, field))
				}
			}
		}
	}
	checkField("group_by", l.GroupBy)
	checkField("sort", l.Sort)
	for _, field := range l.Fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := ts.Fields[field]; !ok {
			warns = append(warns, fmt.Sprintf("%s: unknown fields entry %q", head, field))
		}
	}
	return warns
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
