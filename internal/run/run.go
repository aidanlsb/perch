package run

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/editor"
	"github.com/aidanlsb/perch/internal/proc"
	"github.com/aidanlsb/perch/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

const Version = "0.4.0"

// Main is the perch CLI entrypoint.
func Main(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("perch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, `perch %s — read-only terminal UI for Raven

Usage: perch [flags]

  perch talks to rvn api over HTTP. If no server is running it starts one
  as a child process and stops it on exit. It never writes to the vault.

Flags:
`, Version)
		fs.PrintDefaults()
	}

	url := fs.String("url", envOr("PERCH_URL", envOr("RAVEN_API_URL", "")), "rvn api URL (default http://127.0.0.1:7438)")
	token := fs.String("token", envOr("PERCH_TOKEN", os.Getenv("RAVEN_API_TOKEN")), "bearer token")
	vault := fs.String("vault-path", os.Getenv("PERCH_VAULT"), "vault path passed to rvn api when starting it")
	rvn := fs.String("rvn", envOr("PERCH_RVN", "rvn"), "path to the rvn binary")
	cfgPath := fs.String("config", os.Getenv("PERCH_CONFIG"), "perch config file (default ~/.config/perch/config.toml)")
	noStart := fs.Bool("no-start", false, "never start rvn api; only connect")
	showVer := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVer {
		fmt.Fprintln(stdout, "perch "+Version)
		return nil
	}

	cfgFile := strings.TrimSpace(*cfgPath)
	if cfgFile == "" {
		cfgFile = config.DefaultPath()
	}
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	ctx := context.Background()
	handle, err := proc.Connect(ctx, proc.Options{
		URL:       *url,
		Token:     *token,
		VaultPath: *vault,
		Rvn:       *rvn,
		NoStart:   *noStart,
	})
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()

	client := api.NewClient(handle.URL, handle.Token)
	vaultPath := firstNonEmpty(handle.VaultPath, *vault)
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
		Client:     client,
		Config:     cfg,
		VaultPath:  vaultPath,
		Editor:     ed,
		EditorMode: edMode,
	})

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(stdout))
	_, err = p.Run()
	return err
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
