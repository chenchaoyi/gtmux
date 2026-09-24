package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// A choice of Direct servers (openspec/changes/direct-server-choice). The client must
// carry NO list of its own: a server the operator adds after this binary shipped has to be
// selectable here, which is only true if every server it shows came from the provisioner.

// fakeProvisioner answers /direct/servers and /direct/move, and records what it was asked.
func fakeProvisioner(t *testing.T, servers []map[string]any, current string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	// Resolving this Mac's device id reads the config dir, and a test may never touch the
	// real one.
	if os.Getenv("GTMUX_TEST_HOME_SET") == "" {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("GTMUX_TEST_HOME_SET", "1")
	}
	var asked []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["path"] = r.URL.Path
		asked = append(asked, body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/direct/servers":
			_ = json.NewEncoder(w).Encode(map[string]any{"servers": servers, "current": current})
		case "/direct/move":
			want, _ := body["server"].(string)
			for _, s := range servers {
				if s["id"] == want {
					_ = json.NewEncoder(w).Encode(map[string]any{"url": s["url"], "port": 35047})
					return
				}
			}
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "no such server"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("GTMUX_TUNNEL_API", srv.URL)
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", srv.URL) // never the real provisioner
	return srv, &asked
}

func TestServersComeFromTheProvisionerNotTheBinary(t *testing.T) {
	// A server the operator added after this binary was built.
	_, asked := fakeProvisioner(t, []map[string]any{
		{"id": "newly-added", "url": "https://new.example.test", "region": "cn-shanghai",
			"label": map[string]string{"en": "Shanghai", "zh": "上海"}},
	}, "newly-added")
	servers, current, err := fetchDirectServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].ID != "newly-added" {
		t.Fatalf("got %+v, want the server the provisioner listed", servers)
	}
	if current != "newly-added" {
		t.Fatalf("current = %q, want the provisioner's answer", current)
	}
	if len(*asked) != 1 || (*asked)[0]["deviceId"] == "" {
		t.Fatalf("the ask carried no device id: %+v", *asked)
	}
}

func TestAServerIsNamedInTheReadersLanguage(t *testing.T) {
	var s directServer
	s.ID = "sh"
	s.Region = "cn-shanghai"
	s.Label.EN, s.Label.ZH = "Shanghai", "上海"
	i18n.SetLang("zh")
	if got := s.name(); got != "上海" {
		t.Fatalf("zh name = %q", got)
	}
	i18n.SetLang("en")
	if got := s.name(); got != "Shanghai" {
		t.Fatalf("en name = %q", got)
	}
	// A server added with no label still has to render: region, then id.
	var bare directServer
	bare.ID = "sh"
	bare.Region = "cn-shanghai"
	if got := bare.name(); got != "cn-shanghai" {
		t.Fatalf("unlabelled name = %q, want the region", got)
	}
	bare.Region = ""
	if got := bare.name(); got != "sh" {
		t.Fatalf("bare name = %q, want the id", got)
	}
}

func TestPingSeparatesAnsweringFromNot(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != directPingPath {
			t.Errorf("pinged %q, want %q", r.URL.Path, directPingPath)
		}
		w.WriteHeader(204)
	}))
	defer ok.Close()
	if _, up := pingDirect(ok.URL); !up {
		t.Fatal("a server that answers read as down")
	}
	// A server installed before this path existed answers 404, and it is up: only a
	// server that says nothing at all is down.
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }))
	defer old.Close()
	if _, up := pingDirect(old.URL); !up {
		t.Fatal("a server that answered 404 read as down")
	}
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := dead.URL
	dead.Close()
	if _, up := pingDirect(addr); up {
		t.Fatal("a server that is not there read as up")
	}
}

func TestPingAllTimesEveryServerIncludingOneThatIsDown(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer up.Close()
	down := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	downAddr := down.URL
	down.Close()

	got := pingAll([]directServer{{ID: "up", URL: up.URL}, {ID: "down", URL: downAddr}})
	if _, ok := got["up"]; !ok {
		t.Fatal("the server that answers is missing from the timings")
	}
	if _, ok := got["down"]; ok {
		t.Fatal("a server that never answered was given a round trip")
	}
}

// A move goes through the provisioner with the account this Mac holds, and what comes back
// is written to the config: the next tunnel start dials the new server, nothing else moves.
func TestMoveWritesTheNewServerAndKeepsTheAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_TEST_HOME_SET", "1")
	_, asked := fakeProvisioner(t, []map[string]any{
		{"id": "la", "url": "https://la.example.test"},
		{"id": "sh", "url": "https://sh.example.test"},
	}, "la")
	if err := writeSelfTunnelConf("https://la.example.test", "d1:p1", 35047, "la"); err != nil {
		t.Fatal(err)
	}

	url, port, err := moveDirect("sh", "d1:p1")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://sh.example.test" || port != 35047 {
		t.Fatalf("move gave %q port %d", url, port)
	}
	var move map[string]any
	for _, a := range *asked {
		if a["path"] == "/direct/move" {
			move = a
		}
	}
	if move["secret"] != "d1:p1" {
		t.Fatalf("the move did not carry this Mac's own account: %+v", move)
	}

	if err := writeSelfTunnelConf(url, "d1:p1", port, "sh"); err != nil {
		t.Fatal(err)
	}
	if got := readSelfTunnelServer(); got != "sh" {
		t.Fatalf("server= is %q after the move", got)
	}
	gotURL, gotSecret := readSelfTunnelConf()
	if gotURL != "https://sh.example.test" || gotSecret != "d1:p1" {
		t.Fatalf("config after the move: url=%q secret=%q", gotURL, gotSecret)
	}
	if got := readSelfTunnelPort(); got != 35047 {
		t.Fatalf("port after the move = %d, want the one this device keeps", got)
	}
	b, err := os.ReadFile(selfTunnelConfPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "server=sh") {
		t.Fatalf("config does not record the server:\n%s", b)
	}
}

func TestMovingToAServerThatIsNotThereIsItsOwnAnswer(t *testing.T) {
	fakeProvisioner(t, []map[string]any{{"id": "la", "url": "https://la.example.test"}}, "la")
	if _, _, err := moveDirect("nope", "d1:p1"); err != errNoSuchServer {
		t.Fatalf("err = %v, want errNoSuchServer (a typo is not a network problem)", err)
	}
}

// A config written before servers were a thing has no server= line. It is not broken: it is
// the one server the deployment had, and the client must read it that way.
func TestAConfigFromBeforeTheServerListReadsAsTheOneServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSelfTunnelConf("https://tunnel.example.test", "d1:p1", 35047, ""); err != nil {
		t.Fatal(err)
	}
	if got := readSelfTunnelServer(); got != "" {
		t.Fatalf("server = %q, want empty (the implicit single server)", got)
	}
	if url, secret := readSelfTunnelConf(); url == "" || secret == "" {
		t.Fatal("an older config stopped being readable")
	}
}

// The installer downloads the transport binary, and on a box that cannot reach GitHub it
// downloads it from a mirror. That is only safe because the pinned checksum is verified
// before anything is installed, so the guard is exercised here at command level: a source
// that serves the wrong bytes must be refused, not installed.
func TestDownloadsAreRefusedUnlessTheChecksumMatches(t *testing.T) {
	script := filepath.Join("..", "..", "deploy", "self-tunnel", "verify-download.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("verify-download.sh not present")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if out, err := exec.Command("bash", script, file, helloSHA).CombinedOutput(); err != nil {
		t.Fatalf("the real file was refused: %v\n%s", err, out)
	}
	// What a bad mirror looks like: the right name, different bytes.
	if out, err := exec.Command("bash", script, file, strings.Repeat("0", 64)).CombinedOutput(); err == nil {
		t.Fatalf("a file that does not match the pinned checksum was accepted:\n%s", out)
	}
	if out, err := exec.Command("bash", script, filepath.Join(dir, "nothing.bin"), helloSHA).CombinedOutput(); err == nil {
		t.Fatalf("a download that never arrived was accepted:\n%s", out)
	}
}

// A gtmux newer than the provisioner it talks to must not look broken: a provisioner with
// no server list answers 404 here, and that is the deployment every Direct user had until
// this change — one server, the one they are on.
func TestAProvisionerWithNoServerListIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_TEST_HOME_SET", "1")
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer old.Close()
	t.Setenv("GTMUX_TUNNEL_API", old.URL)
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", old.URL)
	if err := writeSelfTunnelConf("https://tunnel.example.test", "d1:p1", 35047, ""); err != nil {
		t.Fatal(err)
	}

	servers, current, err := fetchDirectServers()
	if err != nil {
		t.Fatalf("an older provisioner read as a failure: %v", err)
	}
	if len(servers) != 1 || servers[0].URL != "https://tunnel.example.test" {
		t.Fatalf("servers = %+v, want the one this Mac is configured for", servers)
	}
	if servers[0].ID != "default" || current != "" {
		t.Fatalf("id = %q current = %q", servers[0].ID, current)
	}
}

// The menu bar renders this JSON (it is a consumer of the CLI, never a second
// implementation), and a Swift Codable struct decodes it. A key that changes name here
// silently empties that panel, so the shape is pinned on this side too.
func TestTheServerListJSONKeepsItsShape(t *testing.T) {
	accepting := false
	servers := []directServer{{ID: "sh", URL: "https://sh.example.test", Region: "cn-shanghai"}}
	servers[0].Label.EN, servers[0].Label.ZH = "Shanghai", "上海"
	servers = append(servers, directServer{ID: "la", URL: "https://la.example.test", Accepting: &accepting})

	out := captureStdout(t, func() {
		printDirectServersJSON(servers, map[string]time.Duration{"sh": 24 * time.Millisecond}, "sh")
	})
	var reply struct {
		Servers []map[string]any `json:"servers"`
		Current string           `json:"current"`
	}
	if err := json.Unmarshal([]byte(out), &reply); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if reply.Current != "sh" || len(reply.Servers) != 2 {
		t.Fatalf("reply = %+v", reply)
	}
	sh, la := reply.Servers[0], reply.Servers[1]
	for _, k := range []string{"id", "url", "name", "current", "accepting", "answering"} {
		if _, ok := sh[k]; !ok {
			t.Fatalf("a server lost its %q key: %+v", k, sh)
		}
	}
	if sh["rtt_ms"] != float64(24) || sh["current"] != true || sh["answering"] != true {
		t.Fatalf("the server in use: %+v", sh)
	}
	if sh["name"] != "Shanghai" && sh["name"] != "上海" {
		t.Fatalf("name = %v, want the label in the reader's language", sh["name"])
	}
	// A server that did not answer carries NO round trip: a zero would draw as instant.
	if _, ok := la["rtt_ms"]; ok {
		t.Fatalf("a server that never answered was given a round trip: %+v", la)
	}
	if la["answering"] != false || la["accepting"] != false {
		t.Fatalf("a closed, silent server: %+v", la)
	}
}

// Routes exist only while this Mac IS on Direct (openspec/changes/phone-moves-the-route).
// On the standard tunnel or a LAN address there is nothing to choose, and the phone must
// not be shown a list of places it cannot be sent to — the MAC answers that, so no surface
// has to guess.
func TestNoRoutesUnlessThisMacIsOnDirect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_TEST_HOME_SET", "1")
	fakeProvisioner(t, []map[string]any{
		{"id": "sh", "url": "https://sh.example.test"},
		{"id": "la", "url": "https://la.example.test"},
	}, "sh")
	if err := writeSelfTunnelConf("https://sh.example.test", "d1:p1", 35047, "sh"); err != nil {
		t.Fatal(err)
	}

	// The standard tunnel: its address belongs to no Direct server.
	writeTunnelURL("https://gtmux-7a3f.example.dev")
	if got, err := directRoutesForServe(); err != nil || len(got) != 0 {
		t.Fatalf("on the standard tunnel: routes = %+v err = %v, want none", got, err)
	}

	// No tunnel at all (a LAN pairing, or nothing running).
	removeTunnelURL()
	if got, _ := directRoutesForServe(); len(got) != 0 {
		t.Fatalf("with no tunnel: routes = %+v, want none", got)
	}

	// On Direct: every route, and the one in use marked.
	writeTunnelURL("https://sh.example.test/p35047")
	got, err := directRoutesForServe()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("on Direct: routes = %+v, want both", got)
	}
	var current int
	for _, r := range got {
		if r.Current {
			current++
		}
		if r.URL == "" {
			t.Fatalf("a route with no address: %+v", r)
		}
	}
	if current != 1 {
		t.Fatalf("%d routes marked as in use, want exactly 1", current)
	}
}
