package screenshot

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestASC002NativeTransportPinsHostAndBoundsReplies(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-host", "wrong-verifier", "wrong-id", "oversize", "timeout", "missing-result"} {
		t.Run(scenario, func(t *testing.T) {
			dir, e := os.MkdirTemp("/tmp", "gsc-")
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			path := filepath.Join(dir, "s")
			l, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			done := make(chan error, 1)
			go func() {
				c, e := l.AcceptUnix()
				if e != nil {
					done <- e
					return
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(time.Second))
				h := make([]byte, 4)
				oob := make([]byte, syscall.CmsgSpace(12))
				n, on, _, _, e := c.ReadMsgUnix(h, oob)
				if scenario == "wrong-host" || scenario == "wrong-verifier" {
					done <- nil
					return
				}
				if e != nil {
					done <- e
					return
				}
				if n < 4 {
					_, e = io.ReadFull(c, h[n:])
					if e != nil {
						done <- e
						return
					}
				}
				messages, e := syscall.ParseSocketControlMessage(oob[:on])
				if e != nil {
					done <- e
					return
				}
				fds := []int{}
				for _, m := range messages {
					v, e := syscall.ParseUnixRights(&m)
					if e != nil {
						done <- e
						return
					}
					fds = append(fds, v...)
				}
				defer func() {
					for _, fd := range fds {
						syscall.Close(fd)
					}
				}()
				if len(fds) != 1 {
					done <- Fail("fixture", "descriptor lost")
					return
				}
				b := make([]byte, binary.BigEndian.Uint32(h))
				if _, e = io.ReadFull(c, b); e != nil {
					done <- e
					return
				}
				var req Request
				if e = Decode(b, &req); e != nil {
					done <- e
					return
				}
				if req.Nonce != "test-nonce" || req.Version != 1 || req.DeadlineMS <= 0 {
					done <- Fail("fixture", "identity missing")
					return
				}
				if scenario == "timeout" {
					_, _ = c.Read(h)
					done <- nil
					return
				}
				result := json.RawMessage(`{"available":true}`)
				if scenario == "missing-result" {
					result = nil
				}
				id := req.ID
				if scenario == "wrong-id" {
					id = "other"
				}
				r, _ := json.Marshal(Reply{Version: 1, ID: id, OK: true, Result: result})
				size := len(r)
				if scenario == "oversize" {
					size = MaxJSON + 1
				}
				binary.BigEndian.PutUint32(h, uint32(size))
				_, _ = c.Write(append(h, r...))
				done <- nil
			}()
			f, e := os.CreateTemp(dir, "input")
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			_, _ = f.WriteString("synthetic")
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			host := Host{Version: 1, PID: os.Getpid(), Nonce: "test-nonce"}
			if scenario == "wrong-host" {
				host.PID++
			}
			r, e := nativeAt(ctx, path, host, Request{Operation: "capabilities"}, []*os.File{f}, func(int) bool { return scenario != "wrong-verifier" })
			if scenario == "valid" {
				if e != nil || !r.OK {
					t.Fatal(r, e)
				}
			} else if e == nil {
				t.Fatal("accepted", scenario)
			}
			if se := <-done; se != nil {
				t.Fatal(se)
			}
		})
	}
}

// Invoked by the Swift fixture against its real native producer. An ordinary Go
// run skips it explicitly; no running user App or recording permission is used.
func TestASC006NativeInterop(t *testing.T) {
	dir := os.Getenv("GTMUX_SCREENSHOT_FIXTURE")
	if dir == "" {
		t.Skip("requires isolated Swift native producer")
	}
	var host Host
	if e := ownedJSON(filepath.Join(dir, "host.json"), &host); e != nil {
		t.Fatal(e)
	}
	b := fixturePNG(t)
	input := filepath.Join(dir, "synthetic-input.png")
	if e := os.WriteFile(input, b, 0600); e != nil {
		t.Fatal(e)
	}
	call := func(c context.Context, r Request, f []*os.File) (Reply, error) {
		return nativeAt(c, filepath.Join(dir, "agent.sock"), host, r, f, func(pid int) bool { return pid == host.PID })
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	d, e := Produce(ctx, Request{Operation: "import"}, input, filepath.Join(dir, "imported.png"), call)
	if e != nil {
		t.Fatal(e)
	}
	if d.Kind != "imported" || d.PixelWidth != 4 || d.PixelHeight != 2 {
		t.Fatal(d)
	}
	spec := Spec{Version: 1, CaptureID: d.CaptureID, ImageSHA: d.ImageSHA, Space: "image_px", Marks: []Mark{{Type: "redact", X: 0, Y: 0, Width: 2, Height: 2}}}
	marked, e := Produce(ctx, Request{Operation: "annotate", ImageSHA: d.ImageSHA, Document: &d, Spec: &spec}, d.ImagePath, filepath.Join(dir, "marked.png"), call)
	if e != nil {
		t.Fatal(e)
	}
	if marked.ImageSHA == d.ImageSHA || marked.BaseDocument == nil {
		t.Fatal("real native renderer did not change image")
	}
}
