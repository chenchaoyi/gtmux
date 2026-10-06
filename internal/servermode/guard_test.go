package servermode

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The guard is a security artifact: it runs as root, forever, on the user's machine.
// These tests are the reason it can be trusted, so they check properties rather than
// spot-checking strings.

// THE invariant. If this ever fails, the guard has gained the power to escalate and
// the entire safety model of server mode is void.
func TestGuardHasNoPathThatDisablesSleep(t *testing.T) {
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	if !GuardCanOnlyRestore(script) {
		t.Fatal("the guard must contain NO instruction that disables sleep")
	}
	// Belt and braces: every pmset invocation in the script must be the restore.
	for _, ln := range strings.Split(script, "\n") {
		code, _, _ := strings.Cut(ln, "#")
		if !strings.Contains(code, "PMSET") && !strings.Contains(code, "pmset") {
			continue
		}
		if strings.Contains(code, "disablesleep") && !strings.Contains(code, "disablesleep 0") {
			t.Errorf("pmset line touches disablesleep without restoring it: %q", strings.TrimSpace(ln))
		}
	}
}

// The guard runs as root, so anything it executes must live somewhere a non-root
// user cannot rewrite. A user-writable path here would be a local privilege
// escalation: replace the file, wait 30s, get root.
func TestGuardOnlyRunsSystemBinaries(t *testing.T) {
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	for _, ln := range strings.Split(script, "\n") {
		code, _, _ := strings.Cut(ln, "#")
		for _, f := range strings.Fields(code) {
			if !strings.HasPrefix(f, "/") || !strings.Contains(f, "/bin/") {
				continue
			}
			f = strings.Trim(f, "\"'")
			if !strings.HasPrefix(f, "/bin/") && !strings.HasPrefix(f, "/usr/bin/") &&
				!strings.HasPrefix(f, "/usr/sbin/") && !strings.HasPrefix(f, "/sbin/") {
				t.Errorf("guard executes something outside the system paths: %q", f)
			}
		}
	}
}

// It must be valid shell. A syntax error would mean the guard silently never runs
// and a Mac quietly stays unable to sleep — a failure with no symptom until the
// battery is flat.
func TestGuardScriptIsValidShell(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(GuardScript("/tmp/state.json", "/tmp/revoke"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("guard script is not valid shell: %v\n%s", err, out)
	}
}

// Every reason the guard can record must be one the rest of the system knows how to
// explain; an unrecognized reason would surface to the user as a bare token.
func TestGuardReasonsAreKnown(t *testing.T) {
	known := map[string]bool{
		ReasonRevoked: true, ReasonBatteryLow: true,
		ReasonStaleHeartbeat: true, ReasonBootReconcile: true,
	}
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	for _, ln := range strings.Split(script, "\n") {
		_, after, ok := strings.Cut(strings.TrimSpace(ln), "restore ")
		if !ok || strings.HasPrefix(strings.TrimSpace(ln), "#") {
			continue
		}
		reason := strings.Fields(after)
		if len(reason) == 0 || strings.HasPrefix(reason[0], "<") {
			continue // the function definition / doc line
		}
		if !known[reason[0]] {
			t.Errorf("guard records an unknown exit reason %q", reason[0])
		}
	}
}

// The paths are baked in at install time because the guard has no user session to
// resolve $HOME from. A relative or empty path would make it silently do nothing.
func TestGuardScriptBakesAbsolutePaths(t *testing.T) {
	script := GuardScript("/Users/x/.local/share/gtmux/server-mode/state.json",
		"/Users/x/.local/share/gtmux/server-mode/revoke")
	for _, want := range []string{
		"/Users/x/.local/share/gtmux/server-mode/state.json",
		"/Users/x/.local/share/gtmux/server-mode/revoke",
		GuardPlistPath, GuardScriptPath,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("guard script missing baked path %q", want)
		}
	}
	if strings.Contains(script, "$HOME") {
		t.Error("the guard has no user session — $HOME must never appear in it")
	}
}

func TestGuardPlistShape(t *testing.T) {
	p := GuardPlist("/tmp/watch")
	for _, want := range []string{GuardLabel, GuardScriptPath, "RunAtLoad", "StartInterval"} {
		if !strings.Contains(p, want) {
			t.Errorf("guard plist missing %q", want)
		}
	}
	// RunAtLoad is what handles the reboot path; without it a machine could come
	// back up unable to sleep with nothing scheduled to notice. Compare with
	// whitespace collapsed so a formatting change can't fail a semantic check.
	flat := strings.Join(strings.Fields(p), "")
	if !strings.Contains(flat, "<key>RunAtLoad</key><true/>") {
		t.Error("RunAtLoad must be true — it is the reboot safety net")
	}
}

// The charge floor is the guardrail that saves a laptop forgotten in a bag, and it is
// the one branch that never runs during normal use — by the time it matters, nobody
// is watching. So it gets exercised here against fabricated `pmset` output, once per
// case that decides whether a machine is rescued or drained.
//
// The guard's binary paths are absolute BY DESIGN (a root daemon must never resolve
// through PATH), so the test rewrites them to point at a fake — in the copy under
// test only. Production keeps its hardcoded /usr/bin/pmset.
func TestGuardChargeFloor(t *testing.T) {
	for _, tc := range []struct {
		name        string
		pmsetOutput string
		wantRestore bool
	}{
		{"on mains, full", "Now drawing from 'AC Power'\n -InternalBattery-0\t100%; charged; 0:00 remaining present: true\n", false},
		{"on mains, low battery — plugged in, so no reason to act",
			"Now drawing from 'AC Power'\n -InternalBattery-0\t9%; charging; 1:20 remaining present: true\n", false},
		{"on battery, plenty left — carrying it between rooms is the point",
			"Now drawing from 'Battery Power'\n -InternalBattery-0\t80%; discharging; 4:10 remaining present: true\n", false},
		{"on battery, just above the floor", "Now drawing from 'Battery Power'\n -InternalBattery-0\t21%; discharging; 0:50 remaining present: true\n", false},
		{"on battery, AT the floor — must rescue", "Now drawing from 'Battery Power'\n -InternalBattery-0\t20%; discharging; 0:45 remaining present: true\n", true},
		{"on battery, nearly flat — must rescue", "Now drawing from 'Battery Power'\n -InternalBattery-0\t4%; discharging; 0:08 remaining present: true\n", true},
		{"desktop: no battery line at all", "Now drawing from 'AC Power'\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// A fake pmset that reports our scenario and LOGS any write it is asked
			// to make, so we can tell rescue from no-op.
			fake := dir + "/pmset"
			os.WriteFile(fake, []byte("#!/bin/sh\n"+
				"if [ \"$1\" = \"-g\" ]; then cat <<'OUT'\n"+tc.pmsetOutput+"OUT\n"+
				"  exit 0\nfi\n"+
				"echo \"$@\" >> "+dir+"/calls\n"), 0o755)

			// State fresh enough that only the charge branch can fire.
			os.WriteFile(dir+"/state.json", []byte(`{"tier":"clamshell"}`), 0o644)

			script := GuardScript(dir+"/state.json", dir+"/revoke")
			script = strings.ReplaceAll(script, "/usr/bin/pmset", fake)
			// The restore reads the kernel back; a fake that says sleep is enabled keeps
			// this test off the real machine's state.
			fakeIoreg := dir + "/ioreg"
			os.WriteFile(fakeIoreg, []byte("#!/bin/sh\necho '+-o IOPMrootDomain'\necho '  \"SleepDisabled\" = No'\n"), 0o755)
			script = strings.ReplaceAll(script, "/usr/sbin/ioreg", fakeIoreg)
			script = strings.ReplaceAll(script, "/bin/sleep", "/usr/bin/true")
			// Neutralise the self-removal + launchctl so the test only measures the
			// decision, not the teardown.
			script = strings.ReplaceAll(script, "/bin/launchctl", "/usr/bin/true")
			script = strings.ReplaceAll(script, "/bin/rm -f", "/usr/bin/true")
			script = strings.ReplaceAll(script, GuardDir+"/last-exit.json", dir+"/last-exit.json")
			sp := dir + "/guard.sh"
			os.WriteFile(sp, []byte(script), 0o755)
			stubbed(t, script)

			out, _ := exec.Command("/bin/sh", sp).CombinedOutput()
			calls, _ := os.ReadFile(dir + "/calls")
			restored := strings.Contains(string(calls), "disablesleep 0")

			if restored != tc.wantRestore {
				t.Errorf("restore=%v, want %v\ncalls: %q\noutput: %s",
					restored, tc.wantRestore, calls, out)
			}
			// Whatever happens, the guard must never be able to disable sleep.
			if strings.Contains(string(calls), "disablesleep 1") {
				t.Fatal("the guard disabled sleep — it must only ever restore it")
			}
		})
	}
}

// Turning server mode off must never require a password — the whole safety model
// rests on de-escalation being free. WatchPaths is what makes that possible without
// also making the user wait for the next tick: writing the unprivileged marker wakes
// the daemon immediately.
func TestGuardWatchesTheStandDownMarker(t *testing.T) {
	plist := GuardPlist("/Users/x/.local/share/gtmux/server-mode")
	flat := strings.Join(strings.Fields(plist), "")
	if !strings.Contains(flat, "<key>WatchPaths</key>") {
		t.Fatal("without WatchPaths, turning it off waits for the next tick — which is " +
			"exactly the pressure that led to prompting for a password")
	}
	if !strings.Contains(plist, "/Users/x/.local/share/gtmux/server-mode") {
		t.Error("the watched path must be where the stand-down marker is written")
	}
	// The interval stays as the backstop for what a path watch cannot observe:
	// charge falling, a stale heartbeat, an abandoned boot.
	if !strings.Contains(flat, "<key>StartInterval</key>") {
		t.Error("StartInterval must remain — WatchPaths cannot see the battery draining")
	}
}

// After the guard restores sleep, gtmux's ownership stamp must be gone too. If the
// stamp outlives the state, the next status read sees "our record says on, the kernel
// says off" and reports a normal shutdown as a LAPSE — alarming the user about a
// failure that did not happen.
func TestGuardClearsTheOwnershipStampWhenItRestores(t *testing.T) {
	dir := t.TempDir()
	fake := dir + "/pmset"
	os.WriteFile(fake, []byte("#!/bin/sh\n"+
		"if [ \"$1\" = \"-g\" ]; then echo \"Now drawing from 'AC Power'\"; exit 0; fi\n"+
		"echo \"$@\" >> "+dir+"/calls\n"), 0o755)
	state := dir + "/state.json"
	revoke := dir + "/revoke"
	os.WriteFile(state, []byte(`{"tier":"clamshell"}`), 0o644)
	os.WriteFile(revoke, []byte("1\n"), 0o644) // stand-down requested

	script := GuardScript(state, revoke)
	script = strings.ReplaceAll(script, "/usr/bin/pmset", fake)
	// The restore reads the kernel back before it clears anything. A fake that says
	// sleep is enabled keeps this off the machine's real state: on a runner without
	// /usr/sbin/ioreg the read fails and the guard rightly keeps everything.
	fakeIoreg := dir + "/ioreg"
	os.WriteFile(fakeIoreg, []byte("#!/bin/sh\necho '+-o IOPMrootDomain'\necho '  \"SleepDisabled\" = No'\n"), 0o755)
	script = strings.ReplaceAll(script, "/usr/sbin/ioreg", fakeIoreg)
	script = strings.ReplaceAll(script, "/bin/sleep", "/usr/bin/true")
	script = strings.ReplaceAll(script, "/bin/launchctl", "/usr/bin/true")
	script = strings.ReplaceAll(script, GuardDir+"/last-exit.json", dir+"/last-exit.json")
	sp := dir + "/guard.sh"
	os.WriteFile(sp, []byte(script), 0o755)
	stubbed(t, script)
	if out, err := exec.Command("/bin/sh", sp).CombinedOutput(); err != nil {
		t.Fatalf("guard failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(state); err == nil {
		t.Error("the ownership stamp must be removed — otherwise a clean shutdown reads as a lapse")
	}
	if _, err := os.Stat(revoke); err == nil {
		t.Error("the stand-down marker must be cleared, or the next enable is undone instantly")
	}
	calls, _ := os.ReadFile(dir + "/calls")
	if !strings.Contains(string(calls), "disablesleep 0") {
		t.Errorf("sleep was not restored: %q", calls)
	}
}

// guardRig runs the generated guard against fakes in a temp dir: pmset (its write exit
// code, and whether a write takes effect), ioreg (the kernel's answer, or no answer),
// launchctl, and the guard's own files, which live in the temp dir instead of /Library.
// The guard's restore decides whether the guard, its state and the stand-down marker
// survive, so those are real files here.
type guardRig struct {
	t                                              *testing.T
	dir, script, state, revoke, self, plist, calls string
}

func newGuardRig(t *testing.T) *guardRig {
	t.Helper()
	d := t.TempDir()
	g := &guardRig{t: t, dir: d, state: d + "/state.json", revoke: d + "/revoke", self: d + "/sleepguard.sh", plist: d + "/guard.plist", calls: d + "/calls"}
	write := func(name, body string) {
		if err := os.WriteFile(d+"/"+name, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The kernel: "1" while sleep is disabled. A write takes effect only when
	// pmset-works exists; pmset-exit sets the write's exit code.
	write("kernel", "1")
	write("pmset", "#!/bin/sh\n[ \"$1\" = -g ] && exit 0\necho \"pmset $*\" >> "+g.calls+"\n"+
		"[ -e "+d+"/pmset-works ] && echo 0 > "+d+"/kernel\n"+
		"[ -e "+d+"/pmset-exit ] && exit $(cat "+d+"/pmset-exit)\nexit 0\n")
	// ioreg: the kernel's answer, the node without the key, or nothing at all.
	write("ioreg", "#!/bin/sh\n[ -e "+d+"/ioreg-fails ] && exit 1\n"+
		"[ -e "+d+"/ioreg-no-node ] && { echo 'some other output'; exit 0; }\n"+
		"echo '+-o IOPMrootDomain  <class IOPMrootDomain>'\n"+
		"[ -e "+d+"/ioreg-no-key ] && exit 0\n"+
		"if [ \"$(cat "+d+"/kernel)\" = 1 ]; then echo '    \"SleepDisabled\" = Yes'; else echo '    \"SleepDisabled\" = No'; fi\n")
	write("launchctl", "#!/bin/sh\necho \"launchctl $*\" >> "+g.calls+"\n")
	script := GuardScript(g.state, g.revoke)
	for from, to := range map[string]string{
		"/usr/bin/pmset": d + "/pmset", "/usr/sbin/ioreg": d + "/ioreg", "/bin/launchctl": d + "/launchctl",
		"/bin/sleep": "/usr/bin/true", GuardScriptPath: g.self, GuardPlistPath: g.plist, GuardDir: d,
	} {
		script = strings.ReplaceAll(script, from, to)
	}
	write("sleepguard.sh", script)
	write("guard.plist", "plist")
	write("state.json", `{"tier":"clamshell"}`)
	write("revoke", "")
	g.script = script
	return g
}

func (g *guardRig) set(name, body string) {
	if err := os.WriteFile(g.dir+"/"+name, []byte(body), 0o644); err != nil {
		g.t.Fatal(err)
	}
}

func (g *guardRig) run() int {
	stubbed(g.t, g.script)
	cmd := exec.Command("/bin/sh", "-c", g.script)
	_ = cmd.Run()
	return cmd.ProcessState.ExitCode()
}

func (g *guardRig) exists(p string) bool { _, err := os.Stat(p); return err == nil }

// A restore that does not take, or cannot be confirmed, leaves the guard, its state and
// the stand-down marker in place and fails, so the next tick tries again. It used to
// delete all of it and record the guard as revoked on a pmset write that did not land
// (%12's reproduction, 2026-10-06), leaving nothing behind to restore sleep with.
func TestGuardKeepsEverythingWhenSleepIsNotBack(t *testing.T) {
	for name, setup := range map[string]func(*guardRig){
		"pmset fails":                   func(g *guardRig) { g.set("pmset-exit", "7") },
		"pmset says yes, nothing moves": func(*guardRig) {},
		"the kernel cannot be read":     func(g *guardRig) { g.set("pmset-works", ""); g.set("ioreg-fails", "") },
		"the power node is missing":     func(g *guardRig) { g.set("pmset-works", ""); g.set("ioreg-no-node", "") },
	} {
		t.Run(name, func(t *testing.T) {
			g := newGuardRig(t)
			setup(g)
			if code := g.run(); code == 0 {
				t.Error("the guard reported success")
			}
			for _, p := range []string{g.state, g.revoke, g.self, g.plist} {
				if !g.exists(p) {
					t.Errorf("%s was removed", p)
				}
			}
			calls, _ := os.ReadFile(g.calls)
			if strings.Contains(string(calls), "launchctl") {
				t.Error("the guard booted itself out")
			}
			if n := strings.Count(string(calls), "pmset -a disablesleep 0"); n != restoreTries {
				t.Errorf("the restore was written %d times, want %d", n, restoreTries)
			}
			if g.exists(g.dir + "/last-exit.json") {
				t.Error("an exit was recorded for a restore that did not take")
			}
			if !g.exists(g.dir + "/restore-unconfirmed.json") {
				t.Error("the failure left no record")
			}
		})
	}
}

// When the kernel confirms sleep is back, the guard records why and removes itself, as
// before; a node without the key reads as enabled, as gtmux reads it. A run that failed
// is retried by the next one, which then cleans up, its failure record included.
func TestGuardCleansUpOnceSleepIsConfirmedBack(t *testing.T) {
	for name, setup := range map[string]func(*guardRig){
		"the write takes":             func(g *guardRig) { g.set("pmset-works", "") },
		"no SleepDisabled key at all": func(g *guardRig) { g.set("ioreg-no-key", "") },
	} {
		t.Run(name, func(t *testing.T) {
			g := newGuardRig(t)
			setup(g)
			if code := g.run(); code != 0 {
				t.Fatalf("exit %d, want 0", code)
			}
			for _, p := range []string{g.state, g.revoke, g.self, g.plist} {
				if g.exists(p) {
					t.Errorf("%s survived a confirmed restore", p)
				}
			}
			exit, _ := os.ReadFile(g.dir + "/last-exit.json")
			if !strings.Contains(string(exit), `"reason":"revoked"`) {
				t.Errorf("exit record = %q", exit)
			}
			calls, _ := os.ReadFile(g.calls)
			if !strings.Contains(string(calls), "launchctl bootout system/"+GuardLabel) {
				t.Error("the guard did not boot itself out")
			}
		})
	}
	t.Run("a failed run, then the next tick", func(t *testing.T) {
		g := newGuardRig(t)
		if g.run() == 0 {
			t.Fatal("the first run should fail: the write does not take")
		}
		g.set("pmset-works", "")
		if code := g.run(); code != 0 {
			t.Fatalf("second run exit %d, want 0", code)
		}
		if g.exists(g.state) || g.exists(g.revoke) || g.exists(g.dir+"/restore-unconfirmed.json") {
			t.Error("the retry did not clean up, failure record included")
		}
	})
}

// stubbed fails the test if the script would still reach a binary that reads or changes
// the machine's power state. Every test that runs the guard calls it first: one that did
// not stub ioreg passed on a Mac, reading the real kernel, and failed on the Linux CI
// runner, which has no ioreg (2026-10-06).
func stubbed(t *testing.T, script string) {
	t.Helper()
	for _, bin := range []string{"/usr/bin/pmset", "/usr/sbin/ioreg", "/bin/launchctl"} {
		if strings.Contains(script, bin) {
			t.Fatalf("the guard under test would still run the real %s", bin)
		}
	}
}
