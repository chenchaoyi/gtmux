package resume

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"golang.org/x/sys/unix"
)

func TestCPR001CrossPaneRecoveryCannotEraseAnotherPublisher(t *testing.T) {
	for _, bypassLease := range []bool{false, true} {
		name := "normal writers share lease"
		if bypassLease {
			name = "observed old writer changes raw bytes"
		}
		t.Run(name, func(t *testing.T) {
			f, _ := crossPaneIndexFixture(t)
			index := fileFor(f.Loc)
			original := indexBytes(t, index)
			other := Record{Agent: "claude", SessionID: "replacement-conversation", Cwd: f.ThreadCwd}
			oldWrite := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = oldWrite })
			var intervening []byte
			interleaved := false
			frontendBindingWrite = func(path string, value any) error {
				if path == frontendPath(f.Instance) {
					interleaved = true
					if !bytes.Equal(indexBytes(t, crossPaneIndexHistoryPath(f.Loc, original)), original) {
						t.Fatal("archive did not precede the interleaving")
					}
					if bypassLease {
						if err := writeBindingJSON(index, other); err != nil {
							t.Fatal(err)
						}
						intervening = indexBytes(t, index)
					} else {
						if err := Save(f.Loc, other); !errors.Is(err, state.ErrLeaseHeld) {
							t.Fatalf("non-Codex Save did not defer to the publication lease: %v", err)
						}
						Remove(f.Loc)
						if !bytes.Equal(indexBytes(t, index), original) {
							t.Fatal("a normal writer changed the protected index")
						}
					}
				}
				return oldWrite(path, value)
			}
			_, err := RefreshCodexFrontend(f)
			if !interleaved {
				t.Fatal("fixture missed the post-backup publication window")
			}
			if bypassLease {
				assertIndexProofError(t, err, "frontend-index-changed")
				if !bytes.Equal(indexBytes(t, index), intervening) {
					t.Fatal("repair overwrote the observed intervening write")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Save(f.Loc, other); err != nil {
				t.Fatalf("retry under a free lease failed: %v", err)
			}
			if saved, _ := Load(f.Loc); saved != other {
				t.Fatalf("retry failed to publish the exact other agent: %+v", saved)
			}
		})
	}
}

func TestCPR002LegacyReleaseRequiresPositiveIncumbentEvidence(t *testing.T) {
	t.Run("missing receipt and explicit retirement", func(t *testing.T) {
		f := expiredIndexFixture(t)
		incumbent := frontendRecord(f)
		if err := writeBindingJSON(fileFor(f.Loc), incumbent); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(frontendPath(f.Instance)); err != nil {
			t.Fatal(err)
		}
		foreign := Record{Agent: "codex", HomeID: f.HomeID, SessionID: "replacement"}
		assertIndexProofError(t, Save(f.Loc, foreign), "frontend-index-owned")
		if saved, _ := Load(f.Loc); saved != incumbent {
			t.Fatal("absence altered or released the incumbent")
		}
		if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
			t.Fatal(err)
		}
		EndCodexFrontend(f)
		retired, exists := Load(f.Loc)
		if !exists || !retired.FrontendRetired || retired.Frontend != f.Instance || retired.SessionID != f.SessionID {
			t.Fatal("explicit retirement did not leave identified history", retired)
		}
		// A lingering receipt, for example after a failed unlink, cannot grant
		// live authority even if it is fresh and otherwise valid.
		f.UpdatedAt = time.Now().UnixMilli()
		if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
			t.Fatal(err)
		}
		if _, ok := LoadLive(f.Loc); ok {
			t.Fatal("an explicitly retired index admitted a live reader")
		}
		if _, ok := CodexFrontendForThread(f.HomeID, f.SessionID); ok {
			t.Fatal("an explicitly retired index granted terminal input")
		}
		if err := Save(f.Loc, foreign); err != nil {
			t.Fatalf("explicit retirement did not permit normal publication: %v", err)
		}
	})

	for _, mode := range []string{"different home", "partial relocation", "partial relocation with new root"} {
		t.Run(mode, func(t *testing.T) {
			f, pane, _ := locationFixture(t)
			if mode != "different home" {
				oldWrite := frontendBindingWrite
				t.Cleanup(func() { frontendBindingWrite = oldWrite })
				pane.Loc = "moved:0.0"
				broken := errors.New("injected destination write error")
				frontendBindingWrite = func(path string, value any) error {
					if path == fileFor(pane.Loc) {
						return broken
					}
					return oldWrite(path, value)
				}
				wantRoot := f.SessionID
				if mode == "partial relocation with new root" {
					wantRoot = "observed-next-root"
					if _, err := BindCodexFrontend(f, wantRoot); err == nil {
						t.Fatal("expected partial relocation while recording the new root")
					}
				} else if _, err := RefreshCodexFrontend(f); !errors.Is(err, broken) {
					t.Fatalf("expected partial relocation: %v", err)
				}
				if saved, ok := readFrontend(f.Instance); !ok || saved.Loc != pane.Loc || saved.SessionID != wantRoot {
					t.Fatal("fixture missed receipt-first relocation")
				}
			} else {
				t.Setenv("CODEX_HOME", t.TempDir())
				if _, ok := readFrontend(f.Instance); ok {
					t.Fatal("foreign receipt acquired scoped live access")
				}
				if _, ok := LoadLive(f.Loc); ok {
					t.Fatal("foreign home index acquired live authority")
				}
			}
			foreign := Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: "replacement"}
			assertIndexProofError(t, Save(f.Loc, foreign), "frontend-index-owned")
			oldProcess, oldGone := frontendProcess, frontendRecoveryGone
			t.Cleanup(func() { frontendProcess, frontendRecoveryGone = oldProcess, oldGone })
			frontendProcess = func(int) (launchhost.AgentProcess, bool) { return launchhost.AgentProcess{}, false }
			frontendRecoveryGone = func(int) bool { return false }
			assertIndexProofError(t, Save(f.Loc, foreign), "frontend-index-owned")
			frontendRecoveryGone = func(pid int) bool { return pid == f.BridgePID }
			if err := Save(f.Loc, foreign); err != nil {
				t.Fatalf("confirmed stopped incumbent blocked publication: %v", err)
			}
			if saved, ok := Load(f.Loc); !ok || saved.SessionID != foreign.SessionID || saved.HomeID != foreign.HomeID {
				t.Fatalf("wrong index after positive release: %+v", saved)
			}
		})
	}
}

func TestCPR003OnlyRelevantStructurallyValidReceiptsCompete(t *testing.T) {
	for _, mode := range []string{"unrelated", "same location", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			f, peer := crossPaneIndexFixture(t)
			original := indexBytes(t, fileFor(f.Loc))
			peerIndex := indexBytes(t, fileFor(peer.Loc))
			ownHome := os.Getenv("CODEX_HOME")
			t.Setenv("CODEX_HOME", t.TempDir())
			other := f
			other.HomeID = transcript.CodexHomeID()
			other.Instance, other.Pane, other.Loc = strings.Repeat("e", 32), "%85", "other-home:0.0"
			other.UpdatedAt = time.Now().UnixMilli()
			if mode == "same location" {
				other.Loc = f.Loc
			}
			if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
				t.Fatal(err)
			}
			if _, ok := readFrontend(other.Instance); !ok {
				t.Fatal("fixture receipt is invalid even in its own home")
			}
			if mode == "malformed" {
				if err := os.WriteFile(frontendPath(other.Instance), []byte("bad receipt"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("CODEX_HOME", ownHome)
			_, err := RefreshCodexFrontend(f)
			if mode == "unrelated" {
				if err != nil {
					t.Fatal(err)
				}
				if own, ok := LoadLive(f.Loc); !ok || own.SessionID != f.SessionID {
					t.Fatal("independent recovery did not restore its own thread")
				}
			} else {
				assertIndexProofError(t, err, "frontend-index-conflict")
				if !bytes.Equal(indexBytes(t, fileFor(f.Loc)), original) {
					t.Fatal("conflicted recovery touched its original index")
				}
			}
			if !bytes.Equal(indexBytes(t, fileFor(peer.Loc)), peerIndex) {
				t.Fatal("recovery touched the independent peer")
			}
		})
	}
}

func TestCPR004VisibleArchiveMustPassDurabilityOnRetry(t *testing.T) {
	f, _ := crossPaneIndexFixture(t)
	index, receipt := fileFor(f.Loc), frontendPath(f.Instance)
	original, oldProof := indexBytes(t, index), indexBytes(t, receipt)
	oldWrite, oldSync := writeCrossPaneArchive, syncCrossPaneArchiveParent
	t.Cleanup(func() { writeCrossPaneArchive, syncCrossPaneArchiveParent = oldWrite, oldSync })
	writes, syncs := 0, 0
	writeCrossPaneArchive = func(path string, body []byte) error {
		writes++
		if err := oldWrite(path, body); err != nil {
			return err
		}
		if writes <= 2 {
			return unix.EIO
		}
		return nil
	}
	syncCrossPaneArchiveParent = func(path string) error {
		syncs++
		if syncs == 1 {
			return unix.EIO
		}
		return oldSync(path)
	}
	for attempt := 1; attempt <= 4; attempt++ {
		_, err := RefreshCodexFrontend(f)
		if attempt < 4 {
			assertIndexProofError(t, err, "frontend-index-preservation-unavailable")
			if !bytes.Equal(original, indexBytes(t, index)) || !bytes.Equal(oldProof, indexBytes(t, receipt)) {
				t.Fatal("failed archive durability changed the index or receipt")
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if writes != attempt || !bytes.Equal(original, indexBytes(t, crossPaneIndexHistoryPath(f.Loc, original))) {
			t.Fatal("visible archive skipped required persistence work")
		}
	}
	if syncs != 2 {
		t.Fatalf("archive parent required sync and retry, got %d attempts", syncs)
	}
	if live, ok := LoadLive(f.Loc); !ok || live.SessionID != f.SessionID {
		t.Fatal("durable archive did not permit recovery")
	}
}
