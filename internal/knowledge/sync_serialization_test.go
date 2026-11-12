package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func syncFixture(t *testing.T, audience string) (knowledgeOp, string, string) {
	t.Helper()
	asHQ(t)
	for _, key := range []string{"CODEX_HOME", "KIMI_CODE_HOME", "DSH_HOME", "OPENCODE_CONFIG_DIR"} {
		t.Setenv(key, "")
	}
	repo := t.TempDir()
	e := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/sync-order", Topic: "pitfalls", Title: "R0 instruction", Body: "R0 action before review.", Kind: KindPitfalls}
	if err := commitKnowledgeOp(e, "synthetic entry"); err != nil {
		t.Fatal(err)
	}
	target := audience
	if audience == AudienceRepo {
		target = "repo:" + repo
	}
	if rc := CmdKnowledge([]string{"promote", e.ID, "--why", "synthetic distribution", "--for", target}); rc != 0 {
		t.Fatal("promotion failed")
	}
	e = mustLive(t, e.ID)
	var path string
	if audience == AudienceMachine {
		if _, err := SyncMachine(false); err != nil {
			t.Fatal(err)
		}
		path = Carriers()[0].Path
	} else {
		var err error
		path, _, err = SyncRepo(repo, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	return e, repo, path
}

func probeKnowledgeLock(t *testing.T) bool {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(state.Dir(), "locks", "knowledge-*.lock"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("stable lock paths: %v %v", paths, err)
	}
	f, err := os.OpenFile(paths[0], os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
		return false
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("lock probe: %v", err)
	}
	return true
}

// The FIFO rendezvous proves that public sync has read R0 and reached carrier IO.
// Its path is then restored to a regular file so review/carry uses ordinary files.
// This runs against the old public functions too, without production test hooks.
func TestPublicSyncSerializesSnapshotAndPublication(t *testing.T) {
	for _, audience := range []string{AudienceMachine, AudienceRepo} {
		t.Run(audience, func(t *testing.T) {
			e, repo, path := syncFixture(t, audience)
			original := mustReadFile(t, path)
			if err := os.Rename(path, path+".fixture-original"); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			synced := make(chan error, 1)
			go func() {
				var err error
				if audience == AudienceMachine {
					_, err = SyncMachine(false)
				} else {
					_, _, err = SyncRepo(repo, false)
				}
				synced <- err
			}()
			var fd int
			deadline := time.Now().Add(5 * time.Second)
			for {
				var err error
				fd, err = syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
				if err == nil {
					break
				}
				if !errors.Is(err, syscall.ENXIO) || time.Now().After(deadline) {
					t.Fatalf("sync did not reach FIFO barrier: %v", err)
				}
				time.Sleep(time.Millisecond)
			}
			pipe := os.NewFile(uintptr(fd), path)
			defer pipe.Close()
			if err := syscall.SetNonblock(fd, false); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path, path+".fixture-fifo"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			locked := probeKnowledgeLock(t)
			if !locked {
				t.Error("public sync read R0 without holding the ledger lock")
			}
			type result struct {
				revision string
				err      error
			}
			reviewed := make(chan result, 1)
			started := make(chan struct{})
			go func() {
				close(started)
				p := proposalFor(e)
				p.Changes[0].Title, p.Changes[0].Body = "R1 instruction", "R1 reviewed action."
				p.ID = proposalDigest(p)
				if _, err := ApplyReview(p, false); err != nil {
					reviewed <- result{err: err}
					return
				}
				live, err := liveKnowledge()
				if err != nil {
					reviewed <- result{err: err}
					return
				}
				current, ok := findLive(live, e.ID)
				if !ok {
					reviewed <- result{err: fmt.Errorf("entry disappeared")}
					return
				}
				_, err = KnowledgeCarry(e.ID)
				reviewed <- result{revision: entryRevision(current), err: err}
			}()
			<-started
			var r result
			if !locked {
				// The regression interleaving: R1 is fully published and landed before R0 resumes.
				select {
				case r = <-reviewed:
				case <-time.After(5 * time.Second):
					t.Fatal("review/carry did not complete")
				}
			}
			latest := mustReadFile(t, path)
			if _, err := pipe.Write(latest); err != nil {
				t.Fatal(err)
			}
			if err := pipe.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-synced:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("sync deadlock")
			}
			if locked {
				select {
				case r = <-reviewed:
				case <-time.After(5 * time.Second):
					t.Fatal("nested carry lock deadlock")
				}
			}
			if r.err != nil {
				t.Fatal(r.err)
			}
			current := mustLive(t, e.ID)
			if current.Body != "R1 reviewed action." || promotionPending(current) || current.LandedAt == 0 {
				t.Fatalf("R1 not landed: %+v", current)
			}
			ops, err := readKnowledgeOps()
			if err != nil || ops[len(ops)-1].ExpectedRevision != r.revision {
				t.Fatalf("landing did not bind R1: %v", err)
			}
			paths := []string{path}
			if audience == AudienceMachine {
				paths = []string{MachinePath()}
				for _, carrier := range Carriers() {
					paths = append(paths, carrier.Path)
				}
			}
			for _, p := range paths {
				text := string(mustReadFile(t, p))
				if !strings.Contains(text, "R1 reviewed action.") || strings.Contains(text, "R0 action before review.") {
					t.Errorf("landed R1 carrier regressed at %s: %s", p, text)
				}
			}
			if probeKnowledgeLock(t) {
				t.Fatal("completed writer left ledger locked")
			}
		})
	}
}

func TestPublicSyncReleasesLockOnErrorAndRefusal(t *testing.T) {
	for _, audience := range []string{AudienceMachine, AudienceRepo} {
		t.Run(audience, func(t *testing.T) {
			e, repo, path := syncFixture(t, audience)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if audience == AudienceMachine {
				if _, err := SyncMachine(false); err == nil {
					t.Fatal("directory target accepted")
				}
			} else if _, _, err := SyncRepo(repo, false); err == nil {
				t.Fatal("directory target accepted")
			}
			if probeKnowledgeLock(t) {
				t.Fatal("error retained ledger lock")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(wrapBlock("old")+"manual suffix\n"), 0600); err != nil {
				t.Fatal(err)
			}
			b := strings.Replace(string(mustReadFile(t, path)), "\nold\n", "\noperator edit\n", 1)
			if err := os.WriteFile(path, []byte(b), 0600); err != nil {
				t.Fatal(err)
			}
			if audience == AudienceMachine {
				rep, err := SyncMachine(false)
				if err != nil || len(rep.Refused) != 1 {
					t.Fatalf("refusal: %+v %v", rep, err)
				}
			} else if _, refused, err := SyncRepo(repo, false); err != nil || !refused {
				t.Fatalf("refusal: %v %v", refused, err)
			}
			if probeKnowledgeLock(t) || string(mustReadFile(t, path)) != b {
				t.Fatal("refusal retained lock or changed manual text")
			}
			// Normal CLI entry shares the public lock and remains callable after both failures.
			args := []string{"sync", "--force"}
			if audience == AudienceRepo {
				args = append(args, "--repo", repo)
			}
			if rc := CmdKnowledge(args); rc != 0 {
				t.Fatalf("CLI sync failed: %d", rc)
			}
			done := make(chan error, 1)
			go func() { _, err := KnowledgeCarry(e.ID); done <- err }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("carry reentered sync lock")
			}
		})
	}
}
