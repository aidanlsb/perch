package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

type liveFake struct {
	mu       sync.Mutex
	gen      string
	counter  int64
	items    []map[string]any
	reads    map[string]string
	types    map[string]any
	schemaN  int
	queryN   int
	readN    int
	revN     int
	failPoll bool
}

func newLiveFake() *liveFake {
	return &liveFake{
		gen:     "gen-a",
		counter: 1,
		items: []map[string]any{
			{"id": "projects/a", "type": "project", "fields": map[string]any{"name": "Alpha"}, "file_path": "projects/a.md"},
			{"id": "projects/b", "type": "project", "fields": map[string]any{"name": "Beta"}, "file_path": "projects/b.md"},
		},
		reads: map[string]string{
			"projects/a": "# Alpha\n\nbody a\n",
			"projects/b": "# Beta\n\nbody b\n",
		},
		types: map[string]any{
			"project": map[string]any{
				"name": "project", "name_field": "name",
				"fields": map[string]any{"name": map[string]any{"type": "string"}},
			},
		},
	}
}

func (f *liveFake) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/v1/health":
			writeJSON(w, map[string]any{"ok": true, "data": map[string]any{"status": "ok", "api_version": "1"}})
		case "/v1/session":
			writeJSON(w, map[string]any{"ok": true, "data": map[string]any{
				"api_version": "1", "vault_path": "/vault",
				"editor": "true", "editor_mode": "gui",
				"revision": map[string]any{"generation": f.gen, "counter": f.counter},
			}})
		case "/v1/revision":
			f.revN++
			if f.failPoll {
				w.WriteHeader(http.StatusInternalServerError)
				writeJSON(w, map[string]any{"ok": false, "error": map[string]any{"code": "DATABASE", "message": "index locked"}})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "data": map[string]any{"generation": f.gen, "counter": f.counter}})
		case "/v1/schema":
			f.schemaN++
			writeJSON(w, map[string]any{"ok": true, "data": map[string]any{
				"version": 1, "types": f.types, "queries": map[string]any{},
			}})
		case "/v1/invoke":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Command string         `json:"command"`
				Args    map[string]any `json:"args"`
			}
			_ = json.Unmarshal(body, &req)
			switch req.Command {
			case "query_saved_list":
				writeJSON(w, map[string]any{"ok": true, "data": map[string]any{"queries": []any{}}})
			case "objects":
				writeJSON(w, map[string]any{"ok": true, "data": map[string]any{
					"items": []any{}, "total": 0, "returned": 0, "offset": 0, "has_more": false,
				}})
			case "query":
				f.queryN++
				q, _ := req.Args["query_string"].(string)
				items := f.items
				if strings.Contains(q, "projects/b") {
					items = []map[string]any{f.items[1]}
				} else if strings.Contains(q, "projects/a") {
					items = []map[string]any{f.items[0]}
				}
				writeJSON(w, map[string]any{"ok": true, "data": map[string]any{
					"query_kind": "object", "type": "project",
					"items": items, "total": len(items), "returned": len(items), "has_more": false,
				}})
			case "read":
				f.readN++
				ref, _ := req.Args["reference"].(string)
				content := f.reads[ref]
				writeJSON(w, map[string]any{"ok": true, "data": map[string]any{
					"object_id": ref, "path": ref + ".md", "content": content,
					"links": []any{}, "backlinks": []any{}, "references": []any{},
				}})
			default:
				writeJSON(w, map[string]any{"ok": false, "error": map[string]any{"code": "NO", "message": req.Command}})
			}
		default:
			http.NotFound(w, r)
		}
	})
}

func (f *liveFake) setRevision(gen string, counter int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gen = gen
	f.counter = counter
}

func (f *liveFake) swapItems() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.items) >= 2 {
		f.items[0], f.items[1] = f.items[1], f.items[0]
	}
}

func (f *liveFake) replaceItems(items []map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = items
}

func (f *liveFake) addType(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.types[name] = map[string]any{
		"name": name, "name_field": "name",
		"fields": map[string]any{"name": map[string]any{"type": "string"}},
	}
}

func writeJSON(w http.ResponseWriter, body map[string]any) {
	_ = json.NewEncoder(w).Encode(body)
}

func startLive(t *testing.T, f *liveFake) (Model, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	m := NewModel(Options{Client: api.NewClient(srv.URL, "t"), PollInterval: -1, Config: &config.File{}})
	m.width, m.height = 80, 24
	m = execCmds(t, m, m.Init())
	if !m.ready {
		t.Fatalf("not ready: %s", m.errMsg)
	}
	if m.revGeneration != "gen-a" || m.revCounter != 1 {
		t.Fatalf("revision %s/%d", m.revGeneration, m.revCounter)
	}
	return m, srv
}

func execCmds(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for i := 0; i < 24 && cmd != nil; i++ {
		msg, ok := waitMsg(cmd, time.Second)
		if !ok {
			return m
		}
		if b, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, sub := range b {
				m = execCmds(t, m, sub)
			}
			return m
		}
		m, cmd = m.update(msg)
	}
	return m
}

func waitMsg(cmd tea.Cmd, d time.Duration) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return msg, true
	case <-time.After(d):
		return nil, false
	}
}

func TestLiveCounterPreservesSelectionByID(t *testing.T) {
	f := newLiveFake()
	m, _ := startLive(t, f)
	if len(m.p().filtered) != 2 {
		t.Fatalf("items %d", len(m.p().filtered))
	}
	focusMain(&m)
	m, _ = m.update(key("j"))
	sel := m.selectedObject()
	if sel == nil || sel.ID != "projects/b" {
		t.Fatalf("want b, got %+v", sel)
	}

	f.replaceItems([]map[string]any{
		{"id": "projects/b", "type": "project", "fields": map[string]any{"name": "Beta"}, "file_path": "projects/b.md"},
		{"id": "projects/c", "type": "project", "fields": map[string]any{"name": "Gamma"}, "file_path": "projects/c.md"},
	})
	f.setRevision("gen-a", 2)
	m, cmd := m.update(revisionMsg{rev: &api.Revision{Generation: "gen-a", Counter: 2}})
	if cmd == nil {
		t.Fatal("expected debounce")
	}
	m = execCmds(t, m, cmd)
	sel = m.selectedObject()
	if sel == nil || sel.ID != "projects/b" {
		t.Fatalf("selection lost: %+v filtered=%v", sel, idsOf(m.p().filtered))
	}
	if m.p().listCur != 0 {
		t.Fatalf("wanted re-find by id at index 0, listCur=%d ids=%v", m.p().listCur, idsOf(m.p().filtered))
	}
}

func TestLiveGenerationDropsCacheAndRefetchesSchema(t *testing.T) {
	f := newLiveFake()
	m, _ := startLive(t, f)
	if len(m.queryCache) == 0 {
		t.Fatal("expected query cache")
	}
	schemaGets := f.schemaN
	f.addType("note")
	f.setRevision("gen-b", 0)
	m, cmd := m.update(revisionMsg{rev: &api.Revision{Generation: "gen-b", Counter: 0}})
	if cmd == nil {
		t.Fatal("expected schema refetch")
	}
	m = execCmds(t, m, cmd)
	if m.revGeneration != "gen-b" {
		t.Fatalf("generation %s", m.revGeneration)
	}
	if _, ok := m.schema.Types["note"]; !ok {
		t.Fatalf("schema not refetched: %v", m.schema.Types)
	}
	if f.schemaN <= schemaGets {
		t.Fatalf("schema gets %d want > %d", f.schemaN, schemaGets)
	}
}

func TestLiveBindReresolvesAfterCounter(t *testing.T) {
	f := newLiveFake()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	cfg := &config.File{
		Views: map[string]config.View{
			"projects": {Source: "project", Layout: config.Present{Kind: kindList}},
			"tasks": {
				Source: "type:project .id=={{selected}}",
				Layout: config.Present{Kind: kindList},
			},
		},
		Layouts: map[string]config.Layout{
			"workspace": {
				Split: "row",
				Panels: []config.Panel{
					{View: "projects"},
					{View: "tasks", Bind: "projects"},
				},
			},
		},
	}
	m := NewModel(Options{Client: api.NewClient(srv.URL, "t"), PollInterval: -1, Config: cfg})
	m.width, m.height = 80, 24
	m = execCmds(t, m, m.Init())
	m, cmd := m.loadNamedLayout("workspace")
	m = execCmds(t, m, cmd)
	left := m.viewByID("projects")
	if left == nil || len(left.filtered) < 2 {
		t.Fatalf("projects %+v", left)
	}
	tasks := m.viewByID("tasks")
	if tasks == nil || !strings.Contains(tasks.sourceQuery, "projects/a") {
		t.Fatalf("bound query %v", tasks)
	}

	m.setFocus("projects")
	m, cmd = m.update(key("j"))
	m = execCmds(t, m, cmd)
	tasks = m.viewByID("tasks")
	if !strings.Contains(tasks.sourceQuery, "projects/b") {
		t.Fatalf("after move %s", tasks.sourceQuery)
	}

	f.swapItems()
	f.setRevision("gen-a", 3)
	m, cmd = m.update(revisionMsg{rev: &api.Revision{Generation: "gen-a", Counter: 3}})
	m = execCmds(t, m, cmd)
	left = m.viewByID("projects")
	if left.selectedID() != "projects/b" {
		t.Fatalf("driver selection %s", left.selectedID())
	}
	tasks = m.viewByID("tasks")
	if !strings.Contains(tasks.sourceQuery, "projects/b") {
		t.Fatalf("bind did not re-resolve: %s", tasks.sourceQuery)
	}
}

func TestLiveNotePreservesScroll(t *testing.T) {
	f := newLiveFake()
	m, _ := startLive(t, f)
	focusMain(&m)
	m, cmd := m.update(key("enter"))
	m = execCmds(t, m, cmd)
	if m.note == nil {
		t.Fatal("note")
	}
	m.noteScroll = 4
	reads := f.readN
	f.setRevision("gen-a", 4)
	m, cmd = m.update(revisionMsg{rev: &api.Revision{Generation: "gen-a", Counter: 4}})
	m = execCmds(t, m, cmd)
	if m.noteScroll != 4 {
		t.Fatalf("scroll %d", m.noteScroll)
	}
	if f.readN <= reads {
		t.Fatal("expected note refetch")
	}
}

func TestLivePollErrorIndicatorAndBackoff(t *testing.T) {
	m := NewModel(Options{PollInterval: time.Second})
	if m.pollDelay() != time.Second {
		t.Fatalf("base %v", m.pollDelay())
	}
	m, _ = m.update(revisionMsg{err: errors.New("boom")})
	if m.liveErr != liveErrPoll {
		t.Fatalf("indicator %q", m.liveErr)
	}
	if m.pollFails != 1 {
		t.Fatalf("fails %d", m.pollFails)
	}
	if m.pollDelay() != 2*time.Second {
		t.Fatalf("backoff %v", m.pollDelay())
	}
	m.width, m.height = 80, 24
	out := m.View()
	if !strings.Contains(out, liveErrPoll) {
		t.Fatalf("view missing indicator: %s", out)
	}
	m, _ = m.update(revisionMsg{rev: &api.Revision{Generation: "g", Counter: 1}})
	if m.liveErr != "" || m.pollFails != 0 {
		t.Fatalf("cleared? err=%q fails=%d", m.liveErr, m.pollFails)
	}
}

func TestLivePollTickFetchesRevision(t *testing.T) {
	f := newLiveFake()
	m, _ := startLive(t, f)
	f.setRevision("gen-a", 9)
	m, cmd := m.update(pollTickMsg{})
	if cmd == nil {
		t.Fatal("expected revision fetch")
	}
	msg := cmd()
	rev, ok := msg.(revisionMsg)
	if !ok || rev.err != nil || rev.rev == nil || rev.rev.Counter != 9 {
		t.Fatalf("got %#v", msg)
	}
	m, cmd = m.update(rev)
	m = execCmds(t, m, cmd)
	if m.revCounter != 9 {
		t.Fatalf("counter %d", m.revCounter)
	}
}

func idsOf(objs []api.Object) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.ID
	}
	return out
}

func TestPollDelayCaps(t *testing.T) {
	m := NewModel(Options{PollInterval: time.Second})
	m.pollFails = 20
	if m.pollDelay() != maxPollBackoff {
		t.Fatalf("%v", m.pollDelay())
	}
	m.pollInterval = 0
	if m.pollDelay() != 0 {
		t.Fatalf("disabled %v", m.pollDelay())
	}
}

func traitItem(id, objectID, name string, line int) map[string]any {
	return map[string]any{
		"id": id, "type": "project", "kind": "trait",
		"object_id": objectID, "line": line, "file_path": objectID + ".md",
		"fields": map[string]any{"name": name},
	}
}

func TestLiveCounterRefindsTraitRowByID(t *testing.T) {
	f := newLiveFake()
	f.replaceItems([]map[string]any{
		traitItem("projects/a.md:trait:0", "projects/a", "Alpha", 10),
		traitItem("projects/b.md:trait:0", "projects/b", "Beta", 20),
	})
	m, _ := startLive(t, f)
	focusMain(&m)
	m, _ = m.update(key("j"))
	sel := m.selectedObject()
	if sel == nil || sel.ID != "projects/b.md:trait:0" {
		t.Fatalf("want trait b, got %+v", sel)
	}
	if sel.OpenRef() != "projects/b" {
		t.Fatalf("OpenRef %q", sel.OpenRef())
	}

	f.replaceItems([]map[string]any{
		traitItem("projects/a.md:trait:0", "projects/a", "Alpha", 10),
		traitItem("projects/c.md:trait:0", "projects/c", "Gamma", 8),
		traitItem("projects/b.md:trait:0", "projects/b", "Beta", 20),
	})
	f.setRevision("gen-a", 2)
	m, cmd := m.update(revisionMsg{rev: &api.Revision{Generation: "gen-a", Counter: 2}})
	m = execCmds(t, m, cmd)
	sel = m.selectedObject()
	if sel == nil || sel.ID != "projects/b.md:trait:0" {
		t.Fatalf("trait row lost: %+v filtered=%v", sel, idsOf(m.p().filtered))
	}
	if sel.OpenRef() != "projects/b" {
		t.Fatalf("bind/open after refresh %q", sel.OpenRef())
	}
	if m.p().listCur != 1 {
		t.Fatalf("wanted re-find by row id at index 1, listCur=%d ids=%v", m.p().listCur, idsOf(m.p().filtered))
	}

	m, cmd = m.update(key("enter"))
	m = execCmds(t, m, cmd)
	if m.errMsg != "" {
		t.Fatalf("enter err %s", m.errMsg)
	}
	if m.note == nil || m.note.ObjectID != "projects/b" {
		t.Fatalf("open should use containing file, note=%+v", m.note)
	}
	if m.currentNoteLine() != 20 {
		t.Fatalf("note line %d", m.currentNoteLine())
	}
}

func TestLivePreservesFocus(t *testing.T) {
	f := newLiveFake()
	m, _ := startLive(t, f)
	focusMain(&m)
	mainID := m.p().id
	if m.focus != mainID {
		t.Fatalf("focus=%s want %s", m.focus, mainID)
	}

	f.setRevision("gen-a", 2)
	m, cmd := m.update(revisionMsg{rev: &api.Revision{Generation: "gen-a", Counter: 2}})
	m = execCmds(t, m, cmd)
	if m.focus != mainID {
		t.Fatalf("counter focus %s want %s", m.focus, mainID)
	}
	if m.lastMainFocus != mainID {
		t.Fatalf("lastMainFocus %s", m.lastMainFocus)
	}

	f.addType("note")
	f.setRevision("gen-b", 0)
	m, cmd = m.update(revisionMsg{rev: &api.Revision{Generation: "gen-b", Counter: 0}})
	m = execCmds(t, m, cmd)
	if m.focus != mainID {
		t.Fatalf("generation focus %s want %s", m.focus, mainID)
	}
}

func TestLiveRefreshErrorStaysSubtle(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	p := m.p()
	p.live = true
	p.gen = m.gen
	m, _ = m.update(queryMsg{gen: p.gen, pane: p.id, err: fmt.Errorf("nope")})
	if m.errMsg != "" {
		t.Fatalf("errMsg should stay clear, got %q", m.errMsg)
	}
	if m.liveErr != liveErrRefresh {
		t.Fatalf("liveErr %q", m.liveErr)
	}
	if !strings.Contains(m.View(), liveErrRefresh) {
		t.Fatal("missing status indicator")
	}
}
