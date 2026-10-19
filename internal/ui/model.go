package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	queryLimit  = 500
	previewMinW = 28

	kindList  = config.KindList
	kindBoard = config.KindBoard
	kindDate  = config.KindDate
	kindNote  = "note"
)

type pane int

const (
	paneMain pane = iota
	panePreview
	paneBacklinks
)

type overlay int

const (
	overlayNone overlay = iota
	overlaySearch
	overlayPicker
	overlayHelp
	overlayDate
	overlayRQL
	overlayArgs
	overlaySave
)

type pickerKind int

const (
	pickerTypes pickerKind = iota
	pickerQueries
	pickerNotes
	pickerViews
	pickerSearch
)

type noteFrame struct {
	ID   string
	Line int
}

type queryCacheEntry struct {
	objects  []api.Object
	typeName string
	hasMore  bool
	total    int
}

// Options configure the TUI at start.
type Options struct {
	Client     *api.Client
	Config     *config.File
	ConfigPath string
	VaultPath  string
	Editor     string
	EditorMode string
	Open       string
	// PollInterval is how often to GET /v1/revision. Zero means 1s.
	// Negative disables polling (tests).
	PollInterval time.Duration
}

// Model is the Bubble Tea application state. Update is a pure-ish reducer
// so tests can drive it without a program.
type Model struct {
	client     *api.Client
	cfg        *config.File
	cfgPath    string
	vaultPath  string
	editor     string
	editorMode string
	queryLimit int

	width  int
	height int

	pane    pane
	overlay overlay
	screen  string // browse | note
	pending string // g | ctrlw | [ | ] | (empty)

	schema api.Schema

	tree          layoutTree
	panes         []*viewState
	focus         string
	lastMainFocus string
	zoomed        bool
	regs          []region
	currentView   string
	currentLayout string
	emptyPane     *viewState
	cfgWarnings   []string
	dateCur       string // YYYY-MM-DD in the daily picker
	helpOff       int

	searchQ    string
	searchHits []int
	searchAt   int

	note       *api.ReadResult
	noteDoc    frontmatter.Doc
	noteBody   string
	noteScroll int
	linkIdx    int
	backIdx    int
	noteStack  []noteFrame
	notePtr    int

	previewOn      bool
	previewID      string
	previewLine    int
	preview        *api.ReadResult
	previewDoc     frontmatter.Doc
	previewBody    string
	previewScroll  int
	previewWidth   int
	previewLoading bool

	pickerKind pickerKind
	pickerQ    string
	pickerAll  []pickItem
	pickerHit  []pickItem
	pickerCur  int
	rql        string
	saveName   string

	argsQuery *api.SavedQuery
	argsNames []string
	argsIdx   int
	argsVals  map[string]string
	argsBuf   string

	daily *api.DailyInfo

	queryCache map[string]queryCacheEntry
	readCache  map[string]*api.ReadResult
	noteIndex  map[string]api.Object

	needPrefetch  bool
	corpusDone    bool
	corpusLoading bool

	status  string
	errMsg  string
	liveErr string
	loading bool
	gen     int
	ready   bool
	open    string

	pollInterval  time.Duration
	revGeneration string
	revCounter    int64
	pollFails     int
	refreshSeq    int
}

// NewModel constructs an empty UI. Call Init to fetch the schema.
func NewModel(opts Options) Model {
	cfg := opts.Config
	if cfg == nil {
		cfg = &config.File{}
	}
	poll := opts.PollInterval
	switch {
	case opts.PollInterval < 0:
		poll = 0
	case opts.PollInterval == 0:
		poll = defaultPollInterval
	}
	return Model{
		client:       opts.Client,
		cfg:          cfg,
		cfgPath:      opts.ConfigPath,
		vaultPath:    opts.VaultPath,
		editor:       opts.Editor,
		editorMode:   opts.EditorMode,
		queryLimit:   queryLimit,
		screen:       "browse",
		pane:         paneMain,
		notePtr:      -1,
		queryCache:   map[string]queryCacheEntry{},
		readCache:    map[string]*api.ReadResult{},
		noteIndex:    map[string]api.Object{},
		open:         strings.TrimSpace(opts.Open),
		pollInterval: poll,
	}
}

func (m Model) Init() tea.Cmd {
	return fetchSchema(m.client)
}

func (m *Model) nextGen() int {
	m.gen++
	return m.gen
}

func (m *Model) applyFilterAndGroup() {
	p := m.p()
	p.applyFilterAndGroup(m.typeSchemaFor(p))
	m.recomputeSearchHits()
}

func cacheKey(query string, inputs map[string]string) string {
	if len(inputs) == 0 {
		return query
	}
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(query)
	for _, k := range keys {
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(inputs[k])
	}
	return b.String()
}

func (m Model) pageSize() int {
	h := m.height - 4
	if h < 8 {
		h = 8
	}
	return max(1, h/2)
}

func (m Model) currentLayoutName() string {
	p := m.p()
	if p.viewName != "" {
		return p.viewName
	}
	return p.spec.DisplayName()
}

func defaultFields(ts api.TypeSchema) []string {
	names := ts.FieldNames()
	if len(names) > 4 {
		return names[:4]
	}
	return names
}

func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[:len(r)-1])
}

func (m *Model) mergeNotes(objs []api.Object) {
	if m.noteIndex == nil {
		m.noteIndex = map[string]api.Object{}
	}
	for _, o := range objs {
		if !o.IsFileObject() {
			continue
		}
		m.noteIndex[o.ID] = o
	}
}
