package resume

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
)

func TestCPI001LegacyWritersPreserveExpiredManagedOwner(t *testing.T) {
	f := expiredIndexFixture(t)
	incumbent := frontendRecord(f)
	if err := writeBindingJSON(fileFor(f.Loc), incumbent); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("an expired index gained input authority")
	}
	foreign := Record{Agent: "codex", HomeID: f.HomeID, SessionID: "another-thread"}
	expectRefusal := func() {
		t.Helper()
		assertIndexProofError(t, Save(f.Loc, foreign), "frontend-index-owned")
		lease, err := lockCodexPublication(f.Loc, incumbent, foreign)
		if lease != nil {
			lease.Close()
		}
		assertIndexProofError(t, err, "frontend-index-owned")
		if saved, _ := Load(f.Loc); saved != incumbent {
			t.Fatalf("legacy publication changed the managed index: %+v", saved)
		}
	}
	expectRefusal()
	oldProcess, oldGone := frontendProcess, frontendRecoveryGone
	t.Cleanup(func() {
		frontendProcess, frontendRecoveryGone = oldProcess, oldGone
	})
	frontendProcess = func(int) (launchhost.AgentProcess, bool) { return launchhost.AgentProcess{}, false }
	frontendRecoveryGone = func(int) bool { return false }
	expectRefusal()

	same := incumbent
	same.Frontend = ""
	if err := Save(f.Loc, same); err != nil {
		t.Fatal(err)
	}
	if saved, _ := Load(f.Loc); saved.Frontend != f.Instance {
		t.Fatalf("same thread lost its frontend reference: %+v", saved)
	}
	for _, retired := range []bool{false, true} {
		if err := writeBindingJSON(fileFor(f.Loc), incumbent); err != nil {
			t.Fatal(err)
		}
		frontendRecoveryGone = func(pid int) bool { return !retired && pid == f.BridgePID }
		if retired {
			EndCodexFrontend(f)
		}
		if err := Save(f.Loc, foreign); err != nil {
			t.Fatalf("legacy publication after end (receipt removed=%t): %v", retired, err)
		}
		if got, _ := Load(f.Loc); got.SessionID != foreign.SessionID {
			t.Fatalf("legacy publication did not proceed: %+v", got)
		}
	}
}

func TestCPI002CurrentHeartbeatRestoresProvenCrossPaneIndex(t *testing.T) {
	for _, failIndexWrite := range []bool{false, true} {
		t.Run(fmt.Sprintf("index-write-failed=%t", failIndexWrite), func(t *testing.T) {
			f, foreign := crossPaneIndexFixture(t)
			original := indexBytes(t, fileFor(f.Loc))
			other := indexBytes(t, fileFor(foreign.Loc))
			foreignReceipt, _ := native.BindingFor(foreign.SessionID, foreign.HomeID)
			archive := crossPaneIndexHistoryPath(f.Loc, original)
			if _, ok := CodexFrontendForThread(f.HomeID, f.SessionID); ok {
				t.Fatal("conflicting index conferred the original frontend's input authority")
			}
			if failIndexWrite {
				oldWrite := frontendBindingWrite
				t.Cleanup(func() { frontendBindingWrite = oldWrite })
				broken := errors.New("injected index write failure")
				frontendBindingWrite = func(path string, v any) error {
					if path == fileFor(f.Loc) {
						return broken
					}
					return oldWrite(path, v)
				}
				if _, err := RefreshCodexFrontend(f); !errors.Is(err, broken) {
					t.Fatalf("expected the injected index write error: %v", err)
				}
				if !bytes.Equal(original, indexBytes(t, fileFor(f.Loc))) ||
					!bytes.Equal(original, indexBytes(t, archive)) {
					t.Fatal("failed publication lost original index or required backup")
				}
				frontendBindingWrite = oldWrite
			}
			next, err := RefreshCodexFrontend(f)
			if err != nil {
				t.Fatal(err)
			}
			record, ok := LoadLive(f.Loc)
			if !ok || record.SessionID != f.SessionID || record.Frontend != f.Instance || next.Instance != f.Instance {
				t.Fatalf("old frontend did not regain its exact association: %+v", record)
			}
			if receiptAfter, _ := native.BindingFor(foreign.SessionID, foreign.HomeID); receiptAfter != foreignReceipt ||
				!bytes.Equal(other, indexBytes(t, fileFor(foreign.Loc))) {
				t.Fatal("recovery changed the separate foreign pane or receipt")
			}
			if !bytes.Equal(original, indexBytes(t, archive)) {
				t.Fatal("recovery did not preserve exact original bytes")
			}
			info, err := os.Stat(archive)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("history is not a private file: %v %v", info, err)
			}
			archives, err := os.ReadDir(filepath.Dir(archive))
			if err != nil || len(archives) != 1 {
				t.Fatalf("retry produced duplicate or missing archives: %v %v", archives, err)
			}
		})
	}
}

func TestCPI003CurrentHeartbeatRefusesUnprovenConflict(t *testing.T) {
	for _, scenario := range []string{"missing-foreign-receipt", "wrong-own-receipt", "same-pane", "foreign-managed-index", "newer-owner", "unsafe-index", "unsafe-archive"} {
		t.Run(scenario, func(t *testing.T) {
			f, foreign := crossPaneIndexFixture(t)
			code := "frontend-index-conflict"
			switch scenario {
			case "missing-foreign-receipt":
				foreign.SessionID = "unobserved"
				withoutFrontend := frontendRecord(foreign)
				withoutFrontend.Frontend = ""
				if err := writeBindingJSON(fileFor(f.Loc), withoutFrontend); err != nil {
					t.Fatal(err)
				}
			case "wrong-own-receipt":
				other := f
				other.Instance = strings.Repeat("c", 32)
				other.UpdatedAt += 2000
				recordCrossPaneBinding(t, other)
			case "same-pane":
				foreign.Pane = f.Pane
				foreign.UpdatedAt += 2000
				recordCrossPaneBinding(t, foreign)
			case "foreign-managed-index":
				if err := writeBindingJSON(fileFor(f.Loc), frontendRecord(foreign)); err != nil {
					t.Fatal(err)
				}
			case "newer-owner":
				other := f
				other.Instance = strings.Repeat("a", 32)
				other.Sequence++
				other.StartedAt++
				if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
					t.Fatal(err)
				}
			case "unsafe-index":
				code = "frontend-index-preservation-unavailable"
				index := fileFor(f.Loc)
				if err := os.Rename(index, index+".before"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(index+".before", index); err != nil {
					t.Fatal(err)
				}
			case "unsafe-archive":
				code = "frontend-index-preservation-unavailable"
				archive := crossPaneIndexHistoryPath(f.Loc, indexBytes(t, fileFor(f.Loc)))
				if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(fileFor(foreign.Loc), archive); err != nil {
					t.Fatal(err)
				}
			}
			original := indexBytes(t, fileFor(f.Loc))
			proof := indexBytes(t, frontendPath(f.Instance))
			_, err := RefreshCodexFrontend(f)
			assertIndexProofError(t, err, code)
			if !bytes.Equal(original, indexBytes(t, fileFor(f.Loc))) ||
				!bytes.Equal(proof, indexBytes(t, frontendPath(f.Instance))) {
				t.Fatal("refused recovery wrote the index or refreshed the frontend")
			}
		})
	}
}

func TestCPI004LegacyWriterCanReuseVerifiedVacatedLocation(t *testing.T) {
	f, pane, _ := locationFixture(t)
	pane.Loc = "moved:0.0"
	foreign := Record{Agent: "codex", HomeID: f.HomeID, SessionID: "new-legacy-thread"}
	oldWrite := frontendBindingWrite
	t.Cleanup(func() { frontendBindingWrite = oldWrite })
	broken := errors.New("injected relocation index failure")
	frontendBindingWrite = func(path string, value any) error {
		if path == fileFor(pane.Loc) {
			return broken
		}
		return oldWrite(path, value)
	}
	if _, err := RefreshCodexFrontend(f); !errors.Is(err, broken) {
		t.Fatalf("expected partial relocation: %v", err)
	}
	assertIndexProofError(t, Save(f.Loc, foreign), "frontend-index-owned")

	frontendBindingWrite = oldWrite
	if _, err := RefreshCodexFrontend(f); err != nil {
		t.Fatal(err)
	}
	if err := Save(f.Loc, foreign); err != nil {
		t.Fatalf("verified relocation still reserved the old location: %v", err)
	}
	if index, ok := LoadLive(pane.Loc); !ok || index.Frontend != f.Instance || index.SessionID != f.SessionID {
		t.Fatalf("reusing the old location broke the moved owner: %+v", index)
	}
	if index, ok := Load(f.Loc); !ok || index.SessionID != foreign.SessionID {
		t.Fatalf("old location did not receive new index: %+v", index)
	}
}

func crossPaneIndexFixture(t *testing.T) (CodexFrontend, CodexFrontend) {
	t.Helper()
	f := expiredIndexFixture(t)
	recordCrossPaneBinding(t, f)
	foreign := f
	foreign.Instance, foreign.Pane, foreign.Loc, foreign.SessionID = strings.Repeat("b", 32), "%88", "other:1.0", "foreign-thread"
	foreign.PanePID, foreign.PID = 801, 802
	recordCrossPaneBinding(t, foreign)
	record := frontendRecord(foreign)
	record.Frontend = ""
	if err := writeBindingJSON(fileFor(foreign.Loc), record); err != nil {
		t.Fatal(err)
	}
	raw := fmt.Appendf(nil, "{\"agent\":\"codex\",\"sessionId\":%q,\"home_id\":%q,\"cwd\":%q,\"updatedAt\":%d,\"future_metadata\":true}\n",
		record.SessionID, record.HomeID, record.Cwd, record.UpdatedAt)
	if err := os.WriteFile(fileFor(f.Loc), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return f, foreign
}

func recordCrossPaneBinding(t *testing.T, f CodexFrontend) {
	t.Helper()
	if err := native.ConfirmBinding(native.Binding{
		Kind: "frontend", ID: f.Instance, HomeID: f.HomeID, SessionID: f.SessionID, Pane: f.Pane, Loc: f.Loc, Cwd: f.ThreadCwd,
		BackendPID: f.BackendPID, BackendStart: f.BackendStart, PanePID: f.PanePID, PID: f.PID,
		PreparedAt: f.StartedAt / 1000, BoundAt: f.UpdatedAt / 1000,
	}, native.Record{}); err != nil {
		t.Fatal(err)
	}
}

func indexBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertIndexProofError(t *testing.T, err error, code string) {
	t.Helper()
	var proof *FrontendBindingError
	if !errors.As(err, &proof) || proof.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}
