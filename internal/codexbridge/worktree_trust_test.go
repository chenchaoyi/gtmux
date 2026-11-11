package codexbridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func registeredTrustFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	checkout := filepath.Join(root, "worker")
	dir := filepath.Join(main, ".git", "worktrees", "worker")
	for _, p := range []string{dir, checkout} {
		if e := os.MkdirAll(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for p, s := range map[string]string{filepath.Join(checkout, ".git"): "gitdir: " + dir + "\n", filepath.Join(dir, "gitdir"): filepath.Join(checkout, ".git") + "\n", filepath.Join(dir, "commondir"): "../..\n"} {
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return main, checkout, dir
}

func TestWTR001OnlyReciprocallyRegisteredWorktreesHaveStableEvidence(t *testing.T) {
	main, checkout, _ := registeredTrustFixture(t)
	sub := filepath.Join(checkout, "nested")
	if e := os.Mkdir(sub, 0700); e != nil {
		t.Fatal(e)
	}
	first, ok := readWorktreeTrust(context.Background(), sub)
	if !ok || first.MainRoot != main || first.Checkout != checkout || first.Cwd != sub || first.Fingerprint == "" {
		t.Fatal(first, ok)
	}
	second, ok := readWorktreeTrust(context.Background(), sub)
	if !ok || first != second {
		t.Fatal("unstable read", first, second, ok)
	}
	// Ordinary working files must not invalidate the directory identity seal.
	if e := os.WriteFile(filepath.Join(sub, "file"), []byte("synthetic"), 0600); e != nil {
		t.Fatal(e)
	}
	third, ok := readWorktreeTrust(context.Background(), sub)
	if !ok || third != first {
		t.Fatal("unrelated directory mtime affected proof", third, ok)
	}
	alias := filepath.Join(filepath.Dir(checkout), "alias")
	if e := os.Symlink(checkout, alias); e != nil {
		t.Fatal(e)
	}
	got, ok := readWorktreeTrust(context.Background(), alias)
	if !ok || got.Cwd != checkout || got.MainRoot != main {
		t.Fatal(got, ok)
	}
}

func TestWTR001MalformedCopiedSymlinkAndUnsupportedMetadataCannotBorrowTrust(t *testing.T) {
	for _, kind := range []string{"copied-pointer", "wrong-backlink", "wrong-common", "symlink-pointer", "oversized", "main-checkout", "relative-cwd", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			main, checkout, dir := registeredTrustFixture(t)
			ctx := context.Background()
			write := func(p, s string) {
				if e := os.WriteFile(p, []byte(s), 0600); e != nil {
					t.Fatal(e)
				}
			}
			switch kind {
			case "copied-pointer":
				other := filepath.Join(filepath.Dir(checkout), "stranger")
				os.Mkdir(other, 0700)
				write(filepath.Join(other, ".git"), "gitdir: "+dir)
				checkout = other
			case "wrong-backlink":
				write(filepath.Join(dir, "gitdir"), filepath.Join(main, ".git"))
			case "wrong-common":
				write(filepath.Join(dir, "commondir"), "../../other")
			case "symlink-pointer":
				p := filepath.Join(checkout, ".git")
				b, e := os.ReadFile(p)
				if e != nil {
					t.Fatal(e)
				}
				if e = os.Remove(p); e != nil {
					t.Fatal(e)
				}
				outside := filepath.Join(t.TempDir(), "pointer")
				write(outside, string(b))
				if e = os.Symlink(outside, p); e != nil {
					t.Fatal(e)
				}
			case "oversized":
				write(filepath.Join(dir, "gitdir"), strings.Repeat("x", trustMetadataLimit+1))
			case "main-checkout":
				checkout = main
			case "relative-cwd":
				checkout = "relative"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if got, ok := readWorktreeTrust(ctx, checkout); ok || got.Fingerprint != "" {
				t.Fatal("unverified directory borrowed trust", kind, got)
			}
		})
	}
}
