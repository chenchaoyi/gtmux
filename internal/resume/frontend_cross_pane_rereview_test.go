package resume

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCXW001SameThreadRepairKeepsContinuousPublicationLease(t *testing.T) {
	for _, oldWriter := range []bool{false, true} {
		name := "upgraded writer"
		if oldWriter {
			name = "visible write from older writer"
		}
		t.Run(name, func(t *testing.T) {
			f := expiredIndexFixture(t)
			incumbent := frontendRecord(f)
			incumbent.Frontend, incumbent.Launcher, incumbent.TerminalClient = "", "legacy-launcher", "legacy-terminal"
			if err := writeBindingJSON(fileFor(f.Loc), incumbent); err != nil {
				t.Fatal(err)
			}
			other := Record{Agent: "codex", HomeID: f.HomeID, SessionID: "intervening-thread", Cwd: f.ThreadCwd}
			clients, writer := frontendClients, frontendBindingWrite
			t.Cleanup(func() { frontendClients, frontendBindingWrite = clients, writer })
			attempted := false
			if !oldWriter {
				frontendClients = func(panes []codexterminal.Pane) map[string]codexterminal.Client {
					if !attempted {
						attempted = true
						if err := Save(f.Loc, other); !errors.Is(err, state.ErrLeaseHeld) {
							t.Fatalf("same-thread check dropped its lease: %v", err)
						}
					}
					return clients(panes)
				}
			}
			var newer []byte
			if oldWriter {
				frontendBindingWrite = func(path string, value any) error {
					if path == frontendPath(f.Instance) {
						attempted = true
						if err := writeBindingJSON(fileFor(f.Loc), other); err != nil {
							t.Fatal(err)
						}
						newer = indexBytes(t, fileFor(f.Loc))
					}
					return writer(path, value)
				}
			}
			_, err := RefreshCodexFrontend(f)
			if !attempted {
				t.Fatal("fixture missed the competing publication window")
			}
			if oldWriter {
				assertIndexProofError(t, err, "frontend-index-changed")
				if !bytes.Equal(newer, indexBytes(t, fileFor(f.Loc))) {
					t.Fatal("same-thread repair overwrote the intervening version")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			r, ok := LoadLive(f.Loc)
			if !ok || r.SessionID != f.SessionID || r.Frontend != f.Instance ||
				r.Launcher != incumbent.Launcher || r.TerminalClient != incumbent.TerminalClient {
				t.Fatal("serialized repair did not preserve same-thread metadata", r)
			}
		})
	}
}

func TestCXW002ClosureReleasesExactLayoutAliasesAndRetainsFailureProof(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "normal close"
		if fail {
			name = "retirement index write failure"
		}
		t.Run(name, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			locations := []string{f.Loc, "moved:0.0", "moved:1.0"}
			for _, loc := range locations[1:] {
				pane.Loc = loc
				var err error
				if f, err = RefreshCodexFrontend(f); err != nil {
					t.Fatal(err)
				}
			}
			unrelated := Record{Agent: "claude", SessionID: "other-agent"}
			if err := Save("other:0.0", unrelated); err != nil {
				t.Fatal(err)
			}
			if fail {
				oldWrite := frontendRetirementWrite
				t.Cleanup(func() { frontendRetirementWrite = oldWrite })
				frontendRetirementWrite = func(path string, value any) error {
					if path == fileFor(locations[0]) {
						return errors.New("injected retirement index write failure")
					}
					return oldWrite(path, value)
				}
			}
			EndCodexFrontend(f)
			if raw, exists := readStoredFrontend(f.Instance); fail != exists || exists && (!raw.Retired || raw.BridgeStart != f.BridgeStart) {
				t.Fatal("closure left the wrong exact receipt proof", raw, exists)
			}
			if _, live := readFrontend(f.Instance); live {
				t.Fatal("closed receipt granted a live authority")
			}
			for i, loc := range locations {
				history, ok := Load(loc)
				if !ok || history.Frontend != f.Instance || history.SessionID != f.SessionID || history.FrontendRetired != (!fail || i != 0) {
					t.Fatalf("unexpected history at %s: %+v", loc, history)
				}
				if _, live := LoadLive(loc); live {
					t.Fatal("closed alias acquired live input authority", loc)
				}
				next := Record{Agent: "codex", SessionID: "replacement-thread", HomeID: f.HomeID}
				if err := Save(loc, next); err != nil {
					t.Fatalf("legitimate admission remains blocked at %s: %v", loc, err)
				}
			}
			if saved, _ := Load("other:0.0"); saved != unrelated {
				t.Fatal("retirement changed another owner", saved)
			}
		})
	}
}

func TestCXW003LegacyUnlinkNeedsPositiveExactNativeExit(t *testing.T) {
	for _, mode := range []string{"matching", "wrong instance", "malformed bridge receipt"} {
		t.Run(mode, func(t *testing.T) {
			f := expiredIndexFixture(t)
			if err := writeBindingJSON(fileFor(f.Loc), frontendRecord(f)); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(frontendPath(f.Instance)); err != nil {
				t.Fatal(err)
			}
			proof := f
			if mode == "wrong instance" {
				proof.Instance = strings.Repeat("d", 32)
			}
			recordCrossPaneBinding(t, proof)
			if mode == "malformed bridge receipt" {
				if err := os.WriteFile(frontendPath(f.Instance), []byte("unreadable"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			oldGone := frontendRecoveryGone
			t.Cleanup(func() { frontendRecoveryGone = oldGone })
			frontendRecoveryGone = func(int) bool { return false }
			other := Record{Agent: "codex", SessionID: "replacement-thread", HomeID: f.HomeID}
			assertIndexProofError(t, Save(f.Loc, other), "frontend-index-owned")
			frontendRecoveryGone = func(pid int) bool { return pid == f.PID }
			err := Save(f.Loc, other)
			if mode == "matching" {
				if err != nil {
					t.Fatal("legacy owner with exact exited client stayed reserved", err)
				}
			} else {
				assertIndexProofError(t, err, "frontend-index-owned")
			}
		})
	}
}
