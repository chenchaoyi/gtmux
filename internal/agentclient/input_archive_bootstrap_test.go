package agentclient

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL026ArchiveBootstrapSurvivesProcessExit(t *testing.T) {
	for _, phase := range []string{"before-rename", "after-rename"} {
		t.Run(phase, func(t *testing.T) {
			_, target, _ := attachmentFixture(t)
			now := time.Now()
			later := now.Add(96 * time.Hour)
			settled := settleRetirementFixture(t, target, "settled-bootstrap", "accepted", now)
			inFlight, err := Prepare(target, "original input", "claimed-bootstrap", false, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := ClaimNative(inFlight); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "-test.run=^TestSPL026ArchiveBootstrapExitChild$")
			cmd.Env = append(os.Environ(), "GTMUX_ARCHIVE_EXIT_PHASE="+phase, "GTMUX_ARCHIVE_EXIT_INDEX="+settled.IndexKey+".index.json")
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 83 {
				t.Fatalf("did not exit at the real rename boundary: %v %s", err, out)
			}

			dir := InputDir()
			catalogue := filepath.Join(dir, archiveCatalogName)
			stage := filepath.Join(dir, archiveCatalogStage)
			if phase == "before-rename" {
				if _, err := os.Stat(catalogue); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("catalogue appeared before rename: %v", err)
				}
				if _, err := os.Stat(stage); err != nil {
					t.Fatalf("interrupted stage is missing: %v", err)
				}
			} else {
				info, err := os.Stat(catalogue)
				if err != nil {
					t.Fatal(err)
				}
				if _, links, err := privateInputIdentity(info); err != nil || links != 1 {
					t.Fatalf("published catalogue is not a private single-link file: links=%d err=%v", links, err)
				}
				if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("renamed stage remains: %v", err)
				}
			}

			if got, err := Prepare(target, settled.Text, "settled-bootstrap", false, later); err != nil || got.ID != settled.ID {
				t.Fatalf("original settlement evidence was lost: %+v %v", got, err)
			}
			if _, err := Prepare(target, inFlight.Text, "claimed-bootstrap", false, later); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("existing unknown claim became eligible: %v", err)
			}
			if TerminalFallbackError(target.Pane, "claimed-bootstrap") == nil {
				t.Fatal("existing unknown ID could use raw input")
			}

			fresh := settleRetirementFixture(t, target, "new-bootstrap", "accepted", later)
			if receipt, ok := ReadReceipt(fresh); !ok || receipt.State != "accepted" {
				t.Fatalf("new stable ID could not settle: %+v %v", receipt, ok)
			}
			if _, err := Prepare(target, settled.Text, "settled-bootstrap", false, later); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("retired acceptance could replay after recovery: %v", err)
			}
			if TerminalFallbackError(target.Pane, "settled-bootstrap") == nil || InspectInputArchive().NeedsRepair {
				t.Fatal("archive did not recover with replay protection")
			}
			if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("stale stage still exists after recovery: %v", err)
			}
		})
	}
}

func TestSPL026ArchiveBootstrapExitChild(t *testing.T) {
	phase := os.Getenv("GTMUX_ARCHIVE_EXIT_PHASE")
	if phase == "" {
		return
	}
	lease, err := state.TryLease("terminal-input")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	store, err := resolveInputStore(state.Dir())
	if err != nil {
		t.Fatal(err)
	}
	ops := syncInput
	ops.renameNew = func(fd int, from, to string) error {
		if from != archiveCatalogStage || to != archiveCatalogName {
			t.Fatalf("unexpected publication names: %q %q", from, to)
		}
		if phase == "before-rename" {
			os.Exit(83)
		}
		if phase != "after-rename" {
			t.Fatalf("unknown fixture phase: %q", phase)
		}
		if err := syncInput.renameNew(fd, from, to); err != nil {
			return err
		}
		os.Exit(83)
		return nil
	}
	q, err := openInputQueue(store, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	_, err = q.reconcileInput(os.Getenv("GTMUX_ARCHIVE_EXIT_INDEX"), time.Now().Add(96*time.Hour))
	t.Fatalf("retirement never reached interruption point: %v", err)
}

func TestSPL026ArchiveBootstrapPreservesExistingDestination(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	r := settleRetirementFixture(t, target, "bootstrap-destination", "accepted", time.Now())
	q, err := openInputQueue(r.store, syncInput)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()

	destination := filepath.Join(InputDir(), archiveCatalogName)
	original := []byte("externally supplied file")
	if err := os.WriteFile(destination, original, 0600); err != nil {
		t.Fatal(err)
	}
	empty := inputArchiveCatalog{Version: 1, Prefixes: make(map[string][]inputArchivePage)}
	if err := q.writeArchiveCatalog(empty, false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("initial publication did not refuse a destination collision: %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != string(original) {
		t.Fatalf("initial publication replaced its destination: %q %v", got, err)
	}
}
