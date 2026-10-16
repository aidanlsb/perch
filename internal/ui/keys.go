package ui

// Binding is one documented key. Help, footer, and tests share this table.
type Binding struct {
	Section string
	Keys    string
	Desc    string
}

// Keymap is the single source of truth for Perch's shortcuts.
var Keymap = []Binding{
	{"global", "ctrl+c", "quit"},
	{"global", "?", "toggle this help"},
	{"global", "e", "open selected file in editor"},

	{"browse", "hjkl / arrows", "move"},
	{"browse", "tab", "sidebar ↔ list ↔ preview"},
	{"browse", "gg / G", "top / bottom"},
	{"browse", "ctrl-d / ctrl-u", "half-page down / up"},
	{"browse", "enter", "open note"},
	{"browse", "q", "saved query picker (args prompt if needed)"},
	{"browse", "t", "type picker"},
	{"browse", "ctrl-p / space f", "fuzzy notes picker"},
	{"browse", "/", "fuzzy search in current view"},
	{"browse", "n / N", "next / previous search hit"},
	{"browse", "d", "today's daily note (or date list)"},
	{"browse", "L", "next named layout"},
	{"browse", "gL", "previous named layout"},
	{"browse", "gl / space l", "layout picker"},
	{"browse", "1-9", "jump to layout N"},
	{"browse", ".", "cycle date granularity (day/week/month)"},
	{"browse", "p", "toggle preview pane"},
	{"browse", ":", "run an ad-hoc RQL query"},
	{"browse", "r", "refresh"},
	{"browse", "space", "leader (t/q/f/d/l)"},
	{"browse", "esc", "clear search / close overlay"},

	{"leader", "space t", "type picker"},
	{"leader", "space q", "saved query picker"},
	{"leader", "space f", "notes picker"},
	{"leader", "space d", "daily note"},
	{"leader", "space l", "layout picker"},

	{"note", "q / esc / backspace", "back to browse"},
	{"note", "hjkl / arrows", "scroll / backlinks"},
	{"note", "gg / G", "top / bottom"},
	{"note", "ctrl-d / ctrl-u", "half-page"},
	{"note", "tab / shift-tab", "cycle wiki links"},
	{"note", "enter", "follow link / open backlink"},
	{"note", "[ / ]", "history back / forward"},
	{"note", "e", "open in editor"},
	{"note", "r", "refresh"},
	{"note", "t / Q / ctrl-p", "pickers (types / queries / notes)"},
	{"note", "d", "daily note"},

	{"search", "enter", "keep search; n/N still jump"},
	{"search", "esc", "clear search"},
	{"search", "backspace", "delete"},

	{"picker", "j/k / ctrl-n/p", "move"},
	{"picker", "enter", "open selection"},
	{"picker", "e", "open note in editor (notes picker)"},
	{"picker", "q / esc", "close"},

	{"args", "enter", "next argument / run query"},
	{"args", "esc", "cancel"},
}

func footerKeys(overlay overlay, screen string, preview bool) string {
	switch overlay {
	case overlayHelp:
		return "? / q / esc close"
	case overlaySearch:
		return "search  enter keep  n/N next  esc clear"
	case overlayPicker:
		return "picker  j/k  enter  e edit  esc"
	case overlayLeader:
		return "t types  q queries  f notes  d daily  l layouts  esc"
	case overlayRQL:
		return "query  enter run  esc"
	case overlayArgs:
		return "args  enter next  esc cancel"
	}
	if screen == kindNote {
		return "hjkl  tab link  enter follow  e edit  q back  ? help"
	}
	if preview {
		return "hjkl  t/q pickers  / search  p preview  e edit  L layout  ? help"
	}
	return "hjkl  t types  q queries  / search  e edit  L layout  p preview  ? help"
}
