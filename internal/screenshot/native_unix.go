//go:build darwin || linux

package screenshot

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type Host struct {
	Version int    `json:"schema_version"`
	PID     int    `json:"pid"`
	Start   string `json:"process_start"`
	Nonce   string `json:"nonce"`
}

// Native is local-only and never launches an App, asks TCC or falls back to TCP.
func Native(ctx context.Context, request Request, files []*os.File) (Reply, error) {
	if runtime.GOOS != "darwin" {
		return Reply{}, Fail("platform-unsupported", "Native screenshots require the macOS App")
	}
	dir := filepath.Join(state.Dir(), "capture-bridge")
	var host Host
	if err := ownedJSON(filepath.Join(dir, "host.json"), &host); err != nil {
		return Reply{}, Fail("app-unavailable", "Open a compatible Gtmux App to use native screenshot operations")
	}
	if host.Version != Version || host.PID <= 1 || len(host.Nonce) != 64 || host.Start == "" {
		return Reply{}, Fail("host-unverified", "Screenshot host metadata is invalid")
	}
	return nativeAt(ctx, filepath.Join(dir, "agent.sock"), host, request, files, func(pid int) bool {
		p, ok := launchhost.ReadAgent(pid)
		observed, e1 := time.Parse("Mon Jan 2 15:04:05 2006", p.Start)
		birth, e2 := time.Parse(time.RFC3339Nano, host.Start)
		return ok && p.PID == host.PID && p.Comm == "GtmuxBar" && e1 == nil && e2 == nil && birth.Sub(observed) >= 0 && birth.Sub(observed) < time.Second
	})
}

func nativeAt(ctx context.Context, path string, host Host, request Request, files []*os.File, verify func(int) bool) (Reply, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	if request.Document != nil {
		frame := *request.Document
		frame.ImagePath, frame.PreviewPath, frame.BasePath, frame.BaseSHA = "", "", "", ""
		frame.BaseDocument, frame.Annotations = nil, nil
		request.Document = &frame
	}
	request.Version, request.Nonce, request.DeadlineMS = Version, host.Nonce, deadline.UnixMilli()
	if request.ID == "" {
		var err error
		request.ID, err = RandomID()
		if err != nil {
			return Reply{}, err
		}
	}
	body, err := json.Marshal(request)
	if err != nil || len(body) > MaxJSON || len(files) > 3 {
		return Reply{}, Fail("invalid-request", "Request exceeds protocol limits")
	}
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return Reply{}, transportFailure(ctx, "app-unavailable", "Screenshot host is not reachable")
	}
	defer raw.Close()
	c, ok := raw.(*net.UnixConn)
	if !ok {
		return Reply{}, Fail("host-unverified", "Invalid local transport")
	}
	pid, valid := codexrpc.LocalPeerPID(c)
	if !valid || pid != host.PID || !verify(pid) {
		return Reply{}, Fail("host-unverified", "Screenshot host process changed")
	}
	if err := c.SetDeadline(deadline); err != nil {
		return Reply{}, err
	}
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-finished:
		}
	}()
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(body)))
	var fds []int
	for _, f := range files {
		fds = append(fds, int(f.Fd()))
	}
	var rights []byte
	if len(fds) > 0 {
		rights = syscall.UnixRights(fds...)
	}
	n, _, err := c.WriteMsgUnix(header, rights, nil)
	if err != nil {
		return Reply{}, transportFailure(ctx, "transport-failed", "Could not write screenshot request")
	}
	if n < 4 {
		if _, err = io.Copy(c, bytes.NewReader(header[n:])); err != nil {
			return Reply{}, err
		}
	}
	if _, err = io.Copy(c, bytes.NewReader(body)); err != nil {
		return Reply{}, transportFailure(ctx, "transport-failed", "Could not finish screenshot request")
	}
	if _, err = io.ReadFull(c, header); err != nil {
		return Reply{}, transportFailure(ctx, "transport-unconfirmed", "Screenshot host response was not confirmed")
	}
	size := binary.BigEndian.Uint32(header)
	if size == 0 || size > MaxJSON {
		return Reply{}, Fail("invalid-response", "Screenshot response exceeds its budget")
	}
	data := make([]byte, size)
	if _, err = io.ReadFull(c, data); err != nil {
		return Reply{}, transportFailure(ctx, "transport-unconfirmed", "Screenshot host response was incomplete")
	}
	var reply Reply
	if err = Decode(data, &reply); err != nil {
		return Reply{}, err
	}
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	if reply.Version != Version || reply.ID != request.ID || reply.OK == (reply.Error != nil) {
		return Reply{}, Fail("invalid-response", "Screenshot response identity or result is inconsistent")
	}
	if reply.OK && (len(reply.Result) == 0 || string(reply.Result) == "null") {
		return Reply{}, Fail("invalid-response", "Screenshot host returned no result")
	}
	if !reply.OK {
		return reply, reply.Error
	}
	return reply, nil
}

func ownedJSON(path string, v any) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 || i.Size() > MaxJSON {
		return Fail("private-file-invalid", "Private screenshot metadata is invalid")
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) {
		return Fail("private-file-invalid", "Private screenshot metadata has a different owner")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxJSON+1))
	if err != nil {
		return err
	}
	return Decode(b, v)
}

func PrivateDir() (string, error) {
	return privateDir(true)
}

func privateDir(create bool) (string, error) {
	p := filepath.Join(state.Dir(), "capture-requests")
	if create {
		if err := state.MkdirAll(p, 0700); err != nil {
			return "", err
		}
	}
	i, err := os.Lstat(p)
	if err != nil || !i.IsDir() || i.Mode().Perm()&0077 != 0 {
		return "", Fail("private-directory-invalid", "Screenshot request directory is not private")
	}
	if s, ok := i.Sys().(*syscall.Stat_t); !ok || s.Uid != uint32(os.Geteuid()) {
		return "", Fail("private-directory-invalid", "Screenshot request directory has a different owner")
	}
	return p, nil
}

func safeID(id string) bool {
	if len(id) < 16 || len(id) > 80 {
		return false
	}
	return strings.Trim(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") == ""
}

func transportFailure(ctx context.Context, code, message string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if end, ok := ctx.Deadline(); ok && !time.Now().Before(end) {
		return context.DeadlineExceeded
	}
	return Fail(code, message)
}
