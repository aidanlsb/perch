package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aidanlsb/perch/internal/config"
)

const layoutsUsage = `perch layouts list|show <name>|add <name>|rm <name>

  list                 List named layouts
  show <name>          Print one layout
  add <name>           Create a layout from flags or --from
  rm <name>            Delete a layout

add flags:
  --split row|col      Default row
  --panel view=name    Repeatable. Also view=name,bind=other
  --from snippet       TOML or JSON body, or a file path, or -

Nested splits belong in --from. Example:

  perch layouts add workspace --split row --panel view=projects --panel view=tasks,bind=projects
  perch layouts add workspace --from 'split = "row"\npanels = [ { view = "projects" } ]'
`

func runLayouts(args []string, stdout, stderr io.Writer) error {
	p, err := parseArgs(args, argSpec{
		bools:   []string{"json", "help", "h"},
		vals:    []string{"config", "split", "from"},
		repeats: []string{"panel"},
	})
	jsonOut := p.Bool["json"]
	if err != nil {
		return writeErr(stdout, stderr, jsonOut, err)
	}
	if p.Bool["help"] || len(p.Pos) == 0 {
		if jsonOut && len(p.Pos) == 0 && !p.Bool["help"] {
			return writeErr(stdout, stderr, true, fail(CodeInvalidArgs, "usage: perch layouts list|show|add|rm"))
		}
		fmt.Fprint(stderr, layoutsUsage)
		return nil
	}
	path := resolveConfig(p)
	sub, rest := p.Pos[0], p.Pos[1:]
	var runErr error
	var data any
	var human string
	switch sub {
	case "list":
		if len(rest) > 0 {
			runErr = fail(CodeInvalidArgs, "layouts list takes no arguments")
			break
		}
		data, human, runErr = layoutsList(path)
	case "show":
		var name string
		name, runErr = requireName(rest, "layout")
		if runErr == nil {
			data, human, runErr = layoutsShow(path, name)
		}
	case "add":
		var name string
		name, runErr = requireName(rest, "layout")
		if runErr == nil {
			data, human, runErr = layoutsAdd(path, name, p, os.Stdin)
		}
	case "rm", "remove", "delete":
		var name string
		name, runErr = requireName(rest, "layout")
		if runErr == nil {
			data, human, runErr = layoutsRM(path, name)
		}
	default:
		runErr = fail(CodeCommandNotFound, "unknown layouts command "+sub)
	}
	if runErr != nil {
		return writeErr(stdout, stderr, jsonOut, runErr)
	}
	return writeOK(stdout, jsonOut, data, human)
}

func layoutsList(path string) (any, string, error) {
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	items := make([]map[string]any, 0, len(f.Layouts))
	var lines []string
	for _, name := range f.LayoutNames() {
		l := f.Layouts[name]
		items = append(items, layoutData(name, l, f.LayoutLine(name)))
		var views []string
		for _, p := range l.Leaves() {
			views = append(views, p.View)
		}
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", name, l.Split, strings.Join(views, ",")))
	}
	return map[string]any{"layouts": items}, strings.Join(lines, "\n"), nil
}

func layoutsShow(path, name string) (any, string, error) {
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	l, ok := f.Layouts[name]
	if !ok {
		return nil, "", fail(CodeNotFound, "layout "+name+" not found")
	}
	block, err := config.EncodeLayout(name, l)
	if err != nil {
		return nil, "", err
	}
	return layoutData(name, l, f.LayoutLine(name)), strings.TrimRight(block, "\n"), nil
}

func layoutsAdd(path, name string, p parsedArgs, stdin io.Reader) (any, string, error) {
	var l config.Layout
	if from := strings.TrimSpace(p.Val["from"]); from != "" {
		parsed, err := parseLayoutSnippetWith(from, stdin)
		if err != nil {
			return nil, "", err
		}
		l = parsed
	}
	if split := strings.TrimSpace(p.Val["split"]); split != "" {
		l.Split = split
	}
	for _, raw := range p.All["panel"] {
		panel, err := parsePanelFlag(raw)
		if err != nil {
			return nil, "", err
		}
		l.Panels = append(l.Panels, panel)
	}
	if len(l.Panels) == 0 {
		return nil, "", fail(CodeMissingArgument, "layouts add needs --panel or --from with panels")
	}
	if err := config.AddLayout(path, name, l); err != nil {
		return nil, "", err
	}
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	saved := f.Layouts[name]
	return layoutData(name, saved, f.LayoutLine(name)), "added layout " + name, nil
}

func layoutsRM(path, name string) (any, string, error) {
	if err := config.RemoveLayout(path, name); err != nil {
		return nil, "", err
	}
	return map[string]any{"name": name, "removed": true}, "removed layout " + name, nil
}

func layoutData(name string, l config.Layout, line int) map[string]any {
	out := map[string]any{
		"name":   name,
		"split":  l.Split,
		"panels": l.Panels,
	}
	if line > 0 {
		out["line"] = line
	}
	return out
}

func parseLayoutSnippetWith(raw string, stdin io.Reader) (config.Layout, error) {
	if raw == "-" {
		if stdin == nil {
			return config.Layout{}, fail(CodeInvalidInput, "--from - needs stdin")
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return config.Layout{}, fail(CodeFileReadError, err.Error())
		}
		return parseLayoutSnippet(string(b))
	}
	return parseLayoutSnippet(raw)
}
