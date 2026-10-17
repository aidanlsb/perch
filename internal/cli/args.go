package cli

import (
	"strings"
)

type argSpec struct {
	bools   []string
	vals    []string
	repeats []string
}

type parsedArgs struct {
	Bool map[string]bool
	Val  map[string]string
	All  map[string][]string
	Pos  []string
}

func parseArgs(args []string, spec argSpec) (parsedArgs, error) {
	out := parsedArgs{
		Bool: map[string]bool{},
		Val:  map[string]string{},
		All:  map[string][]string{},
	}
	bools := setOf(spec.bools)
	vals := setOf(spec.vals)
	reps := setOf(spec.repeats)

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out.Pos = append(out.Pos, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			out.Pos = append(out.Pos, a)
			continue
		}
		name, inline, hasInline := splitFlag(a)
		if name == "h" {
			name = "help"
		}
		if bools[name] {
			if hasInline && inline != "true" && inline != "false" {
				return out, fail(CodeInvalidArgs, "flag --"+name+" does not take a value")
			}
			out.Bool[name] = !hasInline || inline != "false"
			continue
		}
		if vals[name] || reps[name] {
			val := inline
			if !hasInline {
				if i+1 >= len(args) {
					return out, fail(CodeMissingArgument, "flag --"+name+" needs a value")
				}
				i++
				val = args[i]
			}
			if vals[name] {
				out.Val[name] = val
			}
			if reps[name] {
				out.All[name] = append(out.All[name], val)
			}
			continue
		}
		return out, fail(CodeInvalidArgs, "unknown flag --"+name)
	}
	return out, nil
}

func splitFlag(a string) (name, val string, hasVal bool) {
	a = strings.TrimLeft(a, "-")
	if i := strings.IndexByte(a, '='); i >= 0 {
		return a[:i], a[i+1:], true
	}
	return a, "", false
}

func setOf(in []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range in {
		out[s] = true
	}
	return out
}

func peekCommand(args []string) (string, int) {
	known := map[string]bool{
		"views": true, "layouts": true, "config": true, "open": true,
	}
	skipVal := false
	for i, a := range args {
		if skipVal {
			skipVal = false
			continue
		}
		if a == "--" {
			return "", -1
		}
		if strings.HasPrefix(a, "-") {
			name, _, hasInline := splitFlag(a)
			if name == "h" {
				name = "help"
			}
			if !hasInline && !isGlobalBool(name) {
				skipVal = true
			}
			continue
		}
		if known[a] {
			return a, i
		}
		return a, i
	}
	return "", -1
}

func isGlobalBool(name string) bool {
	switch name {
	case "json", "force", "help", "h", "version", "no-start":
		return true
	default:
		return false
	}
}

func configPath(p parsedArgs) string {
	if v := strings.TrimSpace(p.Val["config"]); v != "" {
		return v
	}
	return ""
}

func requireName(pos []string, what string) (string, error) {
	if len(pos) < 1 || strings.TrimSpace(pos[0]) == "" {
		return "", fail(CodeMissingArgument, what+" name is required")
	}
	if len(pos) > 1 {
		return "", fail(CodeInvalidArgs, "unexpected argument "+pos[1])
	}
	return strings.TrimSpace(pos[0]), nil
}
