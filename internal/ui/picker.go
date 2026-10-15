package ui

import (
	"sort"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/fuzzy"
	"github.com/aidanlsb/perch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

type pickItem struct {
	Kind   pickerKind
	Label  string
	Sub    string
	ID     string
	Type   string
	Query  *api.SavedQuery
	Object *api.Object
	Needs  []string
}

func (m Model) openTypePicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerTypes
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, name := range m.schema.SortedTypeNames() {
		spec := layout.Infer(m.schema.Types[name])
		if m.cfg != nil {
			if s, ok := m.cfg.TypeSpec(name); ok {
				spec = s
			}
		}
		items = append(items, pickItem{
			Kind:  pickerTypes,
			Label: name,
			Sub:   spec.Label(),
			ID:    name,
			Type:  name,
		})
	}
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func (m Model) openQueryPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerQueries
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, q := range m.schema.SortedSavedQueries() {
		qq := q
		sub := q.Query
		if len(q.Args) > 0 {
			sub = "needs " + strings.Join(q.Args, ", ")
		}
		items = append(items, pickItem{
			Kind:  pickerQueries,
			Label: q.Name,
			Sub:   sub,
			ID:    q.Name,
			Query: &qq,
			Needs: q.Args,
		})
	}
	if m.cfg != nil {
		for i := range m.cfg.Views {
			v := m.cfg.Views[i]
			items = append(items, pickItem{
				Kind:  pickerQueries,
				Label: v.Name,
				Sub:   "view " + v.Source,
				ID:    "view:" + v.Name,
			})
		}
	}
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func (m Model) openNotesPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerNotes
	m.pickerQ = ""
	m.pickerCur = 0
	m.rebuildNotePicks()
	m.refilterPicker()
	return m, nil
}

func (m *Model) rebuildNotePicks() {
	ids := make([]string, 0, len(m.noteIndex))
	for id := range m.noteIndex {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := make([]pickItem, 0, len(ids))
	for _, id := range ids {
		o := m.noteIndex[id]
		cp := o
		title := o.Title
		if title == "" {
			title = o.ID
		}
		items = append(items, pickItem{
			Kind:   pickerNotes,
			Label:  title,
			Sub:    o.ID,
			ID:     o.ID,
			Type:   o.Type,
			Object: &cp,
		})
	}
	m.pickerAll = items
}

func (m *Model) refilterPicker() {
	q := m.pickerQ
	var hits []pickItem
	type scored struct {
		it    pickItem
		score int
		idx   int
	}
	var rows []scored
	for i, it := range m.pickerAll {
		sc := fuzzy.BestScore(q, it.Label, it.Sub, it.ID, it.Type)
		if sc < 0 {
			continue
		}
		rows = append(rows, scored{it: it, score: sc, idx: i})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].idx < rows[j].idx
	})
	hits = make([]pickItem, len(rows))
	for i, r := range rows {
		hits[i] = r.it
	}
	m.pickerHit = hits
	if m.pickerCur >= len(m.pickerHit) {
		m.pickerCur = max(0, len(m.pickerHit)-1)
	}
}

func (m Model) confirmPicker() (Model, tea.Cmd) {
	if m.pickerCur < 0 || m.pickerCur >= len(m.pickerHit) {
		m.overlay = overlayNone
		return m, nil
	}
	it := m.pickerHit[m.pickerCur]
	m.overlay = overlayNone
	switch m.pickerKind {
	case pickerTypes:
		return m.loadType(it.Type)
	case pickerQueries:
		if strings.HasPrefix(it.ID, "view:") {
			name := strings.TrimPrefix(it.ID, "view:")
			for i, n := range m.nav {
				if n.Kind == navView && n.Label == name {
					return m.loadNav(i)
				}
			}
		}
		if it.Query != nil {
			return m.loadQuery(it.Query)
		}
		return m, nil
	default:
		id := it.ID
		if it.Object != nil && it.Object.ID != "" {
			id = it.Object.ID
		}
		return m.openNote(id)
	}
}

func (m Model) loadType(name string) (Model, tea.Cmd) {
	for i, n := range m.nav {
		if n.Kind == navType && n.Type == name {
			return m.loadNav(i)
		}
	}
	spec := layout.Infer(m.schema.Types[name])
	if m.cfg != nil {
		if s, ok := m.cfg.TypeSpec(name); ok {
			spec = s
		}
	}
	return m.loadSource(name, "type:"+name, name, spec, nil, false)
}

func (m Model) loadQuery(q *api.SavedQuery) (Model, tea.Cmd) {
	if q == nil {
		return m, nil
	}
	for i, n := range m.nav {
		if n.Kind == navQuery && n.Label == q.Name {
			return m.loadNav(i)
		}
	}
	spec := layout.Spec{Kind: kindList}.Normalize()
	if m.cfg != nil {
		if s, ok := m.cfg.QuerySpec(q.Name); ok {
			spec = s
		}
	}
	return m.loadSource(q.Name, q.Name, "", spec, q, true)
}

func (m Model) keysPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if m.pending == "g" {
		m.pending = ""
		if key == "g" {
			m.pickerCur = 0
			return m, nil
		}
	}
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		return m.confirmPicker()
	case "e":
		if m.pickerKind == pickerNotes {
			return m.openEditor()
		}
		return m, nil
	case "j", "down", "ctrl+n":
		if m.pickerCur+1 < len(m.pickerHit) {
			m.pickerCur++
		}
		return m, nil
	case "k", "up", "ctrl+p":
		if m.pickerCur > 0 {
			m.pickerCur--
		}
		return m, nil
	case "g":
		m.pending = "g"
		return m, nil
	case "G":
		m.pickerCur = max(0, len(m.pickerHit)-1)
		return m, nil
	case "ctrl+d":
		m.pickerCur = clamp(m.pickerCur+m.pageSize(), 0, max(0, len(m.pickerHit)-1))
		return m, nil
	case "ctrl+u":
		m.pickerCur = clamp(m.pickerCur-m.pageSize(), 0, max(0, len(m.pickerHit)-1))
		return m, nil
	case "backspace":
		m.pickerQ = dropLastRune(m.pickerQ)
		return m.afterPickerQuery()
	case "ctrl+c":
		return m, tea.Quit
	}
	if msg.Type == tea.KeyRunes {
		m.pickerQ += string(msg.Runes)
		return m.afterPickerQuery()
	}
	return m, nil
}

func (m Model) afterPickerQuery() (Model, tea.Cmd) {
	m.refilterPicker()
	q := strings.TrimSpace(m.pickerQ)
	if m.pickerKind == pickerNotes && len([]rune(q)) >= 2 {
		gen := m.nextGen()
		return m, fetchSearch(m.client, gen, q)
	}
	return m, nil
}
