package proc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aidanlsb/perch/internal/api"
)

func TestParseStartupJSON(t *testing.T) {
	line, _ := json.Marshal(map[string]any{
		"ok": true,
		"data": api.Startup{
			URL: "http://127.0.0.1:7438", Token: "abc", Port: 7438,
			Bind: "127.0.0.1", APIVersion: "1", VaultPath: "/vault",
		},
	})
	s, err := ParseStartupJSON(line)
	if err != nil {
		t.Fatal(err)
	}
	if s.Token != "abc" || s.VaultPath != "/vault" {
		t.Fatalf("%+v", s)
	}
}

func TestConnectExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"UNAUTHORIZED","message":"no"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"status":"ok","api_version":"1"}}`))
	}))
	t.Cleanup(srv.Close)

	h, err := Connect(context.Background(), Options{URL: srv.URL, Token: "tok", NoStart: true})
	if err != nil {
		t.Fatal(err)
	}
	if h.Started {
		t.Fatal("should not start")
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadStartupLineFromChild(t *testing.T) {
	if os.Getenv("PERCH_FAKE_RVN") == "1" {
		fmt.Println(`{"ok":true,"data":{"url":"http://127.0.0.1:9","token":"childtok","port":9,"bind":"127.0.0.1","api_version":"1","vault_path":"/v"}}`)
		time.Sleep(30 * time.Second)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestReadStartupLineFromChild")
	cmd.Env = append(os.Environ(), "PERCH_FAKE_RVN=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := readStartupLine(stdout, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseStartupJSON(line)
	if err != nil {
		t.Fatal(err)
	}
	if s.Token != "childtok" || s.URL != "http://127.0.0.1:9" {
		t.Fatalf("%+v", s)
	}
}
