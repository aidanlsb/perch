package ui

import (
	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/aidanlsb/perch/internal/group"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	sidebarWidth = 24
	queryLimit   = 500

	kindList  = config.KindList
	kindBoard = config.KindBoard
	kindDate  = config.KindDate
	kindNote  = "note"
)

type pane int

const (
	paneSidebar pane = iota
	paneMain
	paneBacklinks
)

type overlay int

const (
	overlayNone overlay = iota
	overlayFilter
	overlaySwitcher
	overlayHelp
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

// Options configure the TUI at start.
type Options struct {
	Client    *api.Client
	Config    *config.File
	VaultPath string
	Editor    string
}

// Model is the Bubble Tea application state. Update is a pure-ish reducer
// so tests can drive it without a program.
type Model struct {
	client     *api.Client
	cfg        *config.File
	vaultPath  string
	editor     string
	queryLimit int

	width  int
	height int

	pane    pane
	overlay overlay
	screen  string // browse | note

	schema api.Schema
	nav    []navItem
	navCur int

	viewKind    string
	sourceQuery string
	sourceLabel string
	typeName    string
	groupBy     string
	granularity string
	listFields  []string
	sortField   string
	objects     []api.Object
	filtered    []api.Object
	hasMore     bool
	total       int
	listCur     int
	listOff     int
	board       []group.Column
	boardCol    int
	boardRow    int
	dateCols    []group.Column
	dateSec     int
	dateRow     int

	filter string

	note       *api.ReadResult
	noteDoc    frontmatter.Doc
	noteBody   string
	noteScroll int
	linkIdx    int
	backIdx    int
	noteStack  []noteFrame
	notePtr    int

	switcherQ   string
	switcherHit []api.SearchMatch
	switcherCur int

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
		queryLimit:  queryLimit,
		screen:      "browse",
		pane:        paneSidebar,
		viewKind:    kindList,
		granularity: config.GranDay,
		notePtr:     -1,
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
	sorted := group.SortObjects(m.objects, m.sortField)
	m.filtered = group.Filter(sorted, m.filter)
	ts := m.typeSchema()
	switch m.viewKind {
	case kindBoard:
		if m.groupBy == "" {
			m.board = nil
			break
		}
		m.board = group.Board(m.filtered, m.groupBy, ts)
		if m.boardCol >= len(m.board) {
			m.boardCol = max(0, len(m.board)-1)
		}
		if len(m.board) > 0 && m.boardRow >= len(m.board[m.boardCol].Items) {
			m.boardRow = max(0, len(m.board[m.boardCol].Items)-1)
		}
	case kindDate:
		if m.groupBy == "" {
			m.dateCols = nil
			break
		}
		m.dateCols = group.Date(m.filtered, m.groupBy, m.granularity)
		if m.dateSec >= len(m.dateCols) {
			m.dateSec = max(0, len(m.dateCols)-1)
		}
		if len(m.dateCols) > 0 && m.dateRow >= len(m.dateCols[m.dateSec].Items) {
			m.dateRow = max(0, len(m.dateCols[m.dateSec].Items)-1)
		}
	default:
		if m.listCur >= len(m.filtered) {
			m.listCur = max(0, len(m.filtered)-1)
		}
	}
}

func (m Model) selectedObject() *api.Object {
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

func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[:len(r)-1])
}
