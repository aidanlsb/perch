package group

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
)

const emptyLabel = "—"

// Column is one board or date bucket.
type Column struct {
	Key   string
	Label string
	Items []api.Object
}

// Board groups objects by an enum or ref field. Enum values keep schema order,
// then an empty column. Ref columns are discovered from data, sorted, plus empty.
func Board(objects []api.Object, field string, schema api.TypeSchema) []Column {
	f, ok := schema.Fields[field]
	if !ok {
		return []Column{{Key: "", Label: field, Items: objects}}
	}
	if f.IsEnum() && !f.IsArray() {
		return enumBoard(objects, field, f.Values)
	}
	return refBoard(objects, field)
}

func enumBoard(objects []api.Object, field string, values []string) []Column {
	seen := map[string][]api.Object{}
	var extra []string
	var empty []api.Object
	known := map[string]struct{}{}
	for _, v := range values {
		known[v] = struct{}{}
	}
	for _, obj := range objects {
		vals := api.ScalarStrings(obj.Fields[field])
		if len(vals) == 0 {
			empty = append(empty, obj)
			continue
		}
		for _, v := range vals {
			if _, ok := known[v]; !ok {
				if _, exists := seen[v]; !exists {
					extra = append(extra, v)
				}
			}
			seen[v] = append(seen[v], obj)
		}
	}
	cols := make([]Column, 0, len(values)+len(extra)+1)
	for _, v := range values {
		cols = append(cols, Column{Key: v, Label: v, Items: seen[v]})
	}
	sort.Strings(extra)
	for _, v := range extra {
		cols = append(cols, Column{Key: v, Label: v, Items: seen[v]})
	}
	cols = append(cols, Column{Key: "", Label: emptyLabel, Items: empty})
	return cols
}

func refBoard(objects []api.Object, field string) []Column {
	seen := map[string][]api.Object{}
	var keys []string
	var empty []api.Object
	for _, obj := range objects {
		vals := api.ScalarStrings(obj.Fields[field])
		if len(vals) == 0 {
			empty = append(empty, obj)
			continue
		}
		for _, v := range vals {
			if _, ok := seen[v]; !ok {
				keys = append(keys, v)
			}
			seen[v] = append(seen[v], obj)
		}
	}
	sort.Strings(keys)
	cols := make([]Column, 0, len(keys)+1)
	for _, k := range keys {
		cols = append(cols, Column{Key: k, Label: k, Items: seen[k]})
	}
	cols = append(cols, Column{Key: "", Label: emptyLabel, Items: empty})
	return cols
}

// Date groups objects by a date/datetime field into day, week, or month buckets.
// Newest first. Empty/unparseable values go last.
func Date(objects []api.Object, field, granularity string) []Column {
	granularity = strings.ToLower(strings.TrimSpace(granularity))
	if granularity == "" {
		granularity = "day"
	}
	type bucket struct {
		key   string
		label string
		sort  string
		items []api.Object
	}
	byKey := map[string]*bucket{}
	var empty []api.Object
	for _, obj := range objects {
		raw := api.ScalarString(obj.Fields[field])
		t, ok := parseDate(raw)
		if !ok {
			empty = append(empty, obj)
			continue
		}
		key, label, sortKey := dateBucket(t, granularity)
		b, ok := byKey[key]
		if !ok {
			b = &bucket{key: key, label: label, sort: sortKey}
			byKey[key] = b
		}
		b.items = append(b.items, obj)
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return byKey[keys[i]].sort > byKey[keys[j]].sort
	})
	cols := make([]Column, 0, len(keys)+1)
	for _, k := range keys {
		b := byKey[k]
		cols = append(cols, Column{Key: b.key, Label: b.label, Items: b.items})
	}
	if len(empty) > 0 {
		cols = append(cols, Column{Key: "", Label: emptyLabel, Items: empty})
	}
	return cols
}

func dateBucket(t time.Time, gran string) (key, label, sortKey string) {
	switch gran {
	case "week":
		y, w := t.ISOWeek()
		key = fmt.Sprintf("%04d-W%02d", y, w)
		label = key
		sortKey = key
	case "month":
		key = t.Format("2006-01")
		label = t.Format("January 2006")
		sortKey = key
	default:
		key = t.Format("2006-01-02")
		label = t.Format("Mon 2 Jan 2006")
		sortKey = key
	}
	return key, label, sortKey
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// SortObjects sorts a copy of objects by a field (asc). Empty field sorts by title/id.
func SortObjects(objects []api.Object, field string) []api.Object {
	out := append([]api.Object(nil), objects...)
	sort.SliceStable(out, func(i, j int) bool {
		a := sortKey(out[i], field)
		b := sortKey(out[j], field)
		if a == b {
			return out[i].ID < out[j].ID
		}
		return a < b
	})
	return out
}

func sortKey(o api.Object, field string) string {
	if field == "" || field == "id" {
		return strings.ToLower(o.ID)
	}
	if s := api.ScalarString(o.Fields[field]); s != "" {
		return strings.ToLower(s)
	}
	return strings.ToLower(o.Title + "\x00" + o.ID)
}

// Filter keeps objects whose id, title, or field values contain q (case-insensitive).
func Filter(objects []api.Object, q string) []api.Object {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return objects
	}
	var out []api.Object
	for _, o := range objects {
		if objectMatches(o, q) {
			out = append(out, o)
		}
	}
	return out
}

func objectMatches(o api.Object, q string) bool {
	if strings.Contains(strings.ToLower(o.ID), q) {
		return true
	}
	if strings.Contains(strings.ToLower(o.Title), q) {
		return true
	}
	if strings.Contains(strings.ToLower(o.Type), q) {
		return true
	}
	for _, v := range o.Fields {
		if strings.Contains(strings.ToLower(api.ScalarString(v)), q) {
			return true
		}
	}
	return false
}
