package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/aidanlsb/perch/internal/config"
)

const viewsUsage = `perch views list|show <name>|add <name>|rm <name>

  list                 List named views
  show <name>          Print one view
  add <name>           Create a view
  rm <name>            Delete a view

add flags:
  --source string      Type name, saved query name, or RQL (required)
  --inputs k=v         Repeatable. Also accepts comma-separated pairs
  --layout spec        kind=board,group_by=state,fields=state,when
                       or a kind word (list, board, date)

rm of a view referenced by a layout fails unless --force (drops those panels).
`

func runViews(args []string, stdout, stderr io.Writer) error {
	p, err := parseArgs(args, argSpec{
		bools:   []string{"json", "force", "help", "h"},
		vals:    []string{"config", "source", "layout"},
		repeats: []string{"inputs"},
	})
	jsonOut := p.Bool["json"]
	if err != nil {
		return writeErr(stdout, stderr, jsonOut, err)
	}
	if p.Bool["help"] || len(p.Pos) == 0 {
		if jsonOut && len(p.Pos) == 0 && !p.Bool["help"] {
			return writeErr(stdout, stderr, true, fail(CodeInvalidArgs, "usage: perch views list|show|add|rm"))
		}
		fmt.Fprint(stderr, viewsUsage)
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
			runErr = fail(CodeInvalidArgs, "views list takes no arguments")
			break
		}
		data, human, runErr = viewsList(path)
	case "show":
		var name string
		name, runErr = requireName(rest, "view")
		if runErr == nil {
			data, human, runErr = viewsShow(path, name)
		}
	case "add":
		var name string
		name, runErr = requireName(rest, "view")
		if runErr == nil {
			data, human, runErr = viewsAdd(path, name, p)
		}
	case "rm", "remove", "delete":
		var name string
		name, runErr = requireName(rest, "view")
		if runErr == nil {
			data, human, runErr = viewsRM(path, name, p.Bool["force"])
		}
	default:
		runErr = fail(CodeCommandNotFound, "unknown views command "+sub)
	}
	if runErr != nil {
		return writeErr(stdout, stderr, jsonOut, runErr)
	}
	return writeOK(stdout, jsonOut, data, human)
}

func viewsList(path string) (any, string, error) {
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	items := make([]map[string]any, 0, len(f.Views))
	var lines []string
	for _, name := range f.ViewNames() {
		item := viewData(name, f.Views[name], f.ViewLine(name))
		items = append(items, item)
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", name, f.Views[name].Source, f.Views[name].Layout.Kind))
	}
	return map[string]any{"views": items}, strings.Join(lines, "\n"), nil
}

func viewsShow(path, name string) (any, string, error) {
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	v, ok := f.Views[name]
	if !ok {
		return nil, "", fail(CodeNotFound, "view "+name+" not found")
	}
	data := viewData(name, v, f.ViewLine(name))
	return data, formatView(name, v), nil
}

func viewsAdd(path, name string, p parsedArgs) (any, string, error) {
	source := strings.TrimSpace(p.Val["source"])
	if source == "" {
		return nil, "", fail(CodeMissingArgument, "--source is required")
	}
	inputs, err := parseKVList(p.All["inputs"])
	if err != nil {
		return nil, "", err
	}
	present, ref, err := parsePresent(p.Val["layout"])
	if err != nil {
		return nil, "", err
	}
	if ref != "" {
		return nil, "", fail(CodeInvalidInput, "view --layout is an inline spec (kind=board,group_by=...), not a layout name")
	}
	v := config.View{Source: source, Inputs: inputs, Layout: present}
	if err := config.AddView(path, name, v); err != nil {
		return nil, "", err
	}
	f, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	saved := f.Views[name]
	return viewData(name, saved, f.ViewLine(name)), "added view " + name, nil
}

func viewsRM(path, name string, force bool) (any, string, error) {
	if err := config.RemoveView(path, name, force); err != nil {
		return nil, "", err
	}
	return map[string]any{"name": name, "removed": true}, "removed view " + name, nil
}

func viewData(name string, v config.View, line int) map[string]any {
	layout := map[string]any{"kind": v.Layout.Kind}
	if v.Layout.GroupBy != "" {
		layout["group_by"] = v.Layout.GroupBy
	}
	if len(v.Layout.Fields) > 0 {
		layout["fields"] = v.Layout.Fields
	}
	if v.Layout.Sort != "" {
		layout["sort"] = v.Layout.Sort
	}
	if v.Layout.Filter != "" {
		layout["filter"] = v.Layout.Filter
	}
	if v.Layout.Granularity != "" {
		layout["granularity"] = v.Layout.Granularity
	}
	out := map[string]any{
		"name":   name,
		"source": v.Source,
		"layout": layout,
	}
	if len(v.Inputs) > 0 {
		out["inputs"] = v.Inputs
	}
	if line > 0 {
		out["line"] = line
	}
	return out
}

func formatView(name string, v config.View) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\nsource: %s\nkind: %s", name, v.Source, v.Layout.Kind)
	if v.Layout.GroupBy != "" {
		fmt.Fprintf(&b, "\ngroup_by: %s", v.Layout.GroupBy)
	}
	if len(v.Layout.Fields) > 0 {
		fmt.Fprintf(&b, "\nfields: %s", strings.Join(v.Layout.Fields, ", "))
	}
	if v.Layout.Sort != "" {
		fmt.Fprintf(&b, "\nsort: %s", v.Layout.Sort)
	}
	if v.Layout.Filter != "" {
		fmt.Fprintf(&b, "\nfilter: %s", v.Layout.Filter)
	}
	if v.Layout.Granularity != "" {
		fmt.Fprintf(&b, "\ngranularity: %s", v.Layout.Granularity)
	}
	if len(v.Inputs) > 0 {
		fmt.Fprintf(&b, "\ninputs: %v", v.Inputs)
	}
	return b.String()
}
