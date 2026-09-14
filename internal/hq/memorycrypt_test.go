package hq

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An encrypted export comes back with the passphrase, and only with it.
func TestEncryptedExportRoundTrip(t *testing.T) {
	root := seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	path, n, err := ExportMemoryEncrypted(dst, "correct horse battery")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.HasSuffix(path, ".tar.gz.age") {
		t.Errorf("path = %q, want the .age suffix added so the name says what it is", path)
	}
	if n <= 0 {
		t.Fatal("export wrote nothing")
	}
	if !IsEncryptedArchive(path) {
		t.Error("the export is not recognised as encrypted")
	}
	// Not a tar.gz any more: the bytes must not open without the passphrase.
	if _, err := ImportMemory(path); err == nil {
		t.Error("the plain import opened an encrypted export")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportMemoryEncrypted(path, "correct horse battery"); err != nil {
		t.Fatalf("import: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "LOCAL.md"))
	if err != nil || !strings.Contains(string(b), "the operator's own charter") {
		t.Errorf("LOCAL.md did not come back: %v %q", err, b)
	}
	rec, ok := LastExport()
	if !ok || !rec.Encrypted || rec.At == 0 {
		t.Errorf("last export not recorded as locked: %+v %v", rec, ok)
	}
}

// A wrong passphrase is refused before anything moves: the memory that is there stays
// exactly where it is, because restoring is done in a hurry.
func TestWrongPassphraseChangesNothing(t *testing.T) {
	root := seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	path, _, err := ExportMemoryEncrypted(dst, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	moved, err := ImportMemoryEncrypted(path, "wrong wrong wrong")
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("err = %v, want ErrWrongPassphrase", err)
	}
	if moved != "" {
		t.Errorf("the existing memory was moved to %q on a wrong passphrase", moved)
	}
	if _, err := os.Stat(filepath.Join(root, "LOCAL.md")); err != nil {
		t.Errorf("the existing memory was touched: %v", err)
	}
	if entries, _ := filepath.Glob(root + ".replaced-*"); len(entries) > 0 {
		t.Errorf("a wrong passphrase left %v behind", entries)
	}
}

func TestAShortPassphraseIsRefused(t *testing.T) {
	seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	if _, _, err := ExportMemoryEncrypted(dst, "1234567"); err == nil {
		t.Error("a seven-character passphrase was accepted")
	}
	if _, err := os.Stat(dst + ".age"); err == nil {
		t.Error("a refused export still wrote a file")
	}
	if PassphraseStrength("12345678") != "ok" || PassphraseStrength("123456789012") != "good" || PassphraseStrength("短口令") != "short" {
		t.Error("strength ladder drifted")
	}
}

// A plain export is still a plain tar.gz, and says so in the record.
func TestPlainExportIsRecordedAsNotLocked(t *testing.T) {
	seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	if _, err := ExportMemory(dst); err != nil {
		t.Fatal(err)
	}
	if IsEncryptedArchive(dst) {
		t.Error("a plain export read as encrypted")
	}
	recordExport(false)
	if rec, ok := LastExport(); !ok || rec.Encrypted {
		t.Errorf("record = %+v %v", rec, ok)
	}
}

func TestPassphraseFromAPipeIsTheFirstLine(t *testing.T) {
	p, err := readPassphraseLine(strings.NewReader("pass phrase here\nnot this\n"))
	if err != nil || p != "pass phrase here" {
		t.Errorf("got %q %v", p, err)
	}
	p, err = readPassphraseLine(strings.NewReader("no newline"))
	if err != nil || p != "no newline" {
		t.Errorf("got %q %v", p, err)
	}
}
