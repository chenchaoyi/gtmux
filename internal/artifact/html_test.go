package artifact

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHTMLImagesOnlyActualAttributes(t *testing.T) {
	source := `<!-- <img src="hidden.png"> --><script>"<img src='secret.png'>"</script><img alt="<fake>" src="images/a&amp;b.png"><picture><source srcset="images/one.png 1x, images/two.png 2x"></picture><a href="private.txt">not image</a>`
	if got := HTMLImages(source); !reflect.DeepEqual(got, []string{"images/a&b.png", "images/one.png", "images/two.png"}) {
		t.Fatal(got)
	}
}
func TestHTMLRelatedImagesUseSnapshotAndKeepDirectoryFence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "images"), 0700)
	for _, name := range []string{"shot.png", "a b.png", "unlinked.png"} {
		os.WriteFile(filepath.Join(root, "images", name), []byte("pixels"), 0600)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(root, "images", "escape.png"))
	os.WriteFile(filepath.Join(root, "private.txt"), []byte("private"), 0600)
	path := filepath.Join(root, "report.html")
	os.WriteFile(path, []byte(`<img src="images/shot.png"><img src="images/a%20b.png"><img src="images/escape.png"><img src="private.txt"><img src="`+outside+`"><!-- <img src="images/unlinked.png"> -->`), 0600)
	p, err := Add(path, "session", "home")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(`<img src="images/unlinked.png">`), 0600)
	for _, ref := range []string{"images/shot.png", "images/a%20b.png"} {
		if f, err := Related(ref, p.ID, "session", "home"); err != nil || f.Kind != "image" {
			t.Fatal(ref, f, err)
		}
	}
	for _, ref := range []string{"images/unlinked.png", "images/escape.png", "private.txt", outside, "https://host/track.png"} {
		if _, err := Related(ref, p.ID, "session", "home"); err == nil {
			t.Fatal("escaped parent", ref)
		}
	}
	if _, err := Related("images/shot.png", p.ID, "other", "home"); err == nil {
		t.Fatal("crossed session")
	}
}
