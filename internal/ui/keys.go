package ui

import tea "github.com/charmbracelet/bubbletea"

// Binding is one documented key. Help, footer, and tests share this table.
type Binding struct {
	Section string
	Keys    string
	Desc    string
}

// Keymap is the single source of truth for Perch's shortcuts.
// Pickers are single keys in normal mode. There is no leader.
var Keymap = []Binding{
	{"global", "ctrl+c", "quit"},
	{"global", "?", "toggle this help"},
	{"global", "e", "open selected file in editor"},
	{"global", ",", "open config at the current view"},
	{"global", "S", "save the on-screen view as [views.name]"},

	{"help", "j/k", "scroll this help"},
	{"help", "q / esc / ?", "close"},

	{"focus", "ctrl-h/j/k/l", "move focus among panels"},
	{"focus", "ctrl-w h/j/k/l", "same as ctrl-h/j/k/l"},
	{"focus", "tab / shift-tab", "cycle panels"},
	{"focus", "ctrl-w o", "zoom focused panel"},
	{"focus", "ctrl-w =", "restore splits"},

	{"browse", "hjkl / arrows", "move inside the focused panel"},
	{"browse", "gg / G", "top / bottom"},
	{"browse", "ctrl-d / ctrl-u", "half-page down / up"},
	{"browse", "enter", "open note, or move to the bound panel"},
	{"browse", "t", "type picker (schema types)"},
	{"browse", "d", "daily-note date picker"},
	{"browse", "q", "saved query picker"},
	{"browse", "v", "views and layouts picker"},
	{"browse", "ctrl-p / f", "fuzzy notes picker"},
	{"browse", "s", "full-text content search"},
	{"browse", "/", "fuzzy search in current view"},
	{"browse", "n / N", "next / previous search hit"},
	{"browse", ".", "cycle date granularity (day/week/month)"},
	{"browse", "p", "toggle preview pane"},
	{"browse", ":", "run an ad-hoc RQL query"},
	{"browse", "r", "refresh (and reload config)"},
	{"browse", "esc", "clear search / close overlay"},

	{"note", "esc / backspace", "back to browse"},
	{"note", "/", "fuzzy search in browse"},
	{"note", "hjkl / arrows", "scroll / backlinks"},
	{"note", "gg / G", "top / bottom"},
	{"note", "ctrl-d / ctrl-u", "half-page"},
	{"note", "tab / shift-tab", "cycle wiki links"},
	{"note", "enter", "follow link / open backlink"},
	{"note", "[ / ]", "history back / forward"},
	{"note", "[d / ]d", "previous / next daily note"},
	{"note", "e", "open in editor"},
	{"note", "r", "refresh"},
	{"note", "t", "type picker"},
	{"note", "d", "daily-note date picker"},
	{"note", "q", "saved query picker"},
	{"note", "v", "views and layouts picker"},
	{"note", "ctrl-p / f", "notes picker"},
	{"note", "s", "full-text content search"},
	{"note", ",", "open config at this view"},

	{"search", "enter", "keep search; n/N still jump"},
	{"search", "esc", "clear search"},
	{"search", "backspace", "delete"},

	{"picker", "j/k / ctrl-n/p", "move"},
	{"picker", "enter", "open selection"},
	{"picker", "e", "open note in editor (notes picker)"},
	{"picker", "q / esc", "close"},

	{"daily", "h/l", "previous / next day"},
	{"daily", "j/k", "previous / next week"},
	{"daily", "t", "today"},
	{"daily", "enter", "open that day's daily note"},
	{"daily", "q / esc", "close"},

	{"args", "enter", "next argument / run query"},
	{"args", "esc", "cancel"},
}

func footerKeys(m Model) string {
	switch m.overlay {
	case overlayHelp:
		return "help  j/k  q / esc close"
	case overlaySearch:
		return "search  enter keep  n/N next  esc clear"
	case overlayPicker:
		return "picker  j/k  enter  e edit  esc"
	case overlayDate:
		return "daily  h/l day  j/k week  t today  enter open  esc"
	case overlayRQL:
		return "query  enter run  esc"
	case overlayArgs:
		return "args  enter next  esc cancel"
	case overlaySave:
		return "save view  enter write  esc"
	}
	if m.screen == kindNote {
		if isDailyNoteID(m.noteID()) {
			return "hjkl  tab link  [d ]d day  enter follow  / search  e edit  esc back  ? help"
		}
		return "hjkl  tab link  enter follow  / search  e edit  esc back  ? help"
	}
	return focusFooter(m) + "  " + panelFooter(m)
}

func focusFooter(m Model) string {
	if m.zoomed {
		return "ctrl-w = restore"
	}
	return "ctrl-hjkl panels"
}

func panelFooter(m Model) string {
	switch m.focus {
	case focusPreview:
		return "j/k scroll  h back  e edit  p close  ? help"
	default:
		p := m.p()
		switch p.spec.Kind {
		case kindBoard:
			return "hjkl board  enter  t/d/q/v/f/s  / search  S save  ? help"
		case kindDate:
			return "jk dates  . gran  enter  t/d/q/v/f/s  / search  S save  ? help"
		default:
			return "jk list  enter  t/d/q/v/f/s  / search  S save  , config  ? help"
		}
	}
}

// keyIdent is the string we switch on for bindings.
//
// Bubble Tea usually reports printable keys as KeyRunes, so String() is the
// character ("/", "a", …). On some terminals — notably macOS Terminal.app —
// punctuation such as '/' arrives as a named KeyType equal to the ASCII
// code. That form is not in Bubble Tea's keyNames map, so String() is empty
// and a `case "/"` never matches. Fall back to the printable rune.
func keyIdent(msg tea.KeyMsg) string {
	if s := msg.String(); s != "" {
		return s
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		return string(msg.Runes)
	}
	r := rune(msg.Type)
	if r >= 32 && r < 127 {
		return string(r)
	}
	return ""
}

func isSlash(msg tea.KeyMsg) bool {
	switch keyIdent(msg) {
	case "/", "slash", "alt+/":
		return true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '/' {
		return true
	}
	return false
}

// typedText is printable input for search / RQL / picker / args overlays.
func typedText(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		return string(msg.Runes)
	}
	id := keyIdent(msg)
	if len(id) != 1 {
		return ""
	}
	r := rune(id[0])
	if r >= 32 && r < 127 {
		return id
	}
	return ""
}
