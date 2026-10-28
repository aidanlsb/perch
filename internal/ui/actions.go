package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

var schemaFieldTypes = []string{
	"string", "enum", "ref", "date", "bool", "number", "url", "datetime",
	"string[]", "enum[]", "ref[]",
}

var schemaTraitTypes = []string{
	"string", "enum", "bool", "number", "date", "url", "datetime",
}

func (m Model) openActionsPicker() (Model, tea.Cmd) {
	m.overlay = overlayPicker
	m.pickerKind = pickerActions
	m.pickerQ = ""
	m.pickerCur = 0
	m.pickerAll = []pickItem{
		{Kind: pickerActions, Label: "New object…", ID: "new", Sub: "c in browse"},
		{Kind: pickerActions, Label: "Quick capture (add)…", ID: "capture"},
		{Kind: pickerActions, Label: "Set / edit field or trait…", ID: "set", Sub: "s"},
		{Kind: pickerActions, Label: "Move / rename object…", ID: "move"},
		{Kind: pickerActions, Label: "Reclassify object…", ID: "reclassify"},
		{Kind: pickerActions, Label: "Delete object to trash…", ID: "delete"},
		{Kind: pickerActions, Label: "Schema: new type…", ID: "schema_add_type"},
		{Kind: pickerActions, Label: "Schema: add field…", ID: "schema_add_field"},
		{Kind: pickerActions, Label: "Schema: add trait…", ID: "schema_add_trait"},
		{Kind: pickerActions, Label: "Schema: rename type…", ID: "schema_rename_type"},
		{Kind: pickerActions, Label: "Schema: rename field…", ID: "schema_rename_field"},
	}
	m.refilterPicker()
	m.status = "actions"
	return m, nil
}

func (m Model) runAction(id string) (Model, tea.Cmd) {
	switch id {
	case "new":
		return m.startNewObject()
	case "capture":
		return m.startCapture()
	case "set":
		return m.startSet()
	case "move":
		return m.startMove()
	case "reclassify":
		return m.startReclassify()
	case "delete":
		return m.startDelete()
	case "schema_add_type":
		return m.startSchemaAddType()
	case "schema_add_field":
		return m.startSchemaAddField()
	case "schema_add_trait":
		return m.startSchemaAddTrait()
	case "schema_rename_type":
		return m.startSchemaRenameType()
	case "schema_rename_field":
		return m.startSchemaRenameField()
	}
	m.overlay = overlayNone
	return m, nil
}

func (m Model) startNewObject() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutNew, fields: map[string]any{}, step: "type"}
	p := m.p()
	if p != nil && strings.TrimSpace(p.typeName) != "" {
		m.mut.objType = p.typeName
		m.prefillNewFields(p)
		return m.promptNewTitle()
	}
	return m.openMutTypePicker()
}

func (m *Model) prefillNewFields(p *viewState) {
	if p == nil {
		return
	}
	if m.mut.fields == nil {
		m.mut.fields = map[string]any{}
	}
	ts := m.schema.Types[m.mut.objType]
	if p.spec.Kind == kindBoard && p.groupBy != "" && p.boardCol >= 0 && p.boardCol < len(p.board) {
		if key := p.board[p.boardCol].Key; key != "" {
			m.mut.fields[p.groupBy] = key
		}
	}
	for k, v := range p.sourceInputs {
		if _, ok := ts.Fields[k]; ok && strings.TrimSpace(v) != "" {
			if _, exists := m.mut.fields[k]; !exists {
				m.mut.fields[k] = v
			}
		}
	}
}

func (m Model) promptNewTitle() (Model, tea.Cmd) {
	m.mut.step = "title"
	return m.openPrompt("title", "", false)
}

func (m Model) commitNewPrompt(val string) (Model, tea.Cmd) {
	switch m.mut.step {
	case "title":
		m.mut.title = val
		ts := m.schema.Types[m.mut.objType]
		if nf := strings.TrimSpace(ts.NameField); nf != "" {
			if _, ok := m.mut.fields[nf]; !ok {
				m.mut.fields[nf] = val
			}
		}
		m.mut.need = requiredFieldsWithoutDefault(ts, m.mut.fields)
		m.mut.needIdx = 0
		return m.nextNewField()
	case "field", "value":
		return m.commitNewFieldValue(val)
	}
	return m, nil
}

func (m Model) nextNewField() (Model, tea.Cmd) {
	if m.mut.needIdx >= len(m.mut.need) {
		return m.invokeNew(m.mut.objType, m.mut.title, m.mut.fields)
	}
	name := m.mut.need[m.mut.needIdx]
	ts := m.schema.Types[m.mut.objType]
	fs := ts.Fields[name]
	m.mut.field = name
	m.mut.schema = fs
	m.mut.optional = false
	m.mut.current = ""
	m.mut.step = "value"
	m.mut.kind = mutNew
	return m.openValueWidget()
}

func (m Model) commitNewFieldValue(raw string) (Model, tea.Cmd) {
	val, err := api.CoerceFieldValue(m.mut.schema.Type, raw)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	if m.mut.fields == nil {
		m.mut.fields = map[string]any{}
	}
	m.mut.fields[m.mut.field] = val
	m.mut.needIdx++
	return m.nextNewField()
}

func requiredFieldsWithoutDefault(ts api.TypeSchema, have map[string]any) []string {
	var out []string
	nameField := strings.TrimSpace(ts.NameField)
	for _, name := range ts.FieldNames() {
		if name == nameField {
			continue
		}
		fs := ts.Fields[name]
		if !fs.Required || strings.TrimSpace(fs.Default) != "" {
			continue
		}
		if _, ok := have[name]; ok {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (m Model) startCapture() (Model, tea.Cmd) {
	to := ""
	if m.screen == kindNote && m.note != nil {
		to = m.note.ObjectID
	}
	m.mut = mutState{kind: mutCapture, captureTo: to, step: "text"}
	label := "capture"
	if to != "" {
		label = "capture to " + to
	}
	return m.openPrompt(label, "", false)
}

func (m Model) startMove() (Model, tea.Cmd) {
	obj := m.targetFileObject()
	if obj == nil || obj.ID == "" {
		m.status = "no object"
		return m, nil
	}
	m.mut = mutState{kind: mutMove, ref: obj.ID, restoreID: obj.ID, step: "dest", dryRun: true}
	return m.openPrompt("destination", obj.ID, false)
}

func (m Model) startDelete() (Model, tea.Cmd) {
	obj := m.targetFileObject()
	if obj == nil || obj.ID == "" {
		m.status = "no object"
		return m, nil
	}
	m.mut = mutState{kind: mutDelete, ref: obj.ID, restoreID: obj.ID, dryRun: true}
	return m.invokeDelete()
}

func (m Model) startReclassify() (Model, tea.Cmd) {
	obj := m.targetFileObject()
	if obj == nil || obj.ID == "" {
		m.status = "no object"
		return m, nil
	}
	m.mut = mutState{kind: mutReclassify, ref: obj.ID, objType: obj.Type, fields: map[string]any{}, restoreID: obj.ID, step: "type"}
	return m.openMutTypePicker()
}

func (m Model) startReclassifyFields() (Model, tea.Cmd) {
	obj := m.targetFileObject()
	have := map[string]any{}
	if obj != nil {
		for k, v := range obj.Fields {
			have[k] = v
		}
	}
	ts := m.schema.Types[m.mut.newType]
	m.mut.need = requiredFieldsWithoutDefault(ts, have)
	m.mut.needIdx = 0
	m.mut.fields = map[string]any{}
	return m.nextReclassifyField()
}

func (m Model) nextReclassifyField() (Model, tea.Cmd) {
	if m.mut.needIdx >= len(m.mut.need) {
		m.mut.force = false
		return m.invokeReclassify()
	}
	name := m.mut.need[m.mut.needIdx]
	ts := m.schema.Types[m.mut.newType]
	m.mut.field = name
	m.mut.schema = ts.Fields[name]
	m.mut.optional = false
	m.mut.current = ""
	m.mut.step = "value"
	return m.openValueWidget()
}

func (m Model) commitReclassifyField(raw string) (Model, tea.Cmd) {
	val, err := api.CoerceFieldValue(m.mut.schema.Type, raw)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	if m.mut.fields == nil {
		m.mut.fields = map[string]any{}
	}
	m.mut.fields[m.mut.field] = val
	m.mut.needIdx++
	return m.nextReclassifyField()
}

func (m Model) startSchemaAddType() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutSchemaAddType, step: "name"}
	return m.openPrompt("type name", "", false)
}

func (m Model) commitSchemaAddTypePrompt(val string) (Model, tea.Cmd) {
	switch m.mut.step {
	case "name":
		m.mut.name = val
		m.mut.step = "name-field"
		return m.openPrompt("name-field (empty skip)", "", true)
	case "name-field":
		m.mut.nameField = val
		m.mut.step = "default-path"
		return m.openPrompt("default-path (empty skip)", "", true)
	case "default-path":
		m.mut.defaultPath = val
		return m.invokeSchemaAddType()
	}
	return m, nil
}

func (m Model) startSchemaAddField() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutSchemaAddField, step: "type"}
	if p := m.p(); p != nil && strings.TrimSpace(p.typeName) != "" {
		m.mut.typeName = p.typeName
		m.mut.step = "field-name"
		return m.openPrompt("field name", "", false)
	}
	return m.openMutTypePicker()
}

func (m Model) commitSchemaAddFieldPrompt(val string) (Model, tea.Cmd) {
	switch m.mut.step {
	case "field-name":
		m.mut.fieldName = val
		return m.openChoicePicker("field type", schemaFieldTypes, "field-type")
	case "values":
		m.mut.fieldValues = val
		return m.afterEnumValues()
	case "default":
		m.mut.fieldDef = val
		return m.invokeSchemaAddField()
	}
	return m, nil
}

func (m Model) afterFieldTypeChosen() (Model, tea.Cmd) {
	base, _ := api.ParseFieldType(m.mut.fieldType)
	switch strings.ToLower(base) {
	case "enum":
		m.mut.step = "values"
		return m.openPrompt("enum values (comma-separated)", "", false)
	case "ref":
		m.mut.step = "target"
		return m.openMutTypePicker()
	default:
		return m.promptRequired()
	}
}

func (m Model) afterEnumValues() (Model, tea.Cmd) {
	if m.mut.kind == mutSchemaAddTrait {
		m.mut.step = "default"
		return m.openPrompt("default (empty skip)", "", true)
	}
	return m.promptRequired()
}

func (m Model) promptRequired() (Model, tea.Cmd) {
	return m.openChoicePicker("required", []string{"yes", "no"}, "required")
}

func (m Model) startSchemaAddTrait() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutSchemaAddTrait, step: "name"}
	return m.openPrompt("trait name", "", false)
}

func (m Model) commitSchemaAddTraitPrompt(val string) (Model, tea.Cmd) {
	switch m.mut.step {
	case "name":
		m.mut.traitName = val
		return m.openChoicePicker("trait type", schemaTraitTypes, "trait-type")
	case "values":
		m.mut.fieldValues = val
		m.mut.step = "default"
		return m.openPrompt("default (empty skip)", "", true)
	case "default":
		m.mut.fieldDef = val
		return m.invokeSchemaAddTrait()
	}
	return m, nil
}

func (m Model) afterTraitTypeChosen() (Model, tea.Cmd) {
	if strings.EqualFold(m.mut.fieldType, "enum") {
		m.mut.step = "values"
		return m.openPrompt("enum values (comma-separated)", "", false)
	}
	m.mut.step = "default"
	return m.openPrompt("default (empty skip)", "", true)
}

func (m Model) startSchemaRenameType() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutSchemaRenameType, step: "type", dryRun: true}
	return m.openMutTypePicker()
}

func (m Model) startSchemaRenameField() (Model, tea.Cmd) {
	m.mut = mutState{kind: mutSchemaRenameField, step: "type", dryRun: true}
	if p := m.p(); p != nil && strings.TrimSpace(p.typeName) != "" {
		m.mut.typeName = p.typeName
		return m.openRenameFieldPicker(p.typeName)
	}
	return m.openMutTypePicker()
}

func (m Model) openRenameFieldPicker(typeName string) (Model, tea.Cmd) {
	ts := m.schema.Types[typeName]
	obj := api.Object{Type: typeName, Fields: map[string]any{}}
	if cur := m.targetObject(); cur != nil && cur.Type == typeName {
		obj = *cur
	}
	m.mut.step = "field"
	return m.openFieldPicker(obj, ts)
}
