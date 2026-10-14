package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout = 15 * time.Second
	maxBody        = 8 << 20
	userAgent      = "perch/0.1.0"
)

// Client talks to rvn api over HTTP. It never imports Raven internals.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	UserAgent  string
}

// NewClient returns a client for a loopback Raven API.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token),
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
		UserAgent: userAgent,
	}
}

// Health GET /v1/health.
func (c *Client) Health(ctx context.Context) (*Health, error) {
	var h Health
	if err := c.getJSON(ctx, "/v1/health", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// Schema GET /v1/schema.
func (c *Client) Schema(ctx context.Context) (*Schema, error) {
	var s Schema
	if err := c.getJSON(ctx, "/v1/schema", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Commands GET /v1/commands.
func (c *Client) Commands(ctx context.Context) (*Commands, error) {
	var out Commands
	if err := c.getJSON(ctx, "/v1/commands", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Invoke POST /v1/invoke. Command failures are HTTP 200 with ok:false.
func (c *Client) Invoke(ctx context.Context, command string, args map[string]any) (*Envelope, error) {
	command = strings.TrimSpace(command)
	if err := rejectForbidden(command); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"command": command,
		"args":    args,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/invoke", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return env, nil
}

// Query runs the query command and returns a typed payload.
func (c *Client) Query(ctx context.Context, queryString string, limit int) (*QueryResult, *Meta, error) {
	args := map[string]any{"query_string": queryString}
	if limit > 0 {
		args["limit"] = limit
	}
	env, err := c.Invoke(ctx, "query", args)
	if err != nil {
		return nil, nil, err
	}
	if err := envelopeErr(env); err != nil {
		return nil, env.Meta, err
	}
	var out QueryResult
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, env.Meta, fmt.Errorf("decode query: %w", err)
	}
	return &out, env.Meta, nil
}

// Read runs enriched read (includes data.links).
func (c *Client) Read(ctx context.Context, reference string) (*ReadResult, *Meta, error) {
	env, err := c.Invoke(ctx, "read", map[string]any{"reference": reference})
	if err != nil {
		return nil, nil, err
	}
	if err := envelopeErr(env); err != nil {
		return nil, env.Meta, err
	}
	var out ReadResult
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, env.Meta, fmt.Errorf("decode read: %w", err)
	}
	return &out, env.Meta, nil
}

// Search runs full-text search.
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	args := map[string]any{"query": query}
	if limit > 0 {
		args["limit"] = limit
	}
	env, err := c.Invoke(ctx, "search", args)
	if err != nil {
		return nil, err
	}
	if err := envelopeErr(env); err != nil {
		return nil, err
	}
	var out SearchResult
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	return &out, nil
}

// Backlinks runs the backlinks command.
func (c *Client) Backlinks(ctx context.Context, reference string) ([]BacklinkGroup, error) {
	env, err := c.Invoke(ctx, "backlinks", map[string]any{"reference": reference})
	if err != nil {
		return nil, err
	}
	if err := envelopeErr(env); err != nil {
		return nil, err
	}
	groups, err := decodeBacklinks(env.Data)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// SavedQueries lists saved queries via query_saved_list.
func (c *Client) SavedQueries(ctx context.Context) ([]SavedQuery, error) {
	env, err := c.Invoke(ctx, "query_saved_list", nil)
	if err != nil {
		return nil, err
	}
	if err := envelopeErr(env); err != nil {
		return nil, err
	}
	var payload struct {
		Queries []SavedQuery `json:"queries"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, fmt.Errorf("decode saved queries: %w", err)
	}
	return payload.Queries, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	env, err := c.do(req)
	if err != nil {
		return err
	}
	if err := envelopeErr(env); err != nil {
		return err
	}
	if dest == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) do(req *http.Request) (*Envelope, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	ua := c.UserAgent
	if ua == "" {
		ua = userAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBody)
	}

	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("raven api %s: HTTP %d: %s", req.URL.Path, resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if env.Error == nil {
			env.Error = &ErrorInfo{Code: "UNAUTHORIZED", Message: "missing or invalid bearer token"}
		}
		return &env, env.Error
	}
	if resp.StatusCode >= 400 && !env.OK {
		if env.Error != nil {
			return &env, env.Error
		}
		return &env, fmt.Errorf("raven api %s: HTTP %d", req.URL.Path, resp.StatusCode)
	}
	return &env, nil
}

func envelopeErr(env *Envelope) error {
	if env == nil {
		return fmt.Errorf("empty raven envelope")
	}
	if env.OK {
		return nil
	}
	if env.Error != nil {
		return env.Error
	}
	return fmt.Errorf("raven command failed")
}

func decodeBacklinks(data json.RawMessage) ([]BacklinkGroup, error) {
	var asList []BacklinkGroup
	if err := json.Unmarshal(data, &asList); err == nil && len(asList) > 0 {
		return asList, nil
	}
	var payload struct {
		Items     []BacklinkGroup `json:"items"`
		Backlinks []BacklinkGroup `json:"backlinks"`
		Groups    []BacklinkGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode backlinks: %w", err)
	}
	switch {
	case len(payload.Backlinks) > 0:
		return payload.Backlinks, nil
	case len(payload.Items) > 0:
		return payload.Items, nil
	case len(payload.Groups) > 0:
		return payload.Groups, nil
	}
	return asList, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
