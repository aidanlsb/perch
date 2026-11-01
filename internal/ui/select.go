package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	cursorGlyph = "▸"
	markGlyph   = "┃"
)

func (v *viewState) ensureSel() {
	if v == nil {
		return
	}
	if v.selected == nil {
		v.selected = map[string]struct{}{}
	}
}

func (v *viewState) clearMarks() {
	if v == nil {
		return
	}
	v.selected = nil
	v.rangeAnchor = ""
}

func (v *viewState) hasMarks() bool {
	if v == nil {
		return false
	}
	return len(v.selected) > 0 || v.rangeAnchor != ""
}

func (v *viewState) committedCount() int {
	if v == nil {
		return 0
	}
	return len(v.selected)
}

func (v *viewState) markCount() int {
	return len(v.markedIDs())
}

func (v *viewState) isMarked(id string) bool {
	if v == nil || id == "" {
		return false
	}
	if _, ok := v.selected[id]; ok {
		return true
	}
	for _, rid := range v.rangeIDs() {
		if rid == id {
			return true
		}
	}
	return false
}

func (v *viewState) toggleID(id string) {
	if v == nil || id == "" {
		return
	}
	v.ensureSel()
	if _, ok := v.selected[id]; ok {
		delete(v.selected, id)
		return
	}
	v.selected[id] = struct{}{}
}

func (v *viewState) selectAllVisible() {
	if v == nil {
		return
	}
	v.rangeAnchor = ""
	v.selected = map[string]struct{}{}
	for _, o := range v.visibleObjects() {
		if o.ID != "" {
			v.selected[o.ID] = struct{}{}
		}
	}
}

func (v *viewState) startOrCommitRange() {
	if v == nil {
		return
	}
	if v.rangeAnchor != "" {
		v.commitRange()
		return
	}
	id := v.selectedRowID()
	if id == "" {
		return
	}
	v.rangeAnchor = id
}

func (v *viewState) commitRange() {
	if v == nil {
		return
	}
	v.ensureSel()
	for _, id := range v.rangeIDs() {
		v.selected[id] = struct{}{}
	}
	v.rangeAnchor = ""
}

func (v *viewState) rangeIDs() []string {
	if v == nil || v.rangeAnchor == "" {
		return nil
	}
	vis := v.visibleObjects()
	ai, ci := -1, -1
	cur := v.selectedRowID()
	for i, o := range vis {
		if o.ID == v.rangeAnchor {
			ai = i
		}
		if o.ID == cur {
			ci = i
		}
	}
	if ai < 0 || ci < 0 {
		return nil
	}
	lo, hi := ai, ci
	if lo > hi {
		lo, hi = hi, lo
	}
	out := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		if id := vis[i].ID; id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (v *viewState) markedIDs() []string {
	if v == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for id := range v.selected {
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, id := range v.rangeIDs() {
		seen[id] = struct{}{}
	}
	vis := v.visibleObjects()
	out := make([]string, 0, len(seen))
	for _, o := range vis {
		if _, ok := seen[o.ID]; ok {
			out = append(out, o.ID)
			delete(seen, o.ID)
		}
	}
	if len(seen) == 0 {
		return out
	}
	rest := make([]string, 0, len(seen))
	for id := range seen {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func (v *viewState) markedObjects() []api.Object {
	if v == nil {
		return nil
	}
	ids := v.markedIDs()
	if len(ids) == 0 {
		return nil
	}
	idx := map[string]api.Object{}
	for _, o := range v.visibleObjects() {
		idx[o.ID] = o
	}
	for _, o := range v.objects {
		if _, ok := idx[o.ID]; !ok {
			idx[o.ID] = o
		}
	}
	out := make([]api.Object, 0, len(ids))
	for _, id := range ids {
		if o, ok := idx[id]; ok {
			out = append(out, o)
		} else {
			out = append(out, api.Object{ID: id})
		}
	}
	return out
}

func (v *viewState) visibleObjects() []api.Object {
	if v == nil {
		return nil
	}
	switch v.spec.Kind {
	case kindBoard:
		var out []api.Object
		for _, col := range v.board {
			out = append(out, col.Items...)
		}
		return out
	case kindDate:
		var out []api.Object
		for _, col := range v.dateCols {
			out = append(out, col.Items...)
		}
		return out
	default:
		return v.filtered
	}
}

func (v *viewState) objectByID(id string) *api.Object {
	if v == nil || id == "" {
		return nil
	}
	for i := range v.objects {
		if v.objects[i].ID == id {
			o := v.objects[i]
			return &o
		}
	}
	for _, o := range v.visibleObjects() {
		if o.ID == id {
			cp := o
			return &cp
		}
	}
	return nil
}

func cloneIDSet(in map[string]struct{}) map[string]struct{} {
	if in == nil {
		return nil
	}
	out := make(map[string]struct{}, len(in))
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

func rowGutter(cursor, marked bool) string {
	switch {
	case cursor && marked:
		return cursorGlyph + markGlyph
	case cursor:
		return cursorGlyph + " "
	case marked:
		return " " + markGlyph
	default:
		return "  "
	}
}

func cardGutter(cursor, marked bool) string {
	switch {
	case cursor:
		return "▌"
	case marked:
		return markGlyph
	default:
		return " "
	}
}

func (m Model) keysSelect(key string) (Model, tea.Cmd, bool) {
	if m.focus == focusPreview {
		return m, nil, false
	}
	p := m.focusedView()
	if p == nil {
		p = m.p()
	}
	if p == nil {
		return m, nil, false
	}
	switch key {
	case "v":
		if p.rangeAnchor != "" {
			p.commitRange()
			return m, nil, true
		}
		if o := p.selectedObject(); o != nil {
			p.toggleID(o.ID)
		}
		p.move(1)
		m, cmd := m.afterMove()
		return m, cmd, true
	case "V":
		p.startOrCommitRange()
		return m, nil, true
	case "ctrl+a", "ctrl-a":
		p.selectAllVisible()
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) actionObjects() []api.Object {
	if m.screen == kindNote {
		if o := m.targetObject(); o != nil {
			return []api.Object{*o}
		}
		return nil
	}
	p := m.p()
	if p == nil {
		return nil
	}
	if p.hasMarks() {
		return p.markedObjects()
	}
	if o := p.selectedObject(); o != nil {
		return []api.Object{*o}
	}
	return nil
}

func (m Model) actionFileObjects() []api.Object {
	seen := map[string]struct{}{}
	var out []api.Object
	for _, o := range m.actionObjects() {
		fo := m.fileObjectOf(o)
		if fo == nil || fo.ID == "" {
			continue
		}
		if _, ok := seen[fo.ID]; ok {
			continue
		}
		seen[fo.ID] = struct{}{}
		out = append(out, *fo)
	}
	return out
}

func (m Model) fileObjectOf(o api.Object) *api.Object {
	if o.IsTrait() || !o.IsFileObject() {
		id := o.OpenRef()
		if id == "" {
			return nil
		}
		if p := m.p(); p != nil {
			if found := p.objectByID(id); found != nil {
				return found
			}
		}
		if existing, ok := m.noteIndex[id]; ok {
			cp := existing
			return &cp
		}
		return &api.Object{ID: id, Type: o.Type, Fields: map[string]any{}}
	}
	cp := o
	return &cp
}

func (m Model) objectByID(id string) *api.Object {
	if p := m.p(); p != nil {
		if o := p.objectByID(id); o != nil {
			return o
		}
	}
	if o, ok := m.noteIndex[id]; ok {
		cp := o
		return &cp
	}
	return nil
}

type commonField struct {
	Name   string
	Schema api.FieldSchema
}

func typeSchemaOf(schema api.Schema, obj api.Object) api.TypeSchema {
	name := strings.TrimSpace(obj.Type)
	if name == "" {
		return api.TypeSchema{}
	}
	return schema.Types[name]
}

func commonFields(schema api.Schema, objects []api.Object) []commonField {
	if len(objects) == 0 {
		return nil
	}
	first := typeSchemaOf(schema, objects[0])
	if len(first.Fields) == 0 {
		return nil
	}
	var out []commonField
	for _, name := range first.FieldNames() {
		fs, ok := first.Fields[name]
		if !ok {
			continue
		}
		merged := fs
		good := true
		for _, o := range objects[1:] {
			ts := typeSchemaOf(schema, o)
			other, exists := ts.Fields[name]
			if !exists || other.Type != fs.Type {
				good = false
				break
			}
			if fs.IsEnum() {
				merged.Values = intersectStrings(merged.Values, other.Values)
			}
			if fs.IsRef() && other.Target != "" && merged.Target != "" && other.Target != merged.Target {
				good = false
				break
			}
		}
		if !good {
			continue
		}
		out = append(out, commonField{Name: name, Schema: merged})
	}
	return out
}

func intersectStrings(a, b []string) []string {
	if len(a) == 0 {
		return append([]string(nil), b...)
	}
	if len(b) == 0 {
		return append([]string(nil), a...)
	}
	set := map[string]struct{}{}
	for _, s := range b {
		set[s] = struct{}{}
	}
	var out []string
	for _, s := range a {
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

func allTraits(objects []api.Object) bool {
	if len(objects) == 0 {
		return false
	}
	for _, o := range objects {
		if !o.IsTrait() {
			return false
		}
	}
	return true
}

func anyTrait(objects []api.Object) bool {
	for _, o := range objects {
		if o.IsTrait() {
			return true
		}
	}
	return false
}

// traitUniformTarget returns the value `x` should apply to every selected
// trait, derived from the first item: bool → all true or all false (the
// toggled value of the first); two-value enum → the other value of the
// first; larger enums → open the picker (needPicker).
func traitUniformTarget(schema api.Schema, objects []api.Object) (value string, needPicker bool, errMsg string) {
	if len(objects) == 0 {
		return "", false, "no object"
	}
	if !allTraits(objects) {
		return "", false, "not a trait"
	}
	first := objects[0]
	ts := schema.Traits[traitTypeOf(first)]
	cur := traitValueOf(first)
	if next, ok := traitToggleValue(ts, cur); ok {
		return next, false, ""
	}
	return "", true, ""
}

func formatBatchStatus(applied, total int, failed []batchItem) string {
	nfail := len(failed)
	if nfail == 0 {
		if total == 1 {
			return "applied"
		}
		return fmt.Sprintf("%d/%d applied", applied, total)
	}
	return fmt.Sprintf("%d/%d applied, %d failed", applied, total, nfail)
}
