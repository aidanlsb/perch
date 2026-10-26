package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
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
	if m.notePtr != 0 || len(m.noteStack) != 1 {
		t.Fatalf("stack after A: ptr=%d n=%d", m.notePtr, len(m.noteStack))
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
	if m.notePtr != 1 || len(m.noteStack) != 2 {
		t.Fatalf("stack after B: ptr=%d n=%d ids=%v", m.notePtr, len(m.noteStack), stackIDs(m))
	}
	if m.noteStack[0].ID != "projects/a" || !m.noteStack[0].HasView {
		t.Fatalf("A not recorded: %+v", m.noteStack[0])
	}
	if m.noteStack[0].Scroll != 3 || m.noteStack[0].Pane != paneBacklinks {
		t.Fatalf("A view not snapshotted: %+v", m.noteStack[0])
	}

	m, cmd = m.update(key("esc"))
	if cmd == nil {
		t.Fatal("esc should load previous note")
	}
	m = applyNote(t, m, noteA)
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
	if m.screen != "browse" {
		t.Fatalf("esc from A should browse, screen=%s", m.screen)
	}
	if m.note != nil {
		t.Fatal("note still set after leaving stack")
	}
	if len(m.noteStack) != 0 || m.notePtr != -1 {
		t.Fatalf("stack leftover ptr=%d n=%d", m.notePtr, len(m.noteStack))
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
	if cmd == nil {
		t.Fatal("backspace should load previous note")
	}
	m = applyNote(t, m, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatalf("backspace got %s", m.note.ObjectID)
	}

	m, _ = m.openNote("projects/b")
	m = applyNote(t, m, noteB)
	m, cmd = m.update(key("ctrl+o"))
	if cmd == nil {
		t.Fatal("ctrl-o should load previous note")
	}
	m = applyNote(t, m, noteA)
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

	m, _ = m.update(key("["))
	m = applyNote(t, m, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatal("back")
	}

	m, cmd := m.update(key("]"))
	if cmd == nil {
		t.Fatal("] should go forward")
	}
	m = applyNote(t, m, noteB)
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
	if cmd == nil {
		t.Fatal("esc to A")
	}
	m = applyNote(t, m, noteA)
	if m.note.ObjectID != "projects/a" {
		t.Fatalf("want A, got %s", m.note.ObjectID)
	}
	m, _ = m.update(key("esc"))
	if m.screen != "browse" {
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

func stackIDs(m Model) []string {
	ids := make([]string, len(m.noteStack))
	for i, fr := range m.noteStack {
		ids[i] = fr.ID
	}
	return ids
}
