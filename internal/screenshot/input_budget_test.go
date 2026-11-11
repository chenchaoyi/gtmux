//go:build darwin || linux

package screenshot

import (
	"bytes"
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestASC009FIFOPathsNeverBlockBeforeRegularFileValidation(t *testing.T) {
	for _, kind := range []string{"image", "document", "spec"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pipe")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				var err error
				switch kind {
				case "image":
					var f *os.File
					f, err = OpenImage(path, "")
					if f != nil {
						f.Close()
					}
				case "document":
					_, err = ReadDocument(path)
				case "spec":
					_, err = ReadSpec(path)
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Error("FIFO accepted as a regular input")
				}
				return
			case <-time.After(150 * time.Millisecond):
			}
			// Release the old blocking open deterministically so the negative control
			// ends with this target assertion rather than a leaked reader/test timeout.
			writer, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
			if err != nil {
				t.Fatal(err)
			}
			writer.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("FIFO control reader did not settle")
			}
			t.Error("FIFO open ignored the bounded regular-file preflight")
		})
	}
}

func TestASC009PipeDeadlineCancellationBytesAndOriginalFlags(t *testing.T) {
	for _, kind := range []string{"normal", "oversized", "timeout", "cancel", "no-deadline"} {
		t.Run(kind, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			flags := func() int {
				raw, e := reader.SyscallConn()
				if e != nil {
					t.Fatal(e)
				}
				value := 0
				e = raw.Control(func(fd uintptr) { value, e = unix.FcntlInt(fd, unix.F_GETFL, 0) })
				if e != nil {
					t.Fatal(e)
				}
				return value
			}
			before := flags()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			if kind == "no-deadline" {
				ctx = context.Background()
			}
			if kind == "cancel" {
				cancel()
			}
			payload := []byte("note \r\n")
			if kind == "oversized" {
				payload = bytes.Repeat([]byte("x"), 17)
			}
			wrote := make(chan struct{})
			if kind == "normal" || kind == "oversized" {
				go func() { defer close(wrote); _, _ = writer.Write(payload); writer.Close() }()
			} else {
				close(wrote)
			}
			got, err := ReadInput(ctx, "-", reader, 16)
			<-wrote
			if kind == "normal" {
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatal("note bytes changed", string(got), err)
				}
			} else if err == nil {
				t.Fatal("invalid or stalled note accepted", kind)
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation cause lost", err)
			}
			if flags() != before {
				t.Fatal("caller stdin flags changed", before, flags())
			}
			if _, e := reader.Stat(); e != nil {
				t.Fatal("caller stdin closed", e)
			}
		})
	}
}
