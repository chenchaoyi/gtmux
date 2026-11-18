package resume

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCNR001NativeRestoreCommandRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	r := Record{Agent: "codex", SessionID: "exact-session", HomeID: transcript.CodexHomeID(), Cwd: "/work with ' quote"}
	for _, embedded := range []bool{false, true} {
		argv, command, err := NativeRestoreCommand(r, "/package with ' quote/bin/codex", home, embedded, "")
		if err != nil {
			t.Fatal(err)
		}
		var words []string
		for _, arg := range argv {
			words = append(words, shellSingleQuote(arg))
		}
		agent, sid := FromCommand(strings.Join(words, " "))
		if agent != "codex" || sid != r.SessionID {
			t.Fatalf("lost real argv: %q %q", agent, sid)
		}
		if strings.Contains(command, "gtmux codex") || !strings.Contains(command, "CODEX_HOME=") ||
			!strings.Contains(command, shellSingleQuote(r.Cwd)) {
			t.Fatal("native recipe lost exact home/cwd or re-entered the bridge")
		}
		if strings.Contains(strings.Join(argv, " "), "--no-daemon") != embedded {
			t.Fatal("capability decision changed after construction")
		}
	}
	t.Run("R2-proxy-is-on-native-env-not-cd", func(t *testing.T) {
		proxy := "https://fixture.invalid/a b'$(never-run);"
		argv, command, err := NativeRestoreCommand(r, "/package with ' quote/bin/codex", home, true, proxy)
		if err != nil {
			t.Fatal(err)
		}
		cd := "cd -- " + shellSingleQuote(r.Cwd) + " && "
		if !strings.HasPrefix(command, cd) {
			t.Fatal("proxy assignments were attached to cd")
		}
		var words []string
		for _, word := range argv {
			words = append(words, shellSingleQuote(word))
		}
		want := "env CODEX_HOME=" + shellSingleQuote(home) + " HTTPS_PROXY=" + shellSingleQuote(proxy) +
			" HTTP_PROXY=" + shellSingleQuote(proxy) + " " + strings.Join(words, " ")
		if strings.TrimPrefix(command, cd) != want {
			t.Fatal("actual native env lost safe proxy quoting")
		}
		_, direct, err := NativeRestoreCommand(r, "/bin/codex", home, false, "")
		if err != nil || strings.Contains(direct, "HTTPS_PROXY=") {
			t.Fatal("unconfigured proxy was invented")
		}
		if _, _, err := NativeRestoreCommand(r, "/bin/codex", home, true, "https://bad\x00proxy"); err == nil {
			t.Fatal("NUL proxy accepted")
		}
	})

	custom := r
	custom.Launcher = "company-wrapper"
	if _, _, err := NativeRestoreCommand(custom, "/bin/codex", home, true, ""); err == nil {
		t.Fatal("custom launcher silently replaced")
	}
	legacy, ok := CommandWithBridge(custom, "/bin/gtmux")
	if !ok || !strings.Contains(legacy, "company-wrapper") {
		t.Fatal("legacy wrapper contract lost")
	}
}

func cnrStoppedFixture(t *testing.T) (native.Record, codexterminal.Pane, Record, int64) {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	r, pane, now := processBindingFixture(t)
	oldProcess, oldGone, oldClients, oldView := frontendProcess, frontendRecoveryGone, frontendClients, frontendPaneView
	oldWriter := codexProcessBindingWrite
	t.Cleanup(func() {
		frontendProcess, frontendRecoveryGone, frontendClients, frontendPaneView = oldProcess, oldGone, oldClients, oldView
		codexProcessBindingWrite = oldWriter
	})
	frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
		if pid != 909 {
			t.Fatal("unexpected frontend PID observation; refuse real process fallback")
		}
		return launchhost.AgentProcess{}, false
	}
	frontendRecoveryGone = func(pid int) bool {
		if pid != 909 {
			t.Fatal("unexpected exit observation; refuse real process fallback")
		}
		return true
	}
	frontendClients = func(panes []codexterminal.Pane) map[string]codexterminal.Client {
		if len(panes) != 1 || panes[0] != pane {
			t.Fatal("unexpected frontend pane snapshot; refuse real tmux fallback")
		}
		return nil
	}
	frontendPaneView = func(id, format string) string {
		if id != pane.ID || format != "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}" {
			t.Fatal("unexpected pane view; refuse real tmux fallback")
		}
		return pane.ID + "\t" + strconv.Itoa(pane.PID) + "\t" + pane.Loc
	}
	instance := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	incumbent := Record{Agent: "codex", SessionID: r.SessionID, HomeID: r.HomeID, Cwd: pane.Cwd,
		Frontend: instance, TerminalClient: "codex", UpdatedAt: now - 20}
	if err := Save(pane.Loc, incumbent); err != nil {
		t.Fatal(err)
	}
	incumbent, _ = Load(pane.Loc)
	f := CodexFrontend{Instance: instance, HomeID: r.HomeID, SessionID: r.SessionID,
		BridgePID: 909, BridgeStart: "old-bridge", BackendPID: 808, BackendStart: "old-backend",
		StartedAt: now*1000 - 1000, UpdatedAt: now * 1000, Sequence: 1,
		CodexBindingTarget: CodexBindingTarget{Pane: pane.ID, Loc: pane.Loc, Cwd: pane.Cwd,
			PID: 303, PanePID: pane.PID, ClientStart: "old-client", RootStart: "root-birth", TTY: "ttys001"}}
	body, _ := json.Marshal(f)
	if err := state.WritePrivateFile(frontendPath(instance), body); err != nil {
		t.Fatal(err)
	}

	return r, pane, incumbent, now
}

func TestCNR002StoppedFrontendMigratesSameSession(t *testing.T) {
	r, pane, before, now := cnrStoppedFixture(t)
	receipt, _ := os.ReadFile(frontendPath(before.Frontend))
	nativeBefore, _ := native.Load(r.SessionID, r.HomeID)
	p := launchhost.AgentProcess{PID: r.PID, Comm: r.Comm, Start: r.ProcessStart, ObservedAt: now}
	if got := BindCodexHookProcess(r.SessionID, r.Cwd, "SessionStart", p, now); got != pane.ID {
		t.Fatal("same SID stopped bridge did not migrate")
	}
	after, _ := Load(pane.Loc)
	if after.SessionID != before.SessionID || after.HomeID != before.HomeID || after.Frontend != "" ||
		after.TerminalClient != before.TerminalClient || after.RecoverySessionID != before.RecoverySessionID ||
		after.RecoveryCwd != before.RecoveryCwd || after.RecoveryAt != before.RecoveryAt {
		t.Fatalf("migration changed ownership/checkpoint: %+v", after)
	}
	still, _ := os.ReadFile(frontendPath(before.Frontend))
	if !bytes.Equal(receipt, still) {
		t.Fatal("migration rewrote historical receipt")
	}
	current, _ := native.Load(r.SessionID, r.HomeID)
	if current != nativeBefore {
		t.Fatal("migration changed native history")
	}
}

func TestCNR003FrontendMigrationRejectsUnknownAndRaces(t *testing.T) {
	for _, kind := range []string{"live", "unknown", "missing", "corrupt", "owner-change", "pane-change", "lease", "write-failure"} {
		t.Run(kind, func(t *testing.T) {
			r, pane, before, now := cnrStoppedFixture(t)
			switch kind {
			case "live":
				if frontendPaneView(pane.ID, "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}") !=
					pane.ID+"\t"+strconv.Itoa(pane.PID)+"\t"+pane.Loc {
					t.Fatal("synthetic frontend view was not installed")
				}
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					if pid != 909 {
						t.Fatal("live fixture reached unexpected PID")
					}
					return launchhost.AgentProcess{PID: pid, Start: "old-bridge"}, true
				}
			case "unknown":
				frontendRecoveryGone = func(pid int) bool {
					if pid != 909 {
						t.Fatal("unknown fixture reached unexpected PID")
					}
					return false
				}
			case "missing":
				if err := os.Remove(frontendPath(before.Frontend)); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(frontendPath(before.Frontend), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "owner-change", "pane-change":
				old := codexProcessPanes
				calls := 0
				codexProcessPanes = func() []codexterminal.Pane {
					calls++
					if calls == 2 {
						if kind == "owner-change" {
							_ = Save(pane.Loc, Record{Agent: "codex", HomeID: r.HomeID, SessionID: "different-owner", UpdatedAt: now + 1})
						} else {
							p := pane
							p.PID++
							p.TTY = "ttys-other"
							return []codexterminal.Pane{p}
						}
					}
					return old()
				}
			case "lease":
				l, err := state.TryLease("codex-frontend-binding")
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
			case "write-failure":
				codexProcessBindingWrite = func(string, any) error { return errors.New("injected private publish failure") }
			}
			p := launchhost.AgentProcess{PID: r.PID, Comm: r.Comm, Start: r.ProcessStart, ObservedAt: now}
			if got := BindCodexHookProcess(r.SessionID, r.Cwd, "SessionStart", p, now); got != "" {
				t.Fatalf("unsafe %s returned a successful native association", kind)
			}
			if after, ok := Load(pane.Loc); ok && after.SessionID == r.SessionID && after.Frontend == "" {
				t.Fatal("unproven same-SID bridge was erased")
			}
		})
	}
}

func TestCNR004ReconcileRetriesStoppedSameSession(t *testing.T) {
	r, pane, _, now := cnrStoppedFixture(t)
	ReconcileCodexProcesses(transcript.NewCodexPathScope(), []native.Record{r}, map[string]string{pane.ID: pane.Loc}, now)
	if got, _ := Load(pane.Loc); got.SessionID != r.SessionID || got.Frontend != "" {
		t.Fatal("same-SID stopped marker was incorrectly treated as already native")
	}
}

func TestCNR005MigratedNativeSaveCannotReviveFrontend(t *testing.T) {
	r, pane, _, now := cnrStoppedFixture(t)
	p := launchhost.AgentProcess{PID: r.PID, Comm: r.Comm, Start: r.ProcessStart, ObservedAt: now}
	if BindCodexHookProcess(r.SessionID, r.Cwd, "SessionStart", p, now) != pane.ID {
		t.Fatal("migration failed")
	}
	if err := Save(pane.Loc, Record{Agent: "codex", SessionID: r.SessionID, HomeID: r.HomeID, Cwd: pane.Cwd, UpdatedAt: now + 1}); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(pane.Loc); got.Frontend != "" {
		t.Fatal("ordinary Save resurrected the stopped bridge")
	}
}

func cnrAttemptIdentity(t *testing.T) (NativeRestoreIdentity, *string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	boot := "11111111-1111-1111-1111-111111111111"
	original := nativeRestoreBootProbe
	nativeRestoreBootProbe = func() (string, error) { return boot, nil }
	t.Cleanup(func() { nativeRestoreBootProbe = original })
	return NativeRestoreIdentity{BootID: boot, Home: home, HomeID: transcript.CodexHomeID(),
		SessionID: "same-session", SaveRevision: "revision", Pane: "%1", Loc: "worker:0.0", RootPID: 101,
		RootBirth: "root-birth", Shell: "bash", TTY: "ttys001", InitialCwd: home, Cwd: home,
		BinaryPath: "/pkg/bin/codex", LookupPath: "/pkg/codex", BinarySHA: "sha", PackageSHA: "pkg",
		BinaryDevice: 1, BinaryInode: 2, BinarySize: 100, BinaryMTime: 1}, &boot
}

func TestCNR107ReservationSurvivesUntilNewBoot(t *testing.T) {
	for _, stateName := range []string{"reserved", "pending", "prefilled", "unknown", "failed", "not-sent", "confirmed"} {
		t.Run(stateName, func(t *testing.T) {
			id, boot := cnrAttemptIdentity(t)
			a, err := ReserveNativeRestoreAttempt(id)
			if err != nil {
				t.Fatal(err)
			}
			if stateName != "reserved" {
				target := stateName
				if target == "confirmed" {
					target = "pending"
				}
				a, err = UpdateNativeRestoreAttempt(a, target)
				if err != nil {
					t.Fatal(err)
				}
				if stateName == "confirmed" {
					a, err = UpdateNativeRestoreAttempt(a, "confirmed")
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			other := id
			other.Pane, other.Loc, other.SaveRevision = "%2", "other:0.0", "new-save"
			if _, err := ReserveNativeRestoreAttempt(other); !errors.Is(err, ErrNativeRestoreReserved) {
				t.Fatal("same-boot state/save/pane bypassed durable occupancy", err)
			}
			*boot = "22222222-2222-2222-2222-222222222222"
			other.BootID = *boot
			newAttempt, err := ReserveNativeRestoreAttempt(other)
			if err != nil || newAttempt.ID == a.ID {
				t.Fatal("verified new boot did not create one new attempt", err)
			}
			if _, err := UpdateNativeRestoreAttempt(a, "unknown"); err == nil {
				t.Fatal("late old attempt CAS overwrote the new boot")
			}
		})
	}
}

func TestCNR006CorruptAttemptAndLeaseRefuseBeforeAnyWrite(t *testing.T) {
	id, _ := cnrAttemptIdentity(t)
	l, err := state.TryLease("restore-native-attempts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReserveNativeRestoreAttempt(id); !errors.Is(err, state.ErrLeaseHeld) {
		t.Fatal("held publication lease was ignored", err)
	}
	l.Close()
	if _, err := ReserveNativeRestoreAttempt(id); err != nil {
		t.Fatal(err)
	}
	path := nativeRestoreAttemptPath(id)
	for _, body := range [][]byte{[]byte("{}"), bytes.Repeat([]byte("x"), 8193)} {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReserveNativeRestoreAttempt(id); err == nil {
			t.Fatal("malformed state was treated as missing")
		}
	}
}

func TestCNR007NativeConfirmationRequiresExactFreshOwner(t *testing.T) {
	for _, kind := range []string{"exact", "frontend", "different-session", "different-home", "stale-hook", "root-reused", "pane-reused", "unknown-result", "late-cas"} {
		t.Run(kind, func(t *testing.T) {
			r, pane, now := processBindingFixture(t)
			home, err := filepath.EvalSymlinks(transcript.CodexHome())
			if err != nil {
				t.Fatal(err)
			}
			boot := "11111111-1111-1111-1111-111111111111"
			oldBoot, oldRoot := nativeRestoreBootProbe, nativeRestoreRootRead
			t.Cleanup(func() { nativeRestoreBootProbe, nativeRestoreRootRead = oldBoot, oldRoot })
			nativeRestoreBootProbe = func() (string, error) { return boot, nil }
			nativeRestoreRootRead = func(pid int) (launchhost.AgentProcess, bool) {
				return launchhost.AgentProcess{PID: pid, Comm: "bash", Start: "root-birth"}, true
			}
			id := NativeRestoreIdentity{BootID: boot, Home: home, HomeID: r.HomeID,
				SessionID: r.SessionID, SaveRevision: "revision", Pane: pane.ID, Loc: pane.Loc,
				RootPID: pane.PID, RootBirth: "root-birth", Shell: "bash", TTY: pane.TTY,
				InitialCwd: pane.Cwd, Cwd: pane.Cwd, BinaryPath: "/pkg/bin/codex", LookupPath: "/pkg/codex",
				BinarySHA: "sha", PackageSHA: "pkg", BinaryInode: 1, BinarySize: 10}
			a := NativeRestoreAttempt{Schema: 1, ID: strings.Repeat("a", 32), State: "pending",
				Identity: id, IssuedAt: (now - 20) * 1000000000, PriorPID: 303, PriorBirth: "old-client"}
			current := Record{Agent: "codex", HomeID: r.HomeID, SessionID: r.SessionID, Cwd: r.Cwd, UpdatedAt: now}
			switch kind {
			case "frontend":
				current.Frontend = "old"
			case "different-session":
				current.SessionID = "other"
			case "different-home":
				current.HomeID = "other"
			case "stale-hook":
				a.IssuedAt = now * 1000000000
			case "root-reused":
				nativeRestoreRootRead = func(pid int) (launchhost.AgentProcess, bool) {
					return launchhost.AgentProcess{PID: pid, Comm: "bash", Start: "different-root-birth"}, true
				}
			case "pane-reused":
				old := codexProcessPanes
				codexProcessPanes = func() []codexterminal.Pane {
					p := old()[0]
					p.PID++
					return []codexterminal.Pane{p}
				}
			case "unknown-result":
				a.State = "unknown"
			}
			if err := publishNativeRestoreAttempt(a); err != nil {
				t.Fatal(err)
			}
			if kind == "late-cas" {
				newer := a
				newer.ID = strings.Repeat("b", 32)
				if err := publishNativeRestoreAttempt(newer); err != nil {
					t.Fatal(err)
				}
			}
			got := ConfirmNativeRestoreAttempt(a, current)
			if got != (kind == "exact") {
				t.Fatalf("confirmation %s=%v", kind, got)
			}
			saved, ok, err := readNativeRestoreAttempt(nativeRestoreAttemptPath(id))
			if err != nil || !ok {
				t.Fatal("confirmation lost durable occupancy", err)
			}
			if kind == "exact" && saved.State != "confirmed" {
				t.Fatal("confirmation was not durable")
			}
			if kind == "late-cas" && saved.ID != strings.Repeat("b", 32) {
				t.Fatal("late confirmation replaced newer attempt")
			}
		})
	}
}

func TestCNR006IncumbentAbsenceIsNotReadFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := state.MkdirAll(Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	missing, err := ReadNativeRestoreIncumbent("missing:0.0")
	if err != nil || missing.Exists || missing != (NativeRestoreIncumbent{}) {
		t.Fatal("true absence was not preserved", err)
	}
	for _, kind := range []string{"valid", "malformed", "empty", "null", "empty-record", "trailing", "oversized", "permission", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			loc := "reader-" + kind + ":0.0"
			path := fileFor(loc)
			body := []byte("{\"agent\":\"codex\",\"sessionId\":\"private-session\"}")
			switch kind {
			case "malformed":
				body = []byte("{")
			case "empty":
				body = nil
			case "null":
				body = []byte("null")
			case "empty-record":
				body = []byte("{}")
			case "trailing":
				body = append(body, []byte(" false")...)
			case "oversized":
				body = bytes.Repeat([]byte("x"), 32769)
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(t.TempDir(), "private-record")
				if err := os.WriteFile(target, body, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "directory" && kind != "symlink" {
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "permission" {
					if err := os.Chmod(path, 0000); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(path, 0600) })
				}
			}
			got, err := ReadNativeRestoreIncumbent(loc)
			if kind != "valid" {
				if err == nil || got.Exists {
					t.Fatal("unreadable/invalid record was treated as absence or valid", kind)
				}
				if kind == "permission" && !errors.Is(err, os.ErrPermission) {
					t.Fatal("permission failure not observable", err)
				}
				return
			}
			if err != nil || !got.Exists || got.Record.SessionID != "private-session" || got.ContentSHA == "" {
				t.Fatal("valid incumbent rejected", err)
			}
			if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			newer, err := ReadNativeRestoreIncumbent(loc)
			if err != nil || newer.Record != got.Record || newer.ContentSHA == got.ContentSHA {
				t.Fatal("same decoded record with changed bytes was not bound", err)
			}
		})
	}
}
