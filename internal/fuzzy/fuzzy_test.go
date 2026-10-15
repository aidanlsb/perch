package fuzzy

import "testing"

func TestScoreEmptyMatches(t *testing.T) {
	if Score("anything", "") != 0 {
		t.Fatal("empty query should match")
	}
	if !Match("abc", "") {
		t.Fatal("match empty")
	}
}

func TestScoreSubsequence(t *testing.T) {
	if Score("projects/raven", "prj") < 0 {
		t.Fatal("prj should match projects/raven")
	}
	if Score("projects/raven", "xyz") >= 0 {
		t.Fatal("xyz should not match")
	}
	// consecutive + word start should beat scattered
	tight := Score("status", "sta")
	loose := Score("so much a thing", "sta")
	if tight <= loose {
		t.Fatalf("tight %d loose %d", tight, loose)
	}
}

func TestRankStringsOrder(t *testing.T) {
	got := RankStrings([]string{"alpha", "alpine", "beta", "alp"}, "alp")
	if len(got) != 3 {
		t.Fatalf("%v", got)
	}
	if got[0] != "alp" {
		t.Fatalf("shortest/tightest first: %v", got)
	}
}

func TestBestScore(t *testing.T) {
	if BestScore("rvn", "unrelated", "projects/raven") < 0 {
		t.Fatal("should match id")
	}
	if BestScore("zzz", "alpha", "beta") >= 0 {
		t.Fatal("no match")
	}
}
