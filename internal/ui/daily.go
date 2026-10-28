package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const dailyLayout = "2006-01-02"

func (m Model) openDatePicker() (Model, tea.Cmd) {
	m.overlay = overlayDate
	m.datePurpose = "daily"
	m.dateCur = m.todayID()
	m.status = "daily " + m.dateCur
	return m, nil
}

func (m Model) todayID() string {
	if m.daily != nil {
		if id := strings.TrimSpace(m.daily.TodayID); isDailyNoteID(id) {
			return id
		}
	}
	return time.Now().Format(dailyLayout)
}

func (m Model) keysDate(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := keyIdent(msg)
	switch key {
	case "esc", "q":
		return m.dateEscape()
	case "enter":
		if m.datePurpose == "value" {
			return m.commitValue(m.dateCur)
		}
		return m.openDailyDate(m.dateCur)
	case "ctrl+d":
		if m.datePurpose == "value" && m.mut.optional {
			return m.applyUnset()
		}
		return m, nil
	case "h", "left":
		m.shiftDateCur(-1)
		return m, nil
	case "l", "right":
		m.shiftDateCur(1)
		return m, nil
	case "k", "up":
		m.shiftDateCur(-7)
		return m, nil
	case "j", "down":
		m.shiftDateCur(7)
		return m, nil
	case "t":
		m.dateCur = m.todayID()
		m.status = "daily " + m.dateCur
		return m, nil
	}
	return m, nil
}

func (m *Model) shiftDateCur(days int) {
	cur, ok := parseDailyID(m.dateCur)
	if !ok {
		m.dateCur = m.todayID()
		cur, ok = parseDailyID(m.dateCur)
		if !ok {
			return
		}
	}
	m.dateCur = cur.AddDate(0, 0, days).Format(dailyLayout)
	m.status = "daily " + m.dateCur
}

func (m Model) openDailyDate(date string) (Model, tea.Cmd) {
	date = strings.TrimSpace(date)
	if !isDailyNoteID(date) {
		m.status = "invalid date"
		m.overlay = overlayNone
		return m, nil
	}
	m.overlay = overlayNone
	m.loading = true
	m.status = "daily " + date + "…"
	gen := m.nextGen()
	return m, fetchDate(m.client, gen, date)
}

func (m Model) openDailyShift(days int) (Model, tea.Cmd) {
	cur, ok := parseDailyID(m.noteID())
	if !ok {
		m.status = "not a daily note"
		return m, nil
	}
	return m.openDailyDate(cur.AddDate(0, 0, days).Format(dailyLayout))
}

func (m Model) onDate(msg dateMsg) (Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if msg.result == nil {
		m.status = "no daily note for " + msg.date
		return m, nil
	}
	if !msg.result.DailyExists || strings.TrimSpace(msg.result.DailyNoteID) == "" {
		date := msg.result.Date
		if date == "" {
			date = msg.date
		}
		m.loading = true
		m.status = "daily " + date + "…"
		return m, fetchEnsureDaily(m.client, m.gen, date)
	}
	m.status = "daily " + msg.result.Date
	return m.openNote(msg.result.DailyNoteID)
}

func (m Model) onDailyNote(msg dailyNoteMsg) (Model, tea.Cmd) {
	if msg.gen != 0 && msg.gen != m.gen {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if msg.result == nil {
		m.status = "no daily note for " + msg.date
		return m, nil
	}
	id := strings.TrimSpace(msg.result.ID)
	if id == "" {
		id = strings.TrimSpace(msg.result.Date)
	}
	if id == "" {
		id = msg.date
	}
	m.status = "daily " + id
	return m.openNote(id)
}

func isDailyNoteID(id string) bool {
	_, ok := parseDailyID(id)
	return ok
}

func parseDailyID(id string) (time.Time, bool) {
	id = strings.TrimSpace(id)
	if len(id) != 10 {
		return time.Time{}, false
	}
	t, err := time.Parse(dailyLayout, id)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
