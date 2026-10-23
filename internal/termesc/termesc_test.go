package termesc

import "testing"

func TestPlainStripsOSCAndCSI(t *testing.T) {
	in := "hello\x1b]0;pwned\x07 world\x1b[31mred\x1b[0m"
	got := Plain(in)
	if got != "hello worldred" {
		t.Fatalf("%q", got)
	}
}

func TestForTUIKeepsSGRDropsOSC(t *testing.T) {
	in := "hi\x1b]0;title\x07\x1b[31mred\x1b[0m"
	got := ForTUI(in)
	if got != "hi\x1b[31mred\x1b[0m" {
		t.Fatalf("%q", got)
	}
}

func TestForTUIDropsClearScreen(t *testing.T) {
	got := ForTUI("keep\x1b[2Jgone")
	if got != "keepgone" {
		t.Fatalf("%q", got)
	}
}

func TestPlainDropsC0AndUTF8C1(t *testing.T) {
	in := "a\x07b\r\nc\xc2\x9dOSC"
	got := Plain(in)
	if got != "ab\ncOSC" {
		t.Fatalf("%q", got)
	}
}

func TestForTUIKeepsBoxDrawingAndEllipsis(t *testing.T) {
	in := "│ perch … ─"
	if got := ForTUI(in); got != in {
		t.Fatalf("%q", got)
	}
}

func TestForTUIDropsIncompleteESC(t *testing.T) {
	if got := ForTUI("x\x1b"); got != "x" {
		t.Fatalf("%q", got)
	}
}
