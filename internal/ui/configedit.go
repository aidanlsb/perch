package ui

import (
	"strings"

	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/editor"
	tea "github.com/charmbracelet/bubbletea"
)

const adHocSaveHint = "Save this query in Raven first (e.g. `rvn query saved set <name> ...`), then reference it with query = \"<name>\""

func (m Model) openConfig(line int) (Model, tea.Cmd) {
	path := strings.TrimSpace(m.cfgPath)
	if path == "" {
		m.errMsg = "no config path"
		return m, nil
	}
	if err := config.Ensure(path); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	ed := editor.Resolve(m.editor)
	cmd, err := editor.CommandLine(ed, path, line)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.overlay = overlayNone
	m.status = "config " + path
	if editor.ShouldSuspend(ed, m.editorMode) {
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return editorDoneMsg{err: err, config: true}
		})
	}
	if err := cmd.Start(); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.status = "config opened — save, then r to reload"
	return m, nil
}

func (m Model) openConfigAtView() (Model, tea.Cmd) {
	line := 0
	if m.cfg != nil {
		if name := m.focusedViewName(); name != "" {
			line = m.cfg.ViewLine(name)
		} else if m.currentLayout != "" {
			line = m.cfg.LayoutLine(m.currentLayout)
		}
	}
	return m.openConfig(line)
}

func (m Model) focusedViewName() string {
	if m.currentView != "" {
		return m.currentView
	}
	if p := m.focusedView(); p != nil && p.viewName != "" {
		return p.viewName
	}
	return ""
}

func (m *Model) focusedView() *viewState {
	return m.viewByID(m.focus)
}

func (m Model) startSaveView() (Model, tea.Cmd) {
	m.overlay = overlaySave
	m.saveName = ""
	m.status = "save view"
	return m, nil
}

func (m Model) keysSave(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc":
		m.overlay = overlayNone
		m.saveName = ""
		return m, nil
	case "enter":
		return m.commitSaveView()
	case "backspace":
		m.saveName = dropLastRune(m.saveName)
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	if text := typedText(msg); text != "" {
		m.saveName += text
	}
	return m, nil
}

func (m Model) commitSaveView() (Model, tea.Cmd) {
	name := strings.TrimSpace(m.saveName)
	m.overlay = overlayNone
	m.saveName = ""
	if name == "" {
		m.status = "save canceled"
		return m, nil
	}
	path := strings.TrimSpace(m.cfgPath)
	if path == "" {
		m.errMsg = "no config path"
		return m, nil
	}
	p := m.p()
	v := config.View{
		Inputs: p.rawInputs,
		Layout: config.Present{}.FromSpec(p.spec),
	}
	switch {
	case p.viewName != "" && m.cfg != nil:
		if existing, ok := m.cfg.Views[p.viewName]; ok {
			v.Type = existing.Type
			v.Query = existing.Query
		} else {
			m.status = adHocSaveHint
			return m, nil
		}
	case p.sourceIsQuery && p.savedQuery != nil && strings.TrimSpace(p.savedQuery.Name) != "":
		v.Query = p.savedQuery.Name
	case p.typeName != "" && (p.sourceBase == "type:"+p.typeName || strings.HasPrefix(p.id, "type:")):
		v.Type = p.typeName
	default:
		m.status = adHocSaveHint
		return m, nil
	}
	if err := config.UpsertView(path, name, v); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.status = "saved views." + name
	return m.reloadConfigKeep()
}

func (m Model) reloadConfigKeep() (Model, tea.Cmd) {
	m2, err := m.reloadConfig()
	if err != nil {
		m2.errMsg = err.Error()
		m2.status = "config error; keeping last good"
		return m2, nil
	}
	return m2, nil
}

func (m Model) reloadConfig() (Model, error) {
	path := strings.TrimSpace(m.cfgPath)
	if path == "" {
		return m, nil
	}
	f, err := config.Load(path)
	if err != nil {
		return m, err
	}
	m.cfg = f
	m.errMsg = ""
	if m.schema.Types != nil || m.schema.Queries != nil {
		m.cfgWarnings = f.Validate(m.schema)
	}
	m.rebuildNav()
	return m, nil
}

func (m Model) onConfigEditorDone(msg editorDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
	}
	m2, err := m.reloadConfig()
	if err != nil {
		m2.errMsg = err.Error()
		m2.status = "config error; keeping last good"
		return m2, nil
	}
	m = m2
	if len(m.cfgWarnings) > 0 {
		m.status = "config: " + m.cfgWarnings[0]
	} else {
		m.status = "config reloaded"
	}
	if m.currentLayout != "" {
		if _, ok := m.cfg.Layouts[m.currentLayout]; ok {
			return m.loadNamedLayout(m.currentLayout)
		}
	}
	if m.currentView != "" {
		if _, ok := m.cfg.Views[m.currentView]; ok {
			return m.loadNamedView(m.currentView)
		}
	}
	return m.refresh()
}
