package knowledge

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// State's companion test faults the actual post-rename directory-sync seam.
// Here the writer error contract is injected per call, and assertions cover the
// real carrier and final ledger boundary. No global hooks or power-loss claim.
func TestLocalDurabilityRetryMustReachWriterBeforeLanding(t *testing.T) {
	for _, failures := range []int{1, 2} {
		name := "retry succeeds"
		if failures == 2 {
			name = "retry fails again"
		}
		t.Run(name, func(t *testing.T) {
			e := pendingLandingFixture(t)
			manual := "# manual prefix\nKeep the operator rules.\n"
			suffix := "\n# Manual suffix\nKeep the later operator note.\n"
			if err := os.WriteFile(LocalPath(), []byte(manual), 0640); err != nil {
				t.Fatal(err)
			}
			before := mustReadFile(t, knowledgeLedgerPath())
			calls := 0
			writer := func(path string, content []byte) error {
				calls++
				if err := state.WritePrivateFile(path, content); err != nil {
					return err
				}
				if calls <= failures {
					return syscall.EIO
				}
				return nil
			}
			for attempt := 1; attempt <= failures+1; attempt++ {
				err := commitKnowledge(landingOp(e), nil, "durability retry", func(current knowledgeOp) (string, error) { return carryIntoLocalWithWriter(current, writer) })
				if calls != attempt {
					t.Errorf("attempt %d skipped durable writer; calls=%d", attempt, calls)
				}
				content := string(mustReadFile(t, LocalPath()))
				if !strings.HasPrefix(content, manual) || strings.Count(content, localSection(e)) != 1 {
					t.Fatal("manual text lost or carrier duplicated")
				}
				if attempt > 1 && !strings.HasSuffix(content, suffix) {
					t.Fatal("same-content retry lost the manual suffix")
				}
				info, statErr := os.Stat(LocalPath())
				if statErr != nil || info.Mode().Perm() != 0600 {
					t.Fatal("carrier is not private")
				}
				live := mustLive(t, e.ID)
				if attempt <= failures {
					var ce *CommitError
					if !errors.As(err, &ce) || ce.Committed || ce.Phase != "carrier" || !errors.Is(err, syscall.EIO) {
						t.Errorf("failed sync settled or wrong phase: %v", err)
					}
					if !bytes.Equal(before, mustReadFile(t, knowledgeLedgerPath())) || !promotionPending(live) || entryRevision(live) != entryRevision(e) {
						t.Error("unsynced retry changed ledger or cleared pending")
					}
					if attempt == 1 {
						if err := os.WriteFile(LocalPath(), []byte(content+suffix), 0600); err != nil {
							t.Fatal(err)
						}
					}
				} else {
					if err != nil || promotionPending(live) {
						t.Errorf("successful durable retry did not settle: %v", err)
					}
					ops, readErr := readKnowledgeOps()
					if readErr != nil {
						t.Fatal(readErr)
					}
					lands := 0
					for _, op := range ops {
						if op.Op == knowledgeOpLand {
							lands++
							if op.ExpectedRevision != entryRevision(e) {
								t.Error("landed different revision")
							}
						}
					}
					if lands != 1 {
						t.Errorf("landing receipts=%d, want 1", lands)
					}
				}
			}
		})
	}
}
