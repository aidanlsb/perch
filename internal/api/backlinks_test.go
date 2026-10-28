package api

import (
	"encoding/json"
	"testing"
)

func TestBacklinkGroupDecodesSnippetsAndLines(t *testing.T) {
	raw := []byte(`{
		"source": "people/odin",
		"title": "Odin",
		"line": 12,
		"snippet": "see [[projects/a]]",
		"lines": ["12: see [[projects/a]]", "also this"]
	}`)
	var g BacklinkGroup
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if g.Source != "people/odin" || g.DisplayTitle() != "Odin" {
		t.Fatalf("%+v", g)
	}
	if g.SourceLine() != 12 {
		t.Fatalf("line %d", g.SourceLine())
	}
	if g.SnippetText() != "see [[projects/a]]" {
		t.Fatalf("snippet %q", g.SnippetText())
	}

	var plain BacklinkGroup
	if err := json.Unmarshal([]byte(`{"source":"notes/x","lines":["hello raven"]}`), &plain); err != nil {
		t.Fatal(err)
	}
	if plain.Source != "notes/x" || plain.SnippetText() != "hello raven" {
		t.Fatalf("%+v", plain)
	}

	var hits BacklinkGroup
	if err := json.Unmarshal([]byte(`{"source":"a","hits":[{"line":4,"text":"link here"}]}`), &hits); err != nil {
		t.Fatal(err)
	}
	if hits.SourceLine() != 4 || hits.SnippetText() != "link here" {
		t.Fatalf("%+v", hits)
	}
}

func TestSessionDisplayName(t *testing.T) {
	s := Session{VaultName: "Aidans", VaultPath: "/vaults/other"}
	if s.DisplayName() != "Aidans" {
		t.Fatalf("%q", s.DisplayName())
	}
	s = Session{VaultPath: "/vaults/raven"}
	if s.DisplayName() != "raven" {
		t.Fatalf("%q", s.DisplayName())
	}
}
