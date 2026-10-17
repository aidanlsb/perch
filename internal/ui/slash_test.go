package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSlashKeyIdentEmptyString(t *testing.T) {
	runes := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	if runes.String() != "/" {
		t.Fatalf("runes String=%q", runes.String())
	}
	ascii := tea.KeyMsg{Type: tea.KeyType('/')}
	if ascii.String() != "" {
		t.Fatalf("ASCII-typed slash should have empty String (macOS), got %q", ascii.String())
	}
	if keyIdent(ascii) != "/" {
		t.Fatalf("keyIdent=%q", keyIdent(ascii))
	}
	if !isSlash(runes) || !isSlash(ascii) {
		t.Fatal("isSlash")
	}
	alt := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}, Alt: true}
	if alt.String() != "alt+/" {
		t.Fatalf("alt String=%q", alt.String())
	}
	if !isSlash(alt) {
		t.Fatal("alt+/")
	}
}

func slashMsgs() []struct {
	name string
	msg  tea.KeyMsg
} {
	return []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"runes", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}},
		{"ascii-type", tea.KeyMsg{Type: tea.KeyType('/')}},
		{"alt-runes", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}, Alt: true}},
	}
}

func TestSlashOpensSearchEveryContext(t *testing.T) {
	contexts := []struct {
		name string
		prep func(*testing.T) Model
	}{
		{"sidebar", func(t *testing.T) Model {
			m := loadBrowse(t)
			m.pane = paneSidebar
			return m
		}},
		{"list", func(t *testing.T) Model {
			m := loadBrowse(t)
			m.pane = paneMain
			return m
		}},
		{"board", func(t *testing.T) Model {
			m := loadProjectBoard(t)
			m, _ = m.update(key("L"))
			m.pane = paneMain
			if m.viewKind != kindBoard {
				t.Fatalf("kind %s", m.viewKind)
			}
			return m
		}},
		{"date", func(t *testing.T) Model {
			m := loadProjectBoard(t)
			m, _ = m.update(key("L"))
			m, _ = m.update(key("L"))
			m.pane = paneMain
			if m.viewKind != kindDate {
				t.Fatalf("kind %s", m.viewKind)
			}
			return m
		}},
		{"preview", func(t *testing.T) Model {
			m := loadBrowse(t)
			m.pane = paneMain
			m.previewOn = true
			m.pane = panePreview
			return m
		}},
		{"note", func(t *testing.T) Model {
			m := loadBrowse(t)
			m.width, m.height = 80, 24
			m, _ = m.update(readMsg{gen: m.gen, result: sampleRead()})
			if m.screen != kindNote {
				t.Fatalf("screen %s", m.screen)
			}
			return m
		}},
	}
	for _, ctx := range contexts {
		for _, sl := range slashMsgs() {
			t.Run(ctx.name+"/"+sl.name, func(t *testing.T) {
				m := ctx.prep(t)
				m, _ = m.update(sl.msg)
				if m.overlay != overlaySearch {
					t.Fatalf("overlay=%v screen=%s pane=%v kind=%s", m.overlay, m.screen, m.pane, m.viewKind)
				}
				if ctx.name == "note" && m.screen != "browse" {
					t.Fatalf("note / should return to browse, screen=%s", m.screen)
				}
			})
		}
	}
}

func TestSlashNotSwallowedByPendingOrDigits(t *testing.T) {
	ascii := tea.KeyMsg{Type: tea.KeyType('/')}

	m := loadBrowse(t)
	m.pending = "g"
	m, _ = m.update(ascii)
	if m.overlay != overlaySearch {
		t.Fatalf("g then / overlay=%v pending=%q", m.overlay, m.pending)
	}
	if m.pending != "" {
		t.Fatalf("pending %q", m.pending)
	}

	m = loadBrowse(t)
	m, _ = m.update(key(" "))
	if m.overlay != overlayLeader {
		t.Fatal("leader")
	}
	m, _ = m.update(ascii)
	if m.overlay != overlaySearch {
		t.Fatalf("space then / overlay=%v", m.overlay)
	}

	m = loadBrowse(t)
	m, _ = m.update(key("1"))
	m, _ = m.update(ascii)
	if m.overlay != overlaySearch {
		t.Fatalf("digit then / overlay=%v", m.overlay)
	}
}

func TestSlashInPickerTypesIntoQuery(t *testing.T) {
	m := loadBrowse(t)
	m, _ = m.update(key("t"))
	if m.overlay != overlayPicker {
		t.Fatal("picker")
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyType('/')})
	if m.overlay != overlayPicker {
		t.Fatalf("picker intercepted, overlay=%v", m.overlay)
	}
	if m.pickerQ != "/" {
		t.Fatalf("pickerQ=%q", m.pickerQ)
	}
}

func TestHelpOverlayListsSlashSearch(t *testing.T) {
	var browse, note bool
	for _, b := range Keymap {
		if b.Keys == "/" && strings.Contains(b.Desc, "fuzzy search") {
			switch b.Section {
			case "browse":
				browse = true
			case "note":
				note = true
			}
		}
	}
	if !browse || !note {
		t.Fatalf("keymap browse=%v note=%v", browse, note)
	}
	m := loadBrowse(t)
	m.width, m.height = 100, 48
	m, _ = m.update(key("?"))
	if m.overlay != overlayHelp {
		t.Fatal("help")
	}
	out := m.View()
	if !strings.Contains(out, "fuzzy search in current view") {
		t.Fatalf("browse / missing in help: %s", out)
	}
	if !strings.Contains(out, "fuzzy search in browse") {
		t.Fatalf("note / missing in help: %s", out)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyType('/')})
	if m.overlay != overlayHelp {
		t.Fatalf("help should keep / , overlay=%v", m.overlay)
	}
}

func TestSlashTypesIntoSearchQuery(t *testing.T) {
	m := loadBrowse(t)
	m.pane = paneMain
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyType('/')})
	if m.overlay != overlaySearch {
		t.Fatal("open")
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyType('e')}) // ASCII-typed e
	obj := m.selectedObject()
	if obj == nil || obj.Title != "Beta" {
		t.Fatalf("%v q=%q", obj, m.searchQ)
	}
}

func TestSlashFromNoteUsesObjects(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = m.update(readMsg{gen: m.gen, result: sampleRead()})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m.screen != "browse" || m.overlay != overlaySearch {
		t.Fatalf("screen=%s overlay=%v", m.screen, m.overlay)
	}
}
