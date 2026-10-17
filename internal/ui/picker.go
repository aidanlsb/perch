package ui

import (
	"fmt"
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
	Index  int
}

func (m Model) openTypePicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerTypes
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, name := range m.schema.SortedTypeNames() {
		ls, _ := m.resolveLayouts(name, "")
		sub := "List"
		if len(ls) > 0 {
			sub = ls[0].DisplayName()
			if len(ls) > 1 {
				sub = fmt.Sprintf("%s · %d layouts", ls[0].DisplayName(), len(ls))
			}
		}
		items = append(items, pickItem{
			Kind:  pickerTypes,
			Label: name,
			Sub:   sub,
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
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func (m Model) openLayoutPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerLayouts
	m.pickerQ = ""
	m.pickerCur = m.layoutIdx
	var items []pickItem
	for i, s := range m.layouts {
		sub := s.Kind
		if s.GroupBy != "" {
			sub += " by " + s.GroupBy
		}
		if s.Kind == layout.KindDate && s.Granularity != "" {
			sub += " / " + s.Granularity
		}
		if s.Filter != "" {
			sub += "  " + s.Filter
		}
		items = append(items, pickItem{
			Kind:  pickerLayouts,
			Label: s.DisplayName(),
			Sub:   sub,
			ID:    s.DisplayName(),
			Index: i,
		})
	}
	m.pickerAll = items
	m.refilterPicker()
	if m.layoutIdx >= 0 && m.layoutIdx < len(m.pickerHit) && m.pickerQ == "" {
		m.pickerCur = m.layoutIdx
	}
	return m, nil
}

func (m Model) openNotesPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerNotes
	m.pickerQ = ""
	m.pickerCur = 0
	m.rebuildNotePicks()
	m.refilterPicker()
	if !m.corpusDone && !m.corpusLoading {
		m.corpusLoading = true
		return m, m.corpusCmd()
	}
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
		if it.Query != nil {
			return m.loadQuery(it.Query)
		}
		return m, nil
	case pickerLayouts:
		return m.selectLayout(it.Index)
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
	return m.loadSource(name, "type:"+name, name, nil, false)
}

func inputsComplete(names []string, inputs map[string]string) bool {
	if len(names) == 0 {
		return true
	}
	if len(inputs) == 0 {
		return false
	}
	for _, name := range names {
		if strings.TrimSpace(inputs[name]) == "" {
			return false
		}
	}
	return true
}

func (m Model) startArgsPrompt(q *api.SavedQuery) (Model, tea.Cmd) {
	m.overlay = overlayArgs
	m.argsQuery = q
	m.argsNames = append([]string(nil), q.Args...)
	m.argsIdx = 0
	m.argsVals = map[string]string{}
	m.argsBuf = ""
	m.status = "args for " + q.Name
	return m, nil
}

func (m Model) keysArgs(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		m.overlay = overlayNone
		m.argsQuery = nil
		m.argsBuf = ""
		return m, nil
	case "enter":
		return m.commitArg()
	case "backspace":
		m.argsBuf = dropLastRune(m.argsBuf)
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	if text := typedText(msg); text != "" {
		m.argsBuf += text
	}
	return m, nil
}

func (m Model) commitArg() (Model, tea.Cmd) {
	if m.argsIdx < 0 || m.argsIdx >= len(m.argsNames) {
		return m.finishArgs()
	}
	name := m.argsNames[m.argsIdx]
	m.argsVals[name] = strings.TrimSpace(m.argsBuf)
	m.argsBuf = ""
	m.argsIdx++
	if m.argsIdx >= len(m.argsNames) {
		return m.finishArgs()
	}
	return m, nil
}

func (m Model) finishArgs() (Model, tea.Cmd) {
	q := m.argsQuery
	inputs := m.argsVals
	m.overlay = overlayNone
	m.argsQuery = nil
	m.argsBuf = ""
	if q == nil {
		return m, nil
	}
	m.sourceInputs = inputs
	return m.loadQuery(q)
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
	return m.loadSource(q.Name, q.Name, "", q, true)
}

func (m Model) keysPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
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
	if text := typedText(msg); text != "" {
		m.pickerQ += text
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
