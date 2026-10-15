package api

import "encoding/json"

// Envelope is the Raven command/HTTP result shape:
// { ok, data, error?, warnings?, meta? }.
type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *ErrorInfo      `json:"error,omitempty"`
	Warnings []Warning       `json:"warnings,omitempty"`
	Meta     *Meta           `json:"meta,omitempty"`
}

// ErrorInfo is the structured error object inside a failed envelope.
type ErrorInfo struct {
	Code       string          `json:"code"`
	Message    string          `json:"message"`
	Details    json.RawMessage `json:"details,omitempty"`
	Suggestion string          `json:"suggestion,omitempty"`
}

func (e *ErrorInfo) Error() string {
	if e == nil {
		return "raven api error"
	}
	if e.Suggestion != "" {
		return e.Code + ": " + e.Message + " (" + e.Suggestion + ")"
	}
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

// Warning is a non-fatal Raven warning.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Ref     string `json:"ref,omitempty"`
}

// Meta is envelope metadata. VaultContext.Path is the pinned vault path.
type Meta struct {
	Count        int           `json:"count,omitempty"`
	QueryTimeMs  int64         `json:"query_time_ms,omitempty"`
	VaultContext *VaultContext `json:"vault_context,omitempty"`
}

// VaultContext identifies the vault used for an invoke.
type VaultContext struct {
	Name   string `json:"name,omitempty"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// Health is GET /v1/health data.
type Health struct {
	Status     string `json:"status"`
	APIVersion string `json:"api_version"`
}

// Session is GET /v1/session data: vault identity and rvn-open editor settings.
type Session struct {
	APIVersion string `json:"api_version"`
	VaultPath  string `json:"vault_path"`
	Editor     string `json:"editor"`
	EditorMode string `json:"editor_mode"`
}

// DateResult is the read-only `date` command payload (today's hub).
// Daily notes are not created; DailyExists is false when the file is missing.
type DateResult struct {
	Date        string          `json:"date"`
	DayOfWeek   string          `json:"day_of_week"`
	DailyNoteID string          `json:"daily_note_id"`
	DailyPath   string          `json:"daily_path"`
	DailyExists bool            `json:"daily_exists"`
	DailyNote   json.RawMessage `json:"daily_note,omitempty"`
	Items       json.RawMessage `json:"items,omitempty"`
	Backlinks   []BacklinkGroup `json:"backlinks,omitempty"`
}

// Commands is GET /v1/commands data.
type Commands struct {
	APIVersion string        `json:"api_version"`
	Commands   []CommandInfo `json:"commands"`
}

// CommandInfo is one allowlisted command description.
type CommandInfo struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
}

// Schema is GET /v1/schema / `schema` command data.
type Schema struct {
	Version int                    `json:"version"`
	Types   map[string]TypeSchema  `json:"types"`
	Traits  map[string]TraitSchema `json:"traits"`
	Queries map[string]SavedQuery  `json:"queries"`
}

// TypeSchema is one vault type, including builtin page/date/section.
type TypeSchema struct {
	Name        string                 `json:"name"`
	Builtin     bool                   `json:"builtin"`
	DefaultPath string                 `json:"default_path,omitempty"`
	Description string                 `json:"description,omitempty"`
	NameField   string                 `json:"name_field,omitempty"`
	Fields      map[string]FieldSchema `json:"fields,omitempty"`
}

// FieldSchema is one type field. Type is a Raven field spec (enum, ref, date, ref[], …).
type FieldSchema struct {
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Values      []string `json:"values,omitempty"`
	Target      string   `json:"target,omitempty"`
	Description string   `json:"description,omitempty"`
}

// TraitSchema is one trait definition.
type TraitSchema struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Values  []string `json:"values,omitempty"`
	Default string   `json:"default,omitempty"`
}

// SavedQuery is a named RQL definition from raven.yaml.
type SavedQuery struct {
	Name        string   `json:"name"`
	Query       string   `json:"query"`
	Args        []string `json:"args,omitempty"`
	Description string   `json:"description,omitempty"`
}

// QueryResult is a `query` success payload (object, trait, section, or link).
type QueryResult struct {
	QueryKind  string           `json:"query_kind"`
	Type       string           `json:"type,omitempty"`
	Trait      string           `json:"trait,omitempty"`
	SavedQuery string           `json:"saved_query,omitempty"`
	Items      []map[string]any `json:"items"`
	Total      int              `json:"total"`
	Returned   int              `json:"returned"`
	Offset     int              `json:"offset"`
	Limit      int              `json:"limit"`
	HasMore    bool             `json:"has_more"`
	NextOffset *int             `json:"next_offset,omitempty"`
}

// Object is a flattened object-query row used by list/board/date views.
type Object struct {
	ID       string
	Type     string
	Title    string
	Fields   map[string]any
	FilePath string
	Kind     string // object, trait, section, link
}

// ReadResult is an enriched `read` payload (not --raw).
type ReadResult struct {
	ObjectID   string          `json:"object_id"`
	Path       string          `json:"path"`
	Content    string          `json:"content"`
	LineCount  int             `json:"line_count"`
	References []ReadReference `json:"references"`
	Backlinks  []BacklinkGroup `json:"backlinks"`
	Links      []ReadLink      `json:"links"`
}

// ReadReference is a rendered reference in the body (human-output leftover).
type ReadReference struct {
	Text string  `json:"text"`
	Path *string `json:"path,omitempty"`
}

// BacklinkGroup is incoming references from one source object.
type BacklinkGroup struct {
	Source string   `json:"source"`
	Lines  []string `json:"lines"`
}

// ReadLink is a wikilink span in data.content. Offsets are 0-based, end-exclusive.
// Use ByteStart/ByteEnd to slice the Go string; do not parse Markdown for links.
type ReadLink struct {
	ByteStart  int      `json:"byte_start"`
	ByteEnd    int      `json:"byte_end"`
	UTF16Start int      `json:"utf16_start"`
	UTF16End   int      `json:"utf16_end"`
	Text       string   `json:"text"`
	Target     string   `json:"target"`
	Status     string   `json:"status"`
	ObjectID   string   `json:"object_id,omitempty"`
	Matches    []string `json:"matches,omitempty"`
}

const (
	LinkResolved   = "resolved"
	LinkUnresolved = "unresolved"
	LinkAmbiguous  = "ambiguous"
)

// SearchResult is the `search` success payload.
type SearchResult struct {
	Query string        `json:"query"`
	Items []SearchMatch `json:"items"`
}

// SearchMatch is one ranked search hit.
type SearchMatch struct {
	ObjectID     string  `json:"object_id"`
	Title        string  `json:"title"`
	FilePath     string  `json:"file_path"`
	Snippet      string  `json:"snippet"`
	Rank         float64 `json:"rank"`
	IsSection    bool    `json:"is_section,omitempty"`
	FileObjectID string  `json:"file_object_id,omitempty"`
}

// Startup is the rvn api --json listen envelope's data object.
type Startup struct {
	URL         string   `json:"url"`
	Token       string   `json:"token"`
	Port        int      `json:"port"`
	Bind        string   `json:"bind"`
	APIVersion  string   `json:"api_version"`
	VaultPath   string   `json:"vault_path"`
	CORSOrigins []string `json:"cors_origins,omitempty"`
}
