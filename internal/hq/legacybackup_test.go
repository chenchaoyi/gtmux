package hq

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func sum(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

// legacyHome is an HQ home holding only a full legacy CLAUDE.md, the shape a migration
// starts from. It lives under the test's own HOME.
func legacyHome(t *testing.T, body string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(hqClaudePointerPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqClaudePointerPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func legacyBackups(t *testing.T) []string {
	t.Helper()
	m, err := filepath.Glob(hqClaudePointerPath() + ".bak-legacy-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A second migration on the same day used to write the same backup name, so the
// original playbook it held was replaced by whatever CLAUDE.md held the second time —
// the one-line pointer the first migration wrote. Reached by deleting AGENTS.md and
// running `gtmux hq` again the same day. Every backup that exists keeps its bytes.
func TestASecondMigrationKeepsTheFirstBackup(t *testing.T) {
	original := "FULL OLD PLAYBOOK + the commander's own notes"
	legacyHome(t, original)
	first, err := seedHQHome("")
	if err != nil || !first.Migrated {
		t.Fatalf("first migration: %+v, %v", first, err)
	}
	if err := os.WriteFile(hqLocalPath(), []byte("my own LOCAL.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The trigger, in this test's own home only.
	if err := os.Remove(hqInstructionsPath()); err != nil {
		t.Fatal(err)
	}
	second, err := seedHQHome("")
	if err != nil || !second.Migrated {
		t.Fatalf("second migration: %+v, %v", second, err)
	}
	if second.BackupPath == first.BackupPath {
		t.Errorf("the second migration reused the first backup's name %s", first.BackupPath)
	}
	if b, _ := os.ReadFile(first.BackupPath); sum(b) != sum([]byte(original)) {
		t.Errorf("the first backup no longer holds the original playbook (%d bytes now)", len(b))
	}
	if b, _ := os.ReadFile(second.BackupPath); string(b) != hqClaudePointer {
		t.Errorf("the second backup must hold what CLAUDE.md held then, the pointer: %q", b)
	}
	if n := len(legacyBackups(t)); n != 2 {
		t.Errorf("%d backups, want 2", n)
	}
	if b, _ := os.ReadFile(hqLocalPath()); string(b) != "my own LOCAL.md" {
		t.Errorf("LOCAL.md was rewritten: %q", b)
	}
}

// A file already at the backup's name — from anything — is left as it is.
func TestABackupNameInUseIsNotOverwritten(t *testing.T) {
	legacyHome(t, "legacy playbook")
	stale := hqClaudePointerPath() + ".bak-legacy-" + legacyBackupNow().Format("20060102")
	if err := os.WriteFile(stale, []byte("an earlier backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := seedHQHome("")
	if err != nil || !r.Migrated {
		t.Fatalf("migration: %+v, %v", r, err)
	}
	if b, _ := os.ReadFile(stale); string(b) != "an earlier backup" {
		t.Errorf("the file already at the backup name was overwritten: %q", b)
	}
	if b, _ := os.ReadFile(r.BackupPath); string(b) != "legacy playbook" {
		t.Errorf("the new backup does not hold the legacy playbook: %q", b)
	}
}

// No backup, no migration: if the backup cannot be written, nothing else is touched —
// the legacy CLAUDE.md, a LOCAL.md the user wrote, and the missing AGENTS.md all stay as
// they were, and the migration is not reported.
func TestNoBackupNoMigration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only directory this relies on")
	}
	legacyHome(t, "legacy playbook")
	if err := os.WriteFile(hqLocalPath(), []byte("my own LOCAL.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(hqClaudePointerPath())
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	r, err := seedHQHome("")
	if err == nil || r.Migrated {
		t.Fatalf("a migration with no backup was reported: %+v, %v", r, err)
	}
	if b, _ := os.ReadFile(hqClaudePointerPath()); string(b) != "legacy playbook" {
		t.Errorf("the legacy CLAUDE.md changed: %q", b)
	}
	if fileExists(hqInstructionsPath()) {
		t.Error("AGENTS.md was written without a backup")
	}
	if b, _ := os.ReadFile(hqLocalPath()); string(b) != "my own LOCAL.md" {
		t.Errorf("LOCAL.md changed: %q", b)
	}
	if n := len(legacyBackups(t)); n != 0 {
		t.Errorf("%d partial backups left behind", n)
	}
}
