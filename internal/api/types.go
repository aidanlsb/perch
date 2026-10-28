package api

import (
	"encoding/json"
	"strings"
)

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
	APIVersion string     `json:"api_version"`
	VaultPath  string     `json:"vault_path"`
	VaultName  string     `json:"vault_name,omitempty"`
	Name       string     `json:"name,omitempty"`
	Editor     string     `json:"editor"`
	EditorMode string     `json:"editor_mode"`
	Daily      *DailyInfo `json:"daily,omitempty"`
	Revision   *Revision  `json:"revision,omitempty"`
}

// DisplayName is the vault's human name from session, else the path's last segment.
func (s Session) DisplayName() string {
	if n := strings.TrimSpace(s.VaultName); n != "" {
		return n
	}
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	p := strings.TrimRight(strings.TrimSpace(s.VaultPath), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}

// Revision is GET /v1/revision (and session.revision): a disposable index identity.
// Generation is random and minted when the index is built from scratch. Counter
// is monotonic and rises with every index projection. Older rvn omits both.
type Revision struct {
	Generation string `json:"generation"`
	Counter    int64  `json:"counter"`
}

// DailyInfo is the read-only today snapshot on GET /v1/session (Raven PR 308).
// Session never creates the file. Older rvn omits this field.
type DailyInfo struct {
	Directory   string `json:"directory"`
	TodayID     string `json:"today_id"`
	TodayPath   string `json:"today_path"`
	TodayExists bool   `json:"today_exists"`
}

// DateResult is the read-only `date` hub payload (never creates a daily note).
type DateResult struct {
	Date        string `json:"date"`
	DayOfWeek   string `json:"day_of_week"`
	DailyNoteID string `json:"daily_note_id"`
	DailyPath   string `json:"daily_path"`
	DailyExists bool   `json:"daily_exists"`
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

// TypeSchema is one vault type from GET /v1/schema.
type TypeSchema struct {
	Name        string                 `json:"name"`
	Builtin     bool                   `json:"builtin"`
	DefaultPath string                 `json:"default_path,omitempty"`
	Description string                 `json:"description,omitempty"`
	NameField   string                 `json:"name_field,omitempty"`
	Fields      map[string]FieldSchema `json:"fields,omitempty"`
	// FieldOrder is the schema's field definition order (JSON object key
	// order from GET /v1/schema). Empty when the value was not unmarshaled
	// from JSON; callers then fall back to a deterministic alphabetical order.
	FieldOrder []string `json:"-"`
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

// ObjectSummary is one row from the `objects` corpus (no body, no field map).
type ObjectSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
}

// ObjectsResult is the paginated `objects` success payload.
type ObjectsResult struct {
	Items      []ObjectSummary `json:"items"`
	Total      int             `json:"total"`
	Returned   int             `json:"returned"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
	HasMore    bool            `json:"has_more"`
	NextOffset *int            `json:"next_offset,omitempty"`
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

// Object is a flattened query row used by list/board/date views.
// Nested kinds (trait, section, link) keep their own ID and carry the
// containing file object separately.
type Object struct {
	ID       string
	Type     string
	Title    string
	Fields   map[string]any
	FilePath string
	Kind     string // object, type, trait, section, link
	Line     int    // 1-based source line when the payload includes one
	ObjectID string // containing object for nested rows; empty for file objects
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
// Raven's `read` payload varies: a source id plus optional line numbers,
// snippet/context text, and `lines` as strings or richer objects.
type BacklinkGroup struct {
	Source  string   `json:"source"`
	Lines   []string `json:"lines"`
	Line    int      `json:"line,omitempty"`
	Snippet string   `json:"snippet,omitempty"`
	Context string   `json:"context,omitempty"`
	Text    string   `json:"text,omitempty"`
	Title   string   `json:"title,omitempty"`
}

// UnmarshalJSON accepts the documented {source, lines} shape plus extra
// context fields (`line`, `snippet`, `title`, object-valued `lines`).
func (g *BacklinkGroup) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	g.Source = firstString(raw, "source", "object_id", "id")
	g.Title = firstString(raw, "title", "name", "display")
	g.Snippet = firstString(raw, "snippet", "context", "text", "display")
	g.Context = firstString(raw, "context")
	g.Text = firstString(raw, "text")
	g.Line = intFromAny(raw["line"])
	if g.Line == 0 {
		g.Line = intFromAny(raw["line_start"])
	}
	g.Lines = backlinkLines(raw["lines"])
	if raw["hits"] != nil {
		parseBacklinkHits(g, raw["hits"])
	} else if raw["references"] != nil {
		parseBacklinkHits(g, raw["references"])
	}
	return nil
}

func backlinkLines(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			switch row := item.(type) {
			case string:
				if s := strings.TrimSpace(row); s != "" {
					out = append(out, s)
				}
			case float64, int, int32, int64:
				if n := intFromAny(row); n > 0 {
					out = append(out, strings.TrimSpace(ScalarString(row)))
				}
			case map[string]any:
				s := firstString(row, "text", "snippet", "context", "line")
				if s == "" {
					if n := intFromAny(row["line"]); n > 0 {
						s = ScalarString(n)
					}
				}
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		if s := strings.TrimSpace(ScalarString(v)); s != "" {
			return []string{s}
		}
		return nil
	}
}

func parseBacklinkHits(g *BacklinkGroup, v any) {
	arr, ok := v.([]any)
	if !ok {
		return
	}
	for _, item := range arr {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if g.Line == 0 {
			g.Line = intFromAny(row["line"])
			if g.Line == 0 {
				g.Line = intFromAny(row["line_start"])
			}
		}
		if g.Snippet == "" {
			g.Snippet = firstString(row, "snippet", "context", "text")
		}
		if s := firstString(row, "text", "snippet", "context"); s != "" {
			g.Lines = append(g.Lines, s)
		}
	}
}

// DisplayTitle is a human label for the backlink source (title, else id).
func (g BacklinkGroup) DisplayTitle() string {
	if t := strings.TrimSpace(g.Title); t != "" {
		return t
	}
	return strings.TrimSpace(g.Source)
}

// SnippetText is the best available context line for the backlink list.
func (g BacklinkGroup) SnippetText() string {
	for _, s := range []string{g.Snippet, g.Context, g.Text} {
		if t := firstLine(strings.TrimSpace(s)); t != "" {
			return t
		}
	}
	for _, line := range g.Lines {
		if t := firstLine(stripLineNumberPrefix(line)); t != "" {
			return t
		}
	}
	return ""
}

// SourceLine is a 1-based source line when the payload includes one.
func (g BacklinkGroup) SourceLine() int {
	if g.Line > 0 {
		return g.Line
	}
	for _, line := range g.Lines {
		if n, _ := parseLineNumberPrefix(line); n > 0 {
			return n
		}
	}
	return 0
}

func stripLineNumberPrefix(s string) string {
	_, rest := parseLineNumberPrefix(s)
	if rest != "" {
		return rest
	}
	return strings.TrimSpace(s)
}

func parseLineNumberPrefix(s string) (int, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i == len(s) {
		if i == len(s) {
			n := intFromAny(s)
			if n > 0 {
				return n, ""
			}
		}
		return 0, s
	}
	switch s[i] {
	case ':', '|', ' ':
		n := intFromAny(s[:i])
		rest := strings.TrimSpace(s[i+1:])
		if n > 0 {
			return n, rest
		}
	}
	return 0, s
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
