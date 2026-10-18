package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestServeStartupChild(t *testing.T) {
	if os.Getenv("GTMUX_TEST_SERVE_CHILD") != "1" {
		return
	}
	tmux.Bin = ""
	bind := os.Getenv("GTMUX_TEST_SERVE_BIND")
	if bind == "" {
		bind = "127.0.0.1"
	}
	os.Exit(cmdServe([]string{"--bind", bind, "--port", os.Getenv("GTMUX_TEST_SERVE_PORT")}))
}

func TestServeBindFailurePreservesRunningStatus(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	home := t.TempDir()
	status := filepath.Join(home, ".local", "share", "gtmux", "status", "serve.json")
	if err := os.MkdirAll(filepath.Dir(status), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte(`{"version":"existing owner","pid":1234}`)
	if err := os.WriteFile(status, before, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestServeStartupChild$")
	cmd.Env = append(os.Environ(), "HOME="+home, "GTMUX_LANG=en", "GTMUX_TEST_SERVE_CHILD=1", "GTMUX_TEST_SERVE_PORT="+strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("occupied listener result: %v (%s)", err, out)
	}
	after, err := os.ReadFile(status)
	if err != nil || string(after) != string(before) {
		t.Fatalf("failed serve overwrote the owner's status: %s (%v)", after, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "gtmux", "serve-token")); !os.IsNotExist(err) {
		t.Fatal("failed serve created a credential before binding")
	}
}

func TestServePortLeaseRejectsOtherBind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	lease, err := state.TryLease("serve-" + strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestServeStartupChild$")
	cmd.Env = append(os.Environ(), "GTMUX_TEST_SERVE_CHILD=1", "GTMUX_TEST_SERVE_BIND=0.0.0.0", "GTMUX_TEST_SERVE_PORT="+strconv.Itoa(port))
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("same port with another bind was not excluded: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "gtmux", "serve-token")); !os.IsNotExist(err) {
		t.Fatal("competing serve created a credential")
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "gtmux", "status", "serve.json")); !os.IsNotExist(err) {
		t.Fatal("competing serve published status")
	}
}

func TestExistingRadarRequiresIdentityAndSavedCredential(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tokenPath := filepath.Join(state.ConfigDir(), "serve-token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		t.Fatal(err)
	}
	var identity atomic.Bool
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			if identity.Load() {
				w.Write([]byte(`{"status":"ok","service":"gtmux"}`))
			} else {
				w.Write([]byte(`{"status":"ok","service":"foreign"}`))
			}
			return
		}
		if r.URL.Path == "/api/devices" && r.Header.Get("Authorization") == "Bearer saved-fixture" {
			w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer gateway.Close()
	port := gateway.Listener.Addr().(*net.TCPAddr).Port
	if _, err := existingRadarToken(port); err == nil {
		t.Fatal("foreign server reused")
	}
	identity.Store(true)
	if _, err := existingRadarToken(port); err == nil {
		t.Fatal("missing credential accepted")
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatal("reuse minted a credential")
	}
	if err := os.WriteFile(tokenPath, []byte("wrong-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := existingRadarToken(port); err == nil {
		t.Fatal("unaccepted credential reused")
	}
	if err := os.WriteFile(tokenPath, []byte("saved-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	token, err := existingRadarToken(port)
	if err != nil || token != "saved-fixture" {
		t.Fatalf("ready API was not reused: %v", err)
	}
}
