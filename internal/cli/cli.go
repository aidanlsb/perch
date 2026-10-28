package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/config"
)

// PeekCommand returns the first non-flag argv that looks like a subcommand.
func PeekCommand(args []string) (string, int) {
	return peekCommand(args)
}

// IsCommand reports whether name is a perch subcommand.
func IsCommand(name string) bool {
	switch name {
	case "views", "layouts", "config", "open", "skill", "skills":
		return true
	default:
		return false
	}
}

// Run executes views, layouts, config, or skill. open is handled by the TUI launcher.
func Run(args []string, stdout, stderr io.Writer) error {
	cmd, idx := peekCommand(args)
	if idx < 0 || !IsCommand(cmd) || cmd == "open" {
		return fail(CodeCommandNotFound, "unknown command")
	}
	rest := append([]string{}, args[:idx]...)
	rest = append(rest, args[idx+1:]...)
	switch cmd {
	case "views":
		return runViews(rest, stdout, stderr)
	case "layouts":
		return runLayouts(rest, stdout, stderr)
	case "config":
		return runConfig(rest, stdout, stderr)
	case "skill", "skills":
		return runSkill(rest, stdout, stderr)
	default:
		return fail(CodeCommandNotFound, "unknown command "+cmd)
	}
}

// ResolveOpen looks up a view or layout name in config. Views win if both
// exist. Saved queries and types need schema; use ResolveTarget for those.
func ResolveOpen(path, name string) (kind string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fail(CodeMissingArgument, "open needs a target name")
	}
	f, err := config.Load(path)
	if err != nil {
		return "", err
	}
	t, err := ResolveTarget(f, nil, name, nil)
	if errors.Is(err, ErrNeedSchema) {
		return "", fail(CodeNotFound, "no view, layout, saved query, or type named "+name)
	}
	if err != nil {
		return "", err
	}
	return t.Kind, nil
}

// JSONRequested reports whether --json appears anywhere in args.
func JSONRequested(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		name, val, has := splitFlag(a)
		if name == "json" && (!has || val != "false") {
			return true
		}
	}
	return false
}

// Report writes err as a JSON envelope or a stderr line.
func Report(stdout, stderr io.Writer, jsonOut bool, err error) error {
	return writeErr(stdout, stderr, jsonOut, err)
}

// Error is a coded CLI failure.
func Error(code, message string) error {
	return fail(code, message)
}

// TUIFlags are the launch flags shared by bare perch and perch open.
type TUIFlags struct {
	URL, Token, Vault, Rvn, Config string
	NoStart, Help, Version, JSON   bool
	PollInterval                   time.Duration
	Pos                            []string
}

// ParseTUI extracts launch flags from args (flags may appear anywhere).
func ParseTUI(args []string) (TUIFlags, error) {
	p, err := parseArgs(args, argSpec{
		bools: []string{"json", "no-start", "version", "help", "h", "force"},
		vals:  []string{"url", "token", "vault-path", "rvn", "config", "poll-interval"},
	})
	out := TUIFlags{
		URL:     firstFlag(p.Val["url"], envOr("PERCH_URL", envOr("RAVEN_API_URL", ""))),
		Token:   firstFlag(p.Val["token"], envOr("PERCH_TOKEN", os.Getenv("RAVEN_API_TOKEN"))),
		Vault:   firstFlag(p.Val["vault-path"], os.Getenv("PERCH_VAULT")),
		Rvn:     firstFlag(p.Val["rvn"], envOr("PERCH_RVN", "rvn")),
		Config:  firstFlag(p.Val["config"], os.Getenv("PERCH_CONFIG")),
		NoStart: p.Bool["no-start"],
		Help:    p.Bool["help"],
		Version: p.Bool["version"],
		JSON:    p.Bool["json"],
		Pos:     p.Pos,
	}
	if err != nil {
		return out, err
	}
	poll, err := parsePollInterval(p.Val["poll-interval"])
	if err != nil {
		return out, err
	}
	out.PollInterval = poll
	return out, nil
}

func parsePollInterval(flag string) (time.Duration, error) {
	if v := strings.TrimSpace(flag); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fail(CodeInvalidArgs, "invalid --poll-interval "+v)
		}
		return d, nil
	}
	if v := strings.TrimSpace(os.Getenv("PERCH_POLL_INTERVAL")); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d, nil
		}
	}
	return 0, nil
}

func firstFlag(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
