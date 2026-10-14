package run

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := Main([]string{"--version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("%s", out.String())
	}
}

func TestHelp(t *testing.T) {
	var out bytes.Buffer
	if err := Main([]string{"--help"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "rvn api") {
		t.Fatalf("%s", out.String())
	}
}
