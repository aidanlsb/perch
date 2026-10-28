package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) startSet() (Model, tea.Cmd) {
	obj := m.targetObject()
	if obj == nil {
		m.status = "no object"
		return m, nil
	}
	if obj.IsTrait() {
		return m.startTraitValue(*obj)
	}
	ts, ok := m.schema.Types[strings.TrimSpace(obj.Type)]
	if !ok {
		if p := m.p(); p != nil && p.typeName != "" {
			ts, ok = m.schema.Types[p.typeName]
			if ok && obj.Type == "" {
				obj.Type = p.typeName
			}
		}
	}
	if !ok || len(ts.Fields) == 0 {
		m.status = "no schema fields"
		return m, nil
	}
	return m.openFieldPicker(*obj, ts)
}

func (m Model) startTraitValue(obj api.Object) (Model, tea.Cmd) {
	tt := traitTypeOf(obj)
	ts := m.schema.Traits[tt]
	m.mut = mutState{
		kind:      mutTrait,
		step:      "value",
		traitID:   obj.ID,
		traitType: tt,
		schema:    ts.AsField(),
		current:   traitValueOf(obj),
		restoreID: obj.ID,
	}
	return m.openValueWidget()
}

func (m Model) startTraitFast(obj api.Object) (Model, tea.Cmd) {
	tt := traitTypeOf(obj)
	ts := m.schema.Traits[tt]
	cur := traitValueOf(obj)
	if next, ok := traitToggleValue(ts, cur); ok {
		m.mut = mutState{kind: mutTrait, traitID: obj.ID, restoreID: obj.ID}
		return m.invokeUpdateTrait(obj.ID, next)
	}
	if ts.IsEnum() && len(ts.Values) > 2 {
		m.mut = mutState{
			kind:      mutTrait,
			step:      "value",
			traitID:   obj.ID,
			traitType: tt,
			schema:    ts.AsField(),
			current:   cur,
			restoreID: obj.ID,
		}
		return m.openEnumPicker()
	}
	return m.startTraitValue(obj)
}

func traitToggleValue(ts api.TraitSchema, current string) (string, bool) {
	if ts.IsBool() {
		return toggleBool(current), true
	}
	if ts.IsEnum() && len(ts.Values) == 2 {
		if current == ts.Values[0] {
			return ts.Values[1], true
		}
		return ts.Values[0], true
	}
	if strings.TrimSpace(ts.Type) == "" && (current == "true" || current == "false") {
		return toggleBool(current), true
	}
	return "", false
}

func toggleBool(cur string) string {
	switch strings.ToLower(strings.TrimSpace(cur)) {
	case "true", "yes", "1":
		return "false"
	default:
		return "true"
	}
}

func traitTypeOf(obj api.Object) string {
	if s := api.ScalarString(obj.Fields["trait_type"]); s != "" {
		return s
	}
	return strings.TrimSpace(obj.Type)
}

func traitValueOf(obj api.Object) string {
	return api.ScalarString(obj.Fields["value"])
}

func (m Model) targetObject() *api.Object {
	if m.screen == kindNote && m.note != nil {
		id := strings.TrimSpace(m.note.ObjectID)
		if o, ok := m.noteIndex[id]; ok {
			cp := o
			return &cp
		}
		typ := ""
		if p := m.p(); p != nil {
			typ = p.typeName
		}
		return &api.Object{ID: id, Type: typ, Fields: map[string]any{}}
	}
	return m.selectedObject()
}

func (m Model) targetFileObject() *api.Object {
	obj := m.targetObject()
	if obj == nil {
		return nil
	}
	if obj.IsTrait() || !obj.IsFileObject() {
		id := obj.OpenRef()
		if id == "" {
			return nil
		}
		if o, ok := m.noteIndex[id]; ok {
			cp := o
			return &cp
		}
		return &api.Object{ID: id, Type: obj.Type, Fields: map[string]any{}}
	}
	return obj
}

func (m Model) openFieldPicker(obj api.Object, ts api.TypeSchema) (Model, tea.Cmd) {
	if m.mut.kind == mutNone {
		m.mut.kind = mutSet
	}
	if m.mut.kind == mutSet {
		m.mut.ref = obj.ID
		m.mut.objType = obj.Type
		m.mut.restoreID = obj.ID
	}
	m.mut.step = "field"
	m.overlay = overlayPicker
	m.pickerKind = pickerFields
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, name := range ts.FieldNames() {
		fs := ts.Fields[name]
		cur := api.ScalarString(obj.Fields[name])
		if cur == "" {
			cur = "—"
		}
		badge := fs.Type
		if fs.Required {
			badge += " required"
		}
		items = append(items, pickItem{
			Kind:  pickerFields,
			Label: name,
			Sub:   cur + "  " + badge,
			ID:    name,
		})
	}
	m.pickerAll = items
	m.refilterPicker()
	m.status = "set field"
	return m, nil
}

func (m Model) afterFieldPicked(name string) (Model, tea.Cmd) {
	if m.mut.kind == mutSchemaRenameField {
		m.mut.field = name
		m.mut.step = "new"
		return m.openPrompt("new field name", "", false)
	}
	obj := m.targetObject()
	if obj == nil {
		m.status = "no object"
		m.clearMut()
		m.overlay = overlayNone
		return m, nil
	}
	ts := m.schema.Types[obj.Type]
	if ts.Name == "" {
		ts = m.schema.Types[m.mut.objType]
	}
	fs := ts.Fields[name]
	m.mut.kind = mutSet
	m.mut.step = "value"
	m.mut.field = name
	m.mut.schema = fs
	m.mut.current = api.ScalarString(obj.Fields[name])
	m.mut.optional = !fs.Required
	m.mut.ref = obj.ID
	m.mut.restoreID = obj.ID
	return m.openValueWidget()
}

func (m Model) openValueWidget() (Model, tea.Cmd) {
	fs := m.mut.schema
	base, array := api.ParseFieldType(fs.Type)
	if array {
		return m.openPrompt(fs.Type, m.mut.current, m.mut.optional)
	}
	switch strings.ToLower(base) {
	case "enum":
		return m.openEnumPicker()
	case "bool", "boolean":
		if m.mut.kind == mutTrait {
			return m.invokeUpdateTrait(m.mut.traitID, toggleBool(m.mut.current))
		}
		if m.mut.kind == mutSet && !m.mut.optional {
			val, err := api.CoerceFieldValue(fs.Type, toggleBool(m.mut.current))
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			return m.invokeSet(m.mut.ref, map[string]any{m.mut.field: val})
		}
		return m.openBoolPicker()
	case "ref":
		return m.openRefPicker()
	case "date":
		return m.openDateValue()
	default:
		label := fs.Type
		if label == "" {
			label = "value"
		}
		return m.openPrompt(label, m.mut.current, m.mut.optional)
	}
}

func (m Model) openEnumPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerValues
	m.pickerQ = ""
	m.mut.step = "value"
	var items []pickItem
	if m.mut.optional {
		items = append(items, pickItem{Kind: pickerValues, Label: "[unset]", ID: unsetPickID, Unset: true})
	}
	cur := 0
	for _, v := range m.mut.schema.Values {
		it := pickItem{Kind: pickerValues, Label: v, ID: v}
		if v == m.mut.current {
			it.Sub = "current"
			cur = len(items)
		}
		items = append(items, it)
	}
	m.pickerAll = items
	m.refilterPicker()
	m.pickerCur = cur
	m.status = m.mut.field
	if m.mut.kind == mutTrait {
		m.status = m.mut.traitType
	}
	return m, nil
}

func (m Model) openBoolPicker() (Model, tea.Cmd) {
	m.mut.schema.Values = []string{"true", "false"}
	return m.openEnumPicker()
}

func (m Model) openRefPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerRefs
	m.pickerQ = ""
	m.mut.step = "value"
	target := strings.TrimSpace(m.mut.schema.Target)
	var items []pickItem
	if m.mut.optional {
		items = append(items, pickItem{Kind: pickerRefs, Label: "[unset]", ID: unsetPickID, Unset: true})
	}
	for _, o := range m.noteIndex {
		if target != "" && o.Type != target {
			continue
		}
		title := o.Title
		if title == "" {
			title = o.ID
		}
		it := pickItem{Kind: pickerRefs, Label: title, Sub: o.ID, ID: o.ID, Type: o.Type}
		if o.ID == m.mut.current {
			it.Sub = "current · " + o.ID
		}
		items = append(items, it)
	}
	m.pickerAll = items
	m.refilterPicker()
	for i, it := range m.pickerHit {
		if it.ID == m.mut.current {
			m.pickerCur = i
			break
		}
	}
	m.status = "ref"
	if target != "" {
		m.status = "ref " + target
	}
	return m, nil
}

func (m Model) openDateValue() (Model, tea.Cmd) {
	m.overlay = overlayDate
	m.datePurpose = "value"
	cur := strings.TrimSpace(m.mut.current)
	if !isDailyNoteID(cur) {
		cur = m.todayID()
	}
	m.dateCur = cur
	m.status = "date " + m.dateCur
	return m, nil
}

func (m Model) openChoicePicker(title string, ids []string, step string) (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerValues
	m.pickerQ = ""
	m.pickerCur = 0
	m.mut.step = step
	var items []pickItem
	for _, id := range ids {
		items = append(items, pickItem{Kind: pickerValues, Label: id, ID: id})
	}
	m.pickerAll = items
	m.refilterPicker()
	m.status = title
	return m, nil
}

func (m Model) confirmMutPicker(it pickItem) (Model, tea.Cmd) {
	if it.Unset || it.ID == unsetPickID {
		return m.applyUnset()
	}
	switch m.pickerKind {
	case pickerFields:
		return m.afterFieldPicked(it.ID)
	case pickerRefs:
		return m.commitValue(it.ID)
	case pickerTypes:
		return m.afterTypePicked(it.Type)
	case pickerValues:
		if m.mut.step == "value" && (m.mut.kind == mutSet || m.mut.kind == mutTrait) {
			return m.commitValue(it.ID)
		}
		return m.afterChoice(it.ID)
	case pickerActions:
		return m.runAction(it.ID)
	}
	m.overlay = overlayNone
	return m, nil
}

func (m Model) afterTypePicked(name string) (Model, tea.Cmd) {
	name = strings.TrimSpace(name)
	if name == "" {
		m.status = "no type"
		return m, nil
	}
	if m.mut.step == "target" {
		m.mut.fieldTarget = name
		return m.promptRequired()
	}
	switch m.mut.kind {
	case mutNew:
		m.mut.objType = name
		m.mut.pickedType = true
		return m.promptNewTitle()
	case mutReclassify:
		m.mut.newType = name
		return m.startReclassifyFields()
	case mutSchemaAddField:
		m.mut.typeName = name
		m.mut.step = "field-name"
		return m.openPrompt("field name", "", false)
	case mutSchemaRenameType:
		m.mut.oldName = name
		m.mut.step = "new"
		return m.openPrompt("new type name", "", false)
	case mutSchemaRenameField:
		m.mut.typeName = name
		return m.openRenameFieldPicker(name)
	}
	return m, nil
}

func (m Model) afterChoice(id string) (Model, tea.Cmd) {
	switch m.mut.step {
	case "field-type":
		m.mut.fieldType = id
		return m.afterFieldTypeChosen()
	case "trait-type":
		m.mut.fieldType = id
		return m.afterTraitTypeChosen()
	case "required":
		m.mut.fieldReq = id == "yes" || id == "true" || id == "y"
		m.mut.step = "default"
		return m.openPrompt("default (empty skip)", "", true)
	case "value":
		return m.commitValue(id)
	}
	return m, nil
}

func (m Model) commitValue(raw string) (Model, tea.Cmd) {
	switch m.mut.kind {
	case mutTrait:
		return m.invokeUpdateTrait(m.mut.traitID, raw)
	case mutSet:
		val, err := api.CoerceFieldValue(m.mut.schema.Type, raw)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		return m.invokeSet(m.mut.ref, map[string]any{m.mut.field: val})
	case mutNew:
		return m.commitNewFieldValue(raw)
	case mutReclassify:
		return m.commitReclassifyField(raw)
	}
	return m, nil
}

func objectInPane(p *viewState, id string) *api.Object {
	if p == nil || id == "" {
		return nil
	}
	for i := range p.objects {
		if p.objects[i].ID == id {
			o := p.objects[i]
			return &o
		}
	}
	return nil
}

func (m Model) moveBoardCard(delta int) (Model, tea.Cmd) {
	p := m.focusedView()
	if p == nil || p.spec.Kind != kindBoard {
		return m, nil
	}
	obj := p.selectedObject()
	if p.restoreID != "" {
		if found := objectInPane(p, p.restoreID); found != nil {
			obj = found
		}
	}
	if obj == nil || obj.IsTrait() {
		m.status = "select an object"
		return m, nil
	}
	target := p.boardCol + delta
	if target < 0 || target >= len(p.board) {
		return m, nil
	}
	col := p.board[target]
	field := p.groupBy
	if field == "" {
		m.status = "no group_by"
		return m, nil
	}
	m.mut = mutState{kind: mutSet, ref: obj.ID, field: field, restoreID: obj.ID}
	p.boardCol = target
	p.restoreID = obj.ID
	p.keepCursor = true
	if col.Key == "" {
		return m.invokeUnset(obj.ID, []string{field})
	}
	return m.invokeSet(obj.ID, map[string]any{field: col.Key})
}

func (m Model) openMutTypePicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerTypes
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, name := range m.schema.SortedTypeNames() {
		items = append(items, pickItem{Kind: pickerTypes, Label: name, ID: name, Type: name})
	}
	m.pickerAll = items
	m.refilterPicker()
	m.status = "type"
	return m, nil
}
