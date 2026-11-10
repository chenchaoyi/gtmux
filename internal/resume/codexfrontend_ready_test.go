package resume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestCFR002AdmissionCausesAndControlledLaunch(t *testing.T) {
	for _, kind := range []string{"ready", "peer", "launch", "backend", "backend-unavailable", "pane", "stdin", "invalid", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			oldPane, oldObserve, oldProcess := frontendLaunchPane, frontendLaunchObserve, frontendLaunchProcess
			t.Cleanup(func() {
				frontendLaunchPane, frontendLaunchObserve, frontendLaunchProcess = oldPane, oldObserve, oldProcess
			})
			frontendLaunchPane = func(context.Context, string) (codexterminal.Pane, bool) {
				return codexterminal.Pane{ID: "%7", PID: 100, TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}, kind != "pane"
			}
			frontendLaunchObserve = func(context.Context, codexterminal.Pane, int, int, int) codexterminal.Observation {
				o := codexterminal.Observation{Client: codexterminal.Client{PID: 300, Start: "client", RootStart: "root", TTY: "ttys001"}, Ancestors: []int{100, os.Getpid(), 300}}
				if kind == "peer" {
					o.PID = 301
				}
				if kind == "launch" {
					o.Ancestors = []int{100, 300}
				}
				if kind == "stdin" {
					o = codexterminal.Observation{Code: "stdin-unavailable"}
				}
				return o
			}
			frontendLaunchProcess = func(_ context.Context, pid int) (launchhost.AgentProcess, bool) {
				name := "gtmux"
				if pid == 200 {
					name = "codex"
				}
				if kind == "backend" && pid == 200 {
					name = "other"
				}
				return launchhost.AgentProcess{PID: pid, Comm: name, Start: "birth"}, kind != "backend-unavailable" || pid != 200
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			instance := "0123456789abcdef0123456789abcdef"
			if kind == "invalid" {
				instance = "../invalid"
			}
			f, err := BeginCodexFrontendFromLaunch(ctx, "%7", instance, 300, 200, 1, 300)
			if kind == "ready" {
				if err != nil || f.PID != 300 || f.BridgePID != os.Getpid() || f.HomeID == "" {
					t.Fatalf("failed exact launch: %+v %v", f, err)
				}
			} else {
				var refusal *FrontendAdmissionError
				if !errors.As(err, &refusal) {
					t.Fatalf("untyped refusal %v", err)
				}
				want := map[string]string{"peer": "peer-mismatch", "launch": "launch-mismatch", "backend": "backend-mismatch", "backend-unavailable": "process-unavailable", "pane": "pane-unavailable", "stdin": "stdin-unavailable", "invalid": "invalid-request", "cancel": "admission-deadline"}[kind]
				if refusal.Code != want || refusal.Retryable != (kind == "stdin" || kind == "backend-unavailable") {
					t.Fatalf("wrong cause: %+v", refusal)
				}
			}
			var files []string
			filepath.WalkDir(os.Getenv("HOME"), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					files = append(files, path)
				}
				return nil
			})
			if len(files) > 0 {
				t.Fatal("admission persisted authority", files)
			}
		})
	}
}
