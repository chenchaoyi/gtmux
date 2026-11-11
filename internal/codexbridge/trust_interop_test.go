package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
	"github.com/gorilla/websocket"
)

// This runs production forwarding and the actual initialized-home observer. It
// does not impersonate production PID admission or claim a real terminal binding.
func trustUnixPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	// Go t.TempDir embeds the long test name, exceeding macOS sun_path.
	dir, err := os.MkdirTemp(os.TempDir(), "wtr-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "ws")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *websocket.Conn, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{}
		c, e := u.Upgrade(w, r, nil)
		if e == nil {
			accepted <- c
		}
	})}
	go func() { _ = server.Serve(listener) }()
	dial := websocket.Dialer{HandshakeTimeout: 2 * time.Second, NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	client, _, err := dial.Dial("ws://localhost/", nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	var peer *websocket.Conn
	select {
	case peer = <-accepted:
	case <-time.After(2 * time.Second):
		client.Close()
		server.Close()
		t.Fatal("fixture upgrade not observed")
	}
	t.Cleanup(func() { client.Close(); peer.Close(); server.Close() })
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	peer.SetReadDeadline(time.Now().Add(5 * time.Second))
	return client, peer
}

func TestWTR003ActualUnixForwardingUsesVerifiedHomeAndPreservesOtherFrames(t *testing.T) {
	main, cwd, _ := registeredTrustFixture(t)
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	downClient, down := trustUnixPair(t)
	up, backend := trustUnixPair(t)
	binds := 0
	o := &observer{home: transcript.CodexHomeID(), bind: func(string, string) { binds++ }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	trust := newTrustConfig(ctx, o.homeVerified)
	ended := make(chan error, 2)
	go func() { ended <- forward(down, up, func(data []byte) { o.request(data); trust.request(data) }) }()
	go func() { ended <- forward(up, down, o.response, trust.response) }()
	t.Cleanup(func() {
		cancel()
		downClient.Close()
		down.Close()
		up.Close()
		backend.Close()
		for i := 0; i < 2; i++ {
			select {
			case <-ended:
			case <-time.After(2 * time.Second):
				t.Error("forward worker did not terminate")
			}
		}
	})
	exchange := func(request, reply []byte, kind int) []byte {
		t.Helper()
		if e := downClient.WriteMessage(websocket.TextMessage, request); e != nil {
			t.Fatal(e)
		}
		gotKind, got, e := backend.ReadMessage()
		if e != nil || gotKind != websocket.TextMessage || !bytes.Equal(got, request) {
			t.Fatal("upstream request changed", e)
		}
		if e := backend.WriteMessage(kind, reply); e != nil {
			t.Fatal(e)
		}
		gotKind, got, e = downClient.ReadMessage()
		if e != nil || gotKind != kind {
			t.Fatal("downstream response failed", e)
		}
		return got
	}
	request := func(id int) []byte {
		v, _ := json.Marshal(map[string]any{"id": id, "method": "config/read", "params": map[string]any{"cwd": cwd, "includeLayers": true}})
		return v
	}
	reply := func(id int, untrusted bool) []byte {
		projects := map[string]any{main: map[string]any{"trust_level": "trusted"}}
		if untrusted {
			projects[cwd] = map[string]any{"trust_level": "untrusted"}
		}
		v, _ := json.Marshal(map[string]any{"id": id, "result": map[string]any{"config": map[string]any{"projects": projects, "approval_policy": "on-request"}, "layers": []any{}}})
		return v
	}
	pre := reply(2, false)
	if got := exchange(request(2), pre, websocket.TextMessage); !bytes.Equal(got, pre) {
		t.Fatal("uninitialized connection borrowed trust")
	}
	init, _ := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"codexHome": home}})
	if got := exchange([]byte(`{"id":1,"method":"initialize"}`), init, websocket.TextMessage); !bytes.Equal(got, init) || !o.homeVerified() {
		t.Fatal("home initialization changed")
	}
	source := reply(3, false)
	got := exchange(request(3), source, websocket.TextMessage)
	var output struct {
		Result struct {
			Config struct {
				Projects map[string]map[string]string `json:"projects"`
			} `json:"config"`
		} `json:"result"`
	}
	if json.Unmarshal(got, &output) != nil || output.Result.Config.Projects[cwd]["trust_level"] != "trusted" {
		t.Fatal("forwarded alias missing", string(got))
	}
	if binds != 0 {
		t.Fatal("configuration created a conversation binding")
	}
	denied := reply(4, true)
	if got := exchange(request(4), denied, websocket.TextMessage); !bytes.Equal(got, denied) {
		t.Fatal("explicit untrusted checkout altered")
	}
	binary := reply(5, false)
	if got := exchange(request(5), binary, websocket.BinaryMessage); !bytes.Equal(got, binary) {
		t.Fatal("binary frame altered")
	}
	if e := os.WriteFile(filepath.Join(cwd, ".git"), []byte("gitdir: /synthetic/unregistered\n"), 0600); e != nil {
		t.Fatal(e)
	}
	invalid := reply(6, false)
	if got := exchange(request(6), invalid, websocket.TextMessage); !bytes.Equal(got, invalid) {
		t.Fatal("invalid registration borrowed trust")
	}
}

// Optional installed-TUI acceptance seam. Both the backend and endpoint are
// private fixture processes. The production PID/admission handler is deliberately
// not bypassed or claimed here: only forwarding + initialized-home adaptation run.
func TestWTR003NativeTUIGateway(t *testing.T) {
	root := os.Getenv("GTMUX_WTR_FIXTURE")
	if root == "" {
		t.Skip("optional installed-TUI fixture; synthetic HOME required")
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" || !filepath.IsAbs(root) || !filepath.IsAbs(home) {
		t.Fatal("fixture paths required")
	}
	endpoint := filepath.Join(root, "gateway.sock")
	backendPath := filepath.Join(root, "backend.sock")
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if e := os.Chmod(endpoint, 0600); e != nil {
		listener.Close()
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	live := make(chan struct{}, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dial := websocket.Dialer{HandshakeTimeout: 2 * time.Second, NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", backendPath)
		}}
		up, _, e := dial.DialContext(ctx, "ws://localhost/", nil)
		if e != nil {
			http.Error(w, "fixture backend unavailable", http.StatusServiceUnavailable)
			return
		}
		defer up.Close()
		u := websocket.Upgrader{}
		down, e := u.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer down.Close()
		o := &observer{home: transcript.CodexHomeIDForPath(home), bind: func(string, string) {}}
		trust := newTrustConfig(r.Context(), o.homeVerified)
		ended := make(chan error, 2)
		go func() { ended <- forward(down, up, func(data []byte) { o.request(data); trust.request(data) }) }()
		transform := trust.response
		if os.Getenv("GTMUX_WTR_MODE") == "transparent" {
			transform = func(data []byte) []byte { return data }
		}
		go func() {
			ended <- forward(up, down, o.response, func(data []byte) []byte {
				result := transform(data)
				var response struct {
					Result struct {
						Config struct {
							Projects map[string]map[string]string `json:"projects"`
						} `json:"config"`
					} `json:"result"`
				}
				if json.Unmarshal(result, &response) == nil && response.Result.Config.Projects != nil {
					record, _ := json.Marshal(map[string]any{"changed": !bytes.Equal(result, data), "projects": response.Result.Config.Projects})
					// Synthetic fixture only, never enabled in the installed application.
					if file, err := os.OpenFile(filepath.Join(root, "observed-config.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
						_, _ = file.Write(append(record, '\n'))
						_ = file.Close()
					}
				}
				return result
			})
		}()
		select {
		case <-ended:
		case <-ctx.Done():
		}
		down.Close()
		up.Close()
		select {
		case live <- struct{}{}:
		default:
		}
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	if e := os.WriteFile(filepath.Join(root, "gateway.ready"), []byte("synthetic forwarding fixture only\n"), 0600); e != nil {
		t.Fatal(e)
	}
	select {
	case <-live:
	case <-time.After(30 * time.Second):
		t.Fatal("native fixture deadline expired")
	}
}
