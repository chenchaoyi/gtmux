package knowledge

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultRoundTripRenameAndDeletion(t *testing.T) {
	e := reviewFixture(t)
	root := t.TempDir()
	before, _ := os.ReadFile(knowledgeLedgerPath())
	if r, err := ExportVault(root); err != nil || r.Written != 1 {
		t.Fatalf("export: %+v %v", r, err)
	}
	base, err := readVaultBase(root)
	if err != nil {
		t.Fatal(err)
	}
	note := base.Entries[0]
	note.Body = "Changed with a concrete verification step."
	renamed := filepath.Join(root, "renamed.md")
	if err := os.Rename(filepath.Join(root, note.Path), renamed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(renamed, []byte(renderVaultEntry(note)), 0600); err != nil {
		t.Fatal(err)
	}
	if r, err := ExportVault(root); err != nil || len(r.Conflicts) != 1 {
		t.Fatalf("must preserve edit: %+v %v", r, err)
	}
	r, err := DiffVault(root)
	if err != nil || len(r.Proposal.Changes) != 1 || r.Proposal.Changes[0].ID != e.ID {
		t.Fatalf("diff: %+v %v", r, err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("export/diff wrote ledger")
	}
	if _, err := ApplyReview(*r.Proposal, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportVault(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(renamed); err != nil {
		t.Fatal("rename lost", err)
	}
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	r, err = DiffVault(root)
	if err != nil || len(r.Missing) != 1 || len(r.Proposal.Changes) != 0 {
		t.Fatalf("delete must not retire: %+v %v", r, err)
	}
	live, _ := liveKnowledge()
	if len(live) != 1 {
		t.Fatal("file deletion retired entry")
	}
}

func TestVaultConflictPrivacyAndRecovery(t *testing.T) {
	e := reviewFixture(t)
	root := t.TempDir()
	if _, err := ExportVault(root); err != nil {
		t.Fatal(err)
	}
	base, _ := readVaultBase(root)
	note := base.Entries[0]
	note.Body = "local edit"
	path := filepath.Join(root, note.Path)
	os.WriteFile(path, []byte(renderVaultEntry(note)), 0600)
	p := proposalFor(e)
	if _, err := ApplyReview(p, false); err != nil {
		t.Fatal(err)
	}
	r, err := DiffVault(root)
	if err != nil || len(r.Conflicts) != 1 || len(r.Proposal.Changes) != 0 {
		t.Fatalf("concurrent change: %+v %v", r, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "local edit") {
		t.Fatal("discarded local edit")
	}
	// Reproduce a crash after the note write but before the baseline write.
	live, _ := liveKnowledge()
	note.Title, note.Body, note.Revision = live[0].Title, live[0].Body, entryRevision(live[0])
	os.WriteFile(path, []byte(renderVaultEntry(note)), 0600)
	if r, err := ExportVault(root); err != nil || len(r.Conflicts) != 0 {
		t.Fatalf("recovery: %+v %v", r, err)
	}
	if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpSensitive, ID: e.ID, Sensitive: true, Confirmed: "synthetic permission"}, "test"); err != nil {
		t.Fatal(err)
	}
	if r, err := ExportVault(root); err != nil || r.Skipped != 1 || len(r.Conflicts) != 1 {
		t.Fatalf("revocation must surface existing copy: %+v %v", r, err)
	}
	fresh := t.TempDir()
	if r, err := ExportVault(fresh); err != nil || r.Written != 0 {
		t.Fatalf("private export: %+v %v", r, err)
	}
}

func TestVaultRefusesSymlinksAndUnrelatedDirectory(t *testing.T) {
	reviewFixture(t)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mine.md"), []byte("# mine\n"), 0600)
	if _, err := ExportVault(root); err == nil {
		t.Fatal("adopted unrelated directory")
	}
	root = t.TempDir()
	if _, err := ExportVault(root); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "secret.md")
	os.WriteFile(target, []byte("# secret"), 0600)
	if err := os.Symlink(target, filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := DiffVault(root); err == nil {
		t.Fatal("followed symlink")
	}
}

func TestVaultPublicationKeepsOldSnapshotsAndReadOnlyFiles(t *testing.T) {
	e := reviewFixture(t)
	root := t.TempDir()
	if _, err := ExportVault(root); err != nil {
		t.Fatal(err)
	}
	base, err := readVaultBase(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(root, base.Entries[0].Path), filepath.Join(root, vaultManifest)}
	var snapshots []*os.File
	var before [][]byte
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before = append(before, data)
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		snapshots = append(snapshots, f)
	}
	if _, err := ApplyReview(proposalFor(e), false); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportVault(root); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		old, err := io.ReadAll(snapshots[i])
		if err != nil || !bytes.Equal(old, before[i]) {
			t.Fatalf("publication truncated an open prior snapshot: %s %v", path, err)
		}
		current, err := os.ReadFile(path)
		if err != nil || bytes.Equal(current, before[i]) {
			t.Fatalf("replacement was not published: %s %v", path, err)
		}
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatalf("published file is not private: %s %v", path, err)
		}
		if err := os.Chmod(path, 0400); err != nil {
			t.Fatal(err)
		}
		if err := publishVaultFile(path, []byte("must not replace a protected file")); err == nil {
			t.Fatal("replaced read-only destination")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, current) {
			t.Fatal("failed publication changed the prior file")
		}
		leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".knowledge-vault-*"))
		if err != nil || len(leftovers) != 0 {
			t.Fatalf("publication left staging files: %v %v", leftovers, err)
		}
	}
}
