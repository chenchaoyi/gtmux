// Package userservice isolates per-user background service managers. Enrollment,
// credentials and remote-mode policy belong to callers, never to these adapters.
package userservice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manager interface {
	Dir() string
	Path(label string) string
	Check() error
	Write(path, label string, args []string, log string) error
	Start(path string) error
	Stop(path string) error
	Active(label string) bool
	Restart(label string) error
	Connector(data []byte) (string, error)
	ReplaceConnector(data []byte, old, next string) ([]byte, error)
	NeedsLinger() bool
}

type Runner func(context.Context, string, ...string) ([]byte, error)

func Command(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Do not include command output or argv in errors: connector tokens may be present.
	return exec.CommandContext(ctx, name, args...).Output()
}

func bounded(run Runner, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if _, err := run(ctx, name, args...); err != nil {
		return fmt.Errorf("%s failed (manager unavailable or operation refused)", name)
	}
	return nil
}

func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gtmux-service-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
