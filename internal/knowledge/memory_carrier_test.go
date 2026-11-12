package knowledge

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMemoryCarrierUpdatesHistoricalTextAndPreservesManualEdits(t *testing.T) {
	e := reviewFixture(t)
	os.WriteFile(LocalPath(), []byte("# operator conventions\n"), 0600)
	if _, err := carryIntoLocal(e); err != nil {
		t.Fatal(err)
	}
	p := proposalFor(e)
	if _, err := ApplyReview(p, false); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	next := live[0]
	if _, err := carryIntoLocal(next); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(LocalPath())
	if strings.Contains(string(b), e.Body) || !strings.Contains(string(b), next.Body) || !strings.HasPrefix(string(b), "# operator conventions") {
		t.Fatalf("wrong section: %s", b)
	}
	if _, err := carryIntoLocal(next); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(LocalPath())
	if !bytes.Equal(b, again) {
		t.Fatal("duplicate landing")
	}
	edited := strings.Replace(string(b), next.Body, "human clarification", 1)
	os.WriteFile(LocalPath(), []byte(edited), 0600)
	if _, err := carryIntoLocal(next); err == nil {
		t.Fatal("overwrote operator edit")
	}
	after, _ := os.ReadFile(LocalPath())
	if string(after) != edited {
		t.Fatal("conflict changed carrier")
	}
}

// A descriptor opened before publication must keep the entire previous inode.
// O_TRUNC fails this deterministic test even when the eventual write succeeds.
func TestMemoryCarrierAtomicPublicationKeepsOldReader(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		name := "append"
		if upgrade {
			name = "upgrade"
		}
		t.Run(name, func(t *testing.T) {
			e := reviewFixture(t)
			original := "# operator conventions\nKeep these manual rules.\n"
			if upgrade {
				original += localSection(e) + "\n# More manual rules\nPreserve this suffix.\n"
				if _, err := ApplyReview(proposalFor(e), false); err != nil {
					t.Fatal(err)
				}
				live, err := liveKnowledge()
				if err != nil {
					t.Fatal(err)
				}
				e = live[0]
			}
			if err := os.WriteFile(LocalPath(), []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			old, err := os.Open(LocalPath())
			if err != nil {
				t.Fatal(err)
			}
			defer old.Close()
			if _, err := carryIntoLocal(e); err != nil {
				t.Fatal(err)
			}
			pinned, err := io.ReadAll(old)
			if err != nil {
				t.Fatal(err)
			}
			if string(pinned) != original {
				t.Error("publication mutated the old inode containing manual rules")
			}
			after, err := os.ReadFile(LocalPath())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(after), "# operator conventions\nKeep these manual rules.\n") || !strings.Contains(string(after), localSection(e)) {
				t.Fatal("new carrier incomplete")
			}
			if upgrade && !strings.HasSuffix(string(after), "\n# More manual rules\nPreserve this suffix.\n") {
				t.Fatal("manual suffix lost")
			}
			st, err := os.Stat(LocalPath())
			if err != nil || st.Mode().Perm() != 0600 {
				t.Fatalf("carrier permissions: %v %v", st, err)
			}
		})
	}
}

func TestMemoryCarrierRefusesUnsafeTargetsWithoutSettling(t *testing.T) {
	for _, kind := range []string{"read-only", "symlink", "duplicate marker"} {
		t.Run(kind, func(t *testing.T) {
			e := pendingLandingFixture(t)
			original := "# manual rules\n" + localSection(e)
			if kind == "duplicate marker" {
				original += localSection(e)
			}
			path := LocalPath()
			if kind == "symlink" {
				path = t.TempDir() + "/outside"
			}
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				if err := os.Symlink(path, LocalPath()); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "read-only" {
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(knowledgeLedgerPath())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := KnowledgeCarry(e.ID); err == nil {
				t.Fatal("accepted unsafe carrier")
			}
			after, err := os.ReadFile(knowledgeLedgerPath())
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal appended landing")
			}
			content, err := os.ReadFile(path)
			if err != nil || string(content) != original {
				t.Fatal("refusal changed manual content")
			}
			st, err := os.Stat(path)
			want := os.FileMode(0600)
			if kind == "read-only" {
				want = 0400
			}
			if err != nil || st.Mode().Perm() != want {
				t.Fatal("refusal changed permissions")
			}
		})
	}
}
