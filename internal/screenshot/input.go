package screenshot

import (
	"context"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// ReadInput bounds explicit CLI note input without leaving a blocked reader
// goroutine. The CLI exclusively owns stdin during this operation; the original
// descriptor remains open and its original flags are restored before returning.
func ReadInput(ctx context.Context, path string, stdin *os.File, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path != "-" {
		data, err := readBounded(path, limit)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return data, err
	}
	if stdin == nil {
		return nil, Fail("note-unavailable", "No standard input is available")
	}
	info, err := stdin.Stat()
	if err != nil {
		return nil, Fail("note-unavailable", "Could not inspect standard input")
	}
	if info.Mode().IsRegular() {
		return readInputBytes(ctx, stdin, limit)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return nil, Fail("note-unavailable", "Use a regular file or pipe for a screenshot note")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, Fail("deadline-required", "A note pipe requires a caller deadline")
	}
	raw, err := stdin.SyscallConn()
	if err != nil {
		return nil, err
	}
	original, flags, fd := -1, 0, -1
	var accessErr error
	err = raw.Control(func(value uintptr) {
		original = int(value)
		flags, accessErr = unix.FcntlInt(value, unix.F_GETFL, 0)
		if accessErr == nil {
			fd, accessErr = unix.Dup(original)
		}
	})
	if err != nil {
		return nil, err
	}
	if accessErr != nil {
		return nil, accessErr
	}
	unix.CloseOnExec(fd)
	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return nil, err
	}
	// NewFile detects the nonblocking descriptor and attaches Go's poller. Reads
	// can now be interrupted by a deadline without a forever-blocked goroutine.
	file := os.NewFile(uintptr(fd), "screenshot-note")
	defer func() { file.Close(); _, _ = unix.FcntlInt(uintptr(original), unix.F_SETFL, flags) }()
	if err = file.SetReadDeadline(deadline); err != nil {
		return nil, Fail("note-unavailable", "Standard input does not support a bounded read")
	}
	canceled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = file.SetReadDeadline(time.Now()); close(canceled) })
	defer func() {
		if !stop() {
			<-canceled
		}
	}()
	return readInputBytes(ctx, file, limit)
}

func readInputBytes(ctx context.Context, file *os.File, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, Fail("file-too-large", "Input exceeds its size budget")
	}
	return data, nil
}
