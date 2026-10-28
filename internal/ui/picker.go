package ui

import (
	"sort"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/fuzzy"
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
	Unset  bool
}

func (m Model) openNavPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerNav
	m.pickerQ = ""
	m.pickerCur = 0
	m.fromNav = false
	var items []pickItem
	items = append(items, pickItem{Kind: pickerNav, Label: "home", Sub: "vault dashboard", ID: "nav:home"})
	hasViews := m.cfg != nil && (len(m.cfg.Views) > 0 || len(m.cfg.Layouts) > 0)
	if hasViews {
		items = append(items, pickItem{Kind: pickerNav, Label: "views & layouts", Sub: "saved views and panel layouts", ID: "nav:views"})
	}
	items = append(items,
		pickItem{Kind: pickerNav, Label: "types", Sub: "schema types as a list", ID: "nav:types"},
		pickItem{Kind: pickerNav, Label: "saved queries", Sub: "from rvn", ID: "nav:queries"},
		pickItem{Kind: pickerNav, Label: "daily note", Sub: "today / pick a date", ID: "nav:daily"},
		pickItem{Kind: pickerNav, Label: "find notes", Sub: "fuzzy notes picker", ID: "nav:notes"},
		pickItem{Kind: pickerNav, Label: "content search", Sub: "full-text search", ID: "nav:search"},
		pickItem{Kind: pickerNav, Label: "actions", Sub: "create, edit, schema", ID: "nav:actions"},
		pickItem{Kind: pickerNav, Label: "open config", Sub: "edit perch config", ID: "nav:config"},
	)
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func (m Model) openTypePicker() (Model, tea.Cmd) {
	m.fromNav = m.overlay == overlayPicker && m.pickerKind == pickerNav
	m.overlay = overlayPicker
	m.pickerKind = pickerTypes
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, name := range m.schema.SortedTypeNames() {
		items = append(items, pickItem{
			Kind:  pickerTypes,
			Label: name,
			Sub:   "list",
			ID:    name,
			Type:  name,
		})
	}
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func (m Model) openQueryPicker() (Model, tea.Cmd) {
	m.fromNav = m.overlay == overlayPicker && m.pickerKind == pickerNav
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

func (m Model) openViewsPicker() (Model, tea.Cmd) {
	m.fromNav = m.overlay == overlayPicker && m.pickerKind == pickerNav
	m.overlay = overlayPicker
	m.pickerKind = pickerViews
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	if m.cfg != nil {
		for _, name := range m.cfg.ViewNames() {
			v := m.cfg.Views[name]
			items = append(items, pickItem{
				Kind:  pickerViews,
				Label: name,
				Sub:   "view · " + viewPickerSub(v),
				ID:    "view:" + name,
			})
		}
		for _, name := range m.cfg.LayoutNames() {
			items = append(items, pickItem{
				Kind:  pickerViews,
				Label: name,
				Sub:   "layout",
				ID:    "layout:" + name,
			})
		}
	}
	m.pickerAll = items
	m.refilterPicker()
	return m, nil
}

func viewPickerSub(v config.View) string {
	if strings.TrimSpace(v.Query) != "" {
		return "query " + v.Query
	}
	return "type " + v.Type
}

func (m Model) openSearchPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerSearch
	m.pickerQ = ""
	m.pickerCur = 0
	m.pickerAll = nil
	m.pickerHit = nil
	return m, nil
}

func (m *Model) applySearchHits(sr *api.SearchResult) {
	var items []pickItem
	if sr != nil {
		for i, hit := range sr.Items {
			id := hit.ObjectID
			if hit.IsSection && hit.FileObjectID != "" {
				id = hit.FileObjectID
			}
			if id == "" {
				continue
			}
			title := hit.Title
			if title == "" {
				title = id
			}
			obj := api.Object{ID: id, Title: title, FilePath: hit.FilePath}
			items = append(items, pickItem{
				Kind:   pickerSearch,
				Label:  title,
				Sub:    hit.Snippet,
				ID:     id,
				Object: &obj,
				Index:  i,
			})
		}
	}
	m.pickerAll = items
	m.pickerHit = items
	if m.pickerCur >= len(m.pickerHit) {
		m.pickerCur = max(0, len(m.pickerHit)-1)
	}
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
	if m.mut.kind != mutNone && m.pickerKind != pickerNav {
		return m.confirmMutPicker(it)
	}
	fromQueries := m.pickerKind == pickerQueries
	switch m.pickerKind {
	case pickerNav:
		return m.confirmNav(it)
	case pickerActions:
		m.overlay = overlayNone
		return m.runAction(it.ID)
	case pickerTypes:
		m.overlay = overlayNone
		return m.loadType(it.Type)
	case pickerQueries:
		m.fromQueries = fromQueries
		m.overlay = overlayNone
		if it.Query != nil {
			return m.loadQuery(it.Query)
		}
		return m, nil
	case pickerViews:
		m.overlay = overlayNone
		if strings.HasPrefix(it.ID, "layout:") {
			return m.loadNamedLayout(strings.TrimPrefix(it.ID, "layout:"))
		}
		name := strings.TrimPrefix(it.ID, "view:")
		if name == "" {
			name = it.Label
		}
		return m.loadNamedView(name)
	case pickerSearch:
		m.overlay = overlayNone
		id := it.ID
		if it.Object != nil && it.Object.ID != "" {
			id = it.Object.ID
		}
		return m.openNote(id)
	default:
		m.overlay = overlayNone
		id := it.ID
		if it.Object != nil && it.Object.ID != "" {
			id = it.Object.ID
		}
		return m.openNote(id)
	}
}

func (m Model) confirmNav(it pickItem) (Model, tea.Cmd) {
	switch it.ID {
	case "nav:home":
		m.overlay = overlayNone
		return m.goHome()
	case "nav:views":
		return m.openViewsPicker()
	case "nav:types":
		return m.openTypePicker()
	case "nav:queries":
		return m.openQueryPicker()
	case "nav:daily":
		return m.openDatePicker()
	case "nav:notes":
		return m.openNotesPicker()
	case "nav:search":
		return m.openSearchPicker()
	case "nav:actions":
		return m.openActionsPicker()
	case "nav:config":
		return m.openConfigAtView()
	default:
		return m, nil
	}
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
	return m.startArgsPromptWith(q, nil)
}

func (m Model) startArgsPromptWith(q *api.SavedQuery, have map[string]string) (Model, tea.Cmd) {
	if m.overlay == overlayPicker && m.pickerKind == pickerQueries {
		m.fromQueries = true
	}
	m.overlay = overlayArgs
	m.argsQuery = q
	m.argsNames = append([]string(nil), q.Args...)
	m.argsVals = map[string]string{}
	for k, v := range have {
		if s := strings.TrimSpace(v); s != "" {
			m.argsVals[k] = s
		}
	}
	m.argsIdx = 0
	for m.argsIdx < len(m.argsNames) && strings.TrimSpace(m.argsVals[m.argsNames[m.argsIdx]]) != "" {
		m.argsIdx++
	}
	m.argsBuf = ""
	m.status = "args for " + q.Name
	if m.argsIdx >= len(m.argsNames) {
		return m.finishArgs()
	}
	return m, nil
}

func (m Model) keysArgs(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		return m.argsEscape()
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
	return m.loadQueryWithInputs(q, inputs)
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
	case "esc":
		return m.pickerEscape()
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
		if m.mut.kind != mutNone && m.mut.optional && (m.pickerKind == pickerValues || m.pickerKind == pickerRefs) {
			return m.applyUnset()
		}
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
	q := strings.TrimSpace(m.pickerQ)
	if m.pickerKind == pickerSearch {
		if q == "" {
			m.pickerAll = nil
			m.pickerHit = nil
			m.pickerCur = 0
			return m, nil
		}
		gen := m.nextGen()
		return m, fetchSearch(m.client, gen, q)
	}
	m.refilterPicker()
	if m.pickerKind == pickerNotes && len([]rune(q)) >= 2 {
		gen := m.nextGen()
		return m, fetchSearch(m.client, gen, q)
	}
	return m, nil
}
