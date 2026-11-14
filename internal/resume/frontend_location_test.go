package resume

import (
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
)

// This is a synthetic terminal/process view. All persistence is the real resume
// implementation under frontendFixture's private HOME and CODEX_HOME.
func locationFixture(t *testing.T) (CodexFrontend, *codexterminal.Pane, *codexterminal.Client) {
	t.Helper()
	f, client := frontendFixture(t)
	pane := &codexterminal.Pane{ID: f.Pane, Command: "bash", PID: f.PanePID, TTY: "/dev/ttys999", Cwd: f.Cwd, Loc: f.Loc}
	codexProcessPanes = func() []codexterminal.Pane { return []codexterminal.Pane{*pane} }
	frontendPaneView = func(id, format string) string {
		if id != pane.ID && id != pane.Loc {
			return ""
		}
		switch format {
		case "#{pane_id}":
			return pane.ID
		case "#{session_name}:#{window_index}.#{pane_index}":
			return pane.Loc
		case "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}":
			if pane.PID != 100 {
				return pane.ID + "\t999\t" + pane.Loc
			}
			return pane.ID + "\t100\t" + pane.Loc
		}
		return ""
	}
	var err error
	f, err = BindCodexFrontend(f, "location-root")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := CodexFrontendForPane(f.Pane); !ok {
		t.Fatal("initial exact frontend is not verified")
	}
	return f, pane, client
}

func TestCFL001HeartbeatMigratesOnlyItsCurrentLocation(t *testing.T) {
	for _, mode := range []string{"empty", "old same SID missing proof", "renamed session", "different window"} {
		t.Run(mode, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			old := f.Loc
			dest := "work:0.1"
			if mode == "renamed session" {
				dest = "renamed:0.0"
			}
			if mode == "different window" {
				dest = "work:3.0"
			}
			if mode == "old same SID missing proof" {
				if err := writeBindingJSON(fileFor(dest), Record{Agent: "codex", HomeID: f.HomeID, SessionID: f.SessionID, Frontend: "abcdef0123456789abcdef0123456789", UpdatedAt: f.UpdatedAt/1000 - 100}); err != nil {
					t.Fatal(err)
				}
			}
			pane.Loc = dest
			migrated, err := RefreshCodexFrontend(f)
			if err != nil {
				t.Fatal("unchanged live frontend failed location migration", err)
			}
			if migrated.Loc != dest || migrated.Instance != f.Instance || migrated.SessionID != f.SessionID || migrated.HomeID != f.HomeID {
				t.Errorf("producer did not publish its exact current location: %+v", migrated)
			}
			r, ok := LoadLive(dest)
			if !ok || r.Frontend != f.Instance || r.SessionID != f.SessionID || r.HomeID != f.HomeID {
				t.Errorf("current location is unassociated or uses the stale instance: %+v live=%v", r, ok)
			}
			read, _, readErr := ConversationRecord(dest)
			if readErr != nil || read.SessionID != f.SessionID || read.Frontend != f.Instance {
				t.Errorf("conversation reader did not follow verified producer: %+v %v", read, readErr)
			}
			input, inputOK := CodexFrontendForPane(f.Pane)
			if !inputOK || input.Instance != f.Instance || input.Loc != dest {
				t.Errorf("input target is not the migrated exact attachment: %+v %v", input, inputOK)
			}
			if _, live := LoadLive(old); live {
				t.Error("old layout position still grants live authority")
			}
			if history, exists := Load(old); !exists || history.SessionID != f.SessionID || history.Frontend != f.Instance {
				t.Error("producer migration deleted its original committed layout history", history, exists)
			}
		})
	}
}

func TestCFL001ObservedRootBindsAfterVerifiedLocationChange(t *testing.T) {
	f, pane, _ := locationFixture(t)
	pane.Loc = "work:2.0"
	next, err := BindCodexFrontend(f, "new-observed-root", "/thread/workspace")
	if err != nil {
		t.Fatal("successful observed root could not bind after layout-only change", err)
	}
	if next.Loc != pane.Loc || next.SessionID != "new-observed-root" || next.Instance != f.Instance || next.ThreadCwd != "/thread/workspace" || next.Cwd != f.Cwd {
		t.Fatal("relocation changed terminal/thread identity", next)
	}
	if got, ok := LoadLive(pane.Loc); !ok || got.SessionID != next.SessionID {
		t.Fatal("new root not published at its current location", got, ok)
	}
}

func TestCFL002LocationChangeCannotHideIdentityOrOwnerConflicts(t *testing.T) {
	for _, kind := range []string{"client PID", "client birth", "root PID", "root birth", "TTY", "physical cwd", "missing terminal", "bridge birth", "backend birth", "missing receipt", "changed receipt root", "expired receipt", "foreign home", "newer destination owner", "malformed destination", "malformed old proof", "unknown destination process"} {
		t.Run(kind, func(t *testing.T) {
			f, pane, client := locationFixture(t)
			dest := "work:0.1"
			pane.Loc = dest
			switch kind {
			case "client PID":
				client.PID++
			case "client birth":
				client.Start = "different-client"
			case "root PID":
				pane.PID++
			case "root birth":
				client.RootStart = "different-root"
			case "TTY":
				client.TTY = "other-tty"
			case "physical cwd":
				pane.Cwd = "/different-workspace"
			case "missing terminal":
				frontendClients = func([]codexterminal.Pane) map[string]codexterminal.Client { return nil }
			case "bridge birth", "backend birth":
				old := frontendProcess
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					p, ok := old(pid)
					if kind == "bridge birth" && pid == f.BridgePID || kind == "backend birth" && pid == f.BackendPID {
						p.Start = "reused"
					}
					return p, ok
				}
			case "missing receipt":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
			case "changed receipt root":
				changed := f
				changed.SessionID = "newer-root"
				if err := writeBindingJSON(frontendPath(f.Instance), changed); err != nil {
					t.Fatal(err)
				}
			case "expired receipt":
				f.StartedAt = time.Now().Add(-time.Minute).UnixMilli()
				f.UpdatedAt = time.Now().Add(-6 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			case "foreign home":
				t.Setenv("CODEX_HOME", t.TempDir())

			case "malformed destination":
				if err := os.WriteFile(fileFor(dest), []byte("{invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			case "malformed old proof", "unknown destination process":
				other := f
				other.Instance = "abcdef0123456789abcdef0123456789"
				other.Loc = dest
				other.Pane = "%10"
				if err := writeBindingJSON(fileFor(dest), frontendRecord(other)); err != nil {
					t.Fatal(err)
				}
				if kind == "malformed old proof" {
					if err := os.WriteFile(frontendPath(other.Instance), []byte("{invalid"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					other.BridgePID = 900
					if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
						t.Fatal(err)
					}
				}
			case "newer destination owner":
				other := f
				other.Instance = "abcdef0123456789abcdef0123456789"
				other.Sequence++
				other.StartedAt++
				other.UpdatedAt++
				other.Loc = dest
				if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
					t.Fatal(err)
				}
				if err := writeBindingJSON(fileFor(dest), frontendRecord(other)); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(fileFor(dest))
			proofBefore, _ := os.ReadFile(frontendPath(f.Instance))
			if _, err := RefreshCodexFrontend(f); err == nil {
				t.Error("identity/owner conflict was accepted as a location migration")
			}
			after, _ := os.ReadFile(fileFor(dest))
			proofAfter, _ := os.ReadFile(frontendPath(f.Instance))
			if string(after) != string(before) || string(proofAfter) != string(proofBefore) {
				t.Error("refused relocation changed destination or its live proof")
			}
		})
	}
}

func TestCFL003MigrationCancellationKeepsOnlyCommittedHistory(t *testing.T) {
	for _, stage := range []string{"proof", "index"} {
		t.Run(stage, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			old := f.Loc
			dest := "work:0.1"
			pane.Loc = dest
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			write := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = write })
			frontendBindingWrite = func(path string, value any) error {
				err := write(path, value)
				if err == nil && (stage == "proof" && path == frontendPath(f.Instance) || stage == "index" && path == fileFor(dest)) {
					cancel()
				}
				return err
			}
			if _, err := RefreshCodexFrontendContext(ctx, f); !errors.Is(err, context.Canceled) {
				t.Fatal("relocation ignored cancellation", err)
			}
			if _, ok := LoadLive(dest); ok {
				t.Error("canceled relocation retained new live authority")
			}
			if _, ok := LoadLive(old); ok {
				t.Error("canceled relocation retained old live authority")
			}
			if r, ok := Load(old); !ok || r.SessionID != f.SessionID {
				t.Error("old committed history was deleted", r, ok)
			}
			if stage == "index" {
				if r, ok := Load(dest); !ok || r.SessionID != f.SessionID {
					t.Error("committed new history was deleted", r, ok)
				}
			}
		})
	}
}

func TestCFL003PartialMigrationCanRetryOnlyTheSameLiveOwner(t *testing.T) {
	for _, stage := range []string{"proof", "index"} {
		t.Run(stage, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			old := f.Loc
			dest := "work:0.1"
			pane.Loc = dest
			write := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = write })
			frontendBindingWrite = func(path string, value any) error {
				if stage == "proof" && path == frontendPath(f.Instance) || stage == "index" && path == fileFor(dest) {
					return errors.New("synthetic write failure")
				}
				return write(path, value)
			}
			if _, err := RefreshCodexFrontend(f); err == nil {
				t.Fatal("partial publication was reported as successful")
			}
			if _, ok := LoadLive(dest); ok {
				t.Error("partial new location gained authority")
			}
			if _, ok := LoadLive(old); ok {
				t.Error("partial old location gained authority")
			}
			frontendBindingWrite = write
			recovered, err := RefreshCodexFrontend(f)
			if err != nil || recovered.Loc != dest {
				t.Fatal("same current connection could not retry its publication", recovered, err)
			}
			if r, ok := LoadLive(dest); !ok || r.Frontend != f.Instance {
				t.Fatal("retry lost exact owner", r, ok)
			}
		})
	}
}

func TestCFL003LateOldOwnerCannotReclaimMovedSameSID(t *testing.T) {
	f, pane, _ := locationFixture(t)
	dest := "work:0.1"
	pane.Loc = dest
	next := f
	next.Instance = "abcdef0123456789abcdef0123456789"
	next.Sequence++
	next.StartedAt++
	next.Loc = dest
	current, err := BindCodexFrontend(next, f.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(fileFor(dest))
	proofBefore, _ := os.ReadFile(frontendPath(current.Instance))
	if _, err := RefreshCodexFrontend(f); err == nil {
		t.Error("late old heartbeat was accepted after sameSID replacement")
	}
	if _, err := BindCodexFrontend(f, "late-old-root"); err == nil {
		t.Error("late old observed root reclaimed current location")
	}
	EndCodexFrontend(f)
	after, _ := os.ReadFile(fileFor(dest))
	proofAfter, _ := os.ReadFile(frontendPath(current.Instance))
	if string(before) != string(after) || string(proofBefore) != string(proofAfter) {
		t.Error("old publication/close changed new owner")
	}
	if r, ok := LoadLive(dest); !ok || r.Frontend != current.Instance || r.SessionID != f.SessionID {
		t.Error("sameSID current owner not retained", r, ok)
	}
}

func TestCFL001FirstObservedRootCanFollowPostAdmissionLayoutChange(t *testing.T) {
	f, _ := frontendFixture(t)
	dest := "work:0.1"
	codexProcessPanes = func() []codexterminal.Pane {
		return []codexterminal.Pane{{ID: f.Pane, Command: "bash", PID: f.PanePID, TTY: "/dev/ttys999", Cwd: f.Cwd, Loc: dest}}
	}
	frontendPaneView = func(_ string, format string) string {
		if format == "#{session_name}:#{window_index}.#{pane_index}" {
			return dest
		}
		return f.Pane + "\t100\t" + dest
	}
	bound, err := BindCodexFrontend(f, "first-observed-root")
	if err != nil || bound.Loc != dest || bound.Instance != f.Instance {
		t.Fatal("admitted connection lost its first successful root after layout changed", bound, err)
	}
	if got, ok := LoadLive(dest); !ok || got.SessionID != bound.SessionID || got.Frontend != f.Instance {
		t.Fatal("first root was not published at current location", got, ok)
	}
}

// Two independently authenticated producers share only the publication lease.
// They must both converge after a swap, without borrowing each other's checkpoint.
func TestCFL003TwoLivePanesExchangePositionsWithoutLosingTheirOwners(t *testing.T) {
	a, _ := frontendFixture(t)
	panes := []codexterminal.Pane{
		{ID: a.Pane, PID: a.PanePID, TTY: "/dev/ttys999", Cwd: a.Cwd, Loc: a.Loc},
		{ID: "%10", PID: 300, TTY: "/dev/ttys998", Cwd: a.Cwd, Loc: "work:0.1"},
	}
	clients := map[string]codexterminal.Client{
		a.Pane: {PID: 101, Start: "client-start", RootStart: "root-start", TTY: "ttys999"},
		"%10":  {PID: 301, Start: "other-client-start", RootStart: "other-root-start", TTY: "ttys998"},
	}
	codexProcessPanes = func() []codexterminal.Pane { return panes }
	frontendClients = func(_ []codexterminal.Pane) map[string]codexterminal.Client { return clients }
	frontendPaneView = func(id, format string) string {
		for _, p := range panes {
			if id != p.ID && id != p.Loc {
				continue
			}
			if format == "#{session_name}:#{window_index}.#{pane_index}" {
				return p.Loc
			}
			if format == "#{pane_id}" {
				return p.ID
			}
			return fmt.Sprintf("%s\t%d\t%s", p.ID, p.PID, p.Loc)
		}
		return ""
	}
	var err error
	a, err = BindCodexFrontend(a, "root-A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := BeginCodexFrontend("%10", "abcdef0123456789abcdef0123456789", 301, 200, 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err = BindCodexFrontend(b, "root-B")
	if err != nil {
		t.Fatal(err)
	}
	// Distinct earlier recovery history in the same cwd exposes accidental inheritance.
	for _, f := range []CodexFrontend{a, b} {
		r, _ := Load(f.Loc)
		r.RecoverySessionID = "checkpoint-" + f.SessionID
		r.RecoveryCwd = f.Cwd
		r.RecoveryAt = 10
		if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
			t.Fatal(err)
		}
	}
	panes[0].Loc, panes[1].Loc = panes[1].Loc, panes[0].Loc
	movedA, err := RefreshCodexFrontend(a)
	if err != nil {
		t.Fatal("first producer could not migrate", err)
	}
	movedB, err := RefreshCodexFrontend(b)
	if err != nil {
		t.Fatal("second producer blocked by first producer's stale index", err)
	}
	for _, f := range []CodexFrontend{movedA, movedB} {
		r, ok := LoadLive(f.Loc)
		if !ok || r.SessionID != f.SessionID || r.Frontend != f.Instance {
			t.Fatal("swap confused current owners", f, r, ok)
		}
		if r.RecoverySessionID != "checkpoint-"+f.SessionID {
			t.Fatal("swap lost or inherited another owner's recovery checkpoint", f.SessionID, r)
		}
		got, ok := CodexFrontendForPane(f.Pane)
		if !ok || got.Instance != f.Instance {
			t.Fatal("strict pane reader confused owner", got, ok)
		}
	}
	EndCodexFrontend(a)
	EndCodexFrontend(b)
	for _, f := range []CodexFrontend{movedA, movedB} {
		if r, ok := Load(f.Loc); !ok || r.SessionID != f.SessionID {
			t.Fatal("exit removed swapped history", r, ok)
		}
	}
}

func TestCFL003HistoryWriteFailureRefusesMigrationBeforeLivePublication(t *testing.T) {
	f, pane, _ := locationFixture(t)
	dest := "work:0.1"
	pane.Loc = dest
	old := Record{Agent: "codex", HomeID: f.HomeID, SessionID: "displaced-old-root", Frontend: "abcdef0123456789abcdef0123456789", Cwd: f.Cwd}
	if err := writeBindingJSON(fileFor(dest), old); err != nil {
		t.Fatal(err)
	}
	beforeProof, _ := os.ReadFile(frontendPath(f.Instance))
	beforeIndex, _ := os.ReadFile(fileFor(dest))
	write := frontendBindingWrite
	t.Cleanup(func() { frontendBindingWrite = write })
	frontendBindingWrite = func(path string, value any) error {
		if strings.Contains(path, string(filepath.Separator)+"codex-location-history"+string(filepath.Separator)) {
			return errors.New("synthetic history IO failure")
		}
		return write(path, value)
	}
	if _, err := RefreshCodexFrontend(f); err == nil {
		t.Fatal("history IO failure was ignored")
	}
	afterProof, _ := os.ReadFile(frontendPath(f.Instance))
	afterIndex, _ := os.ReadFile(fileFor(dest))
	if string(beforeProof) != string(afterProof) || string(beforeIndex) != string(afterIndex) {
		t.Fatal("failed history preservation changed live proof or displaced index")
	}
	frontendBindingWrite = write
	if _, err := RefreshCodexFrontend(f); err != nil {
		t.Fatal("current owner could not retry", err)
	}
	historyDir := filepath.Join(Dir(), "codex-location-history")
	entries, err := os.ReadDir(historyDir)
	if err != nil || len(entries) != 1 {
		t.Fatal("original displaced record was not retained exactly once", entries, err)
	}
	archived, err := os.ReadFile(filepath.Join(historyDir, entries[0].Name()))
	if err != nil || string(archived) != string(beforeIndex) {
		t.Fatal("original displaced record was not preserved", string(archived), err)
	}
}

func TestCFL003ObservedNewRootRetriesItsOwnPartialMovedPublication(t *testing.T) {
	for _, stage := range []string{"proof", "index"} {
		t.Run(stage, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			pane.Loc = "work:2.0"
			write := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = write })
			frontendBindingWrite = func(path string, value any) error {
				if stage == "proof" && path == frontendPath(f.Instance) || stage == "index" && path == fileFor(pane.Loc) {
					return errors.New("synthetic moved root IO failure")
				}
				return write(path, value)
			}
			_, err := BindCodexFrontend(f, "new-observed-root")
			var proof *FrontendBindingError
			if err == nil || !errors.As(err, &proof) || !proof.Retryable {
				t.Error("publication IO did not retain bounded root-recovery eligibility", err)
			}
			if _, ok := LoadLive(pane.Loc); ok {
				t.Error("partial root gained live input authority")
			}
			frontendBindingWrite = write
			recovered, err := BindCodexFrontend(f, "new-observed-root")
			if err != nil || recovered.Loc != pane.Loc || recovered.SessionID != "new-observed-root" {
				t.Fatal("same observed root could not finish its own partial publication", recovered, err)
			}
			if r, ok := LoadLive(pane.Loc); !ok || r.SessionID != recovered.SessionID || r.Frontend != f.Instance {
				t.Fatal("recovered moved root has no exact association", r, ok)
			}
		})
	}
}
