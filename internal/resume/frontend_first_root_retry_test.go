package resume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
)

func firstRootLocationFixture(t *testing.T) (CodexFrontend, *codexterminal.Pane) {
	t.Helper()
	f, _ := frontendFixture(t)
	// A root may finish well after admission; the receipt still gets a fresh tick.
	f.StartedAt = time.Now().Add(-20 * time.Second).UnixMilli()
	f.UpdatedAt = f.StartedAt
	p := &codexterminal.Pane{ID: f.Pane, PID: f.PanePID, TTY: "/dev/ttys999", Cwd: f.Cwd, Loc: f.Loc}
	codexProcessPanes = func() []codexterminal.Pane { return []codexterminal.Pane{*p} }
	frontendPaneView = func(id, format string) string {
		if id != p.ID && id != p.Loc {
			return ""
		}
		switch format {
		case "#{pane_id}":
			return p.ID
		case "#{session_name}:#{window_index}.#{pane_index}":
			return p.Loc
		}
		return fmt.Sprintf("%s\t%d\t%s", p.ID, p.PID, p.Loc)
	}
	if f.SessionID != "" {
		t.Fatal("fixture accidentally starts with an already bound root")
	}
	return f, p
}

func TestCFL006FirstRootAndExistingRootFinishTheirOwnPartialMovedPublication(t *testing.T) {
	for _, initial := range []string{"first root", "existing root"} {
		for _, stage := range []string{"proof", "index"} {
			t.Run(initial+"/"+stage, func(t *testing.T) {
				f, p := firstRootLocationFixture(t)
				if initial == "existing root" {
					var err error
					f, err = BindCodexFrontend(f, "previous-root")
					if err != nil {
						t.Fatal(err)
					}
				}
				p.Loc = "work:2.0"
				write := frontendBindingWrite
				t.Cleanup(func() { frontendBindingWrite = write })
				failed := false
				frontendBindingWrite = func(path string, value any) error {
					if !failed && (stage == "proof" && path == frontendPath(f.Instance) || stage == "index" && path == fileFor(p.Loc)) {
						failed = true
						return errors.New("synthetic one-time publication IO")
					}
					return write(path, value)
				}
				_, err := BindCodexFrontend(f, "observed-root")
				var proof *FrontendBindingError
				if !failed || !errors.As(err, &proof) || !proof.Retryable {
					t.Fatal("did not reach the intended retryable IO boundary", failed, err)
				}
				if _, live := LoadLive(p.Loc); live {
					t.Fatal("partial publication granted live input")
				}
				frontendBindingWrite = write
				got, err := BindCodexFrontend(f, "observed-root")
				if err != nil {
					t.Fatal("exact same first-root metadata retry was rejected", err)
				}
				if got.Instance != f.Instance || got.Sequence != f.Sequence || got.SessionID != "observed-root" || got.Loc != p.Loc {
					t.Fatal("retry changed the admitted connection or root", got)
				}
				if r, live := LoadLive(p.Loc); !live || r.SessionID != got.SessionID || r.Frontend != got.Instance {
					t.Fatal("completed publication has no exact live association", r, live)
				}
				if _, err := RefreshCodexFrontend(got); err != nil {
					t.Fatal("later heartbeat remained suspended/unavailable", err)
				}
			})
		}
	}
}

func seedPartialFirstRoot(t *testing.T) (CodexFrontend, *codexterminal.Pane) {
	t.Helper()
	f, p := firstRootLocationFixture(t)
	p.Loc = "work:2.0"
	write := frontendBindingWrite
	frontendBindingWrite = func(path string, value any) error {
		if path == fileFor(p.Loc) {
			return errors.New("synthetic first index failure")
		}
		return write(path, value)
	}
	_, err := BindCodexFrontend(f, "observed-root")
	frontendBindingWrite = write
	var proof *FrontendBindingError
	if !errors.As(err, &proof) || !proof.Retryable {
		t.Fatal("partial first root did not reach index IO", err)
	}
	if _, ok := readFrontend(f.Instance); !ok {
		t.Fatal("partial proof was not actually written")
	}
	return f, p
}

func TestCFL006FirstRootRetryRejectsOtherRootsGenerationsAndInvalidReceipts(t *testing.T) {
	for _, change := range []string{"different observed root", "different instance", "different sequence", "different admission", "different home", "different client birth", "different bridge birth", "different backend birth", "expired receipt", "future receipt", "malformed receipt", "canceled"} {
		t.Run(change, func(t *testing.T) {
			f, p := seedPartialFirstRoot(t)
			saved, _ := readFrontend(f.Instance)
			sid := "observed-root"
			switch change {
			case "different observed root":
				sid = "another-root"
			case "different instance":
				saved.Instance = "abcdef0123456789abcdef0123456789"
			case "different sequence":
				saved.Sequence++
			case "different admission":
				saved.StartedAt++
			case "different home":
				saved.HomeID = "foreign-home"
			case "different client birth":
				saved.ClientStart = "other-client-birth"
			case "different bridge birth":
				saved.BridgeStart = "other-bridge-birth"
			case "different backend birth":
				saved.BackendStart = "other-backend-birth"
			case "expired receipt":
				saved.UpdatedAt = time.Now().Add(-6 * time.Second).UnixMilli()
			case "future receipt":
				saved.UpdatedAt = time.Now().Add(2 * time.Second).UnixMilli()
			}
			if err := writeBindingJSON(frontendPath(f.Instance), saved); err != nil {
				t.Fatal(err)
			}
			if change == "malformed receipt" {
				if err := os.WriteFile(frontendPath(f.Instance), []byte("{invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(frontendPath(f.Instance))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if change == "canceled" {
				cancel()
			}
			if _, err := BindCodexFrontendContext(ctx, f, sid); err == nil {
				t.Fatal("nonmatching or unavailable initial receipt was accepted")
			}
			after, _ := os.ReadFile(frontendPath(f.Instance))
			if string(before) != string(after) {
				t.Fatal("rejected retry replaced private proof")
			}
			if _, ok := Load(p.Loc); ok {
				t.Fatal("rejected retry published a destination index")
			}
		})
	}
}

func TestCFL006LateFirstRootRetryCannotReplaceANewerDestinationOwner(t *testing.T) {
	f, p := seedPartialFirstRoot(t)
	next := f
	next.Instance = "abcdef0123456789abcdef0123456789"
	next.Sequence++
	newer, err := BindCodexFrontend(next, "newer-root")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(fileFor(p.Loc))
	proof, _ := os.ReadFile(frontendPath(newer.Instance))
	if _, err := BindCodexFrontend(f, "observed-root"); err == nil {
		t.Fatal("late partial first root replaced newer owner")
	}
	after, _ := os.ReadFile(fileFor(p.Loc))
	afterProof, _ := os.ReadFile(frontendPath(newer.Instance))
	if string(before) != string(after) || string(proof) != string(afterProof) {
		t.Fatal("late retry changed newer publication")
	}
	if got, ok := LoadLive(p.Loc); !ok || got.Frontend != newer.Instance {
		t.Fatal("newer exact owner was lost", got, ok)
	}
}
