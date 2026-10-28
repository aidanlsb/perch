package ui

import (
	"strings"
	"testing"

	"github.com/aidanlsb/perch/internal/api"
	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/frontmatter"
	"github.com/charmbracelet/x/ansi"
)

func TestViewStripsOSCFromVaultTitles(t *testing.T) {
	m := loadBrowse(t)
	m.width = 80
	m.height = 24
	m.p().objects = []api.Object{{
		ID: "projects/evil", Title: "x\x1b]0;pwned\x07y", Type: "project",
		Fields:   map[string]any{"name": "x\x1b]0;pwned\x07y", "status": "active"},
		FilePath: "projects/evil.md",
	}}
	m.p().applyFilterAndGroup(api.TypeSchema{})
	out := m.View()
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x1b]0;") || strings.Contains(out, "\x07") {
		t.Fatalf("osc leaked: %q", out)
	}
}

func TestViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 80
	m.height = 24
	out := m.View()
	if out == "" || len(out) < 20 {
		t.Fatalf("empty view %q", out)
	}
	if strings.Contains(out, "Browse") {
		t.Fatalf("sidebar chrome: %s", out)
	}
	m.overlay = overlayHelp
	if help := m.View(); help == "" {
		t.Fatal("help")
	}
}

func TestNoteViewRenders(t *testing.T) {
	m := loadBrowse(t)
	m.width = 100
	m.height = 30
	m, _ = m.update(readMsg{
		gen:    m.gen,
		result: sampleRead(),
	})
	out := m.View()
	if out == "" {
		t.Fatal("empty")
	}
}

func TestDatePickerRendersMonth(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m.daily = &api.DailyInfo{TodayID: "2026-10-15"}
	m, _ = pressLeader(m, "d")
	out := m.View()
	if !strings.Contains(out, "2026-10-15") {
		t.Fatalf("missing selected date: %s", out)
	}
	if !strings.Contains(out, "October") && !strings.Contains(out, "Su Mo") {
		t.Fatalf("missing calendar: %s", out)
	}
}

func sampleTraitRows() []api.Object {
	return api.ObjectsFromQuery(&api.QueryResult{
		QueryKind: "trait",
		Items: []map[string]any{
			{
				"id": "daily/2026-07-06.md:trait:1", "trait_type": "todo", "value": "true",
				"content": "talk to tax person", "file_path": "daily/2026-07-06.md",
				"line": 7.0, "object_id": "2026-07-06#tasks",
			},
			{
				"id":         "type/issue/ambiguous-order-submissions-lose-their-idempotency-key.md:trait:0",
				"trait_type": "priority", "value": "high", "content": "",
				"file_path": "type/issue/ambiguous-order-submissions-lose-their-idempotency-key.md",
				"line":      10.0, "object_id": "issue/ambiguous-order-submissions-lose-their-idempotency-key#description",
			},
			{
				"id": "notes/x.md:trait:2", "trait_type": "mention", "value": "people/freya",
				"content":   "ask [[people/freya|Freya]] about the spec",
				"file_path": "notes/x.md", "line": 4.0, "object_id": "notes/x",
			},
		},
	}, api.TypeSchema{})
}

func TestTypeListLeadsWithDisplayTitle(t *testing.T) {
	objs := []api.Object{{ID: "projects/a", Title: "Alpha", Kind: "object", Fields: map[string]any{"name": "Alpha"}}}
	cols, flex := listColumns(objs, []string{"name"}, api.TypeSchema{NameField: "name"})
	if flex != 0 || len(cols) < 2 || cols[0] != colLabel || cols[1] != "id" {
		t.Fatalf("cols=%v flex=%d", cols, flex)
	}
	cells := rowCells(objs[0], cols)
	if cells[0] != "Alpha" || cells[1] != "projects/a" {
		t.Fatalf("cells=%v", cells)
	}
}

func TestTypeListKeepsIDWhenTitleMatches(t *testing.T) {
	objs := []api.Object{{ID: "a", Title: "a", Kind: "object"}}
	cols, flex := listColumns(objs, nil, api.TypeSchema{})
	if flex != -1 || cols[0] != "id" {
		t.Fatalf("cols=%v flex=%d", cols, flex)
	}
}

func TestRowCellsUsesNamedField(t *testing.T) {
	obj := api.Object{ID: "a", Title: "Title", Type: "item", Fields: map[string]any{"xyz": "X", "name": "N"}}
	cells := rowCells(obj, []string{"id", "xyz", "name"})
	if cells[0] != "a" || cells[1] != "X" || cells[2] != "N" {
		t.Fatalf("%v", cells)
	}
	obj2 := api.Object{ID: "b", Title: "T", Fields: map[string]any{"title": "FieldTitle"}}
	cells = rowCells(obj2, []string{"id", "title"})
	if cells[1] != "FieldTitle" {
		t.Fatalf("title column must read the field, not obj.Title: %v", cells)
	}
}

func TestTraitRowCellsUseContentNotID(t *testing.T) {
	rows := sampleTraitRows()
	cols, flex := listColumns(rows, nil, api.TypeSchema{})
	if flex != 0 || len(cols) != 4 || cols[0] != "content" {
		t.Fatalf("cols=%v flex=%d", cols, flex)
	}
	cells := rowCells(rows[0], cols)
	if cells[0] != "talk to tax person" {
		t.Fatalf("primary %q", cells[0])
	}
	if cells[1] != "todo" || cells[2] != "true" {
		t.Fatalf("context %v", cells)
	}
	if cells[3] != "daily/2026-07-06.md:7" {
		t.Fatalf("source %q", cells[3])
	}
	for _, c := range cells {
		if strings.Contains(c, ":trait:") {
			t.Fatalf("raw trait id leaked: %v", cells)
		}
	}
	empty := rowCells(rows[1], cols)
	if empty[0] != "ambiguous-order-submissions-lose-their-idempotency-key" {
		t.Fatalf("empty content fallback %q", empty[0])
	}
	if empty[0] == "high" {
		t.Fatal("value must not be the primary label")
	}
	wiki := rowCells(rows[2], cols)
	if wiki[0] != "ask Freya about the spec" {
		t.Fatalf("wikilink cell %q", wiki[0])
	}
}

func TestTraitListGivesContentTheWidth(t *testing.T) {
	m := loadBrowse(t)
	p := m.p()
	p.objects = sampleTraitRows()
	p.listFields = nil
	p.spec.Fields = nil
	p.applyFilterAndGroup(api.TypeSchema{})
	cols, flex := listColumns(p.filtered, p.listFields, api.TypeSchema{})
	widths := columnWidths(p.filtered, cols, 80, flex)
	if flex != 0 || widths[0] < 20 {
		t.Fatalf("content should be the flex column, widths=%v flex=%d", widths, flex)
	}
	if widths[0] <= widths[1] || widths[0] <= widths[2] {
		t.Fatalf("content narrower than secondary cols: %v", widths)
	}
	out := m.viewList(p, 80, 10, true)
	if !strings.Contains(out, "talk to tax person") {
		t.Fatalf("missing content:\n%s", out)
	}
	if !strings.Contains(out, "ask Freya about the spec") {
		t.Fatalf("missing wikilink content:\n%s", out)
	}
	if !strings.Contains(out, "ambiguous-order-submissions") {
		t.Fatalf("missing empty-content fallback:\n%s", out)
	}
	if strings.Contains(out, ":trait:") {
		t.Fatalf("raw trait id in list:\n%s", out)
	}
	if !strings.Contains(out, "todo") || !strings.Contains(out, "true") {
		t.Fatalf("missing type/value:\n%s", out)
	}
	if !strings.Contains(out, "daily/2026-07-06.md:7") {
		t.Fatalf("missing source:\n%s", out)
	}
}

func TestTraitBoardAndDateUseContentTitle(t *testing.T) {
	m := loadBrowse(t)
	p := m.p()
	p.objects = sampleTraitRows()
	p.spec.Kind = kindBoard
	p.spec.GroupBy = "value"
	p.applyFilterAndGroup(api.TypeSchema{})
	board := m.viewBoard(p, 80, 12, true)
	if !strings.Contains(board, "talk to tax person") {
		t.Fatalf("board missing content:\n%s", board)
	}
	if strings.Contains(board, ":trait:") {
		t.Fatalf("raw trait id on board:\n%s", board)
	}
	if !strings.Contains(board, "high") || !strings.Contains(board, "true") {
		t.Fatalf("board should group by value:\n%s", board)
	}

	p.spec.Kind = kindDate
	p.spec.GroupBy = "value"
	p.spec.Granularity = "day"
	p.objects = []api.Object{{
		ID: "n.md:trait:0", Kind: "trait", Title: "pay rent",
		Fields:   map[string]any{"content": "pay rent", "value": "2026-07-06", "trait_type": "due"},
		FilePath: "n.md", Line: 3, ObjectID: "n",
	}}
	p.applyFilterAndGroup(api.TypeSchema{})
	dates := m.viewDate(p, 60, 8, true)
	if !strings.Contains(dates, "pay rent") {
		t.Fatalf("date missing content:\n%s", dates)
	}
	if strings.Contains(dates, ":trait:") {
		t.Fatalf("raw trait id on date:\n%s", dates)
	}
}

func TestTraitEmptyContentUsesParentNoteTitle(t *testing.T) {
	m := loadBrowse(t)
	m.noteIndex = map[string]api.Object{
		"issue/ambiguous-order-submissions-lose-their-idempotency-key": {
			ID:    "issue/ambiguous-order-submissions-lose-their-idempotency-key",
			Title: "Ambiguous order submissions",
		},
	}
	row := sampleTraitRows()[1]
	if got := m.displayTitle(row); got != "Ambiguous order submissions" {
		t.Fatalf("got %q", got)
	}
}

func TestTraitListRespectsConfiguredFields(t *testing.T) {
	rows := sampleTraitRows()
	cols, flex := listColumns(rows, []string{"value", "content"}, api.TypeSchema{})
	if len(cols) != 2 || cols[0] != "value" || flex != 1 {
		t.Fatalf("cols=%v flex=%d", cols, flex)
	}
	cells := rowCells(rows[0], cols)
	if cells[0] != "true" || cells[1] != "talk to tax person" {
		t.Fatalf("%v", cells)
	}
}

func TestNoteHeadingUsesSchemaNameField(t *testing.T) {
	m := NewModel(Options{Config: &config.File{}})
	m.schema = api.Schema{Types: map[string]api.TypeSchema{
		"item": {Name: "item", NameField: "xyz"},
	}}
	m.noteIndex = map[string]api.Object{
		"items/a": {ID: "items/a", Type: "item", Title: "Fallback"},
	}
	doc := frontmatter.Doc{Fields: map[string]string{"xyz": "FromXYZ", "name": "FromName"}}
	if got := m.headingFromDoc("items/a", doc); got != "FromXYZ" {
		t.Fatalf("got %q", got)
	}
	other := frontmatter.Doc{Fields: map[string]string{"name": "FromName", "status": "open"}}
	if got := m.headingFromDoc("items/a", other); got != "Fallback" {
		t.Fatalf("empty name_field should use Title, not name: %q", got)
	}
}

func sampleRead() *api.ReadResult {
	content := "---\nname: Alpha\nstatus: active\n---\n\nSee [[people/freya]].\n"
	return &api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  content,
		Links: []api.ReadLink{{
			ByteStart: 38, ByteEnd: 54, Text: "[[people/freya]]",
			Target: "people/freya", Status: api.LinkResolved, ObjectID: "people/freya",
		}},
		Backlinks: []api.BacklinkGroup{{Source: "people/odin"}},
	}
}

func TestBrowseUsesRoundedPanelHeader(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "╭") {
		t.Fatalf("missing rounded border:\n%s", plain)
	}
	if !strings.Contains(plain, brandMark) {
		t.Fatalf("missing brand:\n%s", plain)
	}
	if !strings.Contains(plain, "list") {
		t.Fatalf("missing kind badge:\n%s", plain)
	}
}

func TestUnfocusedListKeepsSelectionMarker(t *testing.T) {
	m := loadBrowse(t)
	p := m.p()
	p.listCur = 1
	focused := m.viewList(p, 60, 8, true)
	if !strings.Contains(ansi.Strip(focused), "▸") {
		t.Fatalf("focused cursor missing:\n%s", focused)
	}
	idle := m.viewList(p, 60, 8, false)
	plain := ansi.Strip(idle)
	if !strings.Contains(plain, "▸") {
		t.Fatalf("unfocused driver selection must stay marked:\n%s", idle)
	}
	if !strings.Contains(plain, "Beta") {
		t.Fatalf("selected title missing:\n%s", idle)
	}
}

func TestNoteHeaderIsCompactMetadataBar(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 100, 30
	m, _ = m.update(readMsg{gen: m.gen, result: sampleRead()})
	head := m.viewNoteHeader(80)
	plain := ansi.Strip(head)
	if strings.Count(plain, "\n") > 3 {
		t.Fatalf("header too tall:\n%s", plain)
	}
	if !strings.Contains(plain, "status: active") {
		t.Fatalf("missing pill:\n%s", plain)
	}
	if !strings.Contains(plain, "·") && !strings.Contains(plain, "status") {
		t.Fatalf("expected compact pills:\n%s", plain)
	}
	bar := ansi.Strip(m.viewLinkBar(80))
	if !strings.Contains(bar, "links 1/") || !strings.Contains(bar, "[[people/freya]]") {
		t.Fatalf("link bar: %q", bar)
	}
}

func TestPreviewLineNumbersAroundAnchor(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m.previewOn = true
	m.setPreview(&api.ReadResult{
		ObjectID: "projects/a",
		Path:     "projects/a.md",
		Content:  "one\ntwo\nthree\nfour\nfive\nsix\n",
	})
	m.previewBody = "one\ntwo\nthree\nfour\nfive\nsix"
	m.previewLine = 3
	m.previewScroll = 2
	out := ansi.Strip(m.viewPreview(40, 16))
	if !strings.Contains(out, "projects/a.md:3") {
		t.Fatalf("missing path:line:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("missing target mark:\n%s", out)
	}
	if !strings.Contains(out, "3") {
		t.Fatalf("missing line number:\n%s", out)
	}
}

func TestPickerHasPromptAndCounts(t *testing.T) {
	m := loadBrowse(t)
	m.width, m.height = 80, 24
	m, _ = pressLeader(m, " ")
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "navigate") {
		t.Fatalf("missing picker title:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("missing prompt:\n%s", out)
	}
	if !strings.Contains(out, "/") {
		t.Fatalf("missing match count:\n%s", out)
	}
}

func TestDateGroupHeaderUsesRule(t *testing.T) {
	m := loadBrowse(t)
	p := m.p()
	p.spec.Kind = kindDate
	p.spec.GroupBy = "due"
	p.spec.Granularity = "day"
	p.objects = []api.Object{{
		ID: "projects/a", Title: "Alpha", Type: "project",
		Fields: map[string]any{"due": "2026-10-27"},
	}}
	p.applyFilterAndGroup(api.TypeSchema{})
	out := ansi.Strip(m.viewDate(p, 40, 8, false))
	if !strings.Contains(out, "──") || (!strings.Contains(out, "2026-10-27") && !strings.Contains(out, "27 Oct")) {
		t.Fatalf("date header:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("unfocused date selection missing:\n%s", out)
	}
}
