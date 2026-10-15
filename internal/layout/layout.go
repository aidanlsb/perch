// Package layout is Perch's presentation primitive: list, board, or date grouping.
// Any source (type, saved query, ad-hoc RQL) renders through a Spec. Config supplies
// defaults; the TUI cycles Specs at runtime. Grouping itself lives in package group.
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

// Spec is one way to show a set of objects.
type Spec struct {
	Kind        string   // list, board, date
	GroupBy     string   // enum/ref field (board) or date field (date)
	Granularity string   // day, week, month (date)
	Fields      []string // list columns (excluding id)
	Sort        string
}

// Normalize fills defaults and validates kind/granularity. Empty kind becomes list.
func (s Spec) Normalize() Spec {
	s.Kind = strings.ToLower(strings.TrimSpace(s.Kind))
	s.GroupBy = strings.TrimSpace(s.GroupBy)
	s.Granularity = strings.ToLower(strings.TrimSpace(s.Granularity))
	s.Sort = strings.TrimSpace(s.Sort)
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
	return true
}

func (s Spec) Label() string {
	s = s.Normalize()
	switch s.Kind {
	case KindBoard:
		if s.GroupBy != "" {
			return "board by " + s.GroupBy
		}
		return "board"
	case KindDate:
		g := s.GroupBy
		if g == "" {
			g = "date"
		}
		return "date/" + s.Granularity + " by " + g
	default:
		return "list"
	}
}

// Infer picks a zero-config Spec from a type schema. Heuristics are intentionally
// small and documented in README:
//
//  1. scalar enum field named "status" → board grouped by status
//  2. type named "date" with a date/datetime field (prefer "date") → date grouping by day
//  3. otherwise list
func Infer(ts api.TypeSchema) Spec {
	if f, ok := ts.Fields["status"]; ok && f.IsEnum() && !f.IsArray() {
		return Spec{Kind: KindBoard, GroupBy: "status", Sort: ts.NameField}.Normalize()
	}
	if strings.EqualFold(ts.Name, "date") {
		field := "date"
		if f, ok := ts.Fields[field]; !ok || !f.IsDate() || f.IsArray() {
			dates := ts.DateFields()
			if len(dates) == 0 {
				return Spec{Kind: KindList, Sort: ts.NameField}.Normalize()
			}
			field = dates[0]
		}
		return Spec{Kind: KindDate, GroupBy: field, Granularity: GranDay, Sort: field}.Normalize()
	}
	return Spec{Kind: KindList, Sort: ts.NameField}.Normalize()
}

// Options is the cycle list for a type: list, then each board group-by, then each
// date field at day granularity. Config/user Specs that are not in this list still
// cycle back through it via Next.
func Options(ts api.TypeSchema) []Spec {
	first := Spec{Kind: KindList, Sort: ts.NameField}.Normalize()
	out := []Spec{first}
	seen := map[string]struct{}{first.cycleKey(): {}}
	add := func(s Spec) {
		s = s.Normalize()
		k := s.cycleKey()
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	for _, f := range ts.EnumFields() {
		add(Spec{Kind: KindBoard, GroupBy: f, Sort: ts.NameField})
	}
	for _, f := range ts.RefFields() {
		add(Spec{Kind: KindBoard, GroupBy: f, Sort: ts.NameField})
	}
	for _, f := range ts.DateFields() {
		add(Spec{Kind: KindDate, GroupBy: f, Granularity: GranDay, Sort: f})
	}
	return out
}

func (s Spec) cycleKey() string {
	s = s.Normalize()
	switch s.Kind {
	case KindBoard:
		return KindBoard + ":" + s.GroupBy
	case KindDate:
		return KindDate + ":" + s.GroupBy
	default:
		return KindList
	}
}

// Next cycles to the next Option. Date granularity is not part of this cycle
// (use NextGranularity). Fields/sort from current are preserved onto the next Spec.
func Next(current Spec, ts api.TypeSchema) Spec {
	opts := Options(ts)
	current = current.Normalize()
	idx := -1
	for i, o := range opts {
		if o.cycleKey() == current.cycleKey() {
			idx = i
			break
		}
	}
	next := opts[0]
	if idx >= 0 {
		next = opts[(idx+1)%len(opts)]
	} else if len(opts) > 1 {
		next = opts[1]
	}
	next.Fields = current.Fields
	if current.Sort != "" && next.Sort == "" {
		next.Sort = current.Sort
	}
	return next
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

// NextGroupBy cycles board columns among enum then ref fields, or date fields
// for date layouts. No-op when the type has fewer than two candidates.
func NextGroupBy(current Spec, ts api.TypeSchema) Spec {
	current = current.Normalize()
	var fields []string
	switch current.Kind {
	case KindBoard:
		fields = append(ts.EnumFields(), ts.RefFields()...)
	case KindDate:
		fields = ts.DateFields()
	default:
		return current
	}
	if len(fields) == 0 {
		return current
	}
	idx := 0
	for i, f := range fields {
		if f == current.GroupBy {
			idx = i
			break
		}
	}
	current.GroupBy = fields[(idx+1)%len(fields)]
	return current
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
