package ui

import (
	"sort"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/group"
	"github.com/aidanlsb/perch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	sidebarWidth = 24
	queryLimit   = 500
	previewMinW  = 28

	kindList  = config.KindList
	kindBoard = config.KindBoard
	kindDate  = config.KindDate
	kindNote  = "note"
)

type pane int

const (
	paneSidebar pane = iota
	paneMain
	panePreview
	paneBacklinks
)

type overlay int

const (
	overlayNone overlay = iota
	overlaySearch
	overlayPicker
	overlayHelp
	overlayLeader
	overlayRQL
	overlayArgs
)

type pickerKind int

const (
	pickerTypes pickerKind = iota
	pickerQueries
	pickerNotes
)

type navKind int

const (
	navType navKind = iota
	navQuery
	navView
)

type navItem struct {
	Kind   navKind
	Label  string
	Source string
	View   *config.View
	Type   string
	Query  *api.SavedQuery
}

type noteFrame struct {
	ID string
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
	VaultPath  string
	Editor     string
	EditorMode string
}

// Model is the Bubble Tea application state. Update is a pure-ish reducer
// so tests can drive it without a program.
type Model struct {
	client     *api.Client
	cfg        *config.File
	vaultPath  string
	editor     string
	editorMode string
	queryLimit int

	width  int
	height int

	pane    pane
	overlay overlay
	screen  string // browse | note
	pending string // g | (empty)

	schema api.Schema
	nav    []navItem
	navCur int

	spec          layout.Spec
	layoutLocked  bool
	viewKind      string
	sourceQuery   string
	sourceLabel   string
	sourceIsQuery bool
	typeName      string
	groupBy       string
	granularity   string
	listFields    []string
	sortField     string
	objects       []api.Object
	filtered      []api.Object
	hasMore       bool
	total         int
	listCur       int
	listOff       int
	board         []group.Column
	boardCol      int
	boardRow      int
	dateCols      []group.Column
	dateSec       int
	dateRow       int

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

	argsQuery *api.SavedQuery
	argsNames []string
	argsIdx   int
	argsVals  map[string]string
	argsBuf   string

	sourceInputs map[string]string

	daily *api.DailyInfo

	queryCache map[string]queryCacheEntry
	readCache  map[string]*api.ReadResult
	noteIndex  map[string]api.Object

	needPrefetch  bool
	corpusDone    bool
	corpusLoading bool

	status  string
	errMsg  string
	loading bool
	gen     int
	ready   bool
}

// NewModel constructs an empty UI. Call Init to fetch the schema.
func NewModel(opts Options) Model {
	cfg := opts.Config
	if cfg == nil {
		cfg = &config.File{}
	}
	return Model{
		client:      opts.Client,
		cfg:         cfg,
		vaultPath:   opts.VaultPath,
		editor:      opts.Editor,
		editorMode:  opts.EditorMode,
		queryLimit:  queryLimit,
		screen:      "browse",
		pane:        paneSidebar,
		viewKind:    kindList,
		spec:        layout.Spec{Kind: kindList}.Normalize(),
		granularity: config.GranDay,
		notePtr:     -1,
		queryCache:  map[string]queryCacheEntry{},
		readCache:   map[string]*api.ReadResult{},
		noteIndex:   map[string]api.Object{},
	}
}

func (m Model) Init() tea.Cmd {
	return fetchSchema(m.client)
}

func (m *Model) nextGen() int {
	m.gen++
	return m.gen
}

func (m Model) typeSchema() api.TypeSchema {
	if m.typeName == "" {
		return api.TypeSchema{}
	}
	return m.schema.Types[m.typeName]
}

func (m *Model) applySpec(s layout.Spec) {
	s = s.Normalize()
	m.spec = s
	m.viewKind = s.Kind
	m.groupBy = s.GroupBy
	m.granularity = s.Granularity
	if len(s.Fields) > 0 {
		m.listFields = s.Fields
	}
	m.sortField = s.Sort
	m.applyFilterAndGroup()
}

func (m *Model) rebuildNav() {
	m.nav = nil
	for _, name := range m.schema.SortedTypeNames() {
		m.nav = append(m.nav, navItem{
			Kind:   navType,
			Label:  name,
			Source: "type:" + name,
			Type:   name,
		})
	}
	queries := m.schema.SortedSavedQueries()
	for i := range queries {
		q := queries[i]
		m.nav = append(m.nav, navItem{
			Kind:   navQuery,
			Label:  q.Name,
			Source: q.Name,
			Query:  &queries[i],
		})
	}
	if m.cfg != nil {
		for i := range m.cfg.Views {
			v := m.cfg.Views[i]
			m.nav = append(m.nav, navItem{
				Kind:   navView,
				Label:  v.Name,
				Source: v.Source,
				View:   &m.cfg.Views[i],
			})
		}
	}
	if m.navCur >= len(m.nav) {
		m.navCur = max(0, len(m.nav)-1)
	}
}

func (m *Model) applyFilterAndGroup() {
	r := layout.Apply(m.objects, m.spec, m.typeSchema(), "", false)
	m.filtered = r.Objects
	m.board = r.Board
	m.dateCols = r.Date
	if m.boardCol >= len(m.board) {
		m.boardCol = max(0, len(m.board)-1)
	}
	if len(m.board) > 0 && m.boardRow >= len(m.board[m.boardCol].Items) {
		m.boardRow = max(0, len(m.board[m.boardCol].Items)-1)
	}
	if m.dateSec >= len(m.dateCols) {
		m.dateSec = max(0, len(m.dateCols)-1)
	}
	if len(m.dateCols) > 0 && m.dateRow >= len(m.dateCols[m.dateSec].Items) {
		m.dateRow = max(0, len(m.dateCols[m.dateSec].Items)-1)
	}
	if m.listCur >= len(m.filtered) {
		m.listCur = max(0, len(m.filtered)-1)
	}
	m.recomputeSearchHits()
}

func (m Model) selectedObject() *api.Object {
	if m.overlay == overlayPicker && m.pickerKind == pickerNotes {
		if m.pickerCur >= 0 && m.pickerCur < len(m.pickerHit) {
			it := m.pickerHit[m.pickerCur]
			if it.Object != nil {
				o := *it.Object
				return &o
			}
			if o, ok := m.noteIndex[it.ID]; ok {
				return &o
			}
		}
	}
	switch m.viewKind {
	case kindBoard:
		if m.boardCol < 0 || m.boardCol >= len(m.board) {
			return nil
		}
		items := m.board[m.boardCol].Items
		if m.boardRow < 0 || m.boardRow >= len(items) {
			return nil
		}
		o := items[m.boardRow]
		return &o
	case kindDate:
		if m.dateSec < 0 || m.dateSec >= len(m.dateCols) {
			return nil
		}
		items := m.dateCols[m.dateSec].Items
		if m.dateRow < 0 || m.dateRow >= len(items) {
			return nil
		}
		o := items[m.dateRow]
		return &o
	default:
		if m.listCur < 0 || m.listCur >= len(m.filtered) {
			return nil
		}
		o := m.filtered[m.listCur]
		return &o
	}
}

func (m *Model) selectID(id string) bool {
	if id == "" {
		return false
	}
	switch m.viewKind {
	case kindBoard:
		for c := range m.board {
			for r, obj := range m.board[c].Items {
				if obj.ID == id {
					m.boardCol = c
					m.boardRow = r
					m.pane = paneMain
					return true
				}
			}
		}
	case kindDate:
		for s := range m.dateCols {
			for r, obj := range m.dateCols[s].Items {
				if obj.ID == id {
					m.dateSec = s
					m.dateRow = r
					m.pane = paneMain
					return true
				}
			}
		}
	default:
		for i, obj := range m.filtered {
			if obj.ID == id {
				m.listCur = i
				m.pane = paneMain
				return true
			}
		}
	}
	return false
}

func (m *Model) mergeNotes(objs []api.Object) {
	if m.noteIndex == nil {
		m.noteIndex = map[string]api.Object{}
	}
	for _, o := range objs {
		if o.ID == "" {
			continue
		}
		m.noteIndex[o.ID] = o
	}
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

func (m *Model) cacheQuery() {
	if m.sourceQuery == "" || m.queryCache == nil {
		return
	}
	m.queryCache[cacheKey(m.sourceQuery, m.sourceInputs)] = queryCacheEntry{
		objects:  m.objects,
		typeName: m.typeName,
		hasMore:  m.hasMore,
		total:    m.total,
	}
}

func (m Model) pageSize() int {
	h := m.height - 4
	if h < 8 {
		h = 8
	}
	return max(1, h/2)
}

func (m Model) specForNav(item navItem) layout.Spec {
	switch item.Kind {
	case navView:
		if item.View != nil {
			return item.View.Spec()
		}
	case navQuery:
		if m.cfg != nil {
			if s, ok := m.cfg.QuerySpec(item.Label); ok {
				return s
			}
		}
	case navType:
		if m.cfg != nil {
			if s, ok := m.cfg.TypeSpec(item.Type); ok {
				return s
			}
		}
		return layout.Infer(m.schema.Types[item.Type])
	}
	return layout.Spec{Kind: kindList}.Normalize()
}

func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[:len(r)-1])
}
