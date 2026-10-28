package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type invokeCall struct {
	Command string
	Args    map[string]any
}

type mutLog struct {
	mu    sync.Mutex
	calls []invokeCall
}

func (l *mutLog) add(c invokeCall) {
	l.mu.Lock()
	l.calls = append(l.calls, c)
	l.mu.Unlock()
}

func (l *mutLog) last(cmd string) invokeCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.calls) - 1; i >= 0; i-- {
		if l.calls[i].Command == cmd {
			return l.calls[i]
		}
	}
	return invokeCall{}
}

func (l *mutLog) count(cmd string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.calls {
		if c.Command == cmd {
			n++
		}
	}
	return n
}

func newMutServer(t *testing.T) (*httptest.Server, *mutLog) {
	t.Helper()
	log := &mutLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/session":
			writeOK(w, map[string]any{
				"api_version": "1", "vault_path": "/vault",
				"editor": "code", "editor_mode": "gui",
				"revision": map[string]any{"generation": "g1", "counter": 1},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/revision":
			writeOK(w, map[string]any{"generation": "g1", "counter": 1})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/schema":
			writeOK(w, map[string]any{
				"version": 1,
				"types": map[string]any{
					"project": map[string]any{
						"name": "project", "name_field": "name",
						"fields": map[string]any{
							"name":   map[string]any{"type": "string"},
							"status": map[string]any{"type": "enum", "values": []string{"idea", "active", "done"}},
							"due":    map[string]any{"type": "date"},
							"owner":  map[string]any{"type": "ref", "target": "person"},
						},
					},
					"person": map[string]any{
						"name": "person", "name_field": "name",
						"fields": map[string]any{"name": map[string]any{"type": "string"}},
					},
				},
				"traits": map[string]any{
					"done":     map[string]any{"name": "done", "type": "bool"},
					"priority": map[string]any{"name": "priority", "type": "enum", "values": []string{"low", "high"}},
					"mood":     map[string]any{"name": "mood", "type": "enum", "values": []string{"a", "b", "c"}},
				},
				"queries": map[string]any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoke":
			body, _ := io.ReadAll(r.Body)
			var req invokeCall
			_ = json.Unmarshal(body, &req)
			switch req.Command {
			case "query", "objects", "query_saved_list", "read", "search", "date":
				handleReadInvoke(w, req)
			default:
				log.add(req)
				handleMutInvoke(w, req)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

func handleReadInvoke(w http.ResponseWriter, req invokeCall) {
	switch req.Command {
	case "query_saved_list":
		writeOK(w, map[string]any{"queries": []any{}})
	case "objects":
		writeOK(w, map[string]any{"items": []any{}, "total": 0, "returned": 0, "has_more": false})
	case "query":
		writeOK(w, map[string]any{
			"query_kind": "object", "type": "project",
			"items": []any{map[string]any{
				"id": "projects/a", "type": "project",
				"fields":    map[string]any{"name": "Alpha", "status": "active", "due": "2026-10-01"},
				"file_path": "projects/a.md",
			}},
			"total": 1, "returned": 1, "has_more": false,
		})
	case "read":
		ref, _ := req.Args["reference"].(string)
		writeOK(w, map[string]any{
			"object_id": ref, "path": ref + ".md", "content": "body\n",
			"links": []any{}, "backlinks": []any{}, "references": []any{},
		})
	case "date":
		date, _ := req.Args["date"].(string)
		writeOK(w, map[string]any{
			"date": date, "daily_note_id": date, "daily_path": "daily/" + date + ".md",
			"daily_exists": false,
		})
	default:
		writeOK(w, map[string]any{})
	}
}

func handleMutInvoke(w http.ResponseWriter, req invokeCall) {
	switch req.Command {
	case "set":
		writeOK(w, map[string]any{"file": "a.md", "object_id": "projects/a", "type": "project",
			"updated_fields": req.Args["fields-json"], "previous_fields": map[string]any{}})
	case "unset":
		writeOK(w, map[string]any{"file": "a.md", "object_id": "projects/a", "type": "project",
			"removed_fields": map[string]any{}, "missing_fields": []any{}, "modified": true})
	case "update":
		writeOK(w, map[string]any{"action": "update_traits", "items": []any{map[string]any{
			"id": req.Args["trait_id"], "new_value": req.Args["value"], "status": "modified",
		}}, "total": 1, "modified": 1})
	case "new":
		title, _ := req.Args["title"].(string)
		writeOK(w, map[string]any{"id": "projects/new", "file": "projects/new.md", "title": title, "type": req.Args["type"]})
	case "daily":
		date, _ := req.Args["date"].(string)
		writeOK(w, map[string]any{"id": date, "file": "daily/" + date + ".md", "date": date, "created": true, "opened": false})
	case "add":
		writeOK(w, map[string]any{"file": "inbox.md", "line": 3, "content": req.Args["text"]})
	case "delete":
		dry, _ := req.Args["dry-run"].(bool)
		if dry {
			writeOK(w, map[string]any{"preview": true, "object_id": req.Args["reference"], "behavior": "trash",
				"trash_dir": ".trash", "backlinks": []any{"x", "y"}})
			return
		}
		writeOK(w, map[string]any{"deleted": req.Args["reference"], "behavior": "trash", "trash_path": ".trash/a.md"})
	case "move":
		dry, _ := req.Args["dry-run"].(bool)
		if dry {
			writeOK(w, map[string]any{"source": req.Args["source"], "destination": req.Args["destination"],
				"updated_refs": []any{"a"}, "needs_confirm": false})
			return
		}
		writeOK(w, map[string]any{"source": req.Args["source"], "destination": req.Args["destination"],
			"updated_refs": []any{"a"}})
	case "reclassify":
		force, _ := req.Args["force"].(bool)
		if !force {
			writeOK(w, map[string]any{"object_id": req.Args["reference"], "old_type": "project", "new_type": req.Args["new-type"],
				"dropped_fields": []any{"status"}, "needs_confirm": true, "reason": "fields dropped"})
			return
		}
		writeOK(w, map[string]any{"object_id": req.Args["reference"], "old_type": "project", "new_type": req.Args["new-type"],
			"moved": true, "needs_confirm": false, "dropped_fields": []any{"status"}})
	case "query_saved_set":
		writeOK(w, map[string]any{"name": req.Args["name"], "query": req.Args["query_string"], "status": "created"})
	case "schema_add_type":
		writeOK(w, map[string]any{"name": req.Args["name"]})
	case "schema_add_field":
		writeOK(w, map[string]any{"field": req.Args["field_name"]})
	case "schema_add_trait":
		writeOK(w, map[string]any{"name": req.Args["name"]})
	case "schema_rename_type", "schema_rename_field":
		confirm, _ := req.Args["confirm"].(bool)
		writeOK(w, map[string]any{"preview": !confirm, "total_changes": 2, "files_to_move": 1})
	default:
		writeErr(w, "NO", req.Command)
	}
}

func loadMutBrowse(t *testing.T) (Model, *mutLog) {
	t.Helper()
	srv, log := newMutServer(t)
	m := loadBrowse(t)
	m.client = api.NewClient(srv.URL, "t")
	m.schema.Traits = map[string]api.TraitSchema{
		"done":     {Name: "done", Type: "bool"},
		"priority": {Name: "priority", Type: "enum", Values: []string{"low", "high"}},
		"mood":     {Name: "mood", Type: "enum", Values: []string{"a", "b", "c"}},
	}
	m.mergeNotes([]api.Object{
		{ID: "people/x", Title: "X", Type: "person"},
		{ID: "projects/a", Title: "Alpha", Type: "project"},
	})
	return m, log
}

func flushMut(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	m, _ = m.update(cmd())
	return m
}

func pickByID(m *Model, id string) {
	for i, it := range m.pickerHit {
		if it.ID == id || it.Label == id {
			m.pickerCur = i
			return
		}
	}
}

func TestSetFieldEnum(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("s"))
	if m.overlay != overlayPicker || m.pickerKind != pickerFields {
		t.Fatalf("field picker overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	pickByID(&m, "status")
	m, _ = m.update(key("enter"))
	if m.overlay != overlayPicker || m.pickerKind != pickerValues {
		t.Fatalf("enum overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	pickByID(&m, "done")
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	got := log.last("set")
	if got.Command != "set" {
		t.Fatalf("calls %+v", log.calls)
	}
	fj, _ := got.Args["fields-json"].(map[string]any)
	if got.Args["reference"] != "projects/a" || fj["status"] != "done" {
		t.Fatalf("set args %+v", got.Args)
	}
}

func TestUnsetOptionalDateField(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("s"))
	pickByID(&m, "due")
	m, _ = m.update(key("enter"))
	if m.overlay != overlayDate || m.datePurpose != "value" {
		t.Fatalf("date widget overlay=%v purpose=%s", m.overlay, m.datePurpose)
	}
	m, cmd := m.update(key("ctrl+d"))
	m = flushMut(t, m, cmd)
	got := log.last("unset")
	fields, _ := got.Args["fields"].([]any)
	if got.Command != "unset" || len(fields) != 1 || fields[0] != "due" {
		t.Fatalf("unset %+v", got.Args)
	}
}

func TestTraitToggleX(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m.p().objects = []api.Object{{
		ID: "notes/x.md:trait:0", Kind: "trait", Type: "done",
		Fields:   map[string]any{"trait_type": "done", "value": "false"},
		ObjectID: "notes/x",
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	m, cmd := m.update(key("x"))
	m = flushMut(t, m, cmd)
	got := log.last("update")
	if got.Args["trait_id"] != "notes/x.md:trait:0" || got.Args["value"] != "true" {
		t.Fatalf("update %+v", got.Args)
	}
}

func TestTraitEnumTwoToggleAndManyPicker(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m.p().objects = []api.Object{{
		ID: "notes/x.md:trait:1", Kind: "trait",
		Fields: map[string]any{"trait_type": "priority", "value": "low"},
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	m, cmd := m.update(key("x"))
	m = flushMut(t, m, cmd)
	if log.last("update").Args["value"] != "high" {
		t.Fatalf("priority toggle %+v", log.last("update").Args)
	}

	m.p().objects = []api.Object{{
		ID: "notes/x.md:trait:2", Kind: "trait",
		Fields: map[string]any{"trait_type": "mood", "value": "a"},
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	m, _ = m.update(key("x"))
	if m.overlay != overlayPicker || m.pickerKind != pickerValues {
		t.Fatalf("mood should open picker, overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	pickByID(&m, "c")
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.last("update").Args["value"] != "c" {
		t.Fatalf("mood %+v", log.last("update").Args)
	}
}

func TestBoardHLMovesCard(t *testing.T) {
	srv, log := newMutServer(t)
	m := loadNamedView(t, namedViewCfg(), "projects")
	m.client = api.NewClient(srv.URL, "t")
	focusMain(&m)
	if m.p().spec.Kind != kindBoard {
		t.Fatal(m.p().spec.Kind)
	}
	obj := m.p().selectedObject()
	if obj == nil {
		t.Fatal("no card")
	}
	next := m.p().board[m.p().boardCol+1]
	m, cmd := m.update(key("L"))
	m = flushMut(t, m, cmd)
	got := log.last("set")
	fj, _ := got.Args["fields-json"].(map[string]any)
	if got.Command != "set" || got.Args["reference"] != obj.ID || fj["status"] != next.Key {
		t.Fatalf("set %+v want status=%q id=%s", got.Args, next.Key, obj.ID)
	}
}

func TestBoardHLUnsetsEmptyColumn(t *testing.T) {
	srv, log := newMutServer(t)
	m := loadNamedView(t, namedViewCfg(), "projects")
	m.client = api.NewClient(srv.URL, "t")
	focusMain(&m)
	empty := -1
	for i, col := range m.p().board {
		if col.Key == "" {
			empty = i
			break
		}
	}
	if empty < 1 {
		t.Fatal("no empty column")
	}
	start := m.p().boardCol
	for i := 0; i < len(m.p().board)+2 && m.p().boardCol < empty; i++ {
		m, cmd := m.update(key("L"))
		if cmd != nil {
			m = flushMut(t, m, cmd)
		}
		if m.p().boardCol == start && i > 0 && cmd == nil {
			t.Fatalf("board col stuck at %d", start)
		}
	}
	if log.count("unset") == 0 {
		t.Fatalf("want unset onto empty column, calls %+v", log.calls)
	}
	fields, _ := log.last("unset").Args["fields"].([]any)
	if len(fields) != 1 || fields[0] != "status" {
		t.Fatalf("unset %+v", log.last("unset").Args)
	}
}

func TestCreateObjectShortcut(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("c"))
	if m.overlay != overlayPrompt {
		t.Fatalf("title prompt %v", m.overlay)
	}
	for _, r := range "Nova" {
		m, _ = m.update(runeKey(r))
	}
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	got := log.last("new")
	if got.Args["type"] != "project" || got.Args["title"] != "Nova" {
		t.Fatalf("new %+v", got.Args)
	}
}

func TestDailyEnsureCreatesAndOpens(t *testing.T) {
	m, log := loadMutBrowse(t)
	m.gen = 4
	m, cmd := m.update(dateMsg{
		gen: 4, date: "2026-10-15",
		result: &api.DateResult{Date: "2026-10-15", DailyExists: false},
	})
	m, next := m.update(cmd())
	if log.last("daily").Args["date"] != "2026-10-15" {
		t.Fatalf("daily %+v", log.last("daily"))
	}
	if next == nil {
		t.Fatal("should read created daily note")
	}
	_ = m
}

func TestSaveAdHocRQLQuery(t *testing.T) {
	m, log := loadMutBrowse(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	m.cfgPath = path
	m, _ = m.loadRQL("type:project .status==active")
	if !m.p().sourceIsQuery || m.p().savedQuery != nil {
		t.Fatalf("adhoc query pane %+v", m.p())
	}
	focusMain(&m)
	m, _ = m.update(key("S"))
	for _, r := range "from-rql" {
		m, _ = m.update(runeKey(r))
	}
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	got := log.last("query_saved_set")
	if got.Args["name"] != "from-rql" || got.Args["query_string"] != "type:project .status==active" {
		t.Fatalf("save query %+v", got.Args)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "[views.from-rql]") || !strings.Contains(string(body), `query = "from-rql"`) {
		t.Fatalf("config:\n%s", body)
	}
}

func TestDeletePreviewThenConfirm(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, cmd := pressLeader(m, "a")
	if cmd != nil {
		t.Fatal("actions is local")
	}
	pickByID(&m, "delete")
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if m.overlay != overlayConfirm {
		t.Fatalf("preview overlay %v status %s", m.overlay, m.status)
	}
	if !strings.Contains(m.mut.confirmText, "2 backlinks") {
		t.Fatalf("confirm %s", m.mut.confirmText)
	}
	if log.count("delete") != 1 {
		t.Fatalf("preview calls %d", log.count("delete"))
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.count("delete") != 2 {
		t.Fatalf("apply calls %d", log.count("delete"))
	}
	if log.last("delete").Args["dry-run"] != false {
		t.Fatalf("apply args %+v", log.last("delete").Args)
	}
}

func TestMovePreviewThenConfirm(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, _ = pressLeader(m, "a")
	pickByID(&m, "move")
	m, _ = m.update(key("enter"))
	if m.overlay != overlayPrompt {
		t.Fatalf("dest prompt %v", m.overlay)
	}
	m.mut.promptBuf = "projects/z"
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if m.overlay != overlayConfirm {
		t.Fatalf("preview %v %s", m.overlay, m.status)
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.count("move") != 2 {
		t.Fatalf("move calls %d %+v", log.count("move"), log.calls)
	}
	if log.last("move").Args["dry-run"] != false {
		t.Fatalf("apply %+v", log.last("move").Args)
	}
}

func TestReclassifyPreviewThenForce(t *testing.T) {
	m, log := loadMutBrowse(t)
	focusMain(&m)
	m, _ = pressLeader(m, "a")
	pickByID(&m, "reclassify")
	m, _ = m.update(key("enter"))
	pickByID(&m, "person")
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if m.overlay != overlayConfirm {
		t.Fatalf("needs_confirm overlay %v %s", m.overlay, m.status)
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.count("reclassify") != 2 {
		t.Fatalf("reclassify %d", log.count("reclassify"))
	}
	if log.last("reclassify").Args["force"] != true {
		t.Fatalf("force %+v", log.last("reclassify").Args)
	}
}

func TestSchemaFlows(t *testing.T) {
	m, log := loadMutBrowse(t)
	m, _ = pressLeader(m, "a")
	pickByID(&m, "schema_add_type")
	m, _ = m.update(key("enter"))
	for _, r := range "task" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))    // name
	m, _ = m.update(key("enter"))    // skip name-field
	m, cmd := m.update(key("enter")) // skip default-path
	m = flushMut(t, m, cmd)
	if log.last("schema_add_type").Args["name"] != "task" {
		t.Fatalf("add type %+v", log.last("schema_add_type").Args)
	}

	m, _ = pressLeader(m, "a")
	pickByID(&m, "schema_add_field")
	m, _ = m.update(key("enter"))
	for _, r := range "state" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))
	pickByID(&m, "enum")
	m, _ = m.update(key("enter"))
	for _, r := range "open,done" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))
	pickByID(&m, "yes")
	m, _ = m.update(key("enter"))
	m, cmd = m.update(key("enter")) // skip default
	m = flushMut(t, m, cmd)
	got := log.last("schema_add_field")
	if got.Args["field_name"] != "state" || got.Args["type"] != "enum" {
		t.Fatalf("add field %+v", got.Args)
	}

	m, _ = pressLeader(m, "a")
	pickByID(&m, "schema_add_trait")
	m, _ = m.update(key("enter"))
	for _, r := range "blocked" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))
	pickByID(&m, "bool")
	m, _ = m.update(key("enter"))
	m, cmd = m.update(key("enter")) // skip default
	m = flushMut(t, m, cmd)
	if log.last("schema_add_trait").Args["name"] != "blocked" {
		t.Fatalf("add trait %+v", log.last("schema_add_trait").Args)
	}

	m, _ = pressLeader(m, "a")
	pickByID(&m, "schema_rename_type")
	m, _ = m.update(key("enter"))
	pickByID(&m, "project")
	m, _ = m.update(key("enter"))
	for _, r := range "item" {
		m, _ = m.update(runeKey(r))
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if m.overlay != overlayConfirm {
		t.Fatalf("rename preview %v", m.overlay)
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.last("schema_rename_type").Args["confirm"] != true {
		t.Fatalf("rename apply %+v", log.last("schema_rename_type").Args)
	}

	m, _ = pressLeader(m, "a")
	pickByID(&m, "schema_rename_field")
	m, _ = m.update(key("enter"))
	pickByID(&m, "status")
	m, _ = m.update(key("enter"))
	for _, r := range "state" {
		m, _ = m.update(runeKey(r))
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if m.overlay != overlayConfirm {
		t.Fatalf("rename field preview %v", m.overlay)
	}
	m, cmd = m.update(key("enter"))
	m = flushMut(t, m, cmd)
	gotRF := log.last("schema_rename_field")
	if gotRF.Args["old_field"] != "status" || gotRF.Args["new_field"] != "state" || gotRF.Args["confirm"] != true {
		t.Fatalf("rename field %+v", gotRF.Args)
	}
}

func TestNoteModeSetField(t *testing.T) {
	m, log := loadMutBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "hi\n"}})
	if m.screen != kindNote {
		t.Fatal("note")
	}
	m, _ = m.update(key("s"))
	if m.overlay != overlayPicker || m.pickerKind != pickerFields {
		t.Fatalf("note set overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	pickByID(&m, "status")
	m, _ = m.update(key("enter"))
	pickByID(&m, "idea")
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	if log.last("set").Args["reference"] != "projects/a" {
		t.Fatalf("set %+v", log.last("set").Args)
	}
}

func TestCaptureFromNoteDefaultsToNote(t *testing.T) {
	m, log := loadMutBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "hi\n"}})
	m, _ = pressLeader(m, "a")
	pickByID(&m, "capture")
	m, _ = m.update(key("enter"))
	for _, r := range "hello" {
		m, _ = m.update(runeKey(r))
	}
	m, cmd := m.update(key("enter"))
	m = flushMut(t, m, cmd)
	got := log.last("add")
	if got.Args["text"] != "hello" || got.Args["to"] != "projects/a" {
		t.Fatalf("add %+v", got.Args)
	}
}

func TestActionsMenuOpens(t *testing.T) {
	m := loadBrowse(t)
	m, _ = pressLeader(m, "a")
	if m.overlay != overlayPicker || m.pickerKind != pickerActions {
		t.Fatalf("overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
	if len(m.pickerHit) < 8 {
		t.Fatalf("actions %v", labelsOf(m.pickerHit))
	}
}

func TestCreatePrefillsBoardColumn(t *testing.T) {
	m := loadNamedView(t, namedViewCfg(), "projects")
	focusMain(&m)
	colKey := m.p().board[m.p().boardCol].Key
	m, _ = m.update(key("c"))
	if m.mut.fields["status"] != colKey && colKey != "" {
		t.Fatalf("prefill status=%v want %q", m.mut.fields["status"], colKey)
	}
}

func TestBareSOpensFieldPicker(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("s"))
	if m.overlay != overlayPicker || m.pickerKind != pickerFields {
		t.Fatalf("overlay=%v kind=%v", m.overlay, m.pickerKind)
	}
}
