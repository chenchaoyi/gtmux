package resume

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
)

func TestCBA006HeartbeatCanceledStrictProofCannotRebindOrRecoverIndex(t *testing.T) {
	for _, kind := range []string{"legacy-hook", "missing-index"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := frontendFixture(t)
			var err error
			if f, err = BindCodexFrontend(f, "root"); err != nil {
				t.Fatal(err)
			}
			receipt, err := os.ReadFile(frontendPath(f.Instance))
			if err != nil {
				t.Fatal(err)
			}
			if kind == "legacy-hook" {
				r, _ := Load(f.Loc)
				r.Frontend = ""
				if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(fileFor(f.Loc)); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clients, write := frontendClients, frontendBindingWrite
			t.Cleanup(func() { frontendClients, frontendBindingWrite = clients, write })
			frontendClients = func(p []codexterminal.Pane) map[string]codexterminal.Client { cancel(); return clients(p) }
			writes := 0
			frontendBindingWrite = func(path string, value any) error { writes++; return write(path, value) }
			if _, err := RefreshCodexFrontendContext(ctx, f); !errors.Is(err, context.Canceled) {
				t.Fatal("heartbeat ignored cancellation", err)
			}
			if writes != 0 {
				t.Fatal("canceled strict proof published", writes)
			}
			if after, err := os.ReadFile(frontendPath(f.Instance)); err != nil || string(after) != string(receipt) {
				t.Fatal("unpublished proof changed its receipt", err)
			}
			if kind == "legacy-hook" {
				if r, ok := Load(f.Loc); !ok || r.SessionID != "root" || r.Frontend != "" {
					t.Fatal("canceled heartbeat rebound legacy index", r, ok)
				}
			} else if _, err := os.Stat(fileFor(f.Loc)); !os.IsNotExist(err) {
				t.Fatal("canceled heartbeat recreated index", err)
			}
		})
	}
}

func TestCBA006HeartbeatCanceledPartialWritesRetireOnlyExactOwnReceipt(t *testing.T) {
	for _, stage := range []string{"receipt", "index", "replacement"} {
		t.Run(stage, func(t *testing.T) {
			f, _ := frontendFixture(t)
			var err error
			if f, err = BindCodexFrontend(f, "root"); err != nil {
				t.Fatal(err)
			}
			if stage == "index" {
				if err := os.Remove(fileFor(f.Loc)); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			write := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = write })
			var replacement CodexFrontend
			frontendBindingWrite = func(path string, value any) error {
				err := write(path, value)
				if err != nil {
					return err
				}
				if path == frontendPath(f.Instance) {
					if stage == "replacement" {
						replacement = value.(CodexFrontend)
						replacement.SessionID = "replacement"
						replacement.Sequence++
						if err := writeBindingJSON(path, replacement); err != nil {
							return err
						}
					}
					if stage != "index" {
						cancel()
					}
				}
				if stage == "index" && path == fileFor(f.Loc) {
					cancel()
				}
				return nil
			}
			if _, err := RefreshCodexFrontendContext(ctx, f); !errors.Is(err, context.Canceled) {
				t.Fatal("partial heartbeat lost cancellation", err)
			}
			if stage == "replacement" {
				if saved, ok := readFrontend(f.Instance); !ok || saved != replacement {
					t.Fatal("cancellation removed replacement owner", saved, ok)
				}
			} else {
				if _, err := os.Stat(frontendPath(f.Instance)); !os.IsNotExist(err) {
					t.Fatal("canceled heartbeat kept its own live receipt", err)
				}
				if _, live := LoadLive(f.Loc); live {
					t.Fatal("partial canceled heartbeat remained live")
				}
			}
			if r, ok := Load(f.Loc); !ok || r.SessionID != "root" {
				t.Fatal("canceled heartbeat deleted committed history", r, ok)
			}
		})
	}
}

func TestCBA006UncanceledHeartbeatKeepsSameThreadRecovery(t *testing.T) {
	for _, kind := range []string{"legacy-hook", "missing-index", "ordinary"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := frontendFixture(t)
			var err error
			if f, err = BindCodexFrontend(f, "root"); err != nil {
				t.Fatal(err)
			}
			if kind == "legacy-hook" {
				r, _ := Load(f.Loc)
				r.Frontend = ""
				if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "missing-index" {
				if err := os.Remove(fileFor(f.Loc)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := RefreshCodexFrontendContext(context.Background(), f); err != nil {
				t.Fatal(err)
			}
			if r, live := LoadLive(f.Loc); !live || r.SessionID != "root" || r.Frontend != f.Instance {
				t.Fatal("valid heartbeat recovery stopped", r, live)
			}
		})
	}
}
