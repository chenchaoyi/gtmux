package resume

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
)

func TestCBA004CancellationDuringStrictProofCannotPublish(t *testing.T) {
	f, _ := frontendFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := frontendClients
	frontendClients = func(p []codexterminal.Pane) map[string]codexterminal.Client { cancel(); return original(p) }
	if _, err := BindCodexFrontendContext(ctx, f, "root"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("canceled proof acquired authority")
	}
	if _, err := os.Stat(frontendPath(f.Instance)); !os.IsNotExist(err) {
		t.Fatal("canceled proof wrote a live receipt", err)
	}
}

func TestCBA004CanceledPartialCommitRetiresOnlyItsOwnLiveReceipt(t *testing.T) {
	for _, stage := range []string{"receipt", "index"} {
		t.Run(stage, func(t *testing.T) {
			f, _ := frontendFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := frontendBindingWrite
			t.Cleanup(func() { frontendBindingWrite = original })
			frontendBindingWrite = func(path string, value any) error {
				err := original(path, value)
				if err == nil && (stage == "receipt" && path == frontendPath(f.Instance) || stage == "index" && path == fileFor(f.Loc)) {
					cancel()
				}
				return err
			}
			if _, err := BindCodexFrontendContext(ctx, f, "root"); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if _, ok := LoadLive(f.Loc); ok {
				t.Fatal("canceled publication retained live authority")
			}
			if _, err := os.Stat(frontendPath(f.Instance)); !os.IsNotExist(err) {
				t.Fatal("own receipt not retired", err)
			}
			if stage == "index" {
				if r, ok := Load(f.Loc); !ok || r.SessionID != "root" {
					t.Fatal("already committed history deleted", r, ok)
				}
			}
		})
	}
}

func TestCBA004CancellationNeverRemovesAReplacedOwner(t *testing.T) {
	f, _ := frontendFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := frontendBindingWrite
	t.Cleanup(func() { frontendBindingWrite = original })
	var replacement CodexFrontend
	frontendBindingWrite = func(path string, value any) error {
		err := original(path, value)
		if err == nil && path == frontendPath(f.Instance) {
			replacement = value.(CodexFrontend)
		}
		if err == nil && path == fileFor(f.Loc) {
			replacement.SessionID = "other-owner"
			replacement.Sequence++
			if err = writeBindingJSON(frontendPath(f.Instance), replacement); err != nil {
				return err
			}
			cancel()
		}
		return err
	}
	if _, err := BindCodexFrontendContext(ctx, f, "root"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if saved, ok := readFrontend(f.Instance); !ok || saved != replacement {
		t.Fatal("cancellation removed another owner's receipt", saved, ok)
	}
}
