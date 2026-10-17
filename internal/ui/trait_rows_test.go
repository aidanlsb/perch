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
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/editor"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTraitQueryRowsAgainstFakeAPI(t *testing.T) {
	var mu sync.Mutex
	var reads []string
	noteBody := traitNoteContent()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/session":
			writeOK(w, map[string]any{
				"api_version": "1", "vault_path": "/vault",
				"editor": "nvim", "editor_mode": "gui",
				"revision": map[string]any{"generation": "gen1", "counter": 1},
			})
		case "/v1/revision":
			writeOK(w, map[string]any{"generation": "gen1", "counter": 1})
		case "/v1/schema":
			writeOK(w, map[string]any{
				"version": 1,
				"types": map[string]any{
					"project": map[string]any{
						"name": "project", "name_field": "name",
						"fields": map[string]any{"name": map[string]any{"type": "string"}},
					},
				},
				"queries": map[string]any{
					"priorities": map[string]any{
						"name": "priorities", "query": "trait:priority",
					},
				},
			})
		case "/v1/invoke":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Command string         `json:"command"`
				Args    map[string]any `json:"args"`
			}
			_ = json.Unmarshal(body, &req)
			switch req.Command {
			case "query_saved_list":
				writeOK(w, map[string]any{
					"queries": []any{
						map[string]any{"name": "priorities", "query": "trait:priority"},
					},
				})
			case "objects":
				writeOK(w, map[string]any{
					"items": []any{}, "total": 0, "returned": 0, "has_more": false,
				})
			case "query":
				qs, _ := req.Args["query_string"].(string)
				if strings.Contains(qs, "projects/raven") && strings.Contains(qs, "type:") {
					writeOK(w, map[string]any{
						"query_kind": "type", "type": "project",
						"items": []any{map[string]any{
							"id": "projects/raven", "type": "project",
							"fields":    map[string]any{"name": "Raven"},
							"file_path": "projects/raven.md",
						}},
						"total": 1, "returned": 1, "has_more": false,
					})
					return
				}
				writeOK(w, map[string]any{
					"query_kind": "trait", "trait": "priority",
					"items": []any{map[string]any{
						"num": 1, "id": "projects/raven.md:trait:0",
						"trait_type": "priority", "value": "high",
						"content":   "Ship it",
						"file_path": "projects/raven.md",
						"line":      18,
						"object_id": "projects/raven",
					}},
					"total": 1, "returned": 1, "has_more": false,
				})
			case "read":
				ref, _ := req.Args["reference"].(string)
				mu.Lock()
				reads = append(reads, ref)
				mu.Unlock()
				if ref == "projects/raven.md:trait:0" {
					writeErr(w, "REF_NOT_FOUND", "reference 'projects/raven.md:trait:0' not found")
					return
				}
				if ref != "projects/raven" {
					writeErr(w, "REF_NOT_FOUND", "reference '"+ref+"' not found")
					return
				}
				writeOK(w, map[string]any{
					"object_id": "projects/raven", "path": "projects/raven.md",
					"content": noteBody, "line_count": strings.Count(noteBody, "\n"),
					"links": []any{}, "backlinks": []any{}, "references": []any{},
				})
			default:
				writeErr(w, "NO", req.Command)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.File{
		Views: map[string]config.View{
			"priorities": {Source: "priorities", Layout: config.Present{Kind: kindList}},
			"related": {
				Source: "type:project .id=={{selected}}",
				Layout: config.Present{Kind: kindList},
			},
		},
		Layouts: map[string]config.Layout{
			"workspace": {
				Split: "row",
				Panels: []config.Panel{
					{View: "priorities"},
					{View: "related", Bind: "priorities"},
				},
			},
		},
	}

	m := NewModel(Options{Client: api.NewClient(srv.URL, "t"), Config: cfg, ConfigPath: cfgPath, PollInterval: -1})
	m.width, m.height = 100, 40
	msg := m.Init()()
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if !m.ready {
		t.Fatalf("not ready: %s", m.errMsg)
	}
	if cmd != nil {
		m, _ = m.update(cmd())
	}

	m, cmd = m.loadNamedLayout("workspace")
	if cmd == nil {
		t.Fatal("expected layout queries")
	}
	m, _ = m.update(cmd())
	left := m.viewByID("priorities")
	if left == nil || len(left.filtered) != 1 {
		t.Fatalf("trait rows %+v err=%s", left, m.errMsg)
	}
	row := left.filtered[0]
	if row.ID != "projects/raven.md:trait:0" {
		t.Fatalf("row id %q", row.ID)
	}
	if row.OpenRef() != "projects/raven" {
		t.Fatalf("OpenRef %q", row.OpenRef())
	}
	if row.Line != 18 || row.FilePath != "projects/raven.md" {
		t.Fatalf("line/path %+v", row)
	}
	if _, ok := m.noteIndex["projects/raven.md:trait:0"]; ok {
		t.Fatal("trait ids must not enter the notes index")
	}

	related := m.viewByID("related")
	if related == nil {
		t.Fatal("related pane")
	}
	if !strings.Contains(related.sourceQuery, "projects/raven") {
		t.Fatalf("{{selected}} should bind the containing object, got %q", related.sourceQuery)
	}
	if strings.Contains(related.sourceQuery, "trait:0") {
		t.Fatalf("must not bind trait id: %s", related.sourceQuery)
	}

	// Enter on a driver pane moves focus; open/preview/e are tested on the view alone.
	m, cmd = m.loadNamedView("priorities")
	if cmd == nil {
		t.Fatal("expected trait view query")
	}
	m, _ = m.update(cmd())
	focusMain(&m)
	if got := m.selectedObject(); got == nil || got.OpenRef() != "projects/raven" {
		t.Fatalf("selected %+v", got)
	}

	path, objectID, line := m.editorTarget()
	if objectID != "projects/raven" {
		t.Fatalf("editor object %q", objectID)
	}
	if line != 18 {
		t.Fatalf("editor line %d", line)
	}
	if path != editor.FilePath("/vault", "projects/raven.md") {
		t.Fatalf("editor path %q", path)
	}

	m, cmd = m.update(key("p"))
	if !m.previewOn {
		t.Fatal("preview on")
	}
	if cmd == nil {
		t.Fatal("preview read")
	}
	m, _ = m.update(cmd())
	if m.errMsg != "" {
		t.Fatalf("preview err %s", m.errMsg)
	}
	if m.preview == nil || m.preview.ObjectID != "projects/raven" {
		t.Fatalf("preview %+v", m.preview)
	}
	if m.previewID != "projects/raven" {
		t.Fatalf("previewID %q", m.previewID)
	}
	if m.previewScroll <= 0 {
		t.Fatalf("preview should scroll to the trait line, got %d", m.previewScroll)
	}

	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("enter should read the containing object")
	}
	m, _ = m.update(cmd())
	if m.errMsg != "" {
		t.Fatalf("enter err %s", m.errMsg)
	}
	if m.note == nil || m.note.ObjectID != "projects/raven" {
		t.Fatalf("note %+v", m.note)
	}
	if m.noteScroll <= 0 {
		t.Fatalf("note should scroll to the trait line, got %d", m.noteScroll)
	}
	if m.currentNoteLine() != 18 {
		t.Fatalf("note frame line %d", m.currentNoteLine())
	}

	mu.Lock()
	gotReads := append([]string{}, reads...)
	mu.Unlock()
	for _, ref := range gotReads {
		if strings.Contains(ref, "trait:") {
			t.Fatalf("read used trait id %q", ref)
		}
	}
	if len(gotReads) == 0 {
		t.Fatal("expected read of the containing object")
	}

	m.screen = "browse"
	m.note = nil
	focusMain(&m)
	m, _ = m.update(key("S"))
	for _, r := range "from-traits" {
		m, _ = m.update(runeKey(r))
	}
	m, _ = m.update(key("enter"))
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(body)
	if !strings.Contains(saved, "[views.from-traits]") {
		t.Fatalf("missing view:\n%s", saved)
	}
	if !strings.Contains(saved, `source = "priorities"`) {
		t.Fatalf("save view should keep the query source, not a trait id:\n%s", saved)
	}
	if strings.Contains(saved, "trait:0") {
		t.Fatalf("save view wrote a trait id:\n%s", saved)
	}
}

func TestTraitEditorTargetFromInjectedRow(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m.vaultPath = "/vault"
	m.p().objects = []api.Object{{
		ID:       "notes/x.md:trait:0",
		ObjectID: "notes/x#body",
		Kind:     "trait",
		Title:    "do the thing",
		FilePath: "notes/x.md",
		Line:     9,
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	path, objectID, line := m.editorTarget()
	if objectID != "notes/x" {
		t.Fatalf("OpenRef/file object %q", objectID)
	}
	if line != 9 {
		t.Fatalf("line %d", line)
	}
	if path != editor.FilePath("/vault", "notes/x.md") {
		t.Fatalf("path %q", path)
	}
	if m.p().selectedID() != "notes/x" {
		t.Fatalf("bind id %q", m.p().selectedID())
	}
}

func traitNoteContent() string {
	var b strings.Builder
	b.WriteString("---\nname: Raven\n---\n\n")
	for i := 0; i < 12; i++ {
		b.WriteString("paragraph\n\n")
	}
	b.WriteString("Ship it @priority(high).\n")
	return b.String()
}

func writeOK(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func writeErr(w http.ResponseWriter, code, message string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": false, "error": map[string]any{"code": code, "message": message},
	})
}
