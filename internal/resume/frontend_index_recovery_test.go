package resume

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func expiredIndexFixture(t *testing.T) CodexFrontend {
	t.Helper()
	f, _ := frontendFixture(t)
	f.StartedAt = time.Now().Add(-time.Minute).UnixMilli()
	var err error
	f, err = BindCodexFrontend(f, "same-current-root")
	if err != nil {
		t.Fatal(err)
	}
	f.UpdatedAt = time.Now().Add(-10 * time.Second).UnixMilli()
	if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(fileFor(f.Loc)); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDCA004CurrentProducerRecoversExpiredOwnMissingIndex(t *testing.T) {
	f := expiredIndexFixture(t)
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("expired evidence became reader authority")
	}
	if _, _, err := ConversationRecord(f.Loc); !errors.Is(err, ErrConversationUnassociated) {
		t.Fatal("reader silently repaired stale evidence", err)
	}
	next, err := RefreshCodexFrontend(f)
	if err != nil {
		t.Fatalf("current producer failed to recover its own missing index: %v", err)
	}
	r, ok := LoadLive(f.Loc)
	if !ok || r.SessionID != f.SessionID || r.Frontend != f.Instance || next.Instance != f.Instance || next.UpdatedAt <= f.UpdatedAt {
		t.Fatal("recovery changed identity or did not publish fresh association", r, next, ok)
	}
}

func TestDCA005RecoveryRejectsConflictAndUnknownEvidence(t *testing.T) {
	for _, mode := range []string{"bridge-birth", "backend-birth", "client-birth", "root-birth", "tty", "home", "stored-root", "newer-sequence", "unknown-store", "oversized-store", "malformed-index", "foreign-index", "future-proof"} {
		t.Run(mode, func(t *testing.T) {
			f := expiredIndexFixture(t)
			switch mode {
			case "bridge-birth", "backend-birth":
				old := frontendProcess
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					p, ok := old(pid)
					if mode == "bridge-birth" && pid == f.BridgePID || mode == "backend-birth" && pid == f.BackendPID {
						p.Start = "changed-birth"
					}
					return p, ok
				}
			case "client-birth", "root-birth", "tty":
				old := frontendClients
				frontendClients = func(panes []codexterminal.Pane) map[string]codexterminal.Client {
					clients := old(panes)
					c := clients[f.Pane]
					if mode == "client-birth" {
						c.Start = "changed"
					}
					if mode == "root-birth" {
						c.RootStart = "changed"
					}
					if mode == "tty" {
						c.TTY = "other-tty"
					}
					clients[f.Pane] = c
					return clients
				}
			case "home", "stored-root":
				other := f
				if mode == "home" {
					other.HomeID = "different-home"
				} else {
					other.SessionID = "different-root"
				}
				if err := writeBindingJSON(frontendPath(f.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "newer-sequence":
				other := f
				other.Instance = strings.Repeat("a", 32)
				other.Sequence++
				other.StartedAt++
				if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "unknown-store":
				if err := os.WriteFile(frontendPath(strings.Repeat("b", 32)), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			case "oversized-store":
				for n := 0; n < 129; n++ {
					if err := os.WriteFile(filepath.Join(Dir(), "codex-frontends", fmt.Sprintf("entry-%03d.json", n)), []byte("invalid"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "malformed-index":
				if err := os.WriteFile(fileFor(f.Loc), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-index":
				if err := writeBindingJSON(fileFor(f.Loc), Record{Agent: "codex", SessionID: "other", HomeID: f.HomeID, Frontend: strings.Repeat("c", 32)}); err != nil {
					t.Fatal(err)
				}
			case "future-proof":
				f.UpdatedAt = time.Now().Add(20 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			}
			beforeProof, _ := os.ReadFile(frontendPath(f.Instance))
			beforeIndex, _ := os.ReadFile(fileFor(f.Loc))
			if _, err := RefreshCodexFrontend(f); err == nil {
				t.Fatal("conflict/unknown evidence authorized recovery")
			}
			afterProof, _ := os.ReadFile(frontendPath(f.Instance))
			afterIndex, _ := os.ReadFile(fileFor(f.Loc))
			if !bytes.Equal(beforeProof, afterProof) || !bytes.Equal(beforeIndex, afterIndex) {
				t.Fatal("rejected recovery changed owner or history")
			}
		})
	}
}

func TestDCA006RecoveryHonorsLeaseAndCancellation(t *testing.T) {
	f := expiredIndexFixture(t)
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RefreshCodexFrontend(f); !errors.Is(err, state.ErrLeaseHeld) {
		t.Fatal("held lease did not exclude recovery", err)
	}
	lease.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = RefreshCodexFrontendContext(ctx, f); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled producer recovered", err)
	}
	if _, ok := Load(f.Loc); ok {
		t.Fatal("refused attempts wrote an index")
	}
	if _, err = RefreshCodexFrontend(f); err != nil {
		t.Fatal("legitimate retry could not recover", err)
	}
}

func TestDCA005OwnerReplacementDuringStrictVerification(t *testing.T) {
	f := expiredIndexFixture(t)
	before, _ := os.ReadFile(frontendPath(f.Instance))
	var replacement []byte
	old := frontendClients
	frontendClients = func(panes []codexterminal.Pane) map[string]codexterminal.Client {
		if replacement == nil {
			other := f
			other.SessionID = "replacement-owner"
			if err := writeBindingJSON(frontendPath(f.Instance), other); err != nil {
				t.Fatal(err)
			}
			replacement, _ = os.ReadFile(frontendPath(f.Instance))
		}
		return old(panes)
	}
	if _, err := RefreshCodexFrontend(f); err == nil {
		t.Fatal("producer ignored a changed owner during strict observation")
	}
	after, _ := os.ReadFile(frontendPath(f.Instance))
	want := before
	if replacement != nil {
		want = replacement // fixture represents another writer; recovery must preserve it
	}
	if !bytes.Equal(after, want) {
		t.Fatal("recovery overwrote owner replacement")
	}
	if _, err := os.Stat(fileFor(f.Loc)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected recovery published an index", err)
	}
}
