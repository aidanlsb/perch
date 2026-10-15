package ui

import "github.com/aidanlsb/perch/internal/api"

func (m *Model) recomputeSearchHits() {
	q := m.searchQ
	m.searchHits = m.searchHits[:0]
	for i, o := range m.filtered {
		if objectMatchesSearch(o, q) {
			m.searchHits = append(m.searchHits, i)
		}
	}
	if m.searchAt >= len(m.searchHits) {
		m.searchAt = max(0, len(m.searchHits)-1)
	}
}

func objectMatchesSearch(o api.Object, q string) bool {
	if q == "" {
		return true
	}
	if scoreObject(o, q) >= 0 {
		return true
	}
	return false
}

func scoreObject(o api.Object, q string) int {
	parts := []string{o.ID, o.Title, o.Type}
	for _, v := range o.Fields {
		if s := api.ScalarString(v); s != "" {
			parts = append(parts, s)
		}
	}
	return bestScore(q, parts...)
}
