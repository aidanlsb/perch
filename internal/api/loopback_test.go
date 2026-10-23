package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckLoopbackURL(t *testing.T) {
	ok := []string{
		"http://127.0.0.1:7438",
		"https://127.0.0.1",
		"http://[::1]:9",
		"http://127.0.0.2/v1/health",
	}
	for _, raw := range ok {
		if err := CheckLoopbackURL(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	bad := []string{
		"",
		"http://localhost:7438",
		"http://example.com",
		"http://8.8.8.8",
		"http://0.0.0.0:7438",
		"ftp://127.0.0.1",
		"http://user:pass@127.0.0.1",
		"unix:///tmp/rvn.sock",
	}
	for _, raw := range bad {
		if err := CheckLoopbackURL(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestClientRefusesOffLoopbackRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	_, err := NewClient(srv.URL, "t").Health(context.Background())
	if err == nil {
		t.Fatal("expected redirect to be refused")
	}
	if !strings.Contains(err.Error(), "loopback") && !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("got %v", err)
	}
}
