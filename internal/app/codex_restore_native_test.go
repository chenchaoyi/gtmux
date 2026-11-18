package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func cnrHelp(resumeHelp, flag bool) string {
	header := "Codex CLI\nUsage: codex [OPTIONS]\nCommands:\n  resume Resume a session\n"
	if resumeHelp {
		header = "Usage: codex resume [OPTIONS] [SESSION_ID] [PROMPT]\n"
	}
	option := ""
	if flag {
		option = "      --no-daemon\n          Run without shared server\n"
	}
	return header + "\nOptions:\n" + option + "  -h, --help\n          Print help\n  -V, --version\n          Print version\n"
}

func TestCNR101CapabilityBoundToNativeBinary(t *testing.T) {
	original := nativeRestoreHelp
	t.Cleanup(func() { nativeRestoreHelp = original })
	for _, kind := range []string{"supported", "unsupported", "unknown-exit", "empty", "truncated", "inconsistent", "failed-combined"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			nativeRestoreHelp = func(path string, args ...string) (string, bool) {
				if path != "/physical/bin/codex" {
					t.Fatal("capability did not use bound physical executable")
				}
				calls++
				if kind == "unknown-exit" {
					return "error mentions --no-daemon", false
				}
				if kind == "empty" {
					return "", true
				}
				if kind == "truncated" {
					return strings.TrimSuffix(cnrHelp(false, true), "Print version\n"), true
				}
				if len(args) == 3 && kind == "failed-combined" {
					return "", false
				}
				isResume := len(args) > 1
				flag := kind != "unsupported"
				if kind == "inconsistent" && isResume {
					flag = false
				}
				return cnrHelp(isResume, flag), true
			}
			got := probeNativeRestoreCapability(nativeRestoreBinary{Physical: "/physical/bin/codex"})
			want := nativeCapabilityUnknown
			if kind == "supported" {
				want = nativeCapabilitySupported
			}
			if kind == "unsupported" {
				want = nativeCapabilityUnsupported
			}
			if got != want || calls > 3 {
				t.Fatalf("capability %s=%d calls=%d", kind, got, calls)
			}
		})
	}
	output := nativeRestoreOutput{limit: 16}
	_, _ = output.Write([]byte(strings.Repeat("x", 17)))
	if !output.overflow || output.body.Len() > 16 {
		t.Fatal("oversized helper output was accepted")
	}
}

func cnrAppPlan(t *testing.T) (*nativeRestorePlan, *nativeRestorePaneState, *nativeRestoreBinary) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("CODEX_EXEC_SERVER_URL", "")
	t.Setenv("GTMUX_AGENT_PROXY", "off")
	rec := resume.Record{Agent: "codex", SessionID: "cnr-session", HomeID: transcript.CodexHomeID(), Cwd: home, UpdatedAt: time.Now().Unix()}
	writeRecoveryFixture(t, rec.SessionID, home)
	if err := resume.Save("worker:0.0", rec); err != nil {
		t.Fatal(err)
	}
	pane := &nativeRestorePaneState{Pane: "%7", Loc: "worker:0.0", Cwd: home, TTY: "/dev/ttys001",
		Command: "bash", RootCommand: "bash", RootPID: 101, RootBirth: "root-birth"}
	binary := &nativeRestoreBinary{Lookup: "/lookup/codex", Physical: "/physical/bin/codex",
		SHA: "sha", PackageSHA: "pkg", Inode: 1, Device: 1, Size: 10, MTime: 1}
	oldFind, oldInspect, oldPane, oldProbe, oldBoot := nativeRestoreFindBinary, nativeRestoreInspectBinary, nativeRestoreReadPane, nativeRestoreProbe, nativeRestoreBoot
	oldReserve, oldUpdate, oldSend := nativeRestoreReserve, nativeRestoreUpdate, nativeRestoreSend
	oldIncumbent := nativeRestoreReadIncumbent
	oldSessions, oldTargets := remoteRestoreSessions, remoteRestoreTargets
	t.Cleanup(func() {
		nativeRestoreFindBinary, nativeRestoreInspectBinary, nativeRestoreReadPane, nativeRestoreProbe, nativeRestoreBoot = oldFind, oldInspect, oldPane, oldProbe, oldBoot
		nativeRestoreReserve, nativeRestoreUpdate, nativeRestoreSend = oldReserve, oldUpdate, oldSend
		nativeRestoreReadIncumbent = oldIncumbent
		remoteRestoreSessions, remoteRestoreTargets = oldSessions, oldTargets
	})
	remoteRestoreSessions, remoteRestoreTargets = nil, nil
	nativeRestoreReadIncumbent = resume.ReadNativeRestoreIncumbent
	nativeRestoreFindBinary = func() string { return binary.Lookup }
	nativeRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) { return *binary, true, nil }
	nativeRestoreReadPane = func(id, loc string) (nativeRestorePaneState, bool) {
		return *pane, id == pane.Pane && loc == pane.Loc
	}
	nativeRestoreProbe = func(context.Context, nativeRestoreBinary) nativeRestoreCapability { return nativeCapabilitySupported }
	nativeRestoreBoot = func() (string, error) { return "11111111-1111-1111-1111-111111111111", nil }
	nativeRestoreReserve = func(id resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
		return resume.NativeRestoreAttempt{Schema: 1, ID: strings.Repeat("a", 32), State: "reserved", Identity: id, IssuedAt: time.Now().UnixNano()}, nil
	}
	nativeRestoreUpdate = func(a resume.NativeRestoreAttempt, next string) (resume.NativeRestoreAttempt, error) {
		a.State = next
		return a, nil
	}
	nativeRestoreSend = func(string, string, bool) error {
		t.Fatal("test forgot its isolated send seam")
		return errors.New("unreachable")
	}
	plan, handled, why := prepareNativeRestore(pane.Pane, pane.Loc, rec, map[nativeRestoreBinary]nativeRestoreCapability{})
	if !handled || plan == nil {
		t.Fatalf("fixture plan rejected: %s", why)
	}
	return plan, pane, binary
}

func TestCNR102RestoreTargetRecheckedBeforeWrite(t *testing.T) {
	for _, kind := range []string{"busy", "root-birth", "tty", "cwd", "binary", "boot", "home"} {
		t.Run(kind, func(t *testing.T) {
			plan, pane, binary := cnrAppPlan(t)
			reserves, sends := 0, 0
			nativeRestoreReserve = func(resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
				reserves++
				return resume.NativeRestoreAttempt{}, nil
			}
			nativeRestoreSend = func(string, string, bool) error { sends++; return nil }
			switch kind {
			case "busy":
				nativeRestoreReadPane = func(string, string) (nativeRestorePaneState, bool) { return *pane, false }
			case "root-birth":
				pane.RootBirth = "reused"
			case "tty":
				pane.TTY = "ttys-other"
			case "cwd":
				pane.Cwd = "/different"
			case "binary":
				binary.Inode++
			case "boot":
				nativeRestoreBoot = func() (string, error) { return "22222222-2222-2222-2222-222222222222", nil }
			case "home":
				t.Setenv("CODEX_HOME", t.TempDir())
			}
			got, sent := issueNativeRestore(plan, resumeAuto, map[string]bool{})
			if sent || sends != 0 || reserves != 0 || got.State != "target-changed" {
				t.Fatalf("changed %s reached sender: %+v", kind, got)
			}
		})
	}
}

func TestCNR103ReservePrecedesProductionSend(t *testing.T) {
	plan, _, _ := cnrAppPlan(t)
	// This source proposes the real private attempt store plus the production
	// issue helper; it does not claim coverage of the outer resumeAgents loop.
	nativeRestoreBoot = resume.NativeRestoreBootID
	nativeRestoreReserve = resume.ReserveNativeRestoreAttempt
	nativeRestoreUpdate = resume.UpdateNativeRestoreAttempt
	var handled bool
	var why string
	plan, handled, why = prepareNativeRestore(plan.Pane.Pane, plan.Pane.Loc, plan.Record, map[nativeRestoreBinary]nativeRestoreCapability{})
	if !handled || plan == nil {
		t.Fatalf("platform boot identity unavailable: %s", why)
	}
	used := map[string]bool{}
	nativeRestoreSend = func(pane, command string, enter bool) error {
		if !used[plan.Record.SessionID] || pane != plan.Pane.Pane || !enter {
			t.Fatal("production sender ran before dedup")
		}
		sum := sha256.Sum256([]byte(plan.Identity.Home + "\x00" + plan.Identity.SessionID))
		path := filepath.Join(state.Dir(), "restore-attempts", fmt.Sprintf("%x.json", sum))
		file, err := state.OpenPrivateRead(path)
		if err != nil {
			t.Fatal("first write preceded durable reservation", err)
		}
		defer file.Close()
		var disk resume.NativeRestoreAttempt
		if err := json.NewDecoder(io.LimitReader(file, 8193)).Decode(&disk); err != nil {
			t.Fatal(err)
		}
		if disk.State != "reserved" || disk.Identity != plan.Identity || len(disk.ID) != 32 {
			t.Fatal("first write has no exact durable reservation")
		}
		if !strings.Contains(command, "--no-daemon") || strings.Contains(command, "gtmux codex") {
			t.Fatal("native route re-entered the bridge")
		}
		return nil
	}
	got, sent := issueNativeRestore(plan, resumeAuto, used)
	if !sent || got.State != "pending" {
		t.Fatalf("wrong startup result: %+v", got)
	}
}

func TestCNR104UncertainWriteNeverReplays(t *testing.T) {
	for _, kind := range []string{"literal-unknown", "enter-unknown", "result-write-failed"} {
		t.Run(kind, func(t *testing.T) {
			plan, _, _ := cnrAppPlan(t)
			sends, reserves := 0, 0
			nativeRestoreReserve = func(id resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
				reserves++
				if reserves > 1 {
					return resume.NativeRestoreAttempt{}, resume.ErrNativeRestoreReserved
				}
				return resume.NativeRestoreAttempt{State: "reserved", Identity: id}, nil
			}
			nativeRestoreSend = func(string, string, bool) error {
				sends++
				if kind != "result-write-failed" {
					return errors.New(kind)
				}
				return nil
			}
			if kind == "result-write-failed" {
				nativeRestoreUpdate = func(a resume.NativeRestoreAttempt, next string) (resume.NativeRestoreAttempt, error) {
					return a, errors.New("private result write failed")
				}
			}
			used := map[string]bool{}
			got, _ := issueNativeRestore(plan, resumeAuto, used)
			if got.State != "unknown" || !used[plan.Record.SessionID] {
				t.Fatal("unknown send released ownership")
			}
			_, _ = issueNativeRestore(plan, resumeAuto, used)
			_, _ = issueNativeRestore(plan, resumeAuto, map[string]bool{})
			if sends != 1 || reserves != 2 {
				t.Fatalf("unknown was replayed: sends=%d reserves=%d", sends, reserves)
			}
		})
	}
}

func TestCNR105TypeModeNeverPressesEnter(t *testing.T) {
	plan, _, _ := cnrAppPlan(t)
	sends := 0
	nativeRestoreSend = func(_ string, _ string, enter bool) error {
		sends++
		if enter {
			t.Fatal("type-only pressed Enter")
		}
		return nil
	}
	used := map[string]bool{}
	got, sent := issueNativeRestore(plan, resumeType, used)
	_, _ = issueNativeRestore(plan, resumeType, used)
	if !sent || got.State != "prefilled" || sends != 1 {
		t.Fatal("prefill not exactly once")
	}
}

func TestCNR106LeaseChild(t *testing.T) {
	want := os.Getenv("GTMUX_CNR_CHILD")
	if want == "" {
		t.Skip("private subprocess helper")
	}
	if !strings.Contains(os.Getenv("HOME"), "TestCNR106AttemptLeaseAcrossProcesses") {
		t.Fatal("refuse non-private home")
	}
	lease, err := state.TryLease("restore-native-attempts")
	if want == "held" {
		if !errors.Is(err, state.ErrLeaseHeld) {
			t.Fatal("cross-process lease was bypassed", err)
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		lease.Close()
	}
}

func TestCNR106AttemptLeaseAcrossProcesses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := func(want string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCNR106LeaseChild$", "-test.count=1")
		cmd.Env = append(os.Environ(), "GTMUX_CNR_CHILD="+want)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("private child %s: %v %s", want, err, output)
		}
	}
	lease, err := state.TryLease("restore-native-attempts")
	if err != nil {
		t.Fatal(err)
	}
	child("held")
	lease.Close()
	child("free")
}

func TestCNR109NativePackageClassifierRefusesWrapperAndChangedMetadata(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("native package formats under test")
	}
	arch := map[string]string{"arm64": "aarch64", "amd64": "x86_64"}[runtime.GOARCH]
	if arch == "" {
		t.Skip("package target unsupported")
	}
	for _, kind := range []string{"native", "script", "missing-meta", "invalid-meta", "non-executable", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "packages", "standalone", "releases", "fixture")
			path := filepath.Join(root, "bin", "codex")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			header := []byte{0x7f, 'E', 'L', 'F'}
			target := arch + "-unknown-linux-gnu"
			if runtime.GOOS == "darwin" {
				header = []byte{0xcf, 0xfa, 0xed, 0xfe}
				target = arch + "-apple-darwin"
			}
			body := append(header, []byte("inert fixture; never execute")...)
			if kind == "script" {
				body = []byte("#!/bin/sh\nexit 99\n")
			}
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, body, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "non-executable" {
					if err := os.Chmod(path, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			meta := "{\"layoutVersion\":1,\"version\":\"fixture\",\"target\":\"" + target + "\",\"variant\":\"codex\",\"entrypoint\":\"bin/codex\"}"
			if kind == "invalid-meta" {
				meta = "{}"
			}
			if kind != "missing-meta" {
				if err := os.WriteFile(filepath.Join(root, "codex-package.json"), []byte(meta), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, handled, err := inspectNativeRestoreBinary(path)
			if kind == "native" {
				if err != nil || !handled || got.SHA == "" || got.PackageSHA == "" || got.Inode == 0 {
					t.Fatal("native identity unavailable", err)
				}
			} else if kind == "script" {
				if handled {
					t.Fatal("same-name script was classified as the native package")
				}
			} else if err == nil || !handled {
				t.Fatal("recognized package with unknown metadata must defer")
			}
		})
	}
}

func cnrIncumbentPath(loc string) string {
	return filepath.Join(resume.Dir(), base64.RawURLEncoding.EncodeToString([]byte(loc))+".json")
}

func cnrReadPrivateAttempt(t *testing.T, id resume.NativeRestoreIdentity) resume.NativeRestoreAttempt {
	t.Helper()
	sum := sha256.Sum256([]byte(id.Home + "\x00" + id.SessionID))
	file, err := state.OpenPrivateRead(filepath.Join(state.Dir(), "restore-attempts", fmt.Sprintf("%x.json", sum)))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var a resume.NativeRestoreAttempt
	if err := json.NewDecoder(io.LimitReader(file, 8193)).Decode(&a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCNR102ExecutableFailureNeverChoosesCompatibility(t *testing.T) {
	for _, kind := range []string{"false-with-error", "true-with-error", "missing-physical", "empty-lookup", "confirmed-layout", "explicit-launcher", "explicit-server"} {
		t.Run(kind, func(t *testing.T) {
			plan, _, _ := cnrAppPlan(t)
			// A pre-existing same-boot unknown is real private durable state.
			boot, err := resume.NativeRestoreBootID()
			if err != nil {
				t.Fatal(err)
			}
			id := plan.Identity
			id.BootID = boot
			a, err := resume.ReserveNativeRestoreAttempt(id)
			if err != nil {
				t.Fatal(err)
			}
			a, err = resume.UpdateNativeRestoreAttempt(a, "unknown")
			if err != nil {
				t.Fatal(err)
			}
			reserves, sends, compatibility := 0, 0, 0
			nativeRestoreReserve = func(resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
				reserves++
				return resume.NativeRestoreAttempt{}, resume.ErrNativeRestoreReserved
			}
			nativeRestoreSend = func(string, string, bool) error { sends++; return nil }
			switch kind {
			case "false-with-error":
				nativeRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) { return nativeRestoreBinary{}, false, os.ErrPermission }
			case "true-with-error":
				nativeRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) {
					return nativeRestoreBinary{}, true, errors.New("private read failed")
				}
			case "missing-physical":
				nativeRestoreFindBinary = func() string { return filepath.Join(t.TempDir(), "missing", "codex") }
				nativeRestoreInspectBinary = inspectNativeRestoreBinary
			case "empty-lookup":
				nativeRestoreFindBinary = func() string { return "" }
				nativeRestoreInspectBinary = inspectNativeRestoreBinary
			case "confirmed-layout":
				nativeRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) { return nativeRestoreBinary{}, false, nil }
			case "explicit-launcher":
				plan.Record.Launcher = "company-wrapper"
			case "explicit-server":
				t.Setenv("CODEX_EXEC_SERVER_URL", "unix:///private-fixture.sock")
			}
			result, handled, sent := tryNativeRestore(plan.Pane.Pane, plan.Pane.Loc, plan.Record,
				map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{})
			if !handled {
				compatibility++
			} // exact production routing boundary; never invoke real legacy sender
			wantCompatibility := kind == "confirmed-layout" || kind == "explicit-launcher" || kind == "explicit-server"
			if (compatibility == 1) != wantCompatibility || sent || sends != 0 || reserves != 0 {
				t.Fatalf("wrong fallback/send on %s: %+v compat=%d send=%d reserve=%d", kind, result, compatibility, sends, reserves)
			}
			if !wantCompatibility && (!handled || result.State != "executable-unavailable" || resumeStateLabel(result.State) == "") {
				t.Fatal("native preparation error not observable")
			}
			if after := cnrReadPrivateAttempt(t, id); after != a {
				t.Fatal("failure or compatibility check changed unknown reservation")
			}
		})
	}
}

func TestCNR102IncumbentChangesRefuseBeforeWrite(t *testing.T) {
	for _, kind := range []string{"missing-to-corrupt", "missing-to-permission", "missing-to-io", "missing-to-valid", "valid-to-valid", "valid-to-same-record-new-bytes", "valid-to-missing", "after-reserve-corrupt", "prepare-corrupt"} {
		t.Run(kind, func(t *testing.T) {
			plan, _, _ := cnrAppPlan(t)
			path := cnrIncumbentPath(plan.Pane.Loc)
			if strings.HasPrefix(kind, "missing-") || kind == "after-reserve-corrupt" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				var handled bool
				var why string
				plan, handled, why = prepareNativeRestore(plan.Pane.Pane, plan.Pane.Loc, plan.Record,
					map[nativeRestoreBinary]nativeRestoreCapability{})
				if !handled || plan == nil || plan.Incumbent.Exists {
					t.Fatalf("true missing incumbent not supported: %s", why)
				}
			}
			reserves, sends, notSent, compatibility := 0, 0, 0, 0
			nativeRestoreSend = func(string, string, bool) error { sends++; return nil }
			nativeRestoreReserve = func(id resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
				reserves++
				if kind == "after-reserve-corrupt" {
					if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return resume.NativeRestoreAttempt{State: "reserved", Identity: id}, nil
			}
			nativeRestoreUpdate = func(a resume.NativeRestoreAttempt, next string) (resume.NativeRestoreAttempt, error) {
				if next == "not-sent" {
					notSent++
				}
				a.State = next
				return a, nil
			}
			wantState := "target-changed"
			switch kind {
			case "missing-to-corrupt", "prepare-corrupt":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				wantState = "incumbent-unavailable"
			case "missing-to-permission", "missing-to-io":
				cause := error(os.ErrPermission)
				if kind == "missing-to-io" {
					cause = errors.New("injected private IO failure")
				}
				nativeRestoreReadIncumbent = func(string) (resume.NativeRestoreIncumbent, error) {
					return resume.NativeRestoreIncumbent{}, &os.PathError{Op: "read", Path: "private-fixture", Err: cause}
				}
				wantState = "incumbent-unavailable"
			case "missing-to-valid", "valid-to-valid":
				body, err := json.Marshal(resume.Record{Agent: "codex", SessionID: "different-owner", HomeID: plan.Record.HomeID})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			case "valid-to-same-record-new-bytes":
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
				now, err := resume.ReadNativeRestoreIncumbent(plan.Pane.Loc)
				if err != nil || now.Record != plan.Incumbent.Record || now.ContentSHA == plan.Incumbent.ContentSHA {
					t.Fatal("same-record raw-change fixture invalid", err)
				}
			case "valid-to-missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "after-reserve-corrupt":
				wantState = "incumbent-unavailable"
			}
			used := map[string]bool{}
			var result resumeAttempt
			var sent bool
			if kind == "prepare-corrupt" {
				var handled bool
				result, handled, sent = tryNativeRestore(plan.Pane.Pane, plan.Pane.Loc, plan.Record,
					map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, used)
				if !handled {
					compatibility++
				}
			} else {
				result, sent = issueNativeRestore(plan, resumeAuto, used)
			}
			if sent || sends != 0 || compatibility != 0 || result.State != wantState || resumeStateLabel(result.State) == "" {
				t.Fatalf("unreadable/changed incumbent was sent or hidden: %s %+v", kind, result)
			}
			if kind == "after-reserve-corrupt" {
				if reserves != 1 || notSent != 1 || !used[plan.Record.SessionID] {
					t.Fatal("late read failure released reserved ownership")
				}
			} else if reserves != 0 || notSent != 0 {
				t.Fatal("failed preparation wrote reservation/result")
			}
		})
	}
}

func TestCNR102ConfiguredProxyTargetsNativeCommand(t *testing.T) {
	plan, _, _ := cnrAppPlan(t)
	proxy := "https://private.invalid/a b'$(never-run);"
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("GTMUX_AGENT_PROXY", proxy)
	var handled bool
	var why string
	plan, handled, why = prepareNativeRestore(plan.Pane.Pane, plan.Pane.Loc, plan.Record,
		map[nativeRestoreBinary]nativeRestoreCapability{})
	if !handled || plan == nil {
		t.Fatalf("proxy fixture rejected: %s", why)
	}
	sends := 0
	nativeRestoreSend = func(_ string, command string, _ bool) error {
		sends++
		if command != plan.Command || !strings.HasPrefix(command, "cd -- ") {
			t.Fatal("proxy was wrapped around cd at send")
		}
		_, actual, ok := strings.Cut(command, " && env ")
		if !ok || !strings.Contains(actual, "HTTPS_PROXY='https://private.invalid/a b'") ||
			!strings.Contains(actual, " HTTP_PROXY=") || !strings.Contains(actual, "'/physical/bin/codex'") {
			t.Fatal("configured proxy missing from actual executable env")
		}
		return nil
	}
	if result, sent := issueNativeRestore(plan, resumeAuto, map[string]bool{}); !sent || result.State != "pending" || sends != 1 {
		t.Fatal("proxy-bearing native command not sent exactly once")
	}
}
