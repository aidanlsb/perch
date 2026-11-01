package ui

import (
	"fmt"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) startSet() (Model, tea.Cmd) {
	objs := m.typedActionObjects()
	if len(objs) == 0 {
		m.status = "no object"
		return m, nil
	}
	ids := make([]string, 0, len(objs))
	for _, o := range objs {
		ids = append(ids, o.ID)
	}
	if anyTrait(objs) && !allTraits(objs) {
		m.status = "selection mixes objects and traits"
		return m, nil
	}
	if allTraits(objs) {
		restore := ids[0]
		if p := m.focusedView(); p != nil {
			if cur := p.selectedRowID(); cur != "" {
				restore = cur
			}
		}
		m.mut = mutState{kind: mutTrait, targets: ids, restoreID: restore}
		return m.startTraitValue(objs[0])
	}
	obj := objs[0]
	cf := commonFields(m.schema, objs)
	if len(objs) > 1 && len(cf) == 0 {
		m.status = "no common fields"
		return m, nil
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
	m.mut = mutState{kind: mutSet, ref: obj.ID, objType: obj.Type, restoreID: obj.ID, targets: ids}
	if p := m.focusedView(); p != nil {
		if cur := p.selectedRowID(); cur != "" {
			m.mut.restoreID = cur
		}
	}
	if len(cf) > 0 {
		return m.openCommonFieldPicker(obj, cf)
	}
	return m.openFieldPicker(obj, ts)
}

func (m Model) typedActionObjects() []api.Object {
	objs := m.actionObjects()
	fallback := ""
	if p := m.p(); p != nil {
		fallback = p.typeName
	}
	out := make([]api.Object, 0, len(objs))
	for _, o := range objs {
		if strings.TrimSpace(o.Type) == "" {
			o.Type = fallback
		}
		out = append(out, o)
	}
	return out
}

func (m Model) mutObject() *api.Object {
	if len(m.mut.targets) > 0 {
		if o := m.objectByID(m.mut.targets[0]); o != nil {
			if strings.TrimSpace(o.Type) == "" {
				o.Type = m.mut.objType
			}
			return o
		}
	}
	return m.targetObject()
}

func (m Model) startTraitValue(obj api.Object) (Model, tea.Cmd) {
	tt := traitTypeOf(obj)
	ts := m.schema.Traits[tt]
	targets := m.mut.targets
	restore := m.mut.restoreID
	if restore == "" {
		restore = obj.ID
	}
	if len(targets) == 0 {
		targets = []string{obj.ID}
	}
	m.mut = mutState{
		kind:      mutTrait,
		step:      "value",
		traitID:   obj.ID,
		traitType: tt,
		schema:    ts.AsField(),
		current:   traitValueOf(obj),
		restoreID: restore,
		targets:   targets,
	}
	return m.openValueWidget()
}

func (m Model) startTraitFast(obj api.Object) (Model, tea.Cmd) {
	objs := m.typedActionObjects()
	if len(objs) == 0 {
		objs = []api.Object{obj}
	}
	ids := make([]string, 0, len(objs))
	for _, o := range objs {
		ids = append(ids, o.ID)
	}
	restore := obj.ID
	if p := m.focusedView(); p != nil {
		if cur := p.selectedRowID(); cur != "" {
			restore = cur
		}
	}
	val, picker, errMsg := traitUniformTarget(m.schema, objs)
	if errMsg != "" {
		m.status = errMsg
		return m, nil
	}
	tt := traitTypeOf(objs[0])
	ts := m.schema.Traits[tt]
	cur := traitValueOf(objs[0])
	m.mut = mutState{kind: mutTrait, traitID: objs[0].ID, traitType: tt, restoreID: restore, targets: ids, schema: ts.AsField(), current: cur}
	if picker {
		if ts.IsEnum() && len(ts.Values) > 2 {
			m.mut.step = "value"
			return m.openEnumPicker()
		}
		return m.startTraitValue(objs[0])
	}
	if len(ids) > 1 {
		m.mut.current = val
		return m.openConfirm(fmt.Sprintf("set trait → %s on %d rows [enter/esc]", val, len(ids)))
	}
	return m.invokeUpdateTrait(objs[0].ID, val)
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
	var fields []commonField
	for _, name := range ts.FieldNames() {
		fields = append(fields, commonField{Name: name, Schema: ts.Fields[name]})
	}
	return m.openCommonFieldPicker(obj, fields)
}

func (m Model) openCommonFieldPicker(obj api.Object, fields []commonField) (Model, tea.Cmd) {
	if m.mut.kind == mutNone {
		m.mut.kind = mutSet
	}
	if m.mut.kind == mutSet {
		m.mut.ref = obj.ID
		m.mut.objType = obj.Type
		if m.mut.restoreID == "" {
			m.mut.restoreID = obj.ID
		}
	}
	m.mut.step = "field"
	m.overlay = overlayPicker
	m.pickerKind = pickerFields
	m.pickerQ = ""
	m.pickerCur = 0
	var items []pickItem
	for _, cf := range fields {
		fs := cf.Schema
		cur := m.fieldCurrentLabel(cf.Name, obj)
		badge := fs.Type
		if fs.Required {
			badge += " required"
		}
		items = append(items, pickItem{
			Kind:  pickerFields,
			Label: cf.Name,
			Sub:   cur + "  " + badge,
			ID:    cf.Name,
		})
	}
	m.pickerAll = items
	m.refilterPicker()
	m.status = "set field"
	return m, nil
}

func (m Model) fieldCurrentLabel(name string, obj api.Object) string {
	cur := api.ScalarString(obj.Fields[name])
	ids := m.mut.targets
	if len(ids) > 1 {
		for _, id := range ids[1:] {
			o := m.objectByID(id)
			v := ""
			if o != nil {
				v = api.ScalarString(o.Fields[name])
			}
			if v != cur {
				return "—"
			}
		}
	}
	if cur == "" {
		return "—"
	}
	return cur
}

func (m Model) afterFieldPicked(name string) (Model, tea.Cmd) {
	if m.mut.kind == mutSchemaRenameField {
		m.mut.field = name
		m.mut.step = "new"
		return m.openPrompt("new field name", "", false)
	}
	obj := m.mutObject()
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
	if len(m.mut.targets) > 1 {
		for _, cf := range commonFields(m.schema, m.typedActionObjects()) {
			if cf.Name == name {
				fs = cf.Schema
				break
			}
		}
	}
	m.mut.kind = mutSet
	m.mut.step = "value"
	m.mut.field = name
	m.mut.schema = fs
	m.mut.current = api.ScalarString(obj.Fields[name])
	m.mut.optional = !fs.Required
	m.mut.ref = obj.ID
	if m.mut.restoreID == "" {
		m.mut.restoreID = obj.ID
	}
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
			return m.applyTraitValue(toggleBool(m.mut.current))
		}
		if m.mut.kind == mutSet && !m.mut.optional {
			val, err := api.CoerceFieldValue(fs.Type, toggleBool(m.mut.current))
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			return m.applySetFields(map[string]any{m.mut.field: val})
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
	if len(m.pickerHit) == 0 {
		m.errMsg = "no values"
		m.status = "no values for " + m.mut.field
		if m.mut.kind == mutTrait {
			m.status = "no values for " + m.mut.traitType
		}
		return m, nil
	}
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
	if len(m.pickerHit) == 0 {
		m.errMsg = "no references"
		m.status = "no references"
		return m, nil
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
		return m.applyTraitValue(raw)
	case mutSet:
		val, err := api.CoerceFieldValue(m.mut.schema.Type, raw)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		return m.applySetFields(map[string]any{m.mut.field: val})
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
	objs := []api.Object{}
	if p.hasMarks() {
		objs = p.markedObjects()
	} else {
		obj := p.selectedObject()
		if p.restoreID != "" {
			if found := objectInPane(p, p.restoreID); found != nil {
				obj = found
			}
		}
		if obj != nil {
			objs = []api.Object{*obj}
		}
	}
	field := p.groupBy
	if field == "" {
		m.status = "no group_by"
		return m, nil
	}
	colOf := func(id string) int {
		for c := range p.board {
			for _, it := range p.board[c].Items {
				if it.ID == id {
					return c
				}
			}
		}
		return -1
	}
	var ops []boardOp
	ids := make([]string, 0, len(objs))
	for _, obj := range objs {
		if obj.IsTrait() || obj.ID == "" {
			continue
		}
		cur := colOf(obj.ID)
		if !p.hasMarks() || cur < 0 {
			cur = p.boardCol
		}
		target := cur + delta
		if target < 0 || target >= len(p.board) {
			continue
		}
		ops = append(ops, boardOp{ID: obj.ID, Field: field, Key: p.board[target].Key})
		ids = append(ids, obj.ID)
	}
	if len(ops) == 0 {
		return m, nil
	}
	restore := p.selectedRowID()
	m.mut = mutState{kind: mutSet, field: field, restoreID: restore, targets: ids, boardOps: ops}
	if len(ops) == 1 {
		op := ops[0]
		m.mut.ref = op.ID
		target := p.boardCol + delta
		if target >= 0 && target < len(p.board) {
			p.boardCol = target
		}
		p.restoreID = restore
		p.keepCursor = true
		if op.Key == "" {
			return m.invokeUnset(op.ID, []string{field})
		}
		return m.invokeSet(op.ID, map[string]any{field: op.Key})
	}
	dir := "right"
	if delta < 0 {
		dir = "left"
	}
	return m.openConfirm(fmt.Sprintf("move %d cards one column %s [enter/esc]", len(ops), dir))
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
