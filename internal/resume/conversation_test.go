package resume

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func corruptConversationIndex(t *testing.T, f CodexFrontend) []byte {
	t.Helper()
	b := []byte(`{"agent":"codex","sessionId":"untrusted-prefix"} trailing`)
	if err := os.WriteFile(fileFor(f.Loc), b, 0600); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCAS001IndependentConversationRecovery(t *testing.T) {
	for _, mode := range []string{"corrupt", "missing"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := frontendFixture(t)
			f, err := BindCodexFrontend(f, "verified-root")
			if err != nil {
				t.Fatal(err)
			}
			original := corruptConversationIndex(t, f)
			if mode == "missing" {
				Remove(f.Loc)
			}
			r, recovered, err := ConversationRecord(f.Loc)
			if err != nil || !recovered || r.SessionID != f.SessionID || r.Frontend != f.Instance {
				t.Fatalf("independent history not recovered: %+v %v %v", r, recovered, err)
			}
			if _, ok := LoadLive(f.Loc); ok {
				t.Fatal("read fallback gained primary/input authority")
			}
			if _, ok := CodexFrontendForThread(f.HomeID, f.SessionID); ok {
				t.Fatal("read fallback gained thread input binding")
			}
			b, readErr := os.ReadFile(fileFor(f.Loc))
			if mode == "missing" {
				if !os.IsNotExist(readErr) {
					t.Fatal("read created index")
				}
			} else if readErr != nil || string(b) != string(original) {
				t.Fatal("read mutated damaged index")
			}
		})
	}
}

func TestCAS001ConversationRecoveryRejectsUnknownOrConflictingProof(t *testing.T) {
	for _, mode := range []string{"absent", "expired", "peer-birth", "foreign-home", "ambiguous", "newer-expired", "trailing-json", "symlink", "oversized-store", "valid-replacement"} {
		t.Run(mode, func(t *testing.T) {
			f, client := frontendFixture(t)
			f.StartedAt -= 60000
			f, err := BindCodexFrontend(f, "root")
			if err != nil {
				t.Fatal(err)
			}
			corruptConversationIndex(t, f)
			switch mode {
			case "absent":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
			case "expired":
				f.UpdatedAt = time.Now().Add(-6 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			case "peer-birth":
				client.Start = "reused"
			case "foreign-home":
				f.HomeID = "foreign"
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			case "ambiguous", "newer-expired":
				other := f
				other.Instance = "abcdef0123456789abcdef0123456789"
				other.Sequence++
				other.StartedAt++
				if mode == "newer-expired" {
					other.UpdatedAt = time.Now().Add(-6 * time.Second).UnixMilli()
				}
				if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "trailing-json":
				b, e := os.ReadFile(frontendPath(f.Instance))
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(frontendPath(f.Instance), append(b, []byte(" {}")...), 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				path := frontendPath(f.Instance)
				if err := os.Rename(path, path+".owned"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".owned", path); err != nil {
					t.Fatal(err)
				}
			case "oversized-store":
				for n := 1; n <= 129; n++ {
					other := f
					other.Instance = fmt.Sprintf("%032x", n)
					other.Loc = "other:0.0"
					if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
						t.Fatal(err)
					}
				}
			case "valid-replacement":
				if err := writeBindingJSON(fileFor(f.Loc), Record{Agent: "codex", HomeID: f.HomeID, SessionID: "different", Frontend: "abcdef0123456789abcdef0123456789"}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(fileFor(f.Loc))
			if r, ok, err := ConversationRecord(f.Loc); err == nil || ok || r.SessionID != "" {
				t.Fatalf("%s recovered without unique proof: %+v %v %v", mode, r, ok, err)
			}
			after, _ := os.ReadFile(fileFor(f.Loc))
			if string(before) != string(after) {
				t.Fatal("refusal rewrote primary")
			}
		})
	}
}

func TestCAS002OwnHeartbeatRepairsLostIndex(t *testing.T) {
	for _, mode := range []string{"corrupt", "missing"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := frontendFixture(t)
			f, err := BindCodexFrontend(f, "root")
			if err != nil {
				t.Fatal(err)
			}
			corruptConversationIndex(t, f)
			if mode == "missing" {
				Remove(f.Loc)
			}
			next, err := RefreshCodexFrontend(f)
			if err != nil {
				t.Fatal("own heartbeat could not repair index", err)
			}
			r, ok := LoadLive(f.Loc)
			if !ok || r.SessionID != f.SessionID || r.Frontend != f.Instance || next.SessionID != f.SessionID {
				t.Fatal("repair did not republish verified identity", r, ok)
			}
		})
	}
}

func TestCAS002HeartbeatCannotReclaimReplacementOrUnknownEvidence(t *testing.T) {
	for _, mode := range []string{"replacement", "missing-receipt", "changed-receipt", "changed-process", "ambiguous", "expired"} {
		t.Run(mode, func(t *testing.T) {
			f, client := frontendFixture(t)
			f, err := BindCodexFrontend(f, "root")
			if err != nil {
				t.Fatal(err)
			}
			corruptConversationIndex(t, f)
			switch mode {
			case "replacement":
				if err := writeBindingJSON(fileFor(f.Loc), Record{Agent: "codex", HomeID: f.HomeID, SessionID: "new-owner", Frontend: f.Instance}); err != nil {
					t.Fatal(err)
				}
			case "missing-receipt":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
			case "changed-receipt":
				other := f
				other.SessionID = "new-root"
				other.Sequence++
				if err := writeBindingJSON(frontendPath(f.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "changed-process":
				client.Start = "reused"
			case "ambiguous":
				other := f
				other.Instance = "abcdef0123456789abcdef0123456789"
				other.Sequence++
				if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "expired":
				f.StartedAt -= 60000
				f.UpdatedAt = time.Now().Add(-6 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(fileFor(f.Loc))
			receipt, _ := os.ReadFile(frontendPath(f.Instance))
			if _, err := RefreshCodexFrontend(f); err == nil {
				t.Fatal("heartbeat reclaimed unverified/replaced owner")
			}
			after, _ := os.ReadFile(fileFor(f.Loc))
			end, _ := os.ReadFile(frontendPath(f.Instance))
			if string(before) != string(after) || string(receipt) != string(end) {
				t.Fatal("refused repair altered evidence")
			}
		})
	}
}

// Retain the normal empty legacy behavior; no frontend receipt is invented.
func TestCAS001LegacyEmptyAndForeignHome(t *testing.T) {
	f, _ := frontendFixture(t)
	r := Record{Agent: "claude", SessionID: "legacy"}
	if e := Save(f.Loc, r); e != nil {
		t.Fatal(e)
	}
	got, recovered, e := ConversationRecord(f.Loc)
	if e != nil || recovered || got.SessionID != "legacy" {
		t.Fatal(got, recovered, e)
	}
	r = Record{Agent: "codex", SessionID: "legacy-codex"}
	if e := writeBindingJSON(fileFor(f.Loc), r); e != nil {
		t.Fatal(e)
	}
	if got, recovered, e := ConversationRecord(f.Loc); e != nil || recovered || got.SessionID != "legacy-codex" {
		t.Fatal("home-less legacy read regressed", got, recovered, e)
	}
	r = Record{Agent: "codex", HomeID: "foreign", SessionID: "other"}
	if e := writeBindingJSON(fileFor(f.Loc), r); e != nil {
		t.Fatal(e)
	}
	if _, _, e := ConversationRecord(f.Loc); e == nil {
		t.Fatal("foreign home accepted")
	}
	if _, e := os.Stat(filepath.Join(Dir(), "codex-frontends")); !os.IsNotExist(e) {
		t.Fatal("legacy read manufactured frontend directory")
	}
}
