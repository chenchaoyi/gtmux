package hq

// Passphrase-protected exports (change hq-export-passphrase).
//
// The export carries the board, the knowledge base and LOCAL.md — the commander's project
// detail, and whatever they told HQ to remember about themselves. Inside this machine
// FileVault covers it; the export is the copy that LEAVES, over a USB stick or a synced
// folder, and that copy was plain text. It is now an age file: a standard format any age
// tool can open, so the case the export exists for — gtmux may not be there to read it
// back — still holds. The passphrase is the commander's; gtmux keeps no copy.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ageHeader opens every age file; it is how an import tells an encrypted export from a
// plain one without asking.
const ageHeader = "age-encryption.org/v1"

// ExportSuffix is what an encrypted export ends in. `.tar.gz.age` says both what is inside
// and what wraps it, and an age tool recognises the outer half.
const ExportSuffix = ".age"

// ErrWrongPassphrase is the one import error a person can fix by trying again.
var ErrWrongPassphrase = errors.New("wrong passphrase")

// PassphraseMin is the floor. Below it the export refuses: a four-character passphrase on
// a file that carries months of notes is a lock drawn on the door.
const PassphraseMin = 8

// PassphraseStrength grades a passphrase by length alone — the one thing gtmux can judge
// without pretending to know the commander's threat model. "short" is refused, "ok" is
// accepted, "good" is twelve or more.
func PassphraseStrength(p string) string {
	n := len([]rune(p))
	switch {
	case n < PassphraseMin:
		return "short"
	case n < 12:
		return "ok"
	default:
		return "good"
	}
}

// IsEncryptedArchive reports whether the file at path is an age file.
func IsEncryptedArchive(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(ageHeader))
	n, _ := io.ReadFull(f, head)
	return string(head[:n]) == ageHeader
}

// ExportMemoryEncrypted writes the HQ home to dst as an age-encrypted gzipped tar, locked
// with the passphrase. A dst that does not already end in `.age` gets the suffix, so the
// name says what the file is; the path actually written is returned.
func ExportMemoryEncrypted(dst, passphrase string) (path string, n int64, err error) {
	if PassphraseStrength(passphrase) == "short" {
		return "", 0, fmt.Errorf("passphrase too short (%d characters at least)", PassphraseMin)
	}
	root := MemoryRoot()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return "", 0, fmt.Errorf("no HQ records at %s", root)
	}
	if !strings.HasSuffix(dst, ExportSuffix) {
		dst += ExportSuffix
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, err
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return "", 0, err
	}
	// Same discipline as the plain export: a temp beside the target, renamed only once
	// it is whole — nobody must pick up a half-written file and call it a backup.
	tmp := dst + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", 0, err
	}
	w, err := age.Encrypt(f, recipient)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", 0, err
	}
	err = writeArchive(w, root)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	st, err := os.Stat(dst)
	if err != nil {
		return "", 0, err
	}
	recordExport(true)
	return dst, st.Size(), nil
}

// ImportMemoryEncrypted unlocks an age export with the passphrase and restores it the way
// ImportMemory does. A wrong passphrase fails at the header, before a byte is written or
// the existing memory is moved: nothing changes, and the error says so.
func ImportMemoryEncrypted(src, passphrase string) (moved string, err error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return "", err
	}
	r, err := age.Decrypt(f, identity)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			return "", ErrWrongPassphrase
		}
		return "", fmt.Errorf("not a gtmux memory export: %w", err)
	}
	// Decrypt to a private temp file first, so the import path — verify, move aside,
	// extract — is exactly the plain one, with one implementation of every check.
	tmp, err := os.CreateTemp("", "gtmux-hq-import-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_ = os.Chmod(tmp.Name(), 0o600)
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("could not unlock the export: %w", err)
	}
	return ImportMemory(tmp.Name())
}

// ExportRecord is the last export's when and whether it was locked — what the memory line
// says after "nothing carries it off this disk": the commander can carry it off themselves,
// and the line should know when they last did.
type ExportRecord struct {
	At        int64 `json:"at"`
	Encrypted bool  `json:"encrypted"`
}

func exportRecordPath() string { return filepath.Join(state.Dir(), "hq-export.json") }

func recordExport(encrypted bool) {
	b, err := json.Marshal(ExportRecord{At: time.Now().Unix(), Encrypted: encrypted})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(exportRecordPath()), 0o755)
	_ = os.WriteFile(exportRecordPath(), b, 0o644)
}

// LastExport is the most recent export on this machine, if any.
func LastExport() (ExportRecord, bool) {
	var r ExportRecord
	b, err := os.ReadFile(exportRecordPath())
	if err != nil || json.Unmarshal(b, &r) != nil || r.At == 0 {
		return ExportRecord{}, false
	}
	return r, true
}

// readPassphraseLine takes a passphrase from a pipe: the first line of r, without its
// newline. This is how the menu-bar app hands one over — never on the command line, where
// `ps` would show it.
func readPassphraseLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
