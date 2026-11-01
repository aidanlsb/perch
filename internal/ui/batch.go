package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type batchItem struct {
	ID   string
	Err  error
	Data any
}

type boardOp struct {
	ID    string
	Field string
	Key   string
}

func (m Model) invokeBatch(kind mutKind, ids []string, fn func(ctx context.Context, id string) (any, error)) (Model, tea.Cmd) {
	m.overlay = overlayNone
	m.loading = true
	c := m.client
	n := len(ids)
	if p := m.focusedView(); p != nil && m.mut.restoreID == "" {
		m.mut.restoreID = p.selectedRowID()
	}
	return m, func() tea.Msg {
		if c == nil {
			return mutateMsg{kind: kind, err: errNoClient}
		}
		d := 15 * time.Second
		if n > 1 {
			d *= time.Duration(n)
			if d > 60*time.Second {
				d = 60 * time.Second
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		items := make([]batchItem, 0, n)
		for _, id := range ids {
			data, err := fn(ctx, id)
			items = append(items, batchItem{ID: id, Data: data, Err: err})
		}
		return mutateMsg{kind: kind, batch: items}
	}
}

func (m Model) mutIDs() []string {
	if len(m.mut.targets) > 0 {
		return m.mut.targets
	}
	if m.mut.ref != "" {
		return []string{m.mut.ref}
	}
	if m.mut.traitID != "" {
		return []string{m.mut.traitID}
	}
	return nil
}

func (m Model) isMulti() bool {
	return len(m.mutIDs()) > 1
}

func (m Model) applySetFields(fields map[string]any) (Model, tea.Cmd) {
	if m.mut.fields == nil {
		m.mut.fields = map[string]any{}
	}
	for k, v := range fields {
		m.mut.fields[k] = v
	}
	ids := m.mutIDs()
	if len(ids) > 1 {
		label := m.mut.field
		val := ""
		if label != "" {
			val = api.ScalarString(fields[label])
		}
		if val == "" && len(fields) == 1 {
			for k, v := range fields {
				label = k
				val = api.ScalarString(v)
			}
		}
		return m.openConfirm(fmt.Sprintf("set %s → %s on %d objects [enter/esc]", label, val, len(ids)))
	}
	id := m.mut.ref
	if id == "" && len(ids) == 1 {
		id = ids[0]
	}
	return m.invokeSet(id, fields)
}

func (m Model) applyUnsetFields(fields []string) (Model, tea.Cmd) {
	m.mut.unsetOp = true
	ids := m.mutIDs()
	if len(ids) > 1 {
		name := strings.Join(fields, ", ")
		if name == "" {
			name = m.mut.field
		}
		return m.openConfirm(fmt.Sprintf("unset %s on %d objects [enter/esc]", name, len(ids)))
	}
	id := m.mut.ref
	if id == "" && len(ids) == 1 {
		id = ids[0]
	}
	return m.invokeUnset(id, fields)
}

func (m Model) applyTraitValue(value string) (Model, tea.Cmd) {
	m.mut.current = value
	ids := m.mutIDs()
	if len(ids) > 1 {
		return m.openConfirm(fmt.Sprintf("set trait → %s on %d rows [enter/esc]", value, len(ids)))
	}
	id := m.mut.traitID
	if id == "" && len(ids) == 1 {
		id = ids[0]
	}
	return m.invokeUpdateTrait(id, value)
}

func (m Model) invokeSetBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	fields := m.mut.fields
	if len(ids) <= 1 {
		id := m.mut.ref
		if id == "" && len(ids) == 1 {
			id = ids[0]
		}
		return m.invokeSet(id, fields)
	}
	c := m.client
	return m.invokeBatch(mutSet, ids, func(ctx context.Context, id string) (any, error) {
		return c.SetFields(ctx, id, fields)
	})
}

func (m Model) invokeUnsetBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	fields := []string{m.mut.field}
	if m.mut.field == "" && len(m.mut.fields) > 0 {
		fields = fields[:0]
		for k := range m.mut.fields {
			fields = append(fields, k)
		}
	}
	if len(ids) <= 1 {
		id := m.mut.ref
		if id == "" && len(ids) == 1 {
			id = ids[0]
		}
		return m.invokeUnset(id, fields)
	}
	c := m.client
	return m.invokeBatch(mutSet, ids, func(ctx context.Context, id string) (any, error) {
		return c.UnsetFields(ctx, id, fields)
	})
}

func (m Model) invokeTraitBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	val := m.mut.current
	if len(ids) <= 1 {
		id := m.mut.traitID
		if id == "" && len(ids) == 1 {
			id = ids[0]
		}
		return m.invokeUpdateTrait(id, val)
	}
	c := m.client
	return m.invokeBatch(mutTrait, ids, func(ctx context.Context, id string) (any, error) {
		return c.UpdateTrait(ctx, id, val)
	})
}

func (m Model) invokeDeleteBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	if len(ids) <= 1 {
		return m.invokeDelete()
	}
	dry := m.mut.dryRun
	c := m.client
	return m.invokeBatch(mutDelete, ids, func(ctx context.Context, id string) (any, error) {
		return c.DeleteObject(ctx, id, dry)
	})
}

func (m Model) invokeMoveBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	if len(ids) <= 1 {
		return m.invokeMove()
	}
	dests := m.mut.dests
	if dests == nil {
		dests = map[string]string{}
		multi := len(ids) > 1
		for _, id := range ids {
			dests[id] = moveDestFor(id, m.mut.dest, multi)
		}
		m.mut.dests = dests
	}
	dry, skip := m.mut.dryRun, m.mut.skipTypeCheck
	c := m.client
	return m.invokeBatch(mutMove, ids, func(ctx context.Context, id string) (any, error) {
		return c.MoveObject(ctx, id, dests[id], dry, skip)
	})
}

func (m Model) invokeReclassifyBatch() (Model, tea.Cmd) {
	ids := m.mutIDs()
	if len(ids) <= 1 {
		return m.invokeReclassify()
	}
	nt, fields, force := m.mut.newType, m.mut.fields, m.mut.force
	c := m.client
	return m.invokeBatch(mutReclassify, ids, func(ctx context.Context, id string) (any, error) {
		return c.ReclassifyObject(ctx, id, nt, fields, force)
	})
}

func (m Model) invokeBoardOps() (Model, tea.Cmd) {
	ops := append([]boardOp(nil), m.mut.boardOps...)
	if len(ops) == 0 {
		return m, nil
	}
	if len(ops) == 1 {
		op := ops[0]
		m.mut.kind = mutSet
		m.mut.ref = op.ID
		m.mut.field = op.Field
		m.mut.restoreID = op.ID
		if op.Key == "" {
			return m.invokeUnset(op.ID, []string{op.Field})
		}
		return m.invokeSet(op.ID, map[string]any{op.Field: op.Key})
	}
	m.overlay = overlayNone
	m.loading = true
	c := m.client
	kind := mutSet
	return m, func() tea.Msg {
		if c == nil {
			return mutateMsg{kind: kind, err: errNoClient}
		}
		d := 15 * time.Second * time.Duration(len(ops))
		if d > 60*time.Second {
			d = 60 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		items := make([]batchItem, 0, len(ops))
		for _, op := range ops {
			var data any
			var err error
			if op.Key == "" {
				data, err = c.UnsetFields(ctx, op.ID, []string{op.Field})
			} else {
				data, err = c.SetFields(ctx, op.ID, map[string]any{op.Field: op.Key})
			}
			items = append(items, batchItem{ID: op.ID, Data: data, Err: err})
		}
		return mutateMsg{kind: kind, batch: items}
	}
}

func (m Model) onMutateBatch(msg mutateMsg) (Model, tea.Cmd) {
	if m.mut.kind != mutNone && m.mut.kind != msg.kind {
		return m, nil
	}
	switch msg.kind {
	case mutDelete:
		if m.mut.dryRun {
			return m.confirmDeleteBatch(msg.batch)
		}
		return m.finishBatch(msg, "trashed")
	case mutMove:
		if m.mut.dryRun {
			return m.confirmMoveBatch(msg.batch)
		}
		return m.finishBatch(msg, "moved")
	case mutReclassify:
		if !m.mut.force && batchNeedsConfirm(msg.batch) {
			m.mut.force = true
			return m.openConfirm(reclassifyBatchText(msg.batch))
		}
		return m.finishBatch(msg, "reclassified")
	case mutTrait:
		return m.finishBatch(msg, "updated trait")
	case mutSet:
		label := m.mut.field
		if label == "" {
			label = "set"
		}
		return m.finishBatch(msg, "set "+label)
	}
	return m.finishBatch(msg, "ok")
}

func (m Model) confirmDeleteBatch(items []batchItem) (Model, tea.Cmd) {
	m.mut.dryRun = false
	backlinks := 0
	var fails []string
	for _, it := range items {
		if it.Err != nil {
			fails = append(fails, it.ID+": "+it.Err.Error())
			continue
		}
		if res, ok := it.Data.(*api.DeleteResult); ok {
			backlinks += res.BacklinkCount()
		}
	}
	text := fmt.Sprintf("Move %d objects to trash? (%d backlinks) [enter/esc]", len(items), backlinks)
	if len(fails) > 0 {
		text += "\n" + strings.Join(fails, "\n")
	}
	return m.openConfirm(text)
}

func (m Model) confirmMoveBatch(items []batchItem) (Model, tea.Cmd) {
	refs := 0
	needs := false
	var fails []string
	for _, it := range items {
		if it.Err != nil {
			fails = append(fails, it.ID+": "+it.Err.Error())
			continue
		}
		res, ok := it.Data.(*api.MoveResult)
		if !ok {
			continue
		}
		refs += res.UpdatedRefCount()
		if res.NeedsConfirm {
			needs = true
		}
	}
	if needs {
		m.mut.skipTypeCheck = true
	}
	m.mut.dryRun = false
	text := fmt.Sprintf("Move %d objects → %s? (%d refs) [enter/esc]", len(items), m.mut.dest, refs)
	if len(fails) > 0 {
		text += "\n" + strings.Join(fails, "\n")
	}
	return m.openConfirm(text)
}

func batchNeedsConfirm(items []batchItem) bool {
	for _, it := range items {
		if it.Err != nil {
			continue
		}
		if res, ok := it.Data.(*api.ReclassifyResult); ok && res.NeedsConfirm {
			return true
		}
	}
	return false
}

func reclassifyBatchText(items []batchItem) string {
	dropped := map[string]struct{}{}
	for _, it := range items {
		res, ok := it.Data.(*api.ReclassifyResult)
		if !ok {
			continue
		}
		for _, f := range res.DroppedFields {
			dropped[f] = struct{}{}
		}
		if res.Reason != "" && len(res.DroppedFields) == 0 {
			dropped[res.Reason] = struct{}{}
		}
	}
	names := make([]string, 0, len(dropped))
	for n := range dropped {
		names = append(names, n)
	}
	sort.Strings(names)
	list := strings.Join(names, ", ")
	if list == "" {
		list = "fields"
	}
	return fmt.Sprintf("Reclassify %d objects drops %s [enter/esc]", len(items), list)
}

func (m *Model) rememberBatchItem(kind mutKind, it batchItem) {
	switch kind {
	case mutTrait:
		m.rememberTraitOverlayFor(it.ID, it.Data)
	case mutSet:
		if op, ok := m.boardOpFor(it.ID); ok {
			if op.Key == "" {
				field := op.Field
				if field == "" {
					field = m.mut.field
				}
				names := m.mut.need
				if field != "" {
					names = []string{field}
				}
				m.addOverlay(objectOverlay{id: it.ID, unset: names})
				return
			}
			m.rememberSetOverlayFor(it.ID, map[string]any{op.Field: op.Key}, it.Data)
			return
		}
		m.rememberSetOverlayFor(it.ID, m.mut.fields, it.Data)
	}
}

func (m *Model) boardOpFor(id string) (boardOp, bool) {
	for _, op := range m.mut.boardOps {
		if op.ID == id {
			return op, true
		}
	}
	return boardOp{}, false
}

func (m Model) finishBatch(msg mutateMsg, verb string) (Model, tea.Cmd) {
	applied := 0
	var failed []batchItem
	for _, it := range msg.batch {
		if it.Err != nil {
			failed = append(failed, it)
			continue
		}
		applied++
		m.rememberBatchItem(msg.kind, it)
	}
	status := formatBatchStatus(applied, len(msg.batch), failed)
	if verb != "" && len(failed) == 0 {
		status = fmt.Sprintf("%s on %d objects", verb, applied)
		if applied == 1 {
			status = verb
		}
	}
	m, cmd := m.afterApplied(status)
	if len(failed) > 0 {
		m.mut.resultOnly = true
		m.overlay = overlayConfirm
		m.mut.confirmText = batchFailText(status, failed)
		m.status = status
	}
	return m, cmd
}

func batchFailText(status string, failed []batchItem) string {
	var b strings.Builder
	b.WriteString(status)
	b.WriteByte('\n')
	for _, it := range failed {
		b.WriteString(it.ID)
		b.WriteString(": ")
		if it.Err != nil {
			b.WriteString(it.Err.Error())
		}
		b.WriteByte('\n')
	}
	b.WriteString("[enter/esc]")
	return b.String()
}

func moveDestFor(id, dest string, multi bool) string {
	dest = strings.TrimSpace(dest)
	if !multi {
		return dest
	}
	dest = strings.TrimRight(dest, "/")
	base := id
	if i := strings.LastIndex(id, "/"); i >= 0 && i+1 < len(id) {
		base = id[i+1:]
	}
	if dest == "" {
		return base
	}
	return dest + "/" + base
}
