package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/cli"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/editor"
	"github.com/aidanlsb/perch/internal/proc"
	"github.com/aidanlsb/perch/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

const Version = "0.5.1"

const usage = `perch %s — terminal UI for Raven

Usage:
  perch [flags]                         Launch the TUI
  perch <target> [args] [flags]         Launch the TUI at a target
  perch open <target> [args] [flags]    Same as perch <target>
  perch views list|show|add|rm          Manage named views
  perch layouts list|show|add|rm        Manage named layouts
  perch config path|check               Print the config path or validate it
  perch skill list|install|remove|doctor
                                        Manage the shipped agent skill
                                        (perch skills is an alias)

  <target> is home, a named view, a named layout, a saved query, or a
  schema type, in that order. Views win if a layout shares the name.
  Built-in subcommands win over a target of the same name. Saved queries
  take trailing key=value or positional args; missing args open a prompt.

  perch talks to rvn api over HTTP. If no server is running it starts one
  as a child process and stops it on exit. Vault mutations go through
  POST /v1/invoke; Perch does not write vault files itself.

Every command accepts --json. Failures use Raven's envelope
{ok, data, error:{code,message}} and a nonzero exit.

Flags:
  --config path        Perch config file (default ~/.config/perch/config.toml)
  --json               Machine-readable envelope on stdout
  --url url            rvn api URL (loopback IP only; default http://127.0.0.1:7438)
  --token token        Bearer token
  --vault-path path    Vault path passed to rvn api when starting it
  --rvn path           Path to the rvn binary
  --no-start           Never start rvn api; only connect
  --poll-interval dur  How often to poll GET /v1/revision (default 1s)
  --version            Print version and exit
  --force              views rm: drop referencing layout panels
  --confirm            skill install/remove: apply the preview
  --scope user|project skill install root (default user)
  --dest path          Override the skill install root
`

// Main is the perch CLI entrypoint.
func Main(args []string, stdout, stderr io.Writer) error {
	cmd, _ := cli.PeekCommand(args)
	if wantsHelp(args) && (cmd == "" || onlyFlags(args)) {
		fmt.Fprintf(stderr, usage, Version)
		return nil
	}
	if wantsVersion(args) && (cmd == "" || cmd == "open" && !hasOpenName(args)) {
		fmt.Fprintln(stdout, "perch "+Version)
		return nil
	}
	switch cmd {
	case "views", "layouts", "config", "skill", "skills":
		return cli.Run(args, stdout, stderr)
	case "open":
		return openTUI(args, stdout, stderr)
	default:
		return launchTUI(args, stdout, stderr)
	}
}

const openUsage = `perch open <target> [args]

Launch the TUI at home, a named view or layout, a saved query, or a
schema type. Views win if a layout shares the name. Saved queries accept
trailing key=value or positional args; missing args open a prompt.
Home sits under the opened screen so esc / backspace / [ land there.
`

func openTUI(args []string, stdout, stderr io.Writer) error {
	jsonOut := cli.JSONRequested(args)
	_, idx := cli.PeekCommand(args)
	rest := append([]string{}, args[:idx]...)
	rest = append(rest, args[idx+1:]...)
	flags, err := cli.ParseTUI(rest)
	if err != nil {
		return cli.Report(stdout, stderr, jsonOut, err)
	}
	if flags.Help {
		fmt.Fprint(stderr, openUsage)
		return nil
	}
	if flags.Version {
		fmt.Fprintln(stdout, "perch "+Version)
		return nil
	}
	if len(flags.Pos) == 0 {
		return cli.Report(stdout, stderr, jsonOut, cli.Error(cli.CodeMissingArgument, "open needs a target name"))
	}
	return startNamed(flags, flags.Pos[0], flags.Pos[1:], jsonOut, stdout, stderr)
}

func launchTUI(args []string, stdout, stderr io.Writer) error {
	jsonOut := cli.JSONRequested(args)
	flags, err := cli.ParseTUI(args)
	if err != nil {
		return cli.Report(stdout, stderr, jsonOut, err)
	}
	if flags.Help {
		fmt.Fprintf(stderr, usage, Version)
		return nil
	}
	if flags.Version {
		fmt.Fprintln(stdout, "perch "+Version)
		return nil
	}
	if len(flags.Pos) > 0 {
		return startNamed(flags, flags.Pos[0], flags.Pos[1:], jsonOut, stdout, stderr)
	}
	cfgFile := flags.Config
	if cfgFile == "" {
		cfgFile = config.DefaultPath()
	}
	return startTUI(flags, cfgFile, "", nil, jsonOut, stdout, stderr)
}

func startNamed(flags cli.TUIFlags, name string, extra []string, jsonOut bool, stdout, stderr io.Writer) error {
	cfgFile := flags.Config
	if cfgFile == "" {
		cfgFile = config.DefaultPath()
	}
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return cli.Report(stdout, stderr, jsonOut, err)
	}
	if _, err := cli.ResolveTarget(cfg, nil, name, extra); err != nil && !errors.Is(err, cli.ErrNeedSchema) {
		return cli.Report(stdout, stderr, jsonOut, err)
	}
	return startTUI(flags, cfgFile, name, extra, jsonOut, stdout, stderr)
}

func startTUI(flags cli.TUIFlags, cfgFile, open string, extra []string, jsonOut bool, stdout, stderr io.Writer) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return cli.Report(stdout, stderr, jsonOut, err)
	}

	ctx := context.Background()
	handle, err := proc.Connect(ctx, proc.Options{
		URL:       flags.URL,
		Token:     flags.Token,
		VaultPath: flags.Vault,
		Rvn:       flags.Rvn,
		NoStart:   flags.NoStart,
	})
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()

	client := api.NewClient(handle.URL, handle.Token)
	if _, revErr := client.Revision(ctx); revErr != nil {
		if errors.Is(revErr, api.ErrNoRevision) {
			return fmt.Errorf("%s", api.RevisionRequired)
		}
		return fmt.Errorf("%s: %w", api.RevisionRequired, revErr)
	}
	var openInputs map[string]string
	if open != "" {
		tgt, err := resolveLaunchTarget(ctx, client, cfg, open, extra)
		if err != nil {
			return cli.Report(stdout, stderr, jsonOut, err)
		}
		openInputs = tgt.Inputs
	}
	vaultPath := firstNonEmpty(handle.VaultPath, flags.Vault)
	ed := ""
	edMode := editor.ModeAuto
	if sess, sessErr := client.Session(ctx); sessErr == nil && sess != nil {
		if strings.TrimSpace(sess.VaultPath) != "" {
			vaultPath = sess.VaultPath
		}
		ed = sess.Editor
		edMode = sess.EditorMode
	}
	ed = editor.Resolve(ed)
	model := ui.NewModel(ui.Options{
		Client:       client,
		Config:       cfg,
		ConfigPath:   cfgFile,
		VaultPath:    vaultPath,
		Editor:       ed,
		EditorMode:   edMode,
		Open:         open,
		OpenInputs:   openInputs,
		PollInterval: flags.PollInterval,
	})

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(stdout))
	_, err = p.Run()
	return err
}

func resolveLaunchTarget(ctx context.Context, client *api.Client, cfg *config.File, name string, extra []string) (cli.Target, error) {
	tgt, err := cli.ResolveTarget(cfg, nil, name, extra)
	if err == nil || !errors.Is(err, cli.ErrNeedSchema) {
		return tgt, err
	}
	schema, err := client.Schema(ctx)
	if err != nil {
		return cli.Target{}, err
	}
	return cli.ResolveTarget(cfg, schema, name, extra)
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func wantsVersion(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--version" {
			return true
		}
	}
	return false
}

func onlyFlags(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return true
		}
		if !strings.HasPrefix(a, "-") {
			return false
		}
	}
	return true
}

func hasOpenName(args []string) bool {
	_, idx := cli.PeekCommand(args)
	if idx < 0 {
		return false
	}
	for _, a := range args[idx+1:] {
		if !strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}
