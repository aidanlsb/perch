package ui

import "github.com/aidanlsb/perch/internal/fuzzy"

func bestScore(q string, parts ...string) int {
	return fuzzy.BestScore(q, parts...)
}

func (m *Model) jumpSearch(delta int) {
	if m.searchQ == "" {
		return
	}
	m.recomputeSearchHits()
	if len(m.searchHits) == 0 {
		return
	}
	n := len(m.searchHits)
	m.searchAt = (m.searchAt + delta) % n
	if m.searchAt < 0 {
		m.searchAt += n
	}
	idx := m.searchHits[m.searchAt]
	rows := m.p().filtered
	if idx >= 0 && idx < len(rows) {
		m.selectID(rows[idx].ID)
	}
}

func (m *Model) jumpSearchFirst() {
	m.searchAt = 0
	m.recomputeSearchHits()
	if len(m.searchHits) == 0 {
		return
	}
	idx := m.searchHits[0]
	rows := m.p().filtered
	if idx >= 0 && idx < len(rows) {
		m.selectID(rows[idx].ID)
	}
}
