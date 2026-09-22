package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	chclient "github.com/jpillora/chisel/client"
	chserver "github.com/jpillora/chisel/server"
)

// Direct gives each device its own chisel account, allowed to bind only its own port
// (openspec/changes/direct-per-device-accounts). The provisioner's tests pin the authfile
// it writes (tunnel-worker/src/direct.test.ts); these pin what a REAL chisel 1.12.1 server
// does with an authfile of exactly that shape, and what the client does when the server
// refuses its account. The regex below is the one tunnel-worker/src/direct.ts emits.
func deviceRule(port int) string { return fmt.Sprintf(`^R:127\.0\.0\.1:%d$`, port) }

func zzFree(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// answering serves one HTTP handler that names itself, standing in for a Mac's serve.
func answering(t *testing.T, name string) int {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, name+" saw "+r.Header.Get("Authorization"))
	}))
	t.Cleanup(s.Close)
	return s.Listener.Addr().(*net.TCPAddr).Port
}

// directServer runs chisel the way the Direct server now runs: --reverse, --authfile, and
// no --auth. It returns the URL and the authfile path, which a test may rewrite.
func directServer(t *testing.T, users map[string][]string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.json")
	writeUsers(t, path, users)
	srv, err := chserver.NewServer(&chserver.Config{AuthFile: path, Reverse: true, KeepAlive: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	srv.Logger.Info = false
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	port := zzFree(t)
	if err := srv.StartContext(ctx, "127.0.0.1", fmt.Sprint(port)); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port), path
}

func writeUsers(t *testing.T, path string, users map[string][]string) {
	t.Helper()
	b, _ := json.Marshal(users)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil { // how gtmux-authsync swaps it in
		t.Fatal(err)
	}
}

// open starts a client for one remote and reports whether the server accepted it. A
// refused remote ends the only attempt; an accepted one stays connected.
func open(t *testing.T, server, secret, remote string) bool {
	t.Helper()
	cl, err := chclient.NewClient(&chclient.Config{Server: server, Auth: secret, Remotes: []string{remote}, MaxRetryCount: 0})
	if err != nil {
		t.Fatal(err)
	}
	cl.Logger.Info = false
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = cl.Close() })
	if err := cl.Start(ctx); err != nil {
		return false
	}
	done := make(chan struct{})
	go func() { _ = cl.Wait(); close(done) }()
	select {
	case <-done:
		return false
	case <-time.After(1500 * time.Millisecond):
		return true
	}
}

func fetch(port int) string {
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/agents", port), nil)
	req.Header.Set("Authorization", "Bearer phone-token")
	req.Close = true // a new connection each time: what a revocation is judged on
	res, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func TestADeviceAccountBindsItsOwnPortAndNothingElse(t *testing.T) {
	pA, pB := zzFree(t), zzFree(t)
	server, _ := directServer(t, withSentinel(map[string][]string{
		"da:passa": {deviceRule(pA)},
		"db:passb": {deviceRule(pB)},
	}))
	macA := answering(t, "mac-A")

	if !open(t, server, "da:passa", fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", pA, macA)) {
		t.Fatal("a device could not bind its own port")
	}
	if got := fetch(pA); !strings.HasPrefix(got, "mac-A") {
		t.Fatalf("its own port did not reach it: %q", got)
	}
	// The takeover this change exists to stop: B is offline, A asks for B's port.
	if open(t, server, "da:passa", fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", pB, macA)) {
		t.Error("a device bound ANOTHER device's port")
	}
	if got := fetch(pB); got != "" {
		t.Errorf("another device's port answered: %q", got)
	}
}

// The shared user could also reach every loopback service on the VPS (Caddy's admin API
// among them) with a forward tunnel, and bind on the public interface. Neither matches a
// device rule.
func TestADeviceAccountCannotReachTheServersOwnServicesOrBindPublicly(t *testing.T) {
	pA := zzFree(t)
	server, _ := directServer(t, withSentinel(map[string][]string{"da:passa": {deviceRule(pA)}}))
	private := answering(t, "server-private-service")
	if open(t, server, "da:passa", fmt.Sprintf("%d:127.0.0.1:%d", zzFree(t), private)) {
		t.Error("a device opened a forward tunnel to a service on the server")
	}
	if open(t, server, "da:passa", fmt.Sprintf("R:0.0.0.0:%d:127.0.0.1:%d", pA, private)) {
		t.Error("a device bound its port on every interface of the server")
	}
}

// sentinel is the account gtmux-authsync keeps in every Direct authfile. chisel with NO
// users turns authentication off entirely (next test), so the file is never empty.
const sentinel = "gtmux-sentinel:0123456789abcdef"

func withSentinel(users map[string][]string) map[string][]string {
	out := map[string][]string{sentinel: {"^$"}}
	for k, v := range users {
		out[k] = v
	}
	return out
}

// The sentinel is load-bearing, and this is why. With no users at all chisel does not
// authenticate anyone, and with no user to check, no rule applies: a stranger binds any
// port. A fresh Direct server has no device accounts, and revoking the last one leaves
// none. If this ever stops holding, chisel changed, and the sentinel may no longer be
// needed; until then, the next test is the one that matters.
func TestAnAuthfileWithNoUsersLetsAnyoneIn(t *testing.T) {
	server, _ := directServer(t, map[string][]string{})
	if directAccountRefused(context.Background(), server, "stranger:anything") {
		t.Skip("chisel now authenticates even with no users; the sentinel may be unnecessary")
	}
	if !open(t, server, "stranger:anything", fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", zzFree(t), answering(t, "x"))) {
		t.Error("expected chisel with no users to let a stranger bind a port (the hazard the sentinel exists for)")
	}
}

func TestTheSentinelKeepsAnEmptyDirectServerClosed(t *testing.T) {
	server, _ := directServer(t, withSentinel(nil))
	if !directAccountRefused(context.Background(), server, "stranger:anything") {
		t.Error("a Direct server with no device accounts let a stranger authenticate")
	}
	if open(t, server, sentinel, fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", zzFree(t), answering(t, "x"))) {
		t.Error("the sentinel account bound a port")
	}
}

// TestHelperDirectServer is not a test: it is the chisel server the restart test runs in
// a child process, so that "restart" means what it means on the VPS — the process and
// every session it held end together. (chisel's in-process Close leaves the sessions it
// already upgraded running.)
func TestHelperDirectServer(t *testing.T) {
	if os.Getenv("GTMUX_HELPER_DIRECT_SERVER") == "" {
		t.Skip("helper process for TestARevokedDeviceCannotComeBackAfterTheRestart")
	}
	s, err := chserver.NewServer(&chserver.Config{AuthFile: os.Getenv("AUTHFILE"), Reverse: true, KeepAlive: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	s.Logger.Info = false
	if err := s.StartContext(context.Background(), "127.0.0.1", os.Getenv("PORT")); err != nil {
		t.Fatal(err)
	}
	_ = s.Wait()
}

func startDirectServerProcess(t *testing.T, authfile, port string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperDirectServer$")
	cmd.Env = append(os.Environ(), "GTMUX_HELPER_DIRECT_SERVER=1", "AUTHFILE="+authfile, "PORT="+port)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	waitFor(t, "the chisel server to listen", func() bool {
		c, err := net.Dial("tcp", "127.0.0.1:"+port)
		if err == nil {
			_ = c.Close()
		}
		return err == nil
	})
	return cmd
}

// Revoking has to reach a device that is ONLINE. chisel re-reads the authfile for new
// sessions, but "established tunnels are not interrupted", and a device's reverse port is
// one: measured, a Mac dropped from the authfile kept serving through its port. So the
// sync restarts chisel when an account is removed (deploy/self-tunnel/gtmux-authsync), and
// this pins what that buys: the revoked device, retrying as the real client does, cannot
// come back.
func TestARevokedDeviceCannotComeBackAfterTheRestart(t *testing.T) {
	pA := zzFree(t)
	users := filepath.Join(t.TempDir(), "users.json")
	writeUsers(t, users, withSentinel(map[string][]string{"da:passa": {deviceRule(pA)}}))
	port := fmt.Sprint(zzFree(t))
	server := "http://127.0.0.1:" + port
	first := startDirectServerProcess(t, users, port)

	cl, err := chclient.NewClient(&chclient.Config{Server: server, Auth: "da:passa", MaxRetryCount: -1,
		MaxRetryInterval: 300 * time.Millisecond, KeepAlive: time.Second,
		Remotes: []string{fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", pA, answering(t, "mac-A"))}})
	if err != nil {
		t.Fatal(err)
	}
	cl.Logger.Info = false
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = cl.Close() }()
	if err := cl.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the device to come up", func() bool { return strings.HasPrefix(fetch(pA), "mac-A") })

	// The revocation: the account leaves the authfile, and the sync restarts chisel.
	writeUsers(t, users, withSentinel(nil))
	_ = first.Process.Kill()
	_, _ = first.Process.Wait()
	startDirectServerProcess(t, users, port)

	time.Sleep(2 * time.Second) // several of the client's reconnect attempts
	if got := fetch(pA); got != "" {
		t.Errorf("the revoked device came back after the restart: %q", got)
	}
	if !directAccountRefused(context.Background(), server, "da:passa") {
		t.Error("the revoked account is still accepted")
	}
}

// A NEW account is picked up without a restart: chisel reloads the authfile when the sync
// renames a new one into place, so adding a device interrupts nobody. chisel watches the
// directory with inotify on Linux, which is what the Direct server runs; macOS's kqueue
// does not report a rename over an existing name, so this is pinned where it is true.
func TestANewDeviceIsPickedUpWithoutARestart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("chisel's authfile reload relies on inotify; the Direct server is Linux")
	}
	pA, pB := zzFree(t), zzFree(t)
	server, path := directServer(t, withSentinel(map[string][]string{"da:passa": {deviceRule(pA)}}))
	writeUsers(t, path, withSentinel(map[string][]string{"da:passa": {deviceRule(pA)}, "db:passb": {deviceRule(pB)}}))
	waitFor(t, "the new account to be accepted", func() bool {
		return !directAccountRefused(context.Background(), server, "db:passb")
	})
	if !open(t, server, "db:passb", fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", pB, answering(t, "mac-B"))) {
		t.Error("a device added to the authfile could not bind its port")
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The pre-check: an account the server refuses is reported as refused; one it accepts is
// not; and a server that cannot be reached is NOT a refusal — the long-running client is
// left to wait that out.
func TestTheClientTellsARefusedAccountFromAnUnreachableServer(t *testing.T) {
	pA := zzFree(t)
	server, _ := directServer(t, withSentinel(map[string][]string{"da:passa": {deviceRule(pA)}}))
	ctx := context.Background()
	if !directAccountRefused(ctx, server, "da:wrong") {
		t.Error("a wrong password was not reported as refused")
	}
	if !directAccountRefused(ctx, server, "shared:secret") {
		t.Error("the retired shared user was not reported as refused")
	}
	if directAccountRefused(ctx, server, "da:passa") {
		t.Error("a valid account was reported as refused")
	}
	if directAccountRefused(ctx, fmt.Sprintf("http://127.0.0.1:%d", zzFree(t)), "da:passa") {
		t.Error("an unreachable server was reported as a refusal")
	}
}

// The port the provisioner assigned is the one used, since it is the only one the server
// allows this device; without one the derived port stands (a self-hoster's own server).
func TestTheAssignedPortIsTheOneUsed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	derived := selfTunnelPort()
	if err := writeSelfTunnelConf("https://direct.example.dev", "d1:p1", 0); err != nil {
		t.Fatal(err)
	}
	if got := selfTunnelPort(); got != derived {
		t.Errorf("no assigned port: got %d, want the derived %d", got, derived)
	}
	assigned := selfPortBase + (derived-selfPortBase+1)%selfPortSpan
	if err := writeSelfTunnelConf("https://direct.example.dev", "d1:p1", assigned); err != nil {
		t.Fatal(err)
	}
	if got := selfTunnelPort(); got != assigned {
		t.Errorf("assigned %d, but the tunnel would use %d", assigned, got)
	}
	if got := selfTunnelPairURL("https://direct.example.dev"); got != fmt.Sprintf("https://direct.example.dev/p%d", assigned) {
		t.Errorf("the pairing URL does not follow the assigned port: %s", got)
	}
	// A port outside the band is ignored, never trusted.
	if err := writeSelfTunnelConf("https://direct.example.dev", "d1:p1", 80); err != nil {
		t.Fatal(err)
	}
	if got := selfTunnelPort(); got != derived {
		t.Errorf("an out-of-band port was used: %d", got)
	}
}

// Redeem stores what the provisioner assigned, and a full code writes nothing.
func TestRedeemStoresTheAssignedPortAndAFullCodeWritesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var status int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["deviceId"] == "" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(status)
		if status == 200 {
			fmt.Fprint(w, `{"url":"https://direct.example.dev","secret":"d1:p1","port":31234}`)
		} else {
			fmt.Fprint(w, `{"error":"this code is already in use on 3 devices, the most it can be"}`)
		}
	}))
	defer api.Close()
	t.Setenv("GTMUX_TUNNEL_API", api.URL)
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", api.URL) // never the real provisioner

	status = 409
	if rc := redeemDirectCode("gtd-0123456789abcdef01234567"); rc == 0 {
		t.Error("a full code redeemed")
	}
	if _, err := os.Stat(selfTunnelConfPath()); err == nil {
		t.Error("a refused redeem wrote a config")
	}
	status = 200
	if rc := redeemDirectCode("gtd-0123456789abcdef01234567"); rc != 0 {
		t.Fatalf("redeem failed: rc=%d", rc)
	}
	if got := readSelfTunnelPort(); got != 31234 {
		t.Errorf("stored port = %d, want 31234", got)
	}
}

// The pre-check is wired in: starting Direct with an account the server refuses returns
// at once with a failure, where the tunnel client alone would retry it forever in silence.
func TestStartingDirectWithARefusedAccountStopsAndSaysSo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server, _ := directServer(t, withSentinel(map[string][]string{"da:passa": {deviceRule(zzFree(t))}}))
	done := make(chan int, 1)
	go func() { done <- runSelfTunnelClient(server, "da:revoked-or-replaced", zzFree(t), nil) }()
	select {
	case rc := <-done:
		if rc == 0 {
			t.Error("a refused account reported success")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("starting Direct with a refused account never returned: it is retrying in silence")
	}
}
