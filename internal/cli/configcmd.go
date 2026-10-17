package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aidanlsb/perch/internal/config"
)

const configUsage = `perch config path|check

  path    Print the resolved config file path
  check   Validate the file. Errors include line numbers.

check does not talk to a vault. It reuses the same structural validator
as load (kinds, group_by, layout trees, unknown view references).
`

func runConfig(args []string, stdout, stderr io.Writer) error {
	p, err := parseArgs(args, argSpec{
		bools: []string{"json", "help", "h"},
		vals:  []string{"config"},
	})
	jsonOut := p.Bool["json"]
	if err != nil {
		return writeErr(stdout, stderr, jsonOut, err)
	}
	if p.Bool["help"] || len(p.Pos) == 0 {
		if jsonOut && len(p.Pos) == 0 && !p.Bool["help"] {
			return writeErr(stdout, stderr, true, fail(CodeInvalidArgs, "usage: perch config path|check"))
		}
		fmt.Fprint(stderr, configUsage)
		return nil
	}
	path := resolveConfig(p)
	sub, rest := p.Pos[0], p.Pos[1:]
	if len(rest) > 0 {
		return writeErr(stdout, stderr, jsonOut, fail(CodeInvalidArgs, "unexpected argument "+rest[0]))
	}
	switch sub {
	case "path":
		_, err := os.Stat(path)
		exists := err == nil
		human := path
		if !exists {
			human = path + " (missing)"
		}
		return writeOK(stdout, jsonOut, map[string]any{"path": path, "exists": exists}, human)
	case "check":
		return configCheck(path, stdout, stderr, jsonOut)
	default:
		return writeErr(stdout, stderr, jsonOut, fail(CodeCommandNotFound, "unknown config command "+sub))
	}
}

func configCheck(path string, stdout, stderr io.Writer, jsonOut bool) error {
	_, statErr := os.Stat(path)
	exists := statErr == nil
	issues := config.Check(path)
	errs := make([]map[string]any, 0, len(issues))
	var lines []string
	for _, is := range issues {
		item := map[string]any{"message": is.Message}
		if is.Line > 0 {
			item["line"] = is.Line
		}
		if is.Path != "" {
			item["path"] = is.Path
		}
		errs = append(errs, item)
		loc := path
		if is.Line > 0 {
			loc = fmt.Sprintf("%s:%d", path, is.Line)
		}
		lines = append(lines, loc+": "+strings.TrimSpace(is.Message))
	}
	data := map[string]any{
		"path":   path,
		"exists": exists,
		"valid":  len(issues) == 0,
		"errors": errs,
	}
	if len(issues) == 0 {
		return writeOK(stdout, jsonOut, data, "ok")
	}
	msg := fmt.Sprintf("%d error(s)", len(issues))
	if !jsonOut {
		fmt.Fprintln(stderr, strings.Join(lines, "\n"))
		return ErrReported
	}
	return writeErr(stdout, stderr, true, failDetails(CodeValidationFailed, msg, data))
}

func resolveConfig(p parsedArgs) string {
	if path := configPath(p); path != "" {
		return path
	}
	if v := strings.TrimSpace(os.Getenv("PERCH_CONFIG")); v != "" {
		return v
	}
	return config.DefaultPath()
}
