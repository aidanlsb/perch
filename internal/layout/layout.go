// Package layout is Perch's presentation primitive: list, board, or date grouping.
// Any source (type, saved query, ad-hoc RQL) renders through a named Spec. Config
// supplies an ordered list; the TUI cycles that list at runtime. Grouping itself
// lives in package group.
package layout

import (
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/group"
)

const (
	KindList  = "list"
	KindBoard = "board"
	KindDate  = "date"
	GranDay   = "day"
	GranWeek  = "week"
	GranMonth = "month"
)

// Spec is one named way to show a set of objects.
type Spec struct {
	Name        string   // display name (status bar, picker)
	Kind        string   // list, board, date
	GroupBy     string   // enum/ref field (board) or date field (date)
	Granularity string   // day, week, month (date)
	Fields      []string // list columns (excluding id)
	Sort        string
	Filter      string // RQL predicate appended to the type source
}

// DefaultName is the synthesized name for a kind.
func DefaultName(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindBoard:
		return "Board"
	case KindDate:
		return "Date"
	default:
		return "List"
	}
}

// Normalize fills defaults and validates kind/granularity. Empty kind becomes list.
func (s Spec) Normalize() Spec {
	s.Name = strings.TrimSpace(s.Name)
	s.Kind = strings.ToLower(strings.TrimSpace(s.Kind))
	s.GroupBy = strings.TrimSpace(s.GroupBy)
	s.Granularity = strings.ToLower(strings.TrimSpace(s.Granularity))
	s.Sort = strings.TrimSpace(s.Sort)
	s.Filter = strings.TrimSpace(s.Filter)
	if s.Kind == "" {
		if s.GroupBy != "" && s.Granularity != "" {
			s.Kind = KindDate
		} else if s.GroupBy != "" {
			s.Kind = KindBoard
		} else {
			s.Kind = KindList
		}
	}
	switch s.Kind {
	case KindList, KindBoard, KindDate:
	default:
		s.Kind = KindList
	}
	if s.Name == "" {
		s.Name = DefaultName(s.Kind)
	}
	if s.Kind == KindDate && s.Granularity == "" {
		s.Granularity = GranDay
	}
	if s.Kind == KindDate {
		switch s.Granularity {
		case GranDay, GranWeek, GranMonth:
		default:
			s.Granularity = GranDay
		}
	}
	return s
}

func (s Spec) Equal(o Spec) bool {
	s, o = s.Normalize(), o.Normalize()
	if s.Kind != o.Kind || s.GroupBy != o.GroupBy || s.Granularity != o.Granularity {
		return false
	}
	return s.Name == o.Name && s.Filter == o.Filter
}

// DisplayName is the status-bar / picker label.
func (s Spec) DisplayName() string {
	s = s.Normalize()
	if s.Name != "" {
		return s.Name
	}
	return DefaultName(s.Kind)
}

func (s Spec) Label() string {
	s = s.Normalize()
	if s.Name != "" && s.Name != DefaultName(s.Kind) {
		return s.Name
	}
	switch s.Kind {
	case KindBoard:
		if s.GroupBy != "" {
			return "board by " + s.GroupBy
		}
		return "board"
	case KindDate:
		if s.GroupBy != "" {
			return "date/" + s.Granularity + " by " + s.GroupBy
		}
		return "date/" + s.Granularity
	default:
		return "list"
	}
}

// Synthesize builds the zero-config layout list for a type from schema
// structure only. Field names are not preferred.
//
//  1. List always (first, so it is the default)
//  2. Board if the type has a scalar enum field, grouped by the first such
//     field in schema definition order (named "Board")
//  3. Date if the type has a scalar date/datetime field, grouped by the first
//     such field in schema definition order (named "Date"; granularity day)
//
// Ref fields do not produce a synthesized board. This list is the entire
// L-cycle; Perch does not invent extra group-by options at runtime.
func Synthesize(ts api.TypeSchema) []Spec {
	list := Spec{Name: "List", Kind: KindList, Sort: ts.NameField}.Normalize()
	out := []Spec{list}

	if enums := ts.EnumFields(); len(enums) > 0 {
		out = append(out, Spec{
			Name:    "Board",
			Kind:    KindBoard,
			GroupBy: enums[0],
			Sort:    ts.NameField,
		}.Normalize())
	}

	if dates := ts.DateFields(); len(dates) > 0 {
		out = append(out, Spec{
			Name:        "Date",
			Kind:        KindDate,
			GroupBy:     dates[0],
			Granularity: GranDay,
			Sort:        dates[0],
		}.Normalize())
	}
	return out
}

// Cycle walks an ordered named-layout list. delta is +1 (L) or -1 (gL).
func Cycle(layouts []Spec, idx, delta int) (Spec, int) {
	if len(layouts) == 0 {
		s := Spec{Name: "List", Kind: KindList}.Normalize()
		return s, 0
	}
	n := len(layouts)
	idx = (idx%n + n) % n
	idx = (idx + delta%n + n) % n
	return layouts[idx].Normalize(), idx
}

// NextGranularity cycles day → week → month → day. No-op for non-date specs.
func NextGranularity(current Spec) Spec {
	current = current.Normalize()
	if current.Kind != KindDate {
		return current
	}
	switch current.Granularity {
	case GranDay:
		current.Granularity = GranWeek
	case GranWeek:
		current.Granularity = GranMonth
	default:
		current.Granularity = GranDay
	}
	return current
}

// AppendFilter joins an RQL predicate onto a type source or query body.
// Empty filter returns source unchanged.
func AppendFilter(source, filter string) string {
	source = strings.TrimSpace(source)
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return source
	}
	if source == "" {
		return filter
	}
	return source + " " + filter
}

// Result is objects after sort + optional fuzzy filter, plus board/date columns.
type Result struct {
	Objects []api.Object
	Board   []group.Column
	Date    []group.Column
}

// Apply sorts, filters, and groups objects according to spec.
func Apply(objects []api.Object, spec Spec, ts api.TypeSchema, filter string, fuzzy bool) Result {
	spec = spec.Normalize()
	sorted := group.SortObjects(objects, spec.Sort)
	var rows []api.Object
	if fuzzy {
		rows = FuzzyFilter(sorted, filter)
	} else {
		rows = group.Filter(sorted, filter)
	}
	out := Result{Objects: rows}
	switch spec.Kind {
	case KindBoard:
		if spec.GroupBy != "" {
			out.Board = group.Board(rows, spec.GroupBy, ts)
		}
	case KindDate:
		if spec.GroupBy != "" {
			out.Date = group.Date(rows, spec.GroupBy, spec.Granularity)
		}
	}
	return out
}

// FuzzyFilter keeps objects whose id, title, type, or field values fuzzy-match q.
// Results stay in input order (already sorted by the caller).
func FuzzyFilter(objects []api.Object, q string) []api.Object {
	q = strings.TrimSpace(q)
	if q == "" {
		return objects
	}
	var out []api.Object
	for _, o := range objects {
		if objectFuzzy(o, q) {
			out = append(out, o)
		}
	}
	return out
}

func objectFuzzy(o api.Object, q string) bool {
	if ScoreParts(q, o.ID, o.Title, o.Type) >= 0 {
		return true
	}
	for _, v := range o.Fields {
		if ScoreParts(q, api.ScalarString(v)) >= 0 {
			return true
		}
	}
	return false
}

// ScoreParts is BestScore against object identity strings. Kept here so tests
// can cover grouping + matching without importing ui.
func ScoreParts(q string, parts ...string) int {
	return best(q, parts...)
}

func best(q string, parts ...string) int {
	// local alias to avoid a circular import story; fuzzy is the implementation.
	return fuzzyBest(q, parts...)
}
