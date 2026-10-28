package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/layout"
)

const SelectedToken = "{{selected}}"

const viewShape = `[views.projects]
type = "item"
kind = "board"
group_by = "state"
fields = ["state", "when"]

[views.tasks]
query = "open-tasks"
inputs = { item = "{{selected}}" }
kind = "list"

[layouts.workspace]
split = "row"
panels = [
  { view = "projects" },
  { type = "task", bind = "projects", kind = "list" },
  { query = "inbox", id = "inbox", kind = "list" },
]

A view requires exactly one of type (a schema type name) or query (a Raven saved query name).
A layout panel is a named view, or that same type/query plus kind/group_by/fields inline.
Inline RQL is not allowed in config. kind is list, board, or date. See examples/config.toml.
layout exclusively means [layouts.<name>]; do not set a nested layout = { ... } on a view or panel.`

const nestedLayoutRemoved = `views.*.layout was removed; set kind, group_by, fields, sort, filter, and granularity directly on the view or panel`

const legacyRemoved = `[[types.*.layouts]] and [[queries.*.layouts]] were removed.

Use named views and layouts. Type, query, and field names are yours;
Perch does not treat any of them as special:

` + viewShape

const sourceRemoved = `views.*.source was removed.

Use named views and layouts. Type, query, and field names are yours;
Perch does not treat any of them as special:

` + viewShape

// File is Perch's own config. Raven does not read it.
type File struct {
	Path    string            `toml:"-"`
	Source  string            `toml:"-"`
	Lines   map[string]int    `toml:"-"` // "views.name" / "layouts.name" → 1-based line
	Views   map[string]View   `toml:"views"`
	Layouts map[string]Layout `toml:"layouts"`
}

// View is [views.<name>]: exactly one of type or query, plus a list/board/date presentation.
// Renderer keys (kind, group_by, fields, sort, filter, granularity) sit on the view.
type View struct {
	Type   string            `toml:"type" json:"type,omitempty"`
	Query  string            `toml:"query" json:"query,omitempty"`
	Inputs map[string]string `toml:"inputs" json:"inputs,omitempty"`
	Present
	NestedLayout any `toml:"layout,omitempty" json:"-"`
}

// Present is the list/board/date renderer on a view or inline panel.
type Present struct {
	Kind        string   `toml:"kind" json:"kind,omitempty"`
	GroupBy     string   `toml:"group_by" json:"group_by,omitempty"`
	Fields      []string `toml:"fields" json:"fields,omitempty"`
	Sort        string   `toml:"sort" json:"sort,omitempty"`
	Filter      string   `toml:"filter" json:"filter,omitempty"`
	Granularity string   `toml:"granularity" json:"granularity,omitempty"`
}

// Layout is [layouts.<name>]: a nested split of panels. Each leaf is a named
// view, or an inline type or saved query (with optional kind / inputs / id).
type Layout struct {
	Split  string  `toml:"split" json:"split"`
	Panels []Panel `toml:"panels" json:"panels"`
}

// Panel is a leaf or a nested split ({ split, panels }). A leaf is exactly one of:
// view (a [views.<name>]), type (schema type), or query (saved query).
type Panel struct {
	View   string            `toml:"view" json:"view,omitempty"`
	Type   string            `toml:"type" json:"type,omitempty"`
	Query  string            `toml:"query" json:"query,omitempty"`
	Inputs map[string]string `toml:"inputs" json:"inputs,omitempty"`
	Present
	ID           string  `toml:"id" json:"id,omitempty"`
	Name         string  `toml:"name" json:"name,omitempty"`
	Bind         string  `toml:"bind" json:"bind,omitempty"`
	Split        string  `toml:"split" json:"split,omitempty"`
	Panels       []Panel `toml:"panels" json:"panels,omitempty"`
	NestedLayout any     `toml:"layout,omitempty" json:"-"`
}

const (
	KindList  = layout.KindList
	KindBoard = layout.KindBoard
	KindDate  = layout.KindDate
	GranDay   = layout.GranDay
	GranWeek  = layout.GranWeek
	GranMonth = layout.GranMonth
	SplitRow  = "row"
	SplitCol  = "col"
)

// Load reads a TOML config. Missing file yields an empty File, not an error.
func Load(path string) (*File, error) {
	path = strings.TrimSpace(path)
	f := &File{
		Path:    path,
		Views:   map[string]View{},
		Layouts: map[string]Layout{},
		Lines:   map[string]int{},
	}
	if path == "" {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return nil, fmt.Errorf("perch config %s: %w", path, err)
	}
	return Parse(path, string(data))
}

// Parse validates src as a Perch config. path is used in errors and File.Path.
func Parse(path, src string) (*File, error) {
	f := &File{
		Path:    path,
		Source:  src,
		Views:   map[string]View{},
		Layouts: map[string]Layout{},
		Lines:   tableLines(src),
	}

	var probe struct {
		Views   any `toml:"views"`
		Layouts any `toml:"layouts"`
		Types   any `toml:"types"`
		Queries any `toml:"queries"`
	}
	meta, err := toml.Decode(src, &probe)
	if err != nil {
		return nil, wrapDecode(path, err)
	}
	if meta.IsDefined("types") || meta.IsDefined("queries") {
		return nil, &Error{Path: path, Line: firstLine(src, "types", "queries"), Msg: legacyRemoved}
	}
	if isSlice(probe.Views) {
		return nil, &Error{
			Path: path,
			Line: firstLine(src, "views"),
			Msg:  "[[views]] (array of tables) was removed.\n\n" + legacyRemoved,
		}
	}

	var decoded File
	meta, err = toml.Decode(src, &decoded)
	if err != nil {
		return nil, wrapDecode(path, err)
	}
	f.Views = decoded.Views
	f.Layouts = decoded.Layouts
	if f.Views == nil {
		f.Views = map[string]View{}
	}
	if f.Layouts == nil {
		f.Layouts = map[string]Layout{}
	}

	for name, v := range f.Views {
		if meta.IsDefined("views", name, "source") {
			return nil, &Error{Path: path, Line: f.viewLine(name), Msg: sourceKeyMsg(name)}
		}
		if meta.IsDefined("views", name, "layout") || v.NestedLayout != nil {
			return nil, &Error{Path: path, Line: f.viewLine(name), Msg: nestedLayoutRemoved}
		}
		nv, err := v.normalize()
		if err != nil {
			return nil, &Error{Path: path, Line: f.viewLine(name), Msg: fmt.Sprintf("views.%s: %s", name, err.Error())}
		}
		f.Views[name] = nv
	}
	for name, l := range f.Layouts {
		nl, err := l.normalize()
		if err != nil {
			return nil, &Error{Path: path, Line: f.layoutLine(name), Msg: fmt.Sprintf("layouts.%s: %s", name, err.Error())}
		}
		if err := validateLayoutTree(name, nl, f.Views); err != nil {
			return nil, &Error{Path: path, Line: f.layoutLine(name), Msg: err.Error()}
		}
		f.Layouts[name] = nl
	}
	return f, nil
}

func (v View) normalize() (View, error) {
	if v.NestedLayout != nil {
		return v, fmt.Errorf("%s", nestedLayoutRemoved)
	}
	v.Type = strings.TrimSpace(v.Type)
	v.Query = strings.TrimSpace(v.Query)
	if err := v.typeQueryChoice(); err != nil {
		return v, err
	}
	p, err := v.Present.normalize()
	if err != nil {
		return v, err
	}
	v.Present = p
	v.NestedLayout = nil
	if v.Inputs != nil {
		clean := make(map[string]string, len(v.Inputs))
		for k, val := range v.Inputs {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			clean[k] = val
		}
		v.Inputs = clean
	}
	return v, nil
}

func (v View) typeQueryChoice() error {
	if v.Type != "" && v.Query != "" {
		return fmt.Errorf("type and query are mutually exclusive; set exactly one")
	}
	if v.Type == "" && v.Query == "" {
		return fmt.Errorf("exactly one of type or query is required")
	}
	return nil
}

func sourceKeyMsg(name string) string {
	return fmt.Sprintf("views.%s: source was removed.\n\n%s", name, sourceRemoved)
}

func isTypeQueryChoiceError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "type and query are mutually exclusive") ||
		strings.Contains(s, "exactly one of type or query is required")
}

func (p Present) normalize() (Present, error) {
	p.Kind = strings.ToLower(strings.TrimSpace(p.Kind))
	p.GroupBy = strings.TrimSpace(p.GroupBy)
	p.Granularity = strings.ToLower(strings.TrimSpace(p.Granularity))
	p.Sort = strings.TrimSpace(p.Sort)
	p.Filter = strings.TrimSpace(p.Filter)
	if p.Kind == "" {
		p.Kind = KindList
	}
	switch p.Kind {
	case KindList, KindBoard, KindDate:
	default:
		return p, fmt.Errorf("unknown kind %q (list, board, date)", p.Kind)
	}
	if p.Granularity != "" {
		switch p.Granularity {
		case GranDay, GranWeek, GranMonth:
		default:
			return p, fmt.Errorf("unknown granularity %q (day, week, month)", p.Granularity)
		}
	}
	spec := p.Spec("").Normalize()
	if spec.Kind == KindBoard && spec.GroupBy == "" {
		return p, fmt.Errorf("board layouts require group_by")
	}
	if spec.Kind == KindDate && spec.GroupBy == "" {
		return p, fmt.Errorf("date layouts require group_by")
	}
	p.Kind = spec.Kind
	p.GroupBy = spec.GroupBy
	p.Granularity = spec.Granularity
	p.Sort = spec.Sort
	p.Filter = spec.Filter
	return p, nil
}

func (l Layout) normalize() (Layout, error) {
	l.Split = strings.ToLower(strings.TrimSpace(l.Split))
	if l.Split == "" {
		l.Split = SplitRow
	}
	if l.Split != SplitRow && l.Split != SplitCol {
		return l, fmt.Errorf("split must be row or col, got %q", l.Split)
	}
	if len(l.Panels) == 0 {
		return l, fmt.Errorf("layouts require at least one panel")
	}
	var err error
	l.Panels, err = normalizePanels(l.Panels)
	if err != nil {
		return l, err
	}
	return l, nil
}

func normalizePanels(in []Panel) ([]Panel, error) {
	out := make([]Panel, 0, len(in))
	for i, p := range in {
		np, err := p.normalize()
		if err != nil {
			return nil, fmt.Errorf("panels[%d]: %w", i, err)
		}
		out = append(out, np)
	}
	return out, nil
}

func (p Panel) normalize() (Panel, error) {
	if p.NestedLayout != nil {
		return p, fmt.Errorf("%s", nestedLayoutRemoved)
	}
	p.View = strings.TrimSpace(p.View)
	p.Type = strings.TrimSpace(p.Type)
	p.Query = strings.TrimSpace(p.Query)
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.Bind = strings.TrimSpace(p.Bind)
	p.Split = strings.ToLower(strings.TrimSpace(p.Split))
	if p.ID != "" && p.Name != "" && p.ID != p.Name {
		return p, fmt.Errorf("id and name must match (got %q and %q)", p.ID, p.Name)
	}
	if p.Inputs != nil {
		clean := make(map[string]string, len(p.Inputs))
		for k, val := range p.Inputs {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			clean[k] = val
		}
		p.Inputs = clean
	}

	hasView := p.View != ""
	hasType := p.Type != ""
	hasQuery := p.Query != ""
	hasSplit := p.Split != "" || len(p.Panels) > 0
	n := 0
	if hasView {
		n++
	}
	if hasType {
		n++
	}
	if hasQuery {
		n++
	}
	if hasSplit {
		n++
	}
	if n > 1 {
		return p, fmt.Errorf("panel cannot mix view, type, query, and split")
	}
	if n == 0 {
		return p, fmt.Errorf("panel needs view, type, query, or split")
	}
	if hasSplit {
		if p.Bind != "" {
			return p, fmt.Errorf("bind is only valid on a leaf panel")
		}
		if p.ID != "" || p.Name != "" {
			return p, fmt.Errorf("id/name is only valid on a leaf panel")
		}
		if len(p.Inputs) > 0 || !p.isZero() {
			return p, fmt.Errorf("split panel cannot set inputs, kind, group_by, fields, sort, filter, or granularity")
		}
		if p.Split == "" {
			p.Split = SplitRow
		}
		if p.Split != SplitRow && p.Split != SplitCol {
			return p, fmt.Errorf("split must be row or col, got %q", p.Split)
		}
		if len(p.Panels) == 0 {
			return p, fmt.Errorf("split panel requires panels")
		}
		var err error
		p.Panels, err = normalizePanels(p.Panels)
		return p, err
	}
	if p.Split != "" || len(p.Panels) > 0 {
		return p, fmt.Errorf("leaf panel cannot nest panels")
	}
	if hasView {
		if !p.isZero() {
			return p, fmt.Errorf("view panel cannot set kind, group_by, fields, sort, filter, or granularity (set them on the view)")
		}
		if len(p.Inputs) > 0 {
			return p, fmt.Errorf("view panel cannot set inputs (set them on the view)")
		}
		p.NestedLayout = nil
		return p, nil
	}
	np, err := p.Present.normalize()
	if err != nil {
		return p, err
	}
	p.Present = np
	p.NestedLayout = nil
	return p, nil
}

func (p Present) isZero() bool {
	return p.Kind == "" && p.GroupBy == "" && len(p.Fields) == 0 && p.Sort == "" && p.Filter == "" && p.Granularity == ""
}

// IsLeaf reports whether the panel is a view, type, or query (not a split).
func (p Panel) IsLeaf() bool {
	return p.View != "" || p.Type != "" || p.Query != ""
}

// LeafID is the panel id used for focus, bind, and uniqueness.
// Prefer id, then name, then view, then type:<type> or query:<query>.
func (p Panel) LeafID() string {
	if p.ID != "" {
		return p.ID
	}
	if p.Name != "" {
		return p.Name
	}
	if p.View != "" {
		return p.View
	}
	if p.Type != "" {
		return "type:" + p.Type
	}
	if p.Query != "" {
		return "query:" + p.Query
	}
	return ""
}

// AsView is the view this panel loads: a named [views.*] or an inline type/query.
func (p Panel) AsView(views map[string]View) View {
	if p.View != "" {
		if views == nil {
			return View{}
		}
		return views[p.View]
	}
	return View{Type: p.Type, Query: p.Query, Inputs: p.Inputs, Present: p.Present}
}

// Summary is a short label for CLI lists (view name, type:x, or query:x).
func (p Panel) Summary() string {
	switch {
	case p.View != "":
		return p.View
	case p.Query != "":
		return "query:" + p.Query
	case p.Type != "":
		return "type:" + p.Type
	default:
		return p.LeafID()
	}
}

func validateLayoutTree(name string, l Layout, views map[string]View) error {
	leaves := l.Leaves()
	if len(leaves) == 0 {
		return fmt.Errorf("layouts.%s: no panels", name)
	}
	seen := map[string]struct{}{}
	ids := map[string]struct{}{}
	for _, p := range leaves {
		id := p.LeafID()
		if id == "" {
			return fmt.Errorf("layouts.%s: panel needs id, name, view, type, or query", name)
		}
		if p.View != "" {
			if _, ok := views[p.View]; !ok {
				return fmt.Errorf("layouts.%s: unknown view %q", name, p.View)
			}
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("layouts.%s: duplicate panel %q", name, id)
		}
		seen[id] = struct{}{}
		ids[id] = struct{}{}
	}
	binds := map[string]string{}
	for _, p := range leaves {
		if p.Bind == "" {
			continue
		}
		id := p.LeafID()
		if p.Bind == id {
			return fmt.Errorf("layouts.%s: panel %q cannot bind to itself", name, id)
		}
		if _, ok := ids[p.Bind]; !ok {
			return fmt.Errorf("layouts.%s: panel %q bind %q is not a panel in this layout", name, id, p.Bind)
		}
		binds[id] = p.Bind
	}
	if err := bindCycles(binds); err != nil {
		return fmt.Errorf("layouts.%s: %w", name, err)
	}
	return nil
}

func bindCycles(binds map[string]string) error {
	const visiting, done = 1, 2
	state := map[string]int{}
	var walk func(string) error
	walk = func(n string) error {
		switch state[n] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("bind cycle involving %q", n)
		}
		state[n] = visiting
		if next, ok := binds[n]; ok {
			if err := walk(next); err != nil {
				return err
			}
		}
		state[n] = done
		return nil
	}
	names := make([]string, 0, len(binds))
	for n := range binds {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := walk(n); err != nil {
			return err
		}
	}
	return nil
}

// Leaves returns leaf panels in depth-first order.
func (l Layout) Leaves() []Panel {
	return flattenPanels(l.Panels)
}

func flattenPanels(ps []Panel) []Panel {
	var out []Panel
	for _, p := range ps {
		if p.IsLeaf() {
			out = append(out, p)
			continue
		}
		out = append(out, flattenPanels(p.Panels)...)
	}
	return out
}

// Spec converts a view's presentation into a layout.Spec named after the view.
func (v View) Spec(name string) layout.Spec {
	return v.Present.Spec(name)
}

func (p Present) Spec(name string) layout.Spec {
	return layout.Spec{
		Name:        name,
		Kind:        p.Kind,
		GroupBy:     p.GroupBy,
		Granularity: p.Granularity,
		Fields:      p.Fields,
		Sort:        p.Sort,
		Filter:      p.Filter,
	}.Normalize()
}

func (p Present) FromSpec(s layout.Spec) Present {
	s = s.Normalize()
	return Present{
		Kind:        s.Kind,
		GroupBy:     s.GroupBy,
		Granularity: s.Granularity,
		Fields:      s.Fields,
		Sort:        s.Sort,
		Filter:      s.Filter,
	}
}

// NeedsSelected reports whether type, query, filter, or inputs reference {{selected}}.
func (v View) NeedsSelected() bool {
	if strings.Contains(v.Type, SelectedToken) || strings.Contains(v.Query, SelectedToken) {
		return true
	}
	if strings.Contains(v.Filter, SelectedToken) {
		return true
	}
	for _, val := range v.Inputs {
		if strings.Contains(val, SelectedToken) {
			return true
		}
	}
	return false
}

// SubstituteSelected replaces {{selected}} in s.
func SubstituteSelected(s, id string) string {
	return strings.ReplaceAll(s, SelectedToken, id)
}

func (f *File) ViewNames() []string {
	if f == nil || len(f.Views) == 0 {
		return nil
	}
	names := make([]string, 0, len(f.Views))
	for name := range f.Views {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ViewsInOrder returns view names in config file order (table appearance).
// Names without a recorded line sort after those with lines, then alphabetically.
func (f *File) ViewsInOrder() []string {
	names := f.ViewNames()
	if f == nil || len(names) <= 1 {
		return names
	}
	sort.SliceStable(names, func(i, j int) bool {
		li, lj := f.viewLine(names[i]), f.viewLine(names[j])
		if li == lj {
			return names[i] < names[j]
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
	return names
}

func (f *File) LayoutNames() []string {
	if f == nil || len(f.Layouts) == 0 {
		return nil
	}
	names := make([]string, 0, len(f.Layouts))
	for name := range f.Layouts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LayoutsInOrder returns layout names in config file order (table appearance).
func (f *File) LayoutsInOrder() []string {
	names := f.LayoutNames()
	if f == nil || len(names) <= 1 {
		return names
	}
	sort.SliceStable(names, func(i, j int) bool {
		li, lj := f.layoutLine(names[i]), f.layoutLine(names[j])
		if li == lj {
			return names[i] < names[j]
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
	return names
}

// ReferencingLayouts returns layout names that include view, sorted.
func (f *File) ReferencingLayouts(view string) []string {
	if f == nil {
		return nil
	}
	var out []string
	for _, name := range f.LayoutNames() {
		for _, p := range f.Layouts[name].Leaves() {
			if p.View == view {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

func (f *File) viewLine(name string) int {
	if f == nil || f.Lines == nil {
		return 0
	}
	return f.Lines["views."+name]
}

func (f *File) layoutLine(name string) int {
	if f == nil || f.Lines == nil {
		return 0
	}
	return f.Lines["layouts."+name]
}

// ViewLine is the 1-based line of [views.name], or 0.
func (f *File) ViewLine(name string) int { return f.viewLine(name) }

// LayoutLine is the 1-based line of [layouts.name], or 0.
func (f *File) LayoutLine(name string) int { return f.layoutLine(name) }

// Validate checks configured views against a live schema. Unknown types and
// queries, and unknown fields, become warnings (not load errors). Messages
// include line numbers when known. Layout structure is already a load error.
func (f *File) Validate(schema api.Schema) []string {
	if f == nil {
		return nil
	}
	var warns []string
	for _, name := range f.ViewNames() {
		v := f.Views[name]
		for _, msg := range validateView("views."+name, v, schema) {
			warns = append(warns, f.formatViewIssue(name, msg))
		}
	}
	for _, name := range f.LayoutNames() {
		for _, p := range f.Layouts[name].Leaves() {
			if p.View != "" {
				continue
			}
			prefix := "layouts." + name + "." + p.LeafID()
			for _, msg := range validateView(prefix, p.AsView(nil), schema) {
				warns = append(warns, f.formatLayoutIssue(name, msg))
			}
		}
	}
	return warns
}

func (f *File) formatViewIssue(name, msg string) string {
	e := &Error{Msg: msg}
	if f != nil {
		e.Path = f.Path
		e.Line = f.viewLine(name)
	}
	return e.Error()
}

func (f *File) formatLayoutIssue(name, msg string) string {
	e := &Error{Msg: msg}
	if f != nil {
		e.Path = f.Path
		e.Line = f.layoutLine(name)
	}
	return e.Error()
}

func validateView(prefix string, v View, schema api.Schema) []string {
	var warns []string
	var ts api.TypeSchema
	switch {
	case strings.TrimSpace(v.Type) != "":
		t, ok := schema.Types[v.Type]
		if !ok {
			return []string{fmt.Sprintf("%s: unknown type %q", prefix, v.Type)}
		}
		ts = t
	case strings.TrimSpace(v.Query) != "":
		q, ok := schema.Queries[v.Query]
		if !ok {
			return []string{fmt.Sprintf("%s: unknown query %q", prefix, v.Query)}
		}
		ts = typeSchemaOfSavedQuery(schema, q)
		if ts.Name == "" && len(ts.Fields) == 0 {
			return nil
		}
	default:
		return nil
	}
	spec := v.Spec(prefix)
	check := func(kind, field string) {
		if field == "" {
			return
		}
		f, ok := ts.Fields[field]
		if !ok {
			warns = append(warns, fmt.Sprintf("%s: unknown %s field %q", prefix, kind, field))
			return
		}
		if kind != "group_by" {
			return
		}
		switch spec.Kind {
		case KindBoard:
			if !f.IsEnum() && !f.IsRef() {
				warns = append(warns, fmt.Sprintf("%s: group_by %q is not an enum or ref field", prefix, field))
			}
		case KindDate:
			if !f.IsDate() || f.IsArray() {
				warns = append(warns, fmt.Sprintf("%s: group_by %q is not a date/datetime field", prefix, field))
			}
		}
	}
	check("group_by", spec.GroupBy)
	check("sort", spec.Sort)
	for _, field := range spec.Fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := ts.Fields[field]; !ok {
			warns = append(warns, fmt.Sprintf("%s: unknown fields entry %q", prefix, field))
		}
	}
	return warns
}

func typeSchemaOfSavedQuery(schema api.Schema, q api.SavedQuery) api.TypeSchema {
	body := strings.TrimSpace(q.Query)
	if !strings.HasPrefix(body, "type:") {
		return api.TypeSchema{}
	}
	rest := strings.Fields(strings.TrimPrefix(body, "type:"))
	if len(rest) == 0 {
		return api.TypeSchema{}
	}
	return schema.Types[rest[0]]
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

// Error is a config problem, with a line number when known.
type Error struct {
	Path string
	Line int
	Msg  string
}

// InUseError is returned when removing a view that layouts still reference.
type InUseError struct {
	View    string
	Layouts []string
}

func (e *InUseError) Error() string {
	if e == nil {
		return "view is referenced by a layout"
	}
	return fmt.Sprintf("view %q is referenced by layouts %s (pass --force)", e.View, strings.Join(e.Layouts, ", "))
}

func (e *Error) Error() string {
	if e == nil {
		return "perch config error"
	}
	msg := strings.TrimSpace(e.Msg)
	if e.Line > 0 && e.Path != "" {
		return fmt.Sprintf("perch config %s:%d: %s", e.Path, e.Line, msg)
	}
	if e.Path != "" {
		return fmt.Sprintf("perch config %s: %s", e.Path, msg)
	}
	if e.Line > 0 {
		return fmt.Sprintf("perch config line %d: %s", e.Line, msg)
	}
	return "perch config: " + msg
}

func wrapDecode(path string, err error) error {
	line := tomlLine(err)
	if line == 0 {
		line = 1
	}
	return &Error{Path: path, Line: line, Msg: err.Error()}
}

func tomlLine(err error) int {
	if err == nil {
		return 0
	}
	s := err.Error()
	for _, prefix := range []string{"Near line ", "line "} {
		if i := strings.Index(s, prefix); i >= 0 {
			rest := s[i+len(prefix):]
			n := 0
			for _, r := range rest {
				if r < '0' || r > '9' {
					break
				}
				n = n*10 + int(r-'0')
			}
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

var tableHeader = regexp.MustCompile(`(?m)^[ \t]*\[(views|layouts)\.(("([^"\\]|\\.)*")|([A-Za-z0-9_-]+))\]`)

func tableLines(src string) map[string]int {
	out := map[string]int{}
	for _, m := range tableHeader.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 12 {
			continue
		}
		kind := src[m[2]:m[3]]
		name := src[m[4]:m[5]]
		if strings.HasPrefix(name, `"`) {
			if un, err := strconv.Unquote(name); err == nil {
				name = un
			} else {
				name = strings.Trim(name, `"`)
			}
		}
		line := 1 + strings.Count(src[:m[0]], "\n")
		key := kind + "." + name
		if _, ok := out[key]; !ok {
			out[key] = line
		}
	}
	return out
}

func isSlice(v any) bool {
	if v == nil {
		return false
	}
	return reflect.ValueOf(v).Kind() == reflect.Slice
}

// Issue is one config problem with an optional 1-based line.
type Issue struct {
	Line    int    `json:"line,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// Check reads path and returns every structural problem. A missing file is valid.
func Check(path string) []Issue {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []Issue{{Path: path, Message: err.Error()}}
	}
	return CheckSource(path, string(data))
}

// CheckSource validates src without writing. Decode/legacy failures are one
// issue. After a successful decode it reports every view and layout problem.
func CheckSource(path, src string) []Issue {
	lines := tableLines(src)
	var probe struct {
		Views   any `toml:"views"`
		Layouts any `toml:"layouts"`
		Types   any `toml:"types"`
		Queries any `toml:"queries"`
	}
	meta, err := toml.Decode(src, &probe)
	if err != nil {
		var ce *Error
		wrapped := wrapDecode(path, err)
		if errors.As(wrapped, &ce) {
			return []Issue{{Line: ce.Line, Path: firstNonEmpty(ce.Path, path), Message: ce.Msg}}
		}
		return []Issue{{Path: path, Message: err.Error()}}
	}
	if meta.IsDefined("types") || meta.IsDefined("queries") {
		return []Issue{{Line: firstLine(src, "types", "queries"), Path: path, Message: legacyRemoved}}
	}
	if isSlice(probe.Views) {
		return []Issue{{Line: firstLine(src, "views"), Path: path, Message: "[[views]] (array of tables) was removed.\n\n" + legacyRemoved}}
	}
	var decoded File
	meta, err = toml.Decode(src, &decoded)
	if err != nil {
		var ce *Error
		wrapped := wrapDecode(path, err)
		if errors.As(wrapped, &ce) {
			return []Issue{{Line: ce.Line, Path: firstNonEmpty(ce.Path, path), Message: ce.Msg}}
		}
		return []Issue{{Path: path, Message: err.Error()}}
	}
	if decoded.Views == nil {
		decoded.Views = map[string]View{}
	}
	if decoded.Layouts == nil {
		decoded.Layouts = map[string]Layout{}
	}
	var issues []Issue
	for _, name := range sortedKeys(decoded.Views) {
		line := lines["views."+name]
		hasSource := meta.IsDefined("views", name, "source")
		if hasSource {
			issues = append(issues, Issue{Line: line, Path: path, Message: sourceKeyMsg(name)})
		}
		if meta.IsDefined("views", name, "layout") || decoded.Views[name].NestedLayout != nil {
			issues = append(issues, Issue{Line: line, Path: path, Message: nestedLayoutRemoved})
			continue
		}
		nv, nerr := decoded.Views[name].normalize()
		if nerr != nil {
			if hasSource && isTypeQueryChoiceError(nerr) {
				continue
			}
			issues = append(issues, Issue{Line: line, Path: path, Message: fmt.Sprintf("views.%s: %s", name, nerr.Error())})
			continue
		}
		decoded.Views[name] = nv
	}
	for _, name := range sortedKeys(decoded.Layouts) {
		line := lines["layouts."+name]
		nl, nerr := decoded.Layouts[name].normalize()
		if nerr != nil {
			issues = append(issues, Issue{Line: line, Path: path, Message: fmt.Sprintf("layouts.%s: %s", name, nerr.Error())})
			continue
		}
		if verr := validateLayoutTree(name, nl, decoded.Views); verr != nil {
			issues = append(issues, Issue{Line: line, Path: path, Message: verr.Error()})
			continue
		}
		decoded.Layouts[name] = nl
	}
	return issues
}

func sortedKeys[T any](m map[string]T) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func firstLine(src string, keys ...string) int {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		for _, k := range keys {
			if strings.HasPrefix(trim, "["+k) || strings.HasPrefix(trim, "[["+k) {
				return i + 1
			}
		}
	}
	return 1
}
