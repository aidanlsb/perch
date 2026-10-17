package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type schemaMsg struct {
	schema   *api.Schema
	queries  []api.SavedQuery
	session  *api.Session
	revision *api.Revision
	reload   bool
	err      error
}

type revisionMsg struct {
	rev *api.Revision
	err error
}

type pollTickMsg struct{}

type debounceMsg struct {
	seq int
}

type queryMsg struct {
	gen      int
	pane     string
	objects  []api.Object
	typeName string
	hasMore  bool
	total    int
	vault    string
	err      error
}

type objectsMsg struct {
	objects    []api.Object
	offset     int
	nextOffset int
	hasMore    bool
	total      int
	fallback   bool
	err        error
}

type readMsg struct {
	gen     int
	purpose string // note | preview
	id      string
	result  *api.ReadResult
	vault   string
	keep    bool
	live    bool
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
	date   string
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
	config   bool
}

func fetchSchema(c *api.Client) tea.Cmd {
	return fetchStartup(c, false)
}

func fetchStartup(c *api.Client, reload bool) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return schemaMsg{reload: reload, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		sess, _ := c.Session(ctx)
		rev, rerr := c.Revision(ctx)
		if rerr != nil {
			return schemaMsg{session: sess, reload: reload, err: revisionRequiredErr(rerr)}
		}
		sch, err := c.Schema(ctx)
		if err != nil {
			return schemaMsg{session: sess, revision: rev, reload: reload, err: err}
		}
		qs, qerr := c.SavedQueries(ctx)
		if qerr == nil && sch.Queries == nil && len(qs) > 0 {
			sch.Queries = map[string]api.SavedQuery{}
			for _, q := range qs {
				sch.Queries[q.Name] = q
			}
		}
		return schemaMsg{schema: sch, queries: qs, session: sess, revision: rev, reload: reload}
	}
}

func fetchRevision(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return revisionMsg{err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
		defer cancel()
		rev, err := c.Revision(ctx)
		if err != nil {
			return revisionMsg{err: err}
		}
		return revisionMsg{rev: rev}
	}
}

func fetchQuery(c *api.Client, pane string, gen int, query, typeName string, limit int, ts api.TypeSchema, inputs map[string]string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return queryMsg{gen: gen, pane: pane, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		qr, meta, err := c.QueryInputs(ctx, query, limit, inputs)
		if err != nil {
			return queryMsg{gen: gen, pane: pane, err: err}
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
			pane:     pane,
			objects:  api.ObjectsFromQuery(qr, ts),
			typeName: typeName,
			hasMore:  qr.HasMore,
			total:    qr.Total,
			vault:    vault,
		}
	}
}

func fetchRead(c *api.Client, gen int, ref, purpose string) tea.Cmd {
	return fetchReadFull(c, gen, ref, purpose, false, false)
}

func fetchReadFull(c *api.Client, gen int, ref, purpose string, keep, live bool) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return readMsg{gen: gen, purpose: purpose, id: ref, keep: keep, live: live, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rd, meta, err := c.Read(ctx, ref)
		if err != nil {
			return readMsg{gen: gen, purpose: purpose, id: ref, keep: keep, live: live, err: err}
		}
		vault := ""
		if meta != nil && meta.VaultContext != nil {
			vault = meta.VaultContext.Path
		}
		return readMsg{gen: gen, purpose: purpose, id: ref, result: rd, vault: vault, keep: keep, live: live}
	}
}

func fetchDate(c *api.Client, gen int, date string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return dateMsg{gen: gen, date: date, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dr, err := c.Date(ctx, date)
		if err != nil {
			return dateMsg{gen: gen, date: date, err: err}
		}
		return dateMsg{gen: gen, date: date, result: dr}
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

const objectsPageSize = 200

func fetchObjectsPage(c *api.Client, offset, limit int) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return objectsMsg{fallback: true, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		res, err := c.Objects(ctx, limit, offset)
		if err != nil {
			return objectsMsg{
				err:      err,
				fallback: api.IsCommandNotAllowed(err) || api.IsNotFound(err),
				offset:   offset,
			}
		}
		next := offset + len(res.Items)
		if res.NextOffset != nil {
			next = *res.NextOffset
		}
		return objectsMsg{
			objects:    api.ObjectsFromCorpus(res.Items),
			offset:     res.Offset,
			nextOffset: next,
			hasMore:    res.HasMore,
			total:      res.Total,
		}
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

func revisionRequiredErr(err error) error {
	if err == nil || errors.Is(err, api.ErrNoRevision) {
		return errRevisionRequired
	}
	return fmt.Errorf("%s: %w", api.RevisionRequired, err)
}

const errRevisionRequired staticErr = api.RevisionRequired
