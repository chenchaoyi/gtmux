package native

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := Record{Agent: "claude", SessionID: "sid-1", Cwd: "/x", State: "working", UpdatedAt: 100, Terminal: "Warp"}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	got, ok := Load("sid-1")
	if !ok || got.State != "working" || got.Agent != "claude" || got.Terminal != "Warp" {
		t.Fatalf("Load = %+v, %v", got, ok)
	}
	Remove("sid-1")
	if _, ok := Load("sid-1"); ok {
		t.Error("record should be gone after Remove")
	}
}

// Live returns fresh records and self-prunes stale ones (past StaleAfter).
func TestLivePrunesStale(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_000_000)
	_ = Save(Record{SessionID: "fresh", Agent: "claude", State: "idle", UpdatedAt: now - 60})
	_ = Save(Record{SessionID: "stale", Agent: "claude", State: "idle", UpdatedAt: now - int64(StaleAfter/time.Second) - 1})

	live := Live(now)
	if len(live) != 1 || live[0].SessionID != "fresh" {
		t.Fatalf("Live = %+v, want just the fresh record", live)
	}
	if _, ok := Load("stale"); ok {
		t.Error("stale record should be pruned from disk by Live")
	}
}

// Live prunes a record whose process has EXITED (the phantom "elsewhere" bug) even
// when the record is otherwise fresh; a record for a live process is kept.
func TestLivePrunesDeadProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// The real clock: a record "written" before this test process started would read as
	// one whose pid has since been reused.
	now := time.Now().Unix()

	// A reaped child → its pid is dead (kill → ESRCH; if the OS reused it, its comm
	// won't be "claude" → still pruned).
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run a throwaway process: %v", err)
	}
	deadPID := c.Process.Pid

	_ = Save(Record{SessionID: "dead", Agent: "claude", State: "working", UpdatedAt: now, PID: deadPID, Comm: "claude"})
	_ = Save(Record{SessionID: "alive", Agent: "claude", State: "idle", UpdatedAt: now, PID: os.Getpid()}) // no comm → kept

	live := Live(now)
	if len(live) != 1 || live[0].SessionID != "alive" {
		t.Fatalf("Live = %+v, want just the alive record", live)
	}
	if _, ok := Load("dead"); ok {
		t.Error("record for a dead process should be pruned by Live")
	}
}

// A pid that's alive but whose command no longer matches the recorded comm (pid
// reuse — common right after a reboot) is treated as gone and pruned.
func TestLivePrunesReusedPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_000_000)
	_ = Save(Record{SessionID: "reused", Agent: "claude", State: "idle", UpdatedAt: now,
		PID: os.Getpid(), Comm: "definitely-not-this-process"})
	if live := Live(now); len(live) != 0 {
		t.Fatalf("Live = %+v, want empty (pid reused → comm mismatch → pruned)", live)
	}
}

// A session id with filesystem-hostile chars round-trips (base64url keying).
func TestSaveUnsafeSessionID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sid := "a/b:c d"
	if err := Save(Record{SessionID: sid, Agent: "codex", State: "waiting", UpdatedAt: 5}); err != nil {
		t.Fatal(err)
	}
	if got, ok := Load(sid); !ok || got.State != "waiting" {
		t.Fatalf("unsafe id round-trip failed: %+v %v", got, ok)
	}
}

// %12's reproduction (2026-10-06): an idle session whose process is alive was deleted
// once it had sent no hook for StaleAfter. The spec keeps an idle-but-alive session.
// The record is written now (after this process started) and Live is asked 13 hours
// later; the process is this test, alive throughout.
func TestLiveKeepsAnIdleSessionWhoseProcessIsAlive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	written := time.Now().Unix()
	later := written + 13*3600
	_ = Save(Record{SessionID: "idle-alive", Agent: "claude", State: "idle", UpdatedAt: written, PID: os.Getpid()})
	_ = Save(Record{SessionID: "idle-alive-comm", Agent: "claude", State: "idle", UpdatedAt: written,
		PID: os.Getpid(), Comm: procComm(os.Getpid())})
	if live := Live(later); len(live) != 2 {
		t.Fatalf("Live = %+v, want both alive idle sessions kept", live)
	}
}

// A record nobody can check keeps the grace: kept within it, gone past it.
func TestLiveGivesAnUncheckableRecordTheGrace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	written := time.Now().Unix()
	_ = Save(Record{SessionID: "no-pid", Agent: "claude", State: "idle", UpdatedAt: written})
	if live := Live(written + 3600); len(live) != 1 {
		t.Fatalf("an hour in: %+v, want the record kept", live)
	}
	if live := Live(written + 13*3600); len(live) != 0 {
		t.Fatalf("thirteen hours in: %+v, want it gone", live)
	}

	// ps could not answer for a live pid: also unknown, also the grace.
	saved := procStart
	procStart = func(int) (int64, bool) { return 0, false }
	t.Cleanup(func() { procStart = saved })
	_ = Save(Record{SessionID: "ps-silent", Agent: "claude", State: "idle", UpdatedAt: written, PID: os.Getpid()})
	if live := Live(written + 3600); len(live) != 1 {
		t.Fatalf("ps silent, an hour in: %+v, want kept", live)
	}
	if live := Live(written + 13*3600); len(live) != 0 {
		t.Fatalf("ps silent, thirteen hours in: %+v, want gone: missing evidence is not life", live)
	}
}

// A pid now held by a process that started after the record's last update did not write
// it: the pid was reused, whatever the command is called, and the record goes at once.
func TestLivePrunesAPidReusedByANewerProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	_ = Save(Record{SessionID: "older", Agent: "claude", State: "idle", UpdatedAt: now - 3600,
		PID: os.Getpid(), Comm: procComm(os.Getpid())})
	if live := Live(now); len(live) != 0 {
		t.Fatalf("Live = %+v, want the record gone: this process started after it was written", live)
	}
}

func TestParseEtime(t *testing.T) {
	for in, want := range map[string]int64{"00:05": 5, "12:34": 754, "01:02:03": 3723, "2-01:02:03": 2*86400 + 3723} {
		if got, ok := parseEtime(in); !ok || got != want {
			t.Errorf("parseEtime(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "5", "a:b", "1-2", "-1:00"} {
		if _, ok := parseEtime(bad); ok {
			t.Errorf("parseEtime(%q) accepted", bad)
		}
	}
}

// fakePS puts a stand-in ps first on PATH for this test: it answers `-o comm=` and
// `-o etime=` from the given values, and exits 1 for a value of "fail". Only this test
// process runs it; the pid it is asked about is still checked with a real kill(pid, 0).
func fakePS(t *testing.T, comm, etime string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$2\" in\n" +
		"comm=) [ \"$FAKE_COMM\" = fail ] && exit 1; printf '%s\\n' \"$FAKE_COMM\";;\n" +
		"etime=) [ \"$FAKE_ETIME\" = fail ] && exit 1; printf '%s\\n' \"$FAKE_ETIME\";;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_COMM", comm)
	t.Setenv("FAKE_ETIME", etime)
}

// %12's review of 67c1311b: each reading counts only for what it shows. An empty command
// is no command (it used to read as "."), and a command that plainly differs is gone even
// when the start time cannot be read.
func TestLiveWeighsEachPsReadingOnItsOwn(t *testing.T) {
	const hourAgo = "01:00:00" // started an hour before now: older than a record written now
	for _, tc := range []struct {
		name        string
		comm, etime string
		recorded    string // the record's Comm
		at1h, at13h bool   // kept an hour later / thirteen hours later
	}{
		{"empty command, recorded command", "", hourAgo, "claude", true, false},
		{"empty command, nothing recorded", "", hourAgo, "", true, false},
		{"another command, start unreadable", "other", "fail", "claude", false, false},
		{"another command, start readable", "other", hourAgo, "claude", false, false},
		{"same command, start unreadable", "claude", "fail", "claude", true, false},
		{"command unreadable, start unreadable", "fail", "fail", "claude", true, false},
		{"same command, older start", "claude", hourAgo, "claude", true, true},
		{"nothing recorded, older start", "claude", hourAgo, "", true, true},
		{"same command, newer start", "claude", "00:01", "claude", false, false},
		{"empty command, newer start", "", "00:01", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			fakePS(t, tc.comm, tc.etime)
			written := time.Now().Unix()
			if tc.etime == "00:01" {
				written -= 3600 // the record is an hour old; the process a second old
			}
			r := Record{SessionID: "s", Agent: "claude", State: "idle", UpdatedAt: written, PID: os.Getpid(), Comm: tc.recorded}
			for _, step := range []struct {
				after int64
				want  bool
			}{{3600, tc.at1h}, {13 * 3600, tc.at13h}} {
				_ = Save(r)
				if kept := len(Live(time.Now().Unix()+step.after)) == 1; kept != step.want {
					t.Fatalf("%dh later: kept = %v, want %v", step.after/3600, kept, step.want)
				}
			}
		})
	}
}
