package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

type mutKind string

const (
	mutNone              mutKind = ""
	mutSet               mutKind = "set"
	mutTrait             mutKind = "trait"
	mutNew               mutKind = "new"
	mutCapture           mutKind = "capture"
	mutMove              mutKind = "move"
	mutReclassify        mutKind = "reclassify"
	mutDelete            mutKind = "delete"
	mutSchemaAddType     mutKind = "schema_add_type"
	mutSchemaAddField    mutKind = "schema_add_field"
	mutSchemaAddTrait    mutKind = "schema_add_trait"
	mutSchemaRenameType  mutKind = "schema_rename_type"
	mutSchemaRenameField mutKind = "schema_rename_field"
	mutSaveQuery         mutKind = "save_query"
)

const unsetPickID = "__unset__"

type mutState struct {
	kind          mutKind
	step          string
	ref           string
	objType       string
	traitID       string
	traitType     string
	field         string
	schema        api.FieldSchema
	current       string
	optional      bool
	allowEmpty    bool
	title         string
	fields        map[string]any
	need          []string
	needIdx       int
	dest          string
	newType       string
	force         bool
	dryRun        bool
	skipTypeCheck bool
	confirmText   string
	promptLabel   string
	promptBuf     string
	captureTo     string
	name          string
	nameField     string
	defaultPath   string
	fieldName     string
	fieldType     string
	fieldValues   string
	fieldTarget   string
	fieldReq      bool
	fieldDef      string
	traitName     string
	oldName       string
	newName       string
	typeName      string
	restoreID     string
	openType      string
	viewName      string
	view          config.View
}

type mutateMsg struct {
	kind mutKind
	err  error
	data any
}

type dailyNoteMsg struct {
	gen    int
	date   string
	result *api.DailyNoteResult
	err    error
}

func (m *Model) clearMut() {
	m.mut = mutState{}
	if m.datePurpose == "value" {
		m.datePurpose = ""
	}
}

func (m Model) keysPrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		m.clearMut()
		return m.dismissOverlay()
	case "enter":
		return m.commitPrompt()
	case "backspace":
		m.mut.promptBuf = dropLastRune(m.mut.promptBuf)
		return m, nil
	case "ctrl+d":
		if m.mut.optional && (m.mut.kind == mutSet) && m.mut.step == "value" {
			return m.applyUnset()
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	if text := typedText(msg); text != "" {
		m.mut.promptBuf += text
	}
	return m, nil
}

func (m Model) keysConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc", "q":
		m.clearMut()
		return m.dismissOverlay()
	case "enter":
		return m.applyConfirmed()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) openPrompt(label, initial string, allowEmpty bool) (Model, tea.Cmd) {
	m.overlay = overlayPrompt
	m.mut.promptLabel = label
	m.mut.promptBuf = initial
	m.mut.allowEmpty = allowEmpty
	m.status = label
	return m, nil
}

func (m Model) openConfirm(text string) (Model, tea.Cmd) {
	m.overlay = overlayConfirm
	m.mut.confirmText = text
	m.status = text
	return m, nil
}

func (m Model) commitPrompt() (Model, tea.Cmd) {
	val := strings.TrimSpace(m.mut.promptBuf)
	if val == "" && !m.mut.allowEmpty {
		m.status = m.mut.promptLabel + " required"
		return m, nil
	}
	switch m.mut.kind {
	case mutSet, mutTrait:
		return m.commitValue(val)
	case mutNew:
		return m.commitNewPrompt(val)
	case mutCapture:
		return m.invokeAdd(val, m.mut.captureTo)
	case mutMove:
		m.mut.dest = val
		m.mut.dryRun = true
		return m.invokeMove()
	case mutReclassify:
		return m.commitReclassifyField(val)
	case mutSchemaAddType:
		return m.commitSchemaAddTypePrompt(val)
	case mutSchemaAddField:
		return m.commitSchemaAddFieldPrompt(val)
	case mutSchemaAddTrait:
		return m.commitSchemaAddTraitPrompt(val)
	case mutSchemaRenameType:
		m.mut.newName = val
		m.mut.dryRun = true
		return m.invokeSchemaRenameType(false)
	case mutSchemaRenameField:
		m.mut.newName = val
		m.mut.dryRun = true
		return m.invokeSchemaRenameField(false)
	}
	return m, nil
}

func (m Model) applyConfirmed() (Model, tea.Cmd) {
	switch m.mut.kind {
	case mutDelete:
		m.mut.dryRun = false
		return m.invokeDelete()
	case mutMove:
		m.mut.dryRun = false
		return m.invokeMove()
	case mutReclassify:
		m.mut.force = true
		return m.invokeReclassify()
	case mutSchemaRenameType:
		return m.invokeSchemaRenameType(true)
	case mutSchemaRenameField:
		return m.invokeSchemaRenameField(true)
	}
	return m, nil
}

func (m Model) applyUnset() (Model, tea.Cmd) {
	if m.mut.kind != mutSet || strings.TrimSpace(m.mut.field) == "" {
		m.clearMut()
		m.overlay = overlayNone
		return m, nil
	}
	ref := m.mut.ref
	field := m.mut.field
	m.mut.restoreID = ref
	return m.invokeUnset(ref, []string{field})
}

func (m Model) onMutate(msg mutateMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if m.mut.kind != mutNone && m.mut.kind != msg.kind {
		return m, nil
	}
	switch msg.kind {
	case mutDelete:
		res, _ := msg.data.(*api.DeleteResult)
		if res != nil && (res.Preview || m.mut.dryRun) {
			n := res.BacklinkCount()
			id := m.mut.ref
			m.mut.dryRun = false
			return m.openConfirm(fmt.Sprintf("Move %s to trash? (%d backlinks) [enter/esc]", id, n))
		}
		return m.afterApplied("trashed " + m.mut.ref)
	case mutMove:
		res, _ := msg.data.(*api.MoveResult)
		if m.mut.dryRun && res != nil {
			if res.NeedsConfirm {
				m.mut.skipTypeCheck = true
				m.mut.dryRun = false
				return m.openConfirm(fmt.Sprintf("Move %s → %s needs confirm [enter/esc]", m.mut.ref, m.mut.dest))
			}
			m.mut.dryRun = false
			return m.openConfirm(fmt.Sprintf("Move %s → %s? (%d refs) [enter/esc]", m.mut.ref, m.mut.dest, res.UpdatedRefCount()))
		}
		dest := m.mut.dest
		m.mut.restoreID = dest
		return m.afterApplied("moved to " + dest)
	case mutReclassify:
		res, _ := msg.data.(*api.ReclassifyResult)
		if res != nil && res.NeedsConfirm && !m.mut.force {
			m.mut.force = true
			dropped := strings.Join(res.DroppedFields, ", ")
			if dropped == "" {
				dropped = res.Reason
			}
			return m.openConfirm(fmt.Sprintf("Reclassify drops %s [enter/esc]", dropped))
		}
		return m.afterApplied("reclassified " + m.mut.ref)
	case mutSchemaRenameType, mutSchemaRenameField:
		res, _ := msg.data.(*api.SchemaRenameResult)
		if res != nil && (res.Preview || m.mut.dryRun) {
			m.mut.dryRun = false
			text := fmt.Sprintf("%d changes", res.TotalChanges)
			if msg.kind == mutSchemaRenameType {
				text = fmt.Sprintf("%d changes, %d files to move [enter/esc]", res.TotalChanges, res.FilesToMove)
			} else {
				text += " [enter/esc]"
			}
			return m.openConfirm(text)
		}
		return m.afterApplied("renamed")
	case mutNew:
		res, _ := msg.data.(*api.NewObjectResult)
		id := m.mut.restoreID
		if res != nil && res.ID != "" {
			id = res.ID
			m.mut.restoreID = id
		}
		return m.afterApplied("created " + id)
	case mutSaveQuery:
		return m.afterApplied("saved views." + m.mut.viewName)
	case mutSchemaAddType:
		return m.afterApplied("added type " + m.mut.name)
	case mutSchemaAddField:
		return m.afterApplied("added field " + m.mut.fieldName)
	case mutSchemaAddTrait:
		return m.afterApplied("added trait " + m.mut.traitName)
	case mutSet:
		return m.afterApplied("set " + m.mut.field)
	case mutTrait:
		return m.afterApplied("updated trait")
	case mutCapture:
		return m.afterApplied("captured")
	}
	return m.afterApplied("ok")
}

func (m Model) afterApplied(status string) (Model, tea.Cmd) {
	restore := m.mut.restoreID
	openType := m.mut.openType
	kind := m.mut.kind
	view := m.mut.view
	viewName := m.mut.viewName
	cfgPath := strings.TrimSpace(m.cfgPath)
	m.overlay = overlayNone
	m.errMsg = ""
	m.status = status
	reloadSchema := kind == mutSchemaAddType || kind == mutSchemaAddField || kind == mutSchemaAddTrait ||
		kind == mutSchemaRenameType || kind == mutSchemaRenameField || kind == mutSaveQuery
	if kind == mutSaveQuery {
		if cfgPath == "" {
			m.clearMut()
			m.errMsg = "no config path"
			return m, nil
		}
		if err := config.UpsertView(cfgPath, viewName, view); err != nil {
			m.clearMut()
			m.errMsg = err.Error()
			return m, nil
		}
		m2, err := m.reloadConfig()
		m = m2
		if err != nil {
			m.clearMut()
			m.errMsg = err.Error()
			m.status = "config error; keeping last good"
			return m, nil
		}
		m.pendingOpenView = viewName
	}
	if openType != "" {
		m.pendingOpenType = openType
		reloadSchema = true
	}
	m.clearMut()
	if reloadSchema {
		return m, fetchStartup(m.client, true)
	}
	m, cmd := m.liveRefetch(true)
	if p := m.focusedView(); p != nil && restore != "" {
		p.restoreID = restore
		p.keepCursor = true
	}
	return m, cmd
}

func runMut(c *api.Client, kind mutKind, fn func(context.Context) (any, error)) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return mutateMsg{kind: kind, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		data, err := fn(ctx)
		return mutateMsg{kind: kind, data: data, err: err}
	}
}

func (m Model) invokeSet(ref string, fields map[string]any) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	m.mut.kind = mutSet
	m.mut.restoreID = ref
	c := m.client
	return m, runMut(c, mutSet, func(ctx context.Context) (any, error) {
		return c.SetFields(ctx, ref, fields)
	})
}

func (m Model) invokeUnset(ref string, fields []string) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	m.mut.kind = mutSet
	m.mut.restoreID = ref
	c := m.client
	return m, runMut(c, mutSet, func(ctx context.Context) (any, error) {
		return c.UnsetFields(ctx, ref, fields)
	})
}

func (m Model) invokeUpdateTrait(id, value string) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	m.mut.kind = mutTrait
	m.mut.restoreID = id
	c := m.client
	return m, runMut(c, mutTrait, func(ctx context.Context) (any, error) {
		return c.UpdateTrait(ctx, id, value)
	})
}

func (m Model) invokeNew(typeName, title string, fields map[string]any) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	m.mut.kind = mutNew
	c := m.client
	return m, runMut(c, mutNew, func(ctx context.Context) (any, error) {
		return c.NewObject(ctx, typeName, title, fields)
	})
}

func (m Model) invokeAdd(text, to string) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	m.mut.kind = mutCapture
	c := m.client
	return m, runMut(c, mutCapture, func(ctx context.Context) (any, error) {
		return c.AddCapture(ctx, text, to)
	})
}

func (m Model) invokeDelete() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	ref := m.mut.ref
	dry := m.mut.dryRun
	c := m.client
	return m, runMut(c, mutDelete, func(ctx context.Context) (any, error) {
		return c.DeleteObject(ctx, ref, dry)
	})
}

func (m Model) invokeMove() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	src, dest := m.mut.ref, m.mut.dest
	dry, skip := m.mut.dryRun, m.mut.skipTypeCheck
	c := m.client
	return m, runMut(c, mutMove, func(ctx context.Context) (any, error) {
		return c.MoveObject(ctx, src, dest, dry, skip)
	})
}

func (m Model) invokeReclassify() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	ref, nt := m.mut.ref, m.mut.newType
	fields, force := m.mut.fields, m.mut.force
	c := m.client
	return m, runMut(c, mutReclassify, func(ctx context.Context) (any, error) {
		return c.ReclassifyObject(ctx, ref, nt, fields, force)
	})
}

func (m Model) invokeSaveQuery(name, rql string) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	c := m.client
	return m, runMut(c, mutSaveQuery, func(ctx context.Context) (any, error) {
		return c.SaveQuery(ctx, name, rql, nil, "")
	})
}

func (m Model) invokeSchemaAddType() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	name, nf, dp := m.mut.name, m.mut.nameField, m.mut.defaultPath
	m.mut.openType = name
	c := m.client
	return m, runMut(c, mutSchemaAddType, func(ctx context.Context) (any, error) {
		return c.SchemaAddType(ctx, name, nf, dp, "")
	})
}

func (m Model) invokeSchemaAddField() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	in := api.SchemaAddFieldArgs{
		TypeName:  m.mut.typeName,
		FieldName: m.mut.fieldName,
		Type:      m.mut.fieldType,
		Required:  m.mut.fieldReq,
		Default:   m.mut.fieldDef,
		Target:    m.mut.fieldTarget,
		Values:    splitCommaList(m.mut.fieldValues),
	}
	c := m.client
	return m, runMut(c, mutSchemaAddField, func(ctx context.Context) (any, error) {
		return c.SchemaAddField(ctx, in)
	})
}

func (m Model) invokeSchemaAddTrait() (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	in := api.SchemaAddTraitArgs{
		Name:    m.mut.traitName,
		Type:    m.mut.fieldType,
		Values:  splitCommaList(m.mut.fieldValues),
		Default: m.mut.fieldDef,
	}
	c := m.client
	return m, runMut(c, mutSchemaAddTrait, func(ctx context.Context) (any, error) {
		return c.SchemaAddTrait(ctx, in)
	})
}

func (m Model) invokeSchemaRenameType(confirm bool) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	oldN, newN := m.mut.oldName, m.mut.newName
	c := m.client
	return m, runMut(c, mutSchemaRenameType, func(ctx context.Context) (any, error) {
		return c.SchemaRenameType(ctx, oldN, newN, confirm)
	})
}

func (m Model) invokeSchemaRenameField(confirm bool) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	tn, oldF, newF := m.mut.typeName, m.mut.field, m.mut.newName
	c := m.client
	return m, runMut(c, mutSchemaRenameField, func(ctx context.Context) (any, error) {
		return c.SchemaRenameField(ctx, tn, oldF, newF, confirm)
	})
}

func fetchEnsureDaily(c *api.Client, gen int, date string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return dailyNoteMsg{gen: gen, date: date, err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := c.EnsureDaily(ctx, date)
		return dailyNoteMsg{gen: gen, date: date, result: res, err: err}
	}
}

func splitCommaList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
