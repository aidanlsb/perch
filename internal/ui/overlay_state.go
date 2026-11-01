package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

func objectNeedle(obj api.Object) string {
	if obj.IsTrait() {
		if c := strings.TrimSpace(api.ScalarString(obj.Fields["content"])); c != "" {
			return api.PlainWikilinks(c)
		}
		return strings.TrimSpace(obj.Title)
	}
	return ""
}

func (m Model) anchorNeedles() []string {
	var out []string
	if s := strings.TrimSpace(m.scrollNeedle); s != "" {
		out = append(out, s)
	}
	if obj := m.selectedObject(); obj != nil {
		if n := objectNeedle(*obj); n != "" && n != m.scrollNeedle {
			out = append(out, n)
		}
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func (m *Model) addOverlay(ov objectOverlay) {
	if ov.id == "" && (ov.created == nil || ov.created.ID == "") {
		return
	}
	if ov.id == "" {
		ov.id = ov.created.ID
	}
	for i, e := range m.overlays {
		if e.id != ov.id {
			continue
		}
		m.overlays[i] = mergeOverlay(e, ov)
		return
	}
	m.overlays = append(m.overlays, ov)
}

func mergeOverlay(a, b objectOverlay) objectOverlay {
	out := a
	if b.created != nil {
		out.created = b.created
	}
	if len(b.fields) > 0 {
		if out.fields == nil {
			out.fields = map[string]any{}
		}
		for k, v := range b.fields {
			out.fields[k] = v
			out.unset = removeString(out.unset, k)
		}
	}
	for _, k := range b.unset {
		out.unset = appendUnique(out.unset, k)
		delete(out.fields, k)
	}
	return out
}

func (m *Model) applyMutOverlays(objs []api.Object) []api.Object {
	return m.applyMutOverlaysOpts(objs, true)
}

func (m *Model) applyMutOverlaysOpts(objs []api.Object, appendCreated bool) []api.Object {
	if len(m.overlays) == 0 {
		return objs
	}
	out := make([]api.Object, len(objs))
	copy(out, objs)
	seen := make(map[string]bool, len(out))
	for i := range out {
		seen[out[i].ID] = true
		m.patchObject(&out[i])
	}
	if !appendCreated {
		return out
	}
	for _, ov := range m.overlays {
		if ov.created == nil || seen[ov.created.ID] {
			continue
		}
		o := *ov.created
		if o.Fields == nil {
			o.Fields = map[string]any{}
		}
		m.patchObject(&o)
		out = append(out, o)
		seen[o.ID] = true
	}
	return out
}

func (m *Model) patchObject(o *api.Object) {
	if o == nil {
		return
	}
	for _, ov := range m.overlays {
		if o.ID != ov.id && o.OpenRef() != ov.id {
			continue
		}
		if o.Fields == nil {
			o.Fields = map[string]any{}
		}
		for k, v := range ov.fields {
			o.Fields[k] = v
		}
		for _, k := range ov.unset {
			delete(o.Fields, k)
		}
	}
}

func (m *Model) pruneOverlays(server []api.Object) {
	if len(m.overlays) == 0 {
		return
	}
	byID := make(map[string]api.Object, len(server))
	for _, o := range server {
		byID[o.ID] = o
	}
	kept := m.overlays[:0]
	for _, ov := range m.overlays {
		obj, ok := byID[ov.id]
		if !ok {
			if ov.created != nil {
				continue
			}
			kept = append(kept, ov)
			continue
		}
		if !overlaySatisfied(obj, ov) {
			kept = append(kept, ov)
		}
	}
	if len(kept) == 0 {
		m.overlays = nil
		return
	}
	m.overlays = kept
}

func overlaySatisfied(obj api.Object, ov objectOverlay) bool {
	for k, want := range ov.fields {
		if api.ScalarString(obj.Fields[k]) != api.ScalarString(want) {
			return false
		}
	}
	for _, k := range ov.unset {
		if api.ScalarString(obj.Fields[k]) != "" {
			return false
		}
	}
	return true
}

func (m *Model) applyOverlaysToPanes(restore string) {
	for _, p := range m.panes {
		if p == nil {
			continue
		}
		p.objects = m.applyMutOverlays(p.objects)
		ts := m.typeSchemaFor(p)
		m.refreshObjectTitles(p.objects, ts)
		p.applyFilterAndGroup(ts)
		if restore != "" {
			p.restoreID = restore
			p.keepCursor = true
			p.selectID(restore)
		}
		m.mergeNotes(p.objects)
	}
	if restore != "" {
		if o, ok := m.noteIndex[restore]; ok {
			m.patchObject(&o)
			m.noteIndex[restore] = o
		}
	}
}

func (m Model) openCreatedIfMissing(p *viewState) (Model, tea.Cmd) {
	if m.pendingCreated == "" || p == nil {
		return m, nil
	}
	target := m.focus
	if m.viewByID(target) == nil {
		target = m.lastMainFocus
	}
	if target == "" && len(m.panes) > 0 {
		target = m.panes[0].id
	}
	if p.id != target {
		return m, nil
	}
	id := m.pendingCreated
	if p.selectID(id) {
		m.pendingCreated = ""
		return m, nil
	}
	m.pendingCreated = ""
	m.dropCreatedOverlay(id)
	if m.pendingStatus == "" {
		m.status = "created " + id
	}
	return m.openNote(id)
}

func (m *Model) dropCreatedOverlay(id string) {
	if id == "" {
		return
	}
	kept := m.overlays[:0]
	for _, ov := range m.overlays {
		if ov.created != nil && ov.id == id {
			continue
		}
		kept = append(kept, ov)
	}
	if len(kept) == 0 {
		m.overlays = nil
		return
	}
	m.overlays = kept
}

func (m *Model) refreshObjectTitles(objs []api.Object, ts api.TypeSchema) {
	for i := range objs {
		if objs[i].IsTrait() {
			continue
		}
		objs[i].Title = api.DisplayTitle(objs[i], ts)
	}
}

func appendUnique(dst []string, v string) []string {
	for _, s := range dst {
		if s == v {
			return dst
		}
	}
	return append(dst, v)
}

func removeString(ss []string, v string) []string {
	out := ss[:0]
	for _, s := range ss {
		if s != v {
			out = append(out, s)
		}
	}
	return out
}
