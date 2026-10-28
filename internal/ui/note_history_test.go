package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNoteHistoryBackFromBacklinkThenBrowse(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, cmd := m.update(key("enter"))
	if cmd == nil {
		t.Fatal("open A")
	}
	noteA := &api.ReadResult{
		ObjectID:  "projects/a",
		Path:      "projects/a.md",
		Content:   "Alpha\n",
		Backlinks: []api.BacklinkGroup{{Source: "people/odin", Lines: []string{"see"}}},
	}
	m = applyNote(t, m, noteA)
	if got := noteIDs(m); len(got) < 1 || got[len(got)-1] != "projects/a" {
		t.Fatalf("stack after A: %v", got)
	}
	m.pane = paneBacklinks
	m.backIdx = 0
	m.noteScroll = 3

	m, cmd = m.update(key("enter"))
	if cmd == nil {
		t.Fatal("follow backlink")
	}
	noteB := &api.ReadResult{
		ObjectID: "people/odin",
		Path:     "people/odin.md",
		Content:  "Odin\n",
	}
	m = applyNote(t, m, noteB)
	if m.note.ObjectID != "people/odin" {
		t.Fatalf("B %s", m.note.ObjectID)
	}
	ids := noteIDs(m)
	if len(ids) < 2 || ids[len(ids)-1] != "people/odin" || ids[len(ids)-2] != "projects/a" {
		t.Fatalf("stack after B: %v", ids)
	}
	a := noteFrameByID(m, "projects/a")
	if a == nil || !a.HasView {
		t.Fatalf("A not recorded: %+v", a)
	}
	if a.Scroll != 3 || a.Pane != paneBacklinks {
		t.Fatalf("A view not snapshotted: %+v", a)
	}

	m, cmd = m.update(key("esc"))
	m = applyNoteOrCached(t, m, cmd, noteA)
	if m.screen != kindNote || m.note.ObjectID != "projects/a" {
		t.Fatalf("esc from B should return to A, screen=%s id=%v", m.screen, noteIDOf(m))
	}
	if m.noteScroll != 3 {
		t.Fatalf("restored scroll %d", m.noteScroll)
	}
	if m.pane != paneBacklinks {
		t.Fatalf("restored pane %v", m.pane)
	}

	m, _ = m.update(key("esc"))
	if m.pane != paneMain {
		t.Fatalf("esc from backlinks should focus main first, pane=%v", m.pane)
	}
	if m.screen != kindNote {
		t.Fatalf("still on note, screen=%s", m.screen)
	}

	m, _ = m.update(key("esc"))
	if m.screen != kindBrowse {
		t.Fatalf("esc from A should browse, screen=%s", m.screen)
	}
	if m.note != nil {
		t.Fatal("note still set after leaving stack")
	}
}

func TestNoteHistoryBackspaceAndCtrlO(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("enter"))
	noteA := &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "A\n"}
	m = applyNote(t, m, noteA)
	m, _ = m.openNote("projects/b")
	noteB := &api.ReadResult{ObjectID: "projects/b", Path: "projects/b.md", Content: "B\n"}
	m = applyNote(t, m, noteB)

	m, cmd := m.update(key("backspace"))
	m = applyNoteOrCached(t, m, cmd, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatalf("backspace got %s", m.note.ObjectID)
	}

	m, _ = m.openNote("projects/b")
	m = applyNote(t, m, noteB)
	m, cmd = m.update(key("ctrl+o"))
	m = applyNoteOrCached(t, m, cmd, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatalf("ctrl-o got %s", m.note.ObjectID)
	}
}

func TestNoteHistoryForwardAfterBack(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("enter"))
	noteA := &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "A\n"}
	m = applyNote(t, m, noteA)
	m, _ = m.openNote("people/freya")
	noteB := &api.ReadResult{ObjectID: "people/freya", Path: "people/freya.md", Content: "Freya\n"}
	m = applyNote(t, m, noteB)

	m, cmd := m.update(key("["))
	m = applyNoteOrCached(t, m, cmd, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatal("back")
	}

	m, cmd = m.update(key("]"))
	m = applyNoteOrCached(t, m, cmd, noteB)
	if m.note.ObjectID != "people/freya" {
		t.Fatalf("forward got %s", m.note.ObjectID)
	}
}

func TestNoteWikiLinkPushesHistory(t *testing.T) {
	m := loadBrowse(t)
	focusMain(&m)
	m, _ = m.update(key("enter"))
	content := "See [[people/freya]].\n"
	noteA := &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  content,
		Links: []api.ReadLink{
			{ByteStart: 4, ByteEnd: 20, Text: "[[people/freya]]", Target: "people/freya", Status: api.LinkResolved, ObjectID: "people/freya"},
		},
	}
	m = applyNote(t, m, noteA)
	m, cmd := m.update(key("enter"))
	if cmd == nil {
		t.Fatal("follow wiki link")
	}
	noteB := &api.ReadResult{ObjectID: "people/freya", Path: "people/freya.md", Content: "Freya\n"}
	m = applyNote(t, m, noteB)

	m, cmd = m.update(key("esc"))
	m = applyNoteOrCached(t, m, cmd, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatalf("want A, got %s", m.note.ObjectID)
	}
	m, _ = m.update(key("esc"))
	if m.screen != kindBrowse {
		t.Fatalf("browse, screen=%s", m.screen)
	}
}

func TestNoteFooterMentionsHistory(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 120, 24
	m, _ = m.update(key("enter"))
	m = applyNote(t, m, &api.ReadResult{ObjectID: "projects/a", Path: "projects/a.md", Content: "A\n"})
	out := m.View()
	for _, want := range []string{"esc back", "[ ] hist"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer missing %q in %s", want, out)
		}
	}
}

func applyNoteOrCached(t *testing.T, m Model, cmd tea.Cmd, rd *api.ReadResult) Model {
	t.Helper()
	if m.screen == kindNote && m.note != nil && m.note.ObjectID == rd.ObjectID {
		return m
	}
	if cmd == nil {
		t.Fatalf("expected note fetch for %s, screen=%s id=%s", rd.ObjectID, m.screen, noteIDOf(m))
	}
	return applyNote(t, m, rd)
}

func applyNote(t *testing.T, m Model, rd *api.ReadResult) Model {
	t.Helper()
	m, _ = m.update(readMsg{gen: m.gen, result: rd})
	if m.note == nil || m.note.ObjectID != rd.ObjectID {
		t.Fatalf("note %v want %s", noteIDOf(m), rd.ObjectID)
	}
	if m.screen != kindNote {
		t.Fatalf("screen %s", m.screen)
	}
	return m
}

func noteIDOf(m Model) string {
	if m.note == nil {
		return ""
	}
	return m.note.ObjectID
}

func noteIDs(m Model) []string {
	var ids []string
	for _, fr := range m.hist {
		if fr.Screen == kindNote && fr.NoteID != "" {
			ids = append(ids, fr.NoteID)
		}
	}
	return ids
}

func noteFrameByID(m Model, id string) *histFrame {
	for i := range m.hist {
		if m.hist[i].Screen == kindNote && m.hist[i].NoteID == id {
			fr := m.hist[i]
			return &fr
		}
	}
	return nil
}
