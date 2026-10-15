package ui

import (
	"context"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type schemaMsg struct {
	schema  *api.Schema
	queries []api.SavedQuery
	session *api.Session
	err     error
}

type queryMsg struct {
	gen      int
	objects  []api.Object
	typeName string
	hasMore  bool
	total    int
	vault    string
	err      error
}

type readMsg struct {
	gen     int
	purpose string // note | preview
	id      string
	result  *api.ReadResult
	vault   string
	err     error
}

type searchMsg struct {
	gen    int
	query  string
	result *api.SearchResult
	err    error
}

type dateMsg struct {
	gen    int
	result *api.DateResult
	err    error
}

type prefetchMsg struct {
	objects []api.Object
	rest    []string
}

type editorDoneMsg struct {
	err      error
	objectID string
}

func fetchSchema(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return schemaMsg{err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		sess, _ := c.Session(ctx)
		sch, err := c.Schema(ctx)
		if err != nil {
			return schemaMsg{session: sess, err: err}
		}
		qs, qerr := c.SavedQueries(ctx)
		if qerr == nil && sch.Queries == nil && len(qs) > 0 {
			sch.Queries = map[string]api.SavedQuery{}
			for _, q := range qs {
				sch.Queries[q.Name] = q
			}
		}
		return schemaMsg{schema: sch, queries: qs, session: sess}
	}
}

func fetchQuery(c *api.Client, gen int, query, typeName string, limit int, ts api.TypeSchema) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return queryMsg{gen: gen, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		qr, meta, err := c.Query(ctx, query, limit)
		if err != nil {
			return queryMsg{gen: gen, err: err}
		}
		if typeName == "" {
			typeName = qr.Type
		}
		vault := ""
		if meta != nil && meta.VaultContext != nil {
			vault = meta.VaultContext.Path
		}
		return queryMsg{
			gen:      gen,
			objects:  api.ObjectsFromQuery(qr, ts),
			typeName: typeName,
			hasMore:  qr.HasMore,
			total:    qr.Total,
			vault:    vault,
		}
	}
}

func fetchRead(c *api.Client, gen int, ref, purpose string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return readMsg{gen: gen, purpose: purpose, id: ref, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rd, meta, err := c.Read(ctx, ref)
		if err != nil {
			return readMsg{gen: gen, purpose: purpose, id: ref, err: err}
		}
		vault := ""
		if meta != nil && meta.VaultContext != nil {
			vault = meta.VaultContext.Path
		}
		return readMsg{gen: gen, purpose: purpose, id: ref, result: rd, vault: vault}
	}
}

func fetchSearch(c *api.Client, gen int, q string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return searchMsg{gen: gen, query: q, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sr, err := c.Search(ctx, q, 20)
		if err != nil {
			return searchMsg{gen: gen, query: q, err: err}
		}
		return searchMsg{gen: gen, query: q, result: sr}
	}
}

func fetchDate(c *api.Client, gen int, date string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return dateMsg{gen: gen, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		dr, err := c.Date(ctx, date)
		if err != nil {
			return dateMsg{gen: gen, err: err}
		}
		return dateMsg{gen: gen, result: dr}
	}
}

func fetchPrefetch(c *api.Client, typeName string, rest []string, limit int, ts api.TypeSchema) tea.Cmd {
	return func() tea.Msg {
		if c == nil || typeName == "" {
			return prefetchMsg{rest: rest}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		qr, _, err := c.Query(ctx, "type:"+typeName, limit)
		if err != nil {
			return prefetchMsg{rest: rest}
		}
		return prefetchMsg{objects: api.ObjectsFromQuery(qr, ts), rest: rest}
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

const errNoClient staticErr = "no raven api client"
