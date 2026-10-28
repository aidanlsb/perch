package cli

import (
	"errors"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
)

// Launch target kinds. Resolution order is home, view, layout, query, type.
const (
	TargetHome   = "home"
	TargetView   = "view"
	TargetLayout = "layout"
	TargetQuery  = "query"
	TargetType   = "type"
)

// ErrNeedSchema means the name is not home or a configured view/layout, so
// GET /v1/schema is required to try saved queries and types.
var ErrNeedSchema = errors.New("need schema")

// Target is a TUI launch destination from perch <target> / perch open <target>.
type Target struct {
	Kind   string
	Name   string
	Inputs map[string]string
}

// ResolveTarget maps a CLI name onto home, a named view or layout, a saved
// query, or a schema type. schema may be nil: home/view/layout still resolve;
// query/type return ErrNeedSchema. extra is key=value or positional args for
// a saved query; it is an error on any other kind.
func ResolveTarget(cfg *config.File, schema *api.Schema, name string, extra []string) (Target, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Target{}, fail(CodeMissingArgument, "open needs a target name")
	}
	if name == TargetHome {
		if err := rejectExtra(extra); err != nil {
			return Target{}, err
		}
		return Target{Kind: TargetHome, Name: name}, nil
	}
	if cfg != nil {
		if _, ok := cfg.Views[name]; ok {
			if err := rejectExtra(extra); err != nil {
				return Target{}, err
			}
			return Target{Kind: TargetView, Name: name}, nil
		}
		if _, ok := cfg.Layouts[name]; ok {
			if err := rejectExtra(extra); err != nil {
				return Target{}, err
			}
			return Target{Kind: TargetLayout, Name: name}, nil
		}
	}
	if schema == nil {
		return Target{}, ErrNeedSchema
	}
	if q, ok := schema.Queries[name]; ok {
		inputs, err := BindQueryArgs(q.Args, extra)
		if err != nil {
			return Target{}, err
		}
		return Target{Kind: TargetQuery, Name: name, Inputs: inputs}, nil
	}
	if _, ok := schema.Types[name]; ok && name != "section" {
		if err := rejectExtra(extra); err != nil {
			return Target{}, err
		}
		return Target{Kind: TargetType, Name: name}, nil
	}
	return Target{}, fail(CodeNotFound, "no view, layout, saved query, or type named "+name)
}

// BindQueryArgs maps trailing CLI tokens onto a saved query's arg names.
// key=value sets that arg; a bare token fills the next empty positional arg.
// Incomplete args are allowed (the TUI prompts). Unknown keys and leftover
// tokens are errors.
func BindQueryArgs(argNames, extra []string) (map[string]string, error) {
	if len(extra) == 0 {
		return nil, nil
	}
	if len(argNames) == 0 {
		return nil, fail(CodeInvalidArgs, "unexpected argument "+extra[0])
	}
	known := make(map[string]bool, len(argNames))
	for _, n := range argNames {
		known[n] = true
	}
	out := map[string]string{}
	pos := 0
	for _, raw := range extra {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if k, v, ok := strings.Cut(raw, "="); ok {
			k = strings.TrimSpace(k)
			if k == "" {
				return nil, fail(CodeInvalidArgs, "expected k=v, got "+raw)
			}
			if !known[k] {
				return nil, fail(CodeInvalidArgs, "unknown query argument "+k)
			}
			out[k] = v
			continue
		}
		for pos < len(argNames) && strings.TrimSpace(out[argNames[pos]]) != "" {
			pos++
		}
		if pos >= len(argNames) {
			return nil, fail(CodeInvalidArgs, "unexpected argument "+raw)
		}
		out[argNames[pos]] = raw
		pos++
	}
	return out, nil
}

func rejectExtra(extra []string) error {
	for _, a := range extra {
		if strings.TrimSpace(a) != "" {
			return fail(CodeInvalidArgs, "unexpected argument "+a)
		}
	}
	return nil
}
