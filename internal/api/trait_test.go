package api

import "testing"

func TestPlainWikilinks(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"[[people/freya]]", "people/freya"},
		{"ask [[people/freya|Freya]]", "ask Freya"},
		{"[[a]] and [[b|Bee]] @due(2026-07-01)", "a and Bee @due(2026-07-01)"},
		{"unclosed [[foo", "unclosed [[foo"},
		{"[[]]", ""},
	}
	for _, c := range cases {
		if got := PlainWikilinks(c.in); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestParentLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"2026-07-06#tasks", "2026-07-06"},
		{"issue/ambiguous-order-submissions-lose-their-idempotency-key#description", "ambiguous-order-submissions-lose-their-idempotency-key"},
		{"projects/raven", "raven"},
		{"solo", "solo"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ParentLabel(c.in); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestTraitSourceFallbacks(t *testing.T) {
	if s := TraitSource(Object{ObjectID: "notes/x#body"}); s != "notes/x#body" {
		t.Fatalf("object_id source %q", s)
	}
	if s := TraitSource(Object{FilePath: "notes/x.md"}); s != "notes/x.md" {
		t.Fatalf("path source %q", s)
	}
}
