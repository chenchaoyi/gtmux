package artifact

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedSnapshotIsBoundedAndIndependentOfSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(source, []byte("<html>first</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Add(source, "session-one", "home-one")
	if err != nil {
		t.Fatal(err)
	}
	again, err := Add(source, "session-one", "home-one")
	if err != nil || first.ID != again.ID {
		t.Fatal("same bytes should reuse the registered snapshot", err)
	}
	for _, scope := range [][2]string{{"other", "home-one"}, {"session-one", "other"}, {"", ""}} {
		if _, err := Resolve(first.Reference(), scope[0], scope[1]); err == nil {
			t.Fatal("reference crossed a conversation boundary")
		}
	}
	if got, err := Resolve(source, "session-one", "home-one"); err != nil || got.ID != first.ID {
		t.Fatal("registered original link did not resolve", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	meta, fd, err := Open(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	b, err := io.ReadAll(fd)
	if err != nil || string(b) != "<html>first</html>" || meta.SHA256 != first.SHA256 {
		t.Fatal("source removal changed the published bytes", err)
	}
	for _, suffix := range []string{".data", ".json"} {
		info, err := os.Stat(filepath.Join(directory(), first.ID+suffix))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("snapshot is not private", err)
		}
	}
	if _, err := Resolve(filepath.Join(t.TempDir(), "unpublished.html"), "session-one", "home-one"); err == nil {
		t.Fatal("arbitrary local path was treated as published")
	}
}

func TestPublicationRefusesLinksSecretsAndOversize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	source := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(source, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "linked.txt")
	if err := os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(symlink, "session", ""); err == nil {
		t.Fatal("symlink publication allowed")
	}
	hardlink := filepath.Join(dir, "hardlink.txt")
	if err := os.Link(source, hardlink); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(hardlink, "session", ""); err == nil {
		t.Fatal("hard-link publication allowed")
	}
	for _, name := range []string{".env", "id_rsa.pem", ".ssh/report.txt"} {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("not for publication"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Add(file, "session", ""); err == nil {
			t.Fatal("credential location/format allowed", name)
		}
	}
	large := filepath.Join(dir, "large.txt")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Add(large, "session", ""); err == nil {
		t.Fatal("oversized file allowed")
	}
}

func TestTamperingAndEscapingIDsFailClosed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := Add(source, "session", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory(), f.ID+".data"), []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, fd, err := Open(f.ID); err == nil || fd != nil {
		t.Fatal("modified snapshot served")
	}
	for _, id := range []string{"../report", strings.Repeat("a", 33), "", f.ID + "/other"} {
		if _, err := Get(id); err == nil {
			t.Fatal("bad identifier allowed", id)
		}
	}
}

func TestRepublishingUsesExactReferencesAndNewestRegisteredPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.txt")
	os.WriteFile(source, []byte("version one"), 0600)
	first, err := Add(source, "session", "")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(source, []byte("version two"), 0600)
	second, err := Add(source, "session", "")
	if err != nil {
		t.Fatal(err)
	}
	latest, err := Resolve(source, "session", "")
	if err != nil || latest.ID != second.ID || second.Sequence <= first.Sequence {
		t.Fatal("latest path is ambiguous", err)
	}
	exact, err := Resolve(first.Reference(), "session", "")
	if err != nil || exact.ID != first.ID {
		t.Fatal("an immutable reference drifted", err)
	}
	rows, err := List()
	if err != nil || len(rows) != 2 || rows[0].ID != second.ID {
		t.Fatal(rows, err)
	}
	if err = Remove(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = Open(second.ID); err == nil {
		t.Fatal("removed copy remains downloadable")
	}
	if bytes, err := os.ReadFile(source); err != nil || string(bytes) != "version two" {
		t.Fatal("source was changed by removal", err)
	}
	// A corrupt data entry is not silently accepted as a valid publication.
	os.Remove(filepath.Join(directory(), first.ID+".data"))
	os.WriteFile(source, []byte("version one"), 0600)
	_, _, err = Open(first.ID)
	if err == nil {
		t.Fatal("missing content opened")
	}
}

func TestRegistryQuotaAndRemoval(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.txt")
	var first File
	for i := 0; i < MaxFiles; i++ {
		if err := os.WriteFile(source, []byte(strings.Repeat("x", i+1)), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := Add(source, "session", "")
		if err != nil {
			t.Fatal(i, err)
		}
		if i == 0 {
			first = file
		}
	}
	os.WriteFile(source, []byte("next"), 0600)
	if _, err := Add(source, "session", ""); err != ErrLimit {
		t.Fatal("quota did not stop publication", err)
	}
	if err := Remove(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(source, "session", ""); err != nil {
		t.Fatal("explicit removal did not restore capacity", err)
	}
}
