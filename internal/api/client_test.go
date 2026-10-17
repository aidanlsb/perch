package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientHealthSchemaInvoke(t *testing.T) {
	token := "secret-token"
	var lastInvoke struct {
		Command string         `json:"command"`
		Args    map[string]any `json:"args"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			writeEnv(w, http.StatusUnauthorized, map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "missing or invalid bearer token",
				},
			})
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/health":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{"status": "ok", "api_version": "1"}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/session":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"api_version": "1", "vault_path": "/vault", "editor": "code", "editor_mode": "auto",
				"daily": map[string]any{
					"directory": "daily", "today_id": "2026-10-15",
					"today_path": "daily/2026-10-15.md", "today_exists": true,
				},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/schema":
			writeEnv(w, 200, map[string]any{
				"ok": true,
				"data": map[string]any{
					"version": 1,
					"types": map[string]any{
						"project": map[string]any{
							"name":       "project",
							"name_field": "name",
							"fields": map[string]any{
								"name":   map[string]any{"type": "string"},
								"status": map[string]any{"type": "enum", "values": []string{"idea", "active", "done"}},
								"due":    map[string]any{"type": "date"},
								"owner":  map[string]any{"type": "ref", "target": "person"},
							},
						},
					},
					"queries": map[string]any{
						"open": map[string]any{"name": "open", "query": "type:project .status==active"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoke":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &lastInvoke); err != nil {
				t.Errorf("invoke json: %v", err)
			}
			switch lastInvoke.Command {
			case "objects":
				writeEnv(w, 200, map[string]any{
					"ok": true,
					"data": map[string]any{
						"items": []any{map[string]any{
							"id": "people/freya", "type": "person",
							"name": "Freya", "file_path": "people/freya.md",
						}},
						"total": 1, "returned": 1, "offset": 0, "limit": 200, "has_more": false,
					},
				})
			case "query":
				writeEnv(w, 200, map[string]any{
					"ok": true,
					"data": map[string]any{
						"query_kind": "object",
						"type":       "project",
						"items": []any{
							map[string]any{
								"num": 1, "id": "projects/raven", "type": "project",
								"fields":    map[string]any{"name": "Raven", "status": "active"},
								"file_path": "projects/raven.md",
							},
						},
						"total": 1, "returned": 1, "offset": 0, "limit": 500, "has_more": false,
					},
					"meta": map[string]any{"vault_context": map[string]any{"path": "/vault", "source": "pinned"}},
				})
			case "read":
				content := "Hello [[people/freya]]\n"
				writeEnv(w, 200, map[string]any{
					"ok": true,
					"data": map[string]any{
						"object_id":  "projects/raven",
						"path":       "projects/raven.md",
						"content":    content,
						"line_count": 1,
						"references": []any{},
						"backlinks": []any{
							map[string]any{"source": "people/freya", "lines": []string{"see raven"}},
						},
						"links": []any{
							map[string]any{
								"byte_start": 6, "byte_end": 22,
								"utf16_start": 6, "utf16_end": 21,
								"text": "[[people/freya]]", "target": "people/freya",
								"status": "resolved", "object_id": "people/freya",
							},
						},
					},
				})
			case "search":
				writeEnv(w, 200, map[string]any{
					"ok": true,
					"data": map[string]any{
						"query": lastInvoke.Args["query"],
						"items": []any{
							map[string]any{"object_id": "projects/raven", "title": "Raven", "snippet": "pk", "rank": 1.0, "file_path": "projects/raven.md"},
						},
					},
				})
			case "query_saved_list":
				writeEnv(w, 200, map[string]any{
					"ok": true,
					"data": map[string]any{
						"queries": []any{
							map[string]any{"name": "open", "query": "type:project .status==active", "args": []any{}, "description": "active"},
						},
					},
				})
			default:
				writeEnv(w, 200, map[string]any{
					"ok":    false,
					"error": map[string]any{"code": "COMMAND_NOT_ALLOWED", "message": lastInvoke.Command},
				})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, token)
	ctx := context.Background()

	h, err := c.Health(ctx)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if h.Status != "ok" || h.APIVersion != "1" {
		t.Fatalf("health %+v", h)
	}

	sess, err := c.Session(ctx)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if sess.VaultPath != "/vault" || sess.Editor != "code" || sess.EditorMode != "auto" {
		t.Fatalf("session %+v", sess)
	}
	if sess.Daily == nil || sess.Daily.TodayID != "2026-10-15" || !sess.Daily.TodayExists {
		t.Fatalf("session daily %+v", sess.Daily)
	}

	sch, err := c.Schema(ctx)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	proj := sch.Types["project"]
	if !proj.Fields["status"].IsEnum() || !proj.Fields["due"].IsDate() || !proj.Fields["owner"].IsRef() {
		t.Fatalf("field kinds: %+v", proj.Fields)
	}
	if names := sch.SortedSavedQueries(); len(names) != 1 || names[0].Name != "open" {
		t.Fatalf("queries %+v", names)
	}

	qr, meta, err := c.Query(ctx, "type:project", 500)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if lastInvoke.Command != "query" || lastInvoke.Args["query_string"] != "type:project" {
		t.Fatalf("invoke args %+v", lastInvoke)
	}
	objs := ObjectsFromQuery(qr, proj)
	if len(objs) != 1 || objs[0].Title != "Raven" {
		t.Fatalf("objects %+v", objs)
	}
	if meta == nil || meta.VaultContext == nil || meta.VaultContext.Path != "/vault" {
		t.Fatalf("vault meta %+v", meta)
	}

	rd, _, err := c.Read(ctx, "projects/raven")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rd.Links) != 1 || !rd.Links[0].SpanOK(rd.Content) {
		t.Fatalf("links %+v content %q", rd.Links, rd.Content)
	}
	if rd.Links[0].FollowTarget() != "people/freya" {
		t.Fatalf("follow %q", rd.Links[0].FollowTarget())
	}

	sr, err := c.Search(ctx, "raven", 10)
	if err != nil || len(sr.Items) != 1 {
		t.Fatalf("search %v %+v", err, sr)
	}

	qs, err := c.SavedQueries(ctx)
	if err != nil || len(qs) != 1 {
		t.Fatalf("saved %v %+v", err, qs)
	}

	_, _, err = c.QueryInputs(ctx, "project-todos", 20, map[string]string{"project": "projects/raven"})
	if err != nil {
		t.Fatalf("query inputs: %v", err)
	}
	if lastInvoke.Command != "query" {
		t.Fatalf("command %s", lastInvoke.Command)
	}
	inputs, _ := lastInvoke.Args["inputs"].(map[string]any)
	if inputs["project"] != "projects/raven" {
		t.Fatalf("inputs %+v", lastInvoke.Args)
	}

	or, err := c.Objects(ctx, 200, 0)
	if err != nil || len(or.Items) != 1 || or.Items[0].ID != "people/freya" {
		t.Fatalf("objects %v %+v", err, or)
	}

	cBad := NewClient(srv.URL, "wrong")
	if _, err := cBad.Health(ctx); err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestSessionMissingEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/session" {
			http.NotFound(w, r)
			return
		}
		writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{"status": "ok"}})
	}))
	t.Cleanup(srv.Close)
	_, err := NewClient(srv.URL, "t").Session(context.Background())
	if err == nil || !IsNotFound(err) {
		t.Fatalf("got %v", err)
	}
}

func TestClientRefusesWrites(t *testing.T) {
	c := NewClient("http://127.0.0.1:9", "t")
	ctx := context.Background()
	for _, cmd := range []string{"set", "unset", "new", "move", "daily"} {
		if _, err := c.Invoke(ctx, cmd, map[string]any{"x": 1}); err == nil {
			t.Fatalf("expected refuse %s", cmd)
		} else if !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
}

func TestObjectsCommandNotAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, 200, map[string]any{
			"ok":    false,
			"error": map[string]any{"code": "COMMAND_NOT_ALLOWED", "message": "objects"},
		})
	}))
	t.Cleanup(srv.Close)
	_, err := NewClient(srv.URL, "t").Objects(context.Background(), 200, 0)
	if err == nil || !IsCommandNotAllowed(err) {
		t.Fatalf("got %v", err)
	}
}

func TestCommandFailureEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, 200, map[string]any{
			"ok": false,
			"error": map[string]any{
				"code":       "NOT_FOUND",
				"message":    "no such object",
				"suggestion": "check the id",
			},
		})
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "t")
	_, _, err := c.Read(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Fatalf("got %v", err)
	}
}

func writeEnv(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
