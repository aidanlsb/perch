package proc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
)

const (
	defaultURL     = "http://127.0.0.1:7438"
	startupTimeout = 12 * time.Second
	healthTries    = 20
	healthWait     = 100 * time.Millisecond
	shutdownWait   = 3 * time.Second
)

// Options control how Perch finds or starts rvn api.
type Options struct {
	URL       string
	Token     string
	VaultPath string
	Rvn       string
	NoStart   bool
	ExtraEnv  []string
	LookPath  func(string) (string, error)
	Command   func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// Handle is a live API connection. Close shuts down a child Perch started.
type Handle struct {
	URL       string
	Token     string
	VaultPath string
	Started   bool
	cmd       *exec.Cmd
	cancel    context.CancelFunc
}

// Connect attaches to an existing rvn api or starts one as a child process.
func Connect(ctx context.Context, opts Options) (*Handle, error) {
	url := strings.TrimRight(strings.TrimSpace(opts.URL), "/")
	if url == "" {
		url = defaultURL
	}
	token := strings.TrimSpace(opts.Token)

	if token != "" {
		if err := ping(ctx, url, token); err == nil {
			return &Handle{URL: url, Token: token, VaultPath: opts.VaultPath}, nil
		} else if opts.NoStart {
			return nil, fmt.Errorf("connect to %s: %w", url, err)
		}
	} else if opts.NoStart {
		return nil, fmt.Errorf("token is required when --no-start is set")
	}

	if opts.NoStart {
		return nil, fmt.Errorf("rvn api is not reachable at %s", url)
	}

	return startChild(ctx, opts)
}

func startChild(ctx context.Context, opts Options) (*Handle, error) {
	rvn := strings.TrimSpace(opts.Rvn)
	if rvn == "" {
		rvn = "rvn"
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	bin, err := lookPath(rvn)
	if err != nil {
		return nil, fmt.Errorf("rvn not found in PATH (install Raven, or pass --rvn): %w", err)
	}

	args := []string{"api", "--json", "--port", "0"}
	if strings.TrimSpace(opts.VaultPath) != "" {
		args = append(args, "--vault-path", opts.VaultPath)
	}
	if strings.TrimSpace(opts.Token) != "" {
		args = append(args, "--token", opts.Token)
	}

	command := opts.Command
	if command == nil {
		command = exec.CommandContext
	}
	childCtx, cancel := context.WithCancel(context.Background())
	cmd := command(childCtx, bin, args...)
	cmd.Env = append(os.Environ(), opts.ExtraEnv...)
	cmd.Stderr = os.Stderr
	configureChild(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start rvn api: %w", err)
	}

	line, err := readStartupLine(stdout, startupTimeout)
	if err != nil {
		cancel()
		_ = killProcess(cmd)
		return nil, fmt.Errorf("rvn api startup: %w", err)
	}
	startup, err := ParseStartupJSON(line)
	if err != nil {
		cancel()
		_ = killProcess(cmd)
		return nil, err
	}

	h := &Handle{
		URL:       strings.TrimRight(startup.URL, "/"),
		Token:     startup.Token,
		VaultPath: startup.VaultPath,
		Started:   true,
		cmd:       cmd,
		cancel:    cancel,
	}
	if h.VaultPath == "" {
		h.VaultPath = opts.VaultPath
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, startupTimeout)
	defer waitCancel()
	if err := waitHealthy(waitCtx, h.URL, h.Token); err != nil {
		_ = h.Close()
		return nil, err
	}
	return h, nil
}

// Close signals a child rvn api if Perch started it.
func (h *Handle) Close() error {
	if h == nil || !h.Started || h.cmd == nil {
		return nil
	}
	err := killProcess(h.cmd)
	if h.cancel != nil {
		h.cancel()
	}
	return err
}

// ParseStartupJSON parses the single-line --json listen envelope from rvn api.
func ParseStartupJSON(line []byte) (api.Startup, error) {
	var env api.Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return api.Startup{}, fmt.Errorf("parse startup JSON: %w", err)
	}
	if !env.OK {
		if env.Error != nil {
			return api.Startup{}, env.Error
		}
		return api.Startup{}, fmt.Errorf("rvn api startup failed")
	}
	var s api.Startup
	if err := json.Unmarshal(env.Data, &s); err != nil {
		return api.Startup{}, fmt.Errorf("parse startup data: %w", err)
	}
	if s.URL == "" || s.Token == "" {
		return api.Startup{}, fmt.Errorf("startup JSON missing url or token")
	}
	return s, nil
}

func readStartupLine(r io.Reader, timeout time.Duration) ([]byte, error) {
	type result struct {
		line []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				ch <- result{err: err}
				return
			}
			ch <- result{err: io.EOF}
			return
		}
		ch <- result{line: scanner.Bytes()}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-ch:
		return res.line, res.err
	case <-timer.C:
		return nil, fmt.Errorf("timed out waiting for startup JSON")
	}
}

func ping(ctx context.Context, url, token string) error {
	c := api.NewClient(url, token)
	_, err := c.Health(ctx)
	return err
}

func waitHealthy(ctx context.Context, url, token string) error {
	var last error
	for i := 0; i < healthTries; i++ {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		last = ping(ctx, url, token)
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(healthWait):
		}
	}
	if last == nil {
		return fmt.Errorf("rvn api did not become healthy")
	}
	return last
}
