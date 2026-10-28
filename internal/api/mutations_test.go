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

func TestMutationClientCommands(t *testing.T) {
	var last struct {
		Command string         `json:"command"`
		Args    map[string]any `json:"args"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/invoke" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &last); err != nil {
			t.Errorf("invoke json: %v", err)
		}
		switch last.Command {
		case "set":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"file": "p.md", "object_id": "p", "type": "project",
				"updated_fields":  map[string]any{"status": "done"},
				"previous_fields": map[string]any{"status": "active"},
			}})
		case "unset":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"file": "p.md", "object_id": "p", "type": "project",
				"removed_fields": map[string]any{"due": "2026-01-01"},
				"missing_fields": []any{}, "modified": true,
			}})
		case "update":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"action": "update_traits",
				"items": []any{map[string]any{
					"id": "p.md:trait:1", "trait_type": "done",
					"old_value": "false", "new_value": "true", "status": "modified",
				}},
				"total": 1, "modified": 1,
			}})
		case "new":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"id": "projects/n", "file": "projects/n.md", "title": "N", "type": "project",
			}})
		case "daily":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"id": "2026-10-15", "file": "daily/2026-10-15.md", "date": "2026-10-15",
				"created": true, "opened": false,
			}})
		case "add":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"file": "inbox.md", "line": 14, "content": "hello",
			}})
		case "delete":
			dry, _ := last.Args["dry-run"].(bool)
			if dry {
				writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
					"preview": true, "object_id": "p", "behavior": "trash",
					"trash_dir": ".trash", "backlinks": []any{"a", "b"},
				}})
				return
			}
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"deleted": "p", "behavior": "trash", "trash_path": ".trash/p.md",
			}})
		case "move":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"source": "p", "destination": "q",
				"updated_refs": []any{"a"}, "updated_ref_fields": []any{},
			}})
		case "reclassify":
			force, _ := last.Args["force"].(bool)
			if !force {
				writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
					"object_id": "p", "old_type": "project", "new_type": "note",
					"file": "p.md", "moved": false, "dropped_fields": []any{"status"},
					"needs_confirm": true, "reason": "fields dropped",
				}})
				return
			}
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"object_id": "p", "old_type": "project", "new_type": "note",
				"file": "p.md", "moved": true, "dropped_fields": []any{"status"},
				"needs_confirm": false,
			}})
		case "query_saved_set":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"name": "open", "query": "type:project", "args": []any{}, "status": "created",
			}})
		case "schema_add_type":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{"name": last.Args["name"]}})
		case "schema_add_field":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{"field": last.Args["field_name"]}})
		case "schema_add_trait":
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{"name": last.Args["name"]}})
		case "schema_rename_type":
			confirm, _ := last.Args["confirm"].(bool)
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"preview": !confirm, "total_changes": 3, "files_to_move": 2,
			}})
		case "schema_rename_field":
			confirm, _ := last.Args["confirm"].(bool)
			writeEnv(w, 200, map[string]any{"ok": true, "data": map[string]any{
				"preview": !confirm, "total_changes": 4,
			}})
		default:
			writeEnv(w, 200, map[string]any{
				"ok": false, "error": map[string]any{"code": "NO", "message": last.Command},
			})
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "")
	ctx := context.Background()

	set, err := c.SetFields(ctx, "p", map[string]any{"status": "done"})
	if err != nil || set.ObjectID != "p" || set.UpdatedFields["status"] != "done" {
		t.Fatalf("set %v %+v", err, set)
	}
	if last.Command != "set" {
		t.Fatalf("cmd %s", last.Command)
	}
	fj, _ := last.Args["fields-json"].(map[string]any)
	if fj["status"] != "done" || last.Args["reference"] != "p" {
		t.Fatalf("set args %+v", last.Args)
	}

	un, err := c.UnsetFields(ctx, "p", []string{"due"})
	if err != nil || !un.Modified || un.ObjectID != "p" {
		t.Fatalf("unset %v %+v", err, un)
	}
	fields, _ := last.Args["fields"].([]any)
	if last.Command != "unset" || len(fields) != 1 || fields[0] != "due" {
		t.Fatalf("unset args %+v", last.Args)
	}

	up, err := c.UpdateTrait(ctx, "p.md:trait:1", "true")
	if err != nil || up.Modified != 1 || len(up.Items) != 1 || up.Items[0].NewValue != "true" {
		t.Fatalf("update %v %+v", err, up)
	}
	if last.Command != "update" || last.Args["trait_id"] != "p.md:trait:1" || last.Args["value"] != "true" {
		t.Fatalf("update args %+v", last.Args)
	}

	nw, err := c.NewObject(ctx, "project", "N", map[string]any{"status": "idea"})
	if err != nil || nw.ID != "projects/n" || nw.Type != "project" {
		t.Fatalf("new %v %+v", err, nw)
	}
	if last.Command != "new" || last.Args["type"] != "project" || last.Args["title"] != "N" {
		t.Fatalf("new args %+v", last.Args)
	}

	d, err := c.EnsureDaily(ctx, "2026-10-15")
	if err != nil || d.ID != "2026-10-15" || !d.Created {
		t.Fatalf("daily %v %+v", err, d)
	}
	if last.Command != "daily" || last.Args["date"] != "2026-10-15" {
		t.Fatalf("daily args %+v", last.Args)
	}

	add, err := c.AddCapture(ctx, "hello", "")
	if err != nil || add.Line != 14 {
		t.Fatalf("add %v %+v", err, add)
	}
	if last.Command != "add" || last.Args["text"] != "hello" {
		t.Fatalf("add args %+v", last.Args)
	}
	if _, ok := last.Args["to"]; ok {
		t.Fatalf("empty to should be omitted: %+v", last.Args)
	}
	_, err = c.AddCapture(ctx, "hello", "inbox")
	if err != nil || last.Args["to"] != "inbox" {
		t.Fatalf("add to %v %+v", err, last.Args)
	}

	prev, err := c.DeleteObject(ctx, "p", true)
	if err != nil || !prev.Preview || prev.BacklinkCount() != 2 {
		t.Fatalf("delete preview %v %+v", err, prev)
	}
	del, err := c.DeleteObject(ctx, "p", false)
	if err != nil || del.Deleted != "p" {
		t.Fatalf("delete %v %+v", err, del)
	}
	if last.Args["dry-run"] != false {
		t.Fatalf("delete apply args %+v", last.Args)
	}

	mv, err := c.MoveObject(ctx, "p", "q", false, false)
	if err != nil || mv.Destination != "q" || mv.UpdatedRefCount() != 1 {
		t.Fatalf("move %v %+v", err, mv)
	}
	if last.Args["update-refs"] != true || last.Args["skip-type-check"] != false {
		t.Fatalf("move args %+v", last.Args)
	}

	rc, err := c.ReclassifyObject(ctx, "p", "note", nil, false)
	if err != nil || !rc.NeedsConfirm || len(rc.DroppedFields) != 1 {
		t.Fatalf("reclassify preview %v %+v", err, rc)
	}
	rc, err = c.ReclassifyObject(ctx, "p", "note", map[string]any{"title": "x"}, true)
	if err != nil || rc.NeedsConfirm || !rc.Moved {
		t.Fatalf("reclassify %v %+v", err, rc)
	}

	sq, err := c.SaveQuery(ctx, "open", "type:project", nil, "")
	if err != nil || sq.Status != "created" {
		t.Fatalf("save query %v %+v", err, sq)
	}
	if last.Command != "query_saved_set" || last.Args["query_string"] != "type:project" {
		t.Fatalf("save args %+v", last.Args)
	}
	if _, ok := last.Args["arg"]; ok {
		t.Fatalf("empty arg should be omitted: %+v", last.Args)
	}

	if _, err := c.SchemaAddType(ctx, "task", "title", "tasks", ""); err != nil {
		t.Fatal(err)
	}
	if last.Command != "schema_add_type" || last.Args["name-field"] != "title" || last.Args["default-path"] != "tasks" {
		t.Fatalf("add type args %+v", last.Args)
	}

	if _, err := c.SchemaAddField(ctx, SchemaAddFieldArgs{
		TypeName: "task", FieldName: "state", Type: "enum", Required: true,
		Values: []string{"open", "done"},
	}); err != nil {
		t.Fatal(err)
	}
	if last.Command != "schema_add_field" || last.Args["required"] != true {
		t.Fatalf("add field args %+v", last.Args)
	}
	vals, _ := last.Args["values"].([]any)
	if len(vals) != 2 || vals[0] != "open" {
		t.Fatalf("values %+v", last.Args["values"])
	}

	if _, err := c.SchemaAddTrait(ctx, SchemaAddTraitArgs{Name: "done", Type: "bool"}); err != nil {
		t.Fatal(err)
	}
	if last.Command != "schema_add_trait" || last.Args["type"] != "bool" {
		t.Fatalf("add trait args %+v", last.Args)
	}

	pr, err := c.SchemaRenameType(ctx, "task", "item", false)
	if err != nil || !pr.Preview || pr.TotalChanges != 3 || pr.FilesToMove != 2 {
		t.Fatalf("rename type preview %v %+v", err, pr)
	}
	if last.Args["confirm"] != false || last.Args["rename-default-path"] != true {
		t.Fatalf("rename type args %+v", last.Args)
	}
	ap, err := c.SchemaRenameType(ctx, "task", "item", true)
	if err != nil || ap.Preview {
		t.Fatalf("rename type apply %v %+v", err, ap)
	}

	fp, err := c.SchemaRenameField(ctx, "item", "state", "status", false)
	if err != nil || !fp.Preview || fp.TotalChanges != 4 {
		t.Fatalf("rename field %v %+v", err, fp)
	}
}

func TestCoerceFieldValue(t *testing.T) {
	s, err := CoerceFieldValue("string", " hello ")
	if err != nil || s != "hello" {
		t.Fatalf("string %v %v", s, err)
	}
	n, err := CoerceFieldValue("number", "12")
	if err != nil || n != int64(12) {
		t.Fatalf("int %v %T %v", n, n, err)
	}
	f, err := CoerceFieldValue("number", "1.5")
	if err != nil || f != 1.5 {
		t.Fatalf("float %v %v", f, err)
	}
	b, err := CoerceFieldValue("bool", "true")
	if err != nil || b != true {
		t.Fatalf("bool %v %v", b, err)
	}
	arr, err := CoerceFieldValue("string[]", "a, b, c")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := arr.([]string)
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("array %v", got)
	}
	nums, err := CoerceFieldValue("number[]", "1, 2")
	if err != nil {
		t.Fatal(err)
	}
	nl, _ := nums.([]any)
	if len(nl) != 2 || nl[0] != int64(1) {
		t.Fatalf("number[] %v", nums)
	}
}

func TestJSONArrayLen(t *testing.T) {
	if JSONArrayLen(nil) != 0 || JSONArrayLen(json.RawMessage("null")) != 0 {
		t.Fatal("empty")
	}
	if JSONArrayLen(json.RawMessage(`["a","b"]`)) != 2 {
		t.Fatal("len")
	}
}
