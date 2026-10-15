// Package fuzzy ranks strings by subsequence match. Empty query matches everything.
package fuzzy

import (
	"sort"
	"strings"
	"unicode"
)

// Score returns how well query matches text. Higher is better. -1 means no match.
// Matching is case-insensitive subsequence: "prj" matches "project", "p-r-j" does not
// need to be contiguous. Consecutive runes, word starts, and shorter haystacks score higher.
func Score(text, query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0
	}
	t := []rune(strings.ToLower(text))
	q := []rune(strings.ToLower(query))
	if len(q) > len(t) {
		return -1
	}
	ti := 0
	score := 0
	prev := -2
	for qi := 0; qi < len(q); qi++ {
		found := false
		for ; ti < len(t); ti++ {
			if t[ti] != q[qi] {
				continue
			}
			score++
			if ti == prev+1 {
				score += 5
			}
			if ti == 0 || isWordStart(t[ti-1]) {
				score += 3
			}
			prev = ti
			ti++
			found = true
			break
		}
		if !found {
			return -1
		}
	}
	// Prefer tighter matches against short ids/titles.
	slack := len(t) - len(q)
	if slack < 32 {
		score += 32 - slack
	}
	return score
}

func isWordStart(r rune) bool {
	return r == '/' || r == '-' || r == '_' || r == '.' || r == ' ' || r == ':' || unicode.IsSpace(r)
}

// Match reports whether query is a subsequence of text (or query is empty).
func Match(text, query string) bool {
	return Score(text, query) >= 0
}

// BestScore is the highest Score across parts. -1 if none match (unless query is empty).
func BestScore(query string, parts ...string) int {
	best := -1
	for _, p := range parts {
		if s := Score(p, query); s > best {
			best = s
		}
	}
	return best
}

// RankStrings returns items that match query, highest score first, then original order.
func RankStrings(items []string, query string) []string {
	type row struct {
		s     string
		score int
		idx   int
	}
	rows := make([]row, 0, len(items))
	for i, s := range items {
		sc := Score(s, query)
		if sc < 0 {
			continue
		}
		rows = append(rows, row{s: s, score: sc, idx: i})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].idx < rows[j].idx
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.s
	}
	return out
}
