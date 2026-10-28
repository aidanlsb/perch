package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// SetResult is the `set` success payload.
type SetResult struct {
	File           string         `json:"file"`
	ObjectID       string         `json:"object_id"`
	Type           string         `json:"type"`
	UpdatedFields  map[string]any `json:"updated_fields"`
	PreviousFields map[string]any `json:"previous_fields"`
}

// UnsetResult is the `unset` success payload.
type UnsetResult struct {
	File          string         `json:"file"`
	ObjectID      string         `json:"object_id"`
	Type          string         `json:"type"`
	RemovedFields map[string]any `json:"removed_fields"`
	MissingFields []string       `json:"missing_fields"`
	Modified      bool           `json:"modified"`
}

// UpdateTraitResult is the `update` success payload.
type UpdateTraitResult struct {
	Action   string            `json:"action"`
	Items    []UpdateTraitItem `json:"items"`
	Total    int               `json:"total"`
	Modified int               `json:"modified"`
}

// UpdateTraitItem is one trait row in an `update` payload.
type UpdateTraitItem struct {
	ID        string `json:"id"`
	TraitType string `json:"trait_type"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
	Status    string `json:"status"`
}

// NewObjectResult is the `new` success payload.
type NewObjectResult struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// DailyNoteResult is the `daily` success payload (resolve or create).
type DailyNoteResult struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Date    string `json:"date"`
	Created bool   `json:"created"`
	Opened  bool   `json:"opened"`
}

// AddResult is the `add` (quick capture) success payload.
type AddResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// DeleteResult is `delete` preview or applied payload.
type DeleteResult struct {
	Preview   bool            `json:"preview"`
	ObjectID  string          `json:"object_id"`
	Behavior  string          `json:"behavior"`
	TrashDir  string          `json:"trash_dir"`
	TrashPath string          `json:"trash_path"`
	Deleted   string          `json:"deleted"`
	Backlinks json.RawMessage `json:"backlinks"`
}

// BacklinkCount is the number of preview backlinks when the payload is an array.
func (d DeleteResult) BacklinkCount() int {
	return JSONArrayLen(d.Backlinks)
}

// MoveResult is `move` preview, confirm, or applied payload.
type MoveResult struct {
	Source           string          `json:"source"`
	Destination      string          `json:"destination"`
	UpdatedRefs      json.RawMessage `json:"updated_refs"`
	UpdatedRefFields json.RawMessage `json:"updated_ref_fields"`
	NeedsConfirm     bool            `json:"needs_confirm"`
}

// UpdatedRefCount is len(updated_refs) when that field is an array.
func (r MoveResult) UpdatedRefCount() int {
	return JSONArrayLen(r.UpdatedRefs)
}

// ReclassifyResult is the `reclassify` payload (including needs_confirm).
type ReclassifyResult struct {
	ObjectID      string   `json:"object_id"`
	OldType       string   `json:"old_type"`
	NewType       string   `json:"new_type"`
	File          string   `json:"file"`
	Moved         bool     `json:"moved"`
	DroppedFields []string `json:"dropped_fields"`
	NeedsConfirm  bool     `json:"needs_confirm"`
	Reason        string   `json:"reason"`
}

// SaveQueryResult is the `query_saved_set` payload.
type SaveQueryResult struct {
	Name   string   `json:"name"`
	Query  string   `json:"query"`
	Args   []string `json:"args"`
	Status string   `json:"status"`
}

// SchemaRenameResult is a `schema_rename_type` / `schema_rename_field` preview or apply payload.
type SchemaRenameResult struct {
	Preview      bool `json:"preview"`
	TotalChanges int  `json:"total_changes"`
	FilesToMove  int  `json:"files_to_move"`
}

// SchemaAddFieldArgs is the typed input for schema_add_field.
type SchemaAddFieldArgs struct {
	TypeName    string
	FieldName   string
	Type        string
	Required    bool
	Default     string
	Target      string
	Values      []string
	Description string
}

// SchemaAddTraitArgs is the typed input for schema_add_trait.
type SchemaAddTraitArgs struct {
	Name    string
	Type    string
	Values  []string
	Default string
}

// JSONArrayLen returns the length of a JSON array, or 0 if data is not an array.
func JSONArrayLen(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0
	}
	return len(arr)
}

// SetFields invokes `set` with fields-json.
func (c *Client) SetFields(ctx context.Context, ref string, fields map[string]any) (*SetResult, error) {
	var out SetResult
	err := c.invokeDecode(ctx, "set", map[string]any{
		"reference":   ref,
		"fields-json": fields,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UnsetFields invokes `unset`.
func (c *Client) UnsetFields(ctx context.Context, ref string, fields []string) (*UnsetResult, error) {
	var out UnsetResult
	err := c.invokeDecode(ctx, "unset", map[string]any{
		"reference": ref,
		"fields":    fields,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTrait invokes `update` for a trait row id (`file.md:trait:N`).
func (c *Client) UpdateTrait(ctx context.Context, traitID, value string) (*UpdateTraitResult, error) {
	var out UpdateTraitResult
	err := c.invokeDecode(ctx, "update", map[string]any{
		"trait_id": traitID,
		"value":    value,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// NewObject invokes `new`.
func (c *Client) NewObject(ctx context.Context, typeName, title string, fields map[string]any) (*NewObjectResult, error) {
	args := map[string]any{
		"type":  typeName,
		"title": title,
	}
	if len(fields) > 0 {
		args["fields-json"] = fields
	}
	var out NewObjectResult
	if err := c.invokeDecode(ctx, "new", args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnsureDaily invokes `daily` to resolve or create a daily note.
func (c *Client) EnsureDaily(ctx context.Context, date string) (*DailyNoteResult, error) {
	args := map[string]any{}
	if strings.TrimSpace(date) != "" {
		args["date"] = strings.TrimSpace(date)
	}
	var out DailyNoteResult
	if err := c.invokeDecode(ctx, "daily", args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddCapture invokes `add`. Empty `to` omits the destination (Raven default).
func (c *Client) AddCapture(ctx context.Context, text, to string) (*AddResult, error) {
	args := map[string]any{"text": text}
	if strings.TrimSpace(to) != "" {
		args["to"] = strings.TrimSpace(to)
	}
	var out AddResult
	if err := c.invokeDecode(ctx, "add", args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteObject invokes `delete`. dryRun=true returns a preview.
func (c *Client) DeleteObject(ctx context.Context, ref string, dryRun bool) (*DeleteResult, error) {
	var out DeleteResult
	err := c.invokeDecode(ctx, "delete", map[string]any{
		"reference": ref,
		"dry-run":   dryRun,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// MoveObject invokes `move` with update-refs=true.
func (c *Client) MoveObject(ctx context.Context, source, destination string, dryRun, skipTypeCheck bool) (*MoveResult, error) {
	var out MoveResult
	err := c.invokeDecode(ctx, "move", map[string]any{
		"source":          source,
		"destination":     destination,
		"update-refs":     true,
		"dry-run":         dryRun,
		"skip-type-check": skipTypeCheck,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ReclassifyObject invokes `reclassify` with update-refs=true.
func (c *Client) ReclassifyObject(ctx context.Context, ref, newType string, fields map[string]any, force bool) (*ReclassifyResult, error) {
	args := map[string]any{
		"reference":   ref,
		"new-type":    newType,
		"update-refs": true,
		"force":       force,
	}
	if len(fields) > 0 {
		args["fields-json"] = fields
	}
	var out ReclassifyResult
	if err := c.invokeDecode(ctx, "reclassify", args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveQuery invokes `query_saved_set`.
func (c *Client) SaveQuery(ctx context.Context, name, queryString string, args []string, desc string) (*SaveQueryResult, error) {
	payload := map[string]any{
		"name":         name,
		"query_string": queryString,
	}
	if len(args) > 0 {
		payload["arg"] = args
	}
	if strings.TrimSpace(desc) != "" {
		payload["description"] = desc
	}
	var out SaveQueryResult
	if err := c.invokeDecode(ctx, "query_saved_set", payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SchemaAddType invokes `schema_add_type`.
func (c *Client) SchemaAddType(ctx context.Context, name, nameField, defaultPath, description string) (map[string]any, error) {
	args := map[string]any{"name": name}
	if strings.TrimSpace(nameField) != "" {
		args["name-field"] = strings.TrimSpace(nameField)
	}
	if strings.TrimSpace(defaultPath) != "" {
		args["default-path"] = strings.TrimSpace(defaultPath)
	}
	if strings.TrimSpace(description) != "" {
		args["description"] = strings.TrimSpace(description)
	}
	return c.invokeMap(ctx, "schema_add_type", args)
}

// SchemaAddField invokes `schema_add_field`.
func (c *Client) SchemaAddField(ctx context.Context, in SchemaAddFieldArgs) (map[string]any, error) {
	args := map[string]any{
		"type_name":  in.TypeName,
		"field_name": in.FieldName,
		"type":       in.Type,
		"required":   in.Required,
	}
	if strings.TrimSpace(in.Default) != "" {
		args["default"] = strings.TrimSpace(in.Default)
	}
	if strings.TrimSpace(in.Target) != "" {
		args["target"] = strings.TrimSpace(in.Target)
	}
	if len(in.Values) > 0 {
		args["values"] = in.Values
	}
	if strings.TrimSpace(in.Description) != "" {
		args["description"] = strings.TrimSpace(in.Description)
	}
	return c.invokeMap(ctx, "schema_add_field", args)
}

// SchemaAddTrait invokes `schema_add_trait`.
func (c *Client) SchemaAddTrait(ctx context.Context, in SchemaAddTraitArgs) (map[string]any, error) {
	args := map[string]any{
		"name": in.Name,
		"type": in.Type,
	}
	if len(in.Values) > 0 {
		args["values"] = in.Values
	}
	if strings.TrimSpace(in.Default) != "" {
		args["default"] = strings.TrimSpace(in.Default)
	}
	return c.invokeMap(ctx, "schema_add_trait", args)
}

// SchemaRenameType invokes `schema_rename_type`. confirm=false is a preview.
func (c *Client) SchemaRenameType(ctx context.Context, oldName, newName string, confirm bool) (*SchemaRenameResult, error) {
	var out SchemaRenameResult
	err := c.invokeDecode(ctx, "schema_rename_type", map[string]any{
		"old_name":            oldName,
		"new_name":            newName,
		"rename-default-path": true,
		"confirm":             confirm,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SchemaRenameField invokes `schema_rename_field`. confirm=false is a preview.
func (c *Client) SchemaRenameField(ctx context.Context, typeName, oldField, newField string, confirm bool) (*SchemaRenameResult, error) {
	var out SchemaRenameResult
	err := c.invokeDecode(ctx, "schema_rename_field", map[string]any{
		"type_name": typeName,
		"old_field": oldField,
		"new_field": newField,
		"confirm":   confirm,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) invokeDecode(ctx context.Context, command string, args map[string]any, dest any) error {
	env, err := c.Invoke(ctx, command, args)
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
		return fmt.Errorf("decode %s: %w", command, err)
	}
	return nil
}

func (c *Client) invokeMap(ctx context.Context, command string, args map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.invokeDecode(ctx, command, args, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// CoerceFieldValue converts a text widget value into a fields-json value.
// Array types (`string[]`, `enum[]`, `ref[]`, …) split on commas.
func CoerceFieldValue(typeSpec, raw string) (any, error) {
	base, array := ParseFieldType(typeSpec)
	raw = strings.TrimSpace(raw)
	if array {
		parts := splitComma(raw)
		switch strings.ToLower(base) {
		case "number":
			out := make([]any, 0, len(parts))
			for _, p := range parts {
				n, err := parseNumber(p)
				if err != nil {
					return nil, err
				}
				out = append(out, n)
			}
			return out, nil
		case "bool", "boolean":
			out := make([]any, 0, len(parts))
			for _, p := range parts {
				b, err := parseBool(p)
				if err != nil {
					return nil, err
				}
				out = append(out, b)
			}
			return out, nil
		default:
			return parts, nil
		}
	}
	switch strings.ToLower(base) {
	case "number":
		return parseNumber(raw)
	case "bool", "boolean":
		return parseBool(raw)
	default:
		return raw, nil
	}
}

func splitComma(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseNumber(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty number")
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", s)
	}
	return f, nil
}

func parseBool(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("invalid bool %q", s)
	}
	return b, nil
}
