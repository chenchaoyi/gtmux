package artifact

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMarkdownLinkEvidenceExcludesCodeAndHandlesRealDestinations(t *testing.T) {
	source := "[Report](</tmp/a report.md>) ![shot](./images/shot.png)\n\n[More][ref]\n\n[ref]: /tmp/next.md\n\n`[private](/tmp/inline.txt)`\n\n```md\n[x](/tmp/fenced.txt)\n```\n\n<img src=\"/tmp/html.png\">"
	want := []string{"/tmp/a report.md", "./images/shot.png", "/tmp/next.md"}
	if got := Links(source); !reflect.DeepEqual(got, want) {
		t.Fatalf("destinations = %#v", got)
	}
	for _, ref := range []string{"https://example.org/file", "file://foreign/tmp/file", "//foreign/file", "file:///tmp/file?x=1", "/tmp/%00.txt", "javascript:bad"} {
		if _, ok := LocalPath(ref, "/tmp"); ok {
			t.Fatalf("unsafe reference accepted: %s", ref)
		}
	}
	if path, ok := LocalPath("file:///tmp/a%20report.md#section", ""); !ok || path != "/tmp/a report.md" {
		t.Fatal(path, ok)
	}
}

func TestRelatedMarkdownFilesStayBoundToVerifiedParent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "images"), 0700)
	image := filepath.Join(root, "images/shot.png")
	os.WriteFile(image, []byte("fixture pixels"), 0600)
	other := filepath.Join(root, "unlinked.txt")
	os.WriteFile(other, []byte("private"), 0600)
	outside := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(outside, []byte("outside"), 0600)
	os.Symlink(outside, filepath.Join(root, "images/symlink.png"))
	os.Symlink(filepath.Dir(outside), filepath.Join(root, "escape"))
	source := filepath.Join(root, "report.md")
	os.WriteFile(source, []byte("![shot](images/shot.png)\n![escape]("+outside+")\n![symlink](images/symlink.png)\n![parent link](escape/outside.png)\n`[hidden](unlinked.txt)`"), 0600)
	parent, err := Add(source, "session", "home")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Related("images/shot.png", parent.ID, "session", "home")
	if err != nil || f.Source != image {
		t.Fatal(f, err)
	}
	for _, req := range [][4]string{{"unlinked.txt", parent.ID, "session", "home"}, {"images/shot.png", parent.ID, "other", "home"}, {"images/shot.png", parent.ID, "session", "other"}, {outside, parent.ID, "session", "home"}, {"images/symlink.png", parent.ID, "session", "home"}, {"escape/outside.png", parent.ID, "session", "home"}} {
		if _, err := Related(req[0], req[1], req[2], req[3]); err == nil {
			t.Fatal("unrelated source read", req)
		}
	}
	os.Remove(source)
	if _, err := Related("images/shot.png", parent.ID, "session", "home"); err != nil {
		t.Fatal("parent snapshot must survive original removal", err)
	}
	if err := Remove(parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Related("images/shot.png", parent.ID, "session", "home"); err == nil {
		t.Fatal("revoked parent granted resources")
	}
}

func TestRemovedSnapshotCannotBeAutomaticallyResurrected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.md")
	os.WriteFile(source, []byte("report"), 0600)
	f, err := Linked(source, "session", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Linked(source, "session", ""); err != ErrRevoked {
		t.Fatal("automatic re-publication after removal", err)
	}
	// An explicit local add is still an intentional new grant.
	restored, err := Add(source, "session", "")
	if err != nil || restored.ID != f.ID {
		t.Fatal(restored, err)
	}
	if _, err := Linked(source, "session", ""); err != nil {
		t.Fatal(err)
	}
}
