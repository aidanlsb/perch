package layout

import "github.com/aidanlsb/perch/internal/fuzzy"

func fuzzyBest(q string, parts ...string) int {
	return fuzzy.BestScore(q, parts...)
}
