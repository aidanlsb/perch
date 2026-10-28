package ui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) pickerEscape() (Model, tea.Cmd) {
	if m.fromNav && (m.pickerKind == pickerViews || m.pickerKind == pickerTypes || m.pickerKind == pickerQueries) {
		m.fromNav = false
		m.clearMut()
		return m.openNavPicker()
	}
	if m.mut.kind != mutNone && m.pickerKind != pickerNav {
		return m.mutStepBack()
	}
	m.fromNav = false
	m.overlay = overlayNone
	return m, nil
}

func (m Model) argsEscape() (Model, tea.Cmd) {
	if m.argsIdx > 0 {
		name := m.argsNames[m.argsIdx]
		m.argsVals[name] = m.argsBuf
		m.argsIdx--
		prev := m.argsNames[m.argsIdx]
		m.argsBuf = m.argsVals[prev]
		return m, nil
	}
	if m.fromQueries {
		fromNav := m.fromNav
		m.fromQueries = false
		m.argsQuery = nil
		m.argsBuf = ""
		m.overlay = overlayNone
		m, cmd := m.openQueryPicker()
		m.fromNav = fromNav
		return m, cmd
	}
	m.argsQuery = nil
	m.argsBuf = ""
	m.fromQueries = false
	return m.dismissOverlay()
}

func (m Model) promptEscape() (Model, tea.Cmd) {
	if m.mut.kind != mutNone {
		return m.mutStepBack()
	}
	return m.dismissOverlay()
}

func (m Model) confirmEscape() (Model, tea.Cmd) {
	if m.mut.kind != mutNone {
		return m.mutStepBack()
	}
	return m.dismissOverlay()
}

func (m Model) dateEscape() (Model, tea.Cmd) {
	if m.datePurpose == "value" && m.mut.kind != mutNone {
		return m.mutStepBack()
	}
	return m.dismissOverlay()
}

func (m Model) mutStepBack() (Model, tea.Cmd) {
	switch m.mut.kind {
	case mutSet:
		if m.mut.step == "value" || m.mut.step == "date" {
			obj := m.targetObject()
			if obj == nil {
				return m.abortMut()
			}
			ts := m.schema.Types[obj.Type]
			if ts.Name == "" {
				ts = m.schema.Types[m.mut.objType]
			}
			return m.openFieldPicker(*obj, ts)
		}
		return m.abortMut()
	case mutTrait:
		return m.abortMut()
	case mutNew:
		return m.stepBackNew()
	case mutReclassify:
		return m.stepBackReclassify()
	case mutSchemaAddType:
		return m.stepBackSchemaAddType()
	case mutSchemaAddField:
		return m.stepBackSchemaAddField()
	case mutSchemaAddTrait:
		return m.stepBackSchemaAddTrait()
	case mutSchemaRenameType:
		if m.mut.step == "new" {
			m.mut.step = "type"
			return m.openMutTypePicker()
		}
		return m.abortMut()
	case mutSchemaRenameField:
		if m.mut.step == "new" {
			return m.openRenameFieldPicker(m.mut.typeName)
		}
		if m.mut.step == "field" {
			m.mut.step = "type"
			return m.openMutTypePicker()
		}
		return m.abortMut()
	case mutCapture, mutMove, mutDelete, mutSaveQuery:
		return m.abortMut()
	}
	return m.abortMut()
}

func (m Model) abortMut() (Model, tea.Cmd) {
	fromActions := m.mut.fromActions
	m.clearMut()
	m.overlay = overlayNone
	m.datePurpose = ""
	if fromActions {
		return m.openActionsPicker()
	}
	return m, nil
}

func (m Model) stepBackNew() (Model, tea.Cmd) {
	switch m.mut.step {
	case "value", "field":
		if m.mut.needIdx > 0 {
			m.mut.needIdx--
			name := m.mut.need[m.mut.needIdx]
			delete(m.mut.fields, name)
			return m.nextNewField()
		}
		return m.promptNewTitle()
	case "title":
		if m.mut.pickedType {
			m.mut.objType = ""
			m.mut.pickedType = false
			m.mut.step = "type"
			return m.openMutTypePicker()
		}
		return m.abortMut()
	default:
		return m.abortMut()
	}
}

func (m Model) stepBackReclassify() (Model, tea.Cmd) {
	switch m.mut.step {
	case "value", "field":
		if m.mut.needIdx > 0 {
			m.mut.needIdx--
			name := m.mut.need[m.mut.needIdx]
			delete(m.mut.fields, name)
			return m.nextReclassifyField()
		}
		m.mut.step = "type"
		return m.openMutTypePicker()
	default:
		return m.abortMut()
	}
}

func (m Model) stepBackSchemaAddType() (Model, tea.Cmd) {
	switch m.mut.step {
	case "default-path":
		m.mut.step = "name-field"
		return m.openPrompt("name-field (empty skip)", m.mut.nameField, true)
	case "name-field":
		m.mut.step = "name"
		return m.openPrompt("type name", m.mut.name, false)
	default:
		return m.abortMut()
	}
}

func (m Model) stepBackSchemaAddField() (Model, tea.Cmd) {
	switch m.mut.step {
	case "default":
		return m.promptRequired()
	case "required":
		base := m.mut.fieldType
		if base == "enum" || base == "enum[]" {
			m.mut.step = "values"
			return m.openPrompt("enum values (comma-separated)", m.mut.fieldValues, false)
		}
		if base == "ref" || base == "ref[]" {
			m.mut.step = "target"
			return m.openMutTypePicker()
		}
		m.mut.step = "field-type"
		return m.openChoicePicker("field type", schemaFieldTypes, "field-type")
	case "values", "target":
		m.mut.step = "field-type"
		return m.openChoicePicker("field type", schemaFieldTypes, "field-type")
	case "field-type":
		m.mut.step = "field-name"
		return m.openPrompt("field name", m.mut.fieldName, false)
	case "field-name":
		m.mut.step = "type"
		return m.openMutTypePicker()
	default:
		return m.abortMut()
	}
}

func (m Model) stepBackSchemaAddTrait() (Model, tea.Cmd) {
	switch m.mut.step {
	case "default":
		if m.mut.fieldType == "enum" {
			m.mut.step = "values"
			return m.openPrompt("enum values (comma-separated)", m.mut.fieldValues, false)
		}
		return m.openChoicePicker("trait type", schemaTraitTypes, "trait-type")
	case "values", "trait-type":
		m.mut.step = "name"
		return m.openPrompt("trait name", m.mut.traitName, false)
	default:
		return m.abortMut()
	}
}
