package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/editor"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInitAgainstFakeAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"data": map[string]any{
					"api_version": "1", "vault_path": "/vault",
					"editor": "code", "editor_mode": "gui",
				},
			})
		case "/v1/schema":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"data": map[string]any{
					"version": 1,
					"types": map[string]any{
						"project": map[string]any{
							"name": "project", "name_field": "name",
							"fields": map[string]any{"name": map[string]any{"type": "string"}, "status": map[string]any{"type": "enum", "values": []string{"active"}}},
						},
					},
					"queries": map[string]any{},
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
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"queries": []any{}}})
			case "query":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"data": map[string]any{
						"query_kind": "object", "type": "project",
						"items": []any{map[string]any{
							"id": "projects/a", "type": "project",
							"fields":    map[string]any{"name": "Alpha", "status": "active"},
							"file_path": "projects/a.md",
						}},
						"total": 1, "returned": 1, "has_more": false,
					},
				})
			case "read":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"data": map[string]any{
						"object_id": "projects/a", "path": "projects/a.md",
						"content": "Alpha\n", "links": []any{}, "backlinks": []any{}, "references": []any{},
					},
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "NO", "message": req.Command}})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	m := NewModel(Options{Client: api.NewClient(srv.URL, "t")})
	m.width, m.height = 80, 24
	msg := m.Init()()
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if !m.ready {
		t.Fatalf("not ready: %+v %v", msg, m.errMsg)
	}
	if m.vaultPath != "/vault" || m.editor != "code" || editor.NormalizeMode(m.editorMode) != editor.ModeGUI {
		t.Fatalf("session apply vault=%q editor=%q mode=%q", m.vaultPath, m.editor, m.editorMode)
	}
	if cmd == nil {
		t.Fatal("expected query")
	}
	m, _ = m.update(cmd())
	if len(m.p().filtered) != 1 || m.p().filtered[0].Title != "Alpha" {
		t.Fatalf("%+v err=%s", m.p().filtered, m.errMsg)
	}
	focusMain(&m)
	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("read")
	}
	m, _ = m.update(cmd())
	if m.note == nil || m.note.ObjectID != "projects/a" {
		t.Fatalf("note %+v %s", m.note, m.errMsg)
	}
}
