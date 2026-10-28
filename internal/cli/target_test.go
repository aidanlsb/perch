package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
)

func TestResolveTargetOrder(t *testing.T) {
	cfg := &config.File{
		Views: map[string]config.View{
			"projects": {Type: "item"},
			"home":     {Type: "item"},
		},
		Layouts: map[string]config.Layout{
			"ws":       {Split: "row"},
			"projects": {Split: "col"},
		},
	}
	schema := &api.Schema{
		Queries: map[string]api.SavedQuery{
			"todos":    {Name: "todos", Query: "type:task"},
			"projects": {Name: "projects", Query: "type:item"},
			"need":     {Name: "need", Query: "type:task refs([[{{args.x}}]])", Args: []string{"x"}},
		},
		Types: map[string]api.TypeSchema{
			"task":    {Name: "task"},
			"section": {Name: "section", Builtin: true},
			"todos":   {Name: "todos"},
		},
	}

	cases := []struct {
		name  string
		extra []string
		kind  string
		key   string
		input string
	}{
		{name: "home", kind: TargetHome, key: "home"},
		{name: "projects", kind: TargetView, key: "projects"},
		{name: "ws", kind: TargetLayout, key: "ws"},
		{name: "todos", kind: TargetQuery, key: "todos"},
		{name: "task", kind: TargetType, key: "task"},
		{name: "need", extra: []string{"x=p"}, kind: TargetQuery, key: "need", input: "p"},
		{name: "need", extra: []string{"p"}, kind: TargetQuery, key: "need", input: "p"},
	}
	for _, tc := range cases {
		got, err := ResolveTarget(cfg, schema, tc.name, tc.extra)
		if err != nil {
			t.Fatalf("%s extra=%v: %v", tc.name, tc.extra, err)
		}
		if got.Kind != tc.kind || got.Name != tc.key {
			t.Fatalf("%s: got %+v want kind=%s name=%s", tc.name, got, tc.kind, tc.key)
		}
		if tc.input != "" && got.Inputs["x"] != tc.input {
			t.Fatalf("%s inputs %+v", tc.name, got.Inputs)
		}
	}

	if _, err := ResolveTarget(cfg, schema, "ghost", nil); err == nil {
		t.Fatal("ghost")
	} else if ce := asCLI(err); ce.code != CodeNotFound {
		t.Fatalf("ghost code %s", ce.code)
	}
	if _, err := ResolveTarget(cfg, schema, "section", nil); err == nil {
		t.Fatal("section type should not resolve")
	}
	if _, err := ResolveTarget(cfg, nil, "todos", nil); !errors.Is(err, ErrNeedSchema) {
		t.Fatalf("need schema, got %v", err)
	}
}

func TestResolveTargetRejectsExtraOnNonQuery(t *testing.T) {
	cfg := &config.File{Views: map[string]config.View{"projects": {Type: "item"}}}
	if _, err := ResolveTarget(cfg, nil, "home", []string{"x"}); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("home extra: %v", err)
	}
	if _, err := ResolveTarget(cfg, nil, "projects", []string{"x"}); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("view extra: %v", err)
	}
}

func TestBindQueryArgs(t *testing.T) {
	got, err := BindQueryArgs([]string{"status", "owner"}, []string{"open", "owner=ada"})
	if err != nil {
		t.Fatal(err)
	}
	if got["status"] != "open" || got["owner"] != "ada" {
		t.Fatalf("%v", got)
	}
	if _, err := BindQueryArgs(nil, []string{"x"}); err == nil {
		t.Fatal("no args")
	}
	if _, err := BindQueryArgs([]string{"x"}, []string{"y=1"}); err == nil {
		t.Fatal("unknown key")
	}
	partial, err := BindQueryArgs([]string{"a", "b"}, []string{"a=1"})
	if err != nil || partial["a"] != "1" || partial["b"] != "" {
		t.Fatalf("%v %v", partial, err)
	}
}
