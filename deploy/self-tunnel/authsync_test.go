// Package selftunnel holds no code, only the test that runs gtmux-authsync, the shell
// script a Direct server uses to pull its chisel authfile, against a stubbed provisioner
// in a private directory. Nothing here touches a real server, account or network.
package selftunnel

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const sentinel = "gtmux-sentinel:0123456789abcdef"

// The curl stub writes the canned body to -o and the canned headers to -D, or fails.
const curlStub = `#!/bin/sh
out=; hdr=
while [ $# -gt 0 ]; do
  case "$1" in -o) out="$2"; shift ;; -D) hdr="$2"; shift ;; esac
  shift
done
[ "${STUB_FAIL:-0}" = 1 ] && exit 22
cat "$STUB_BODY" >"$out"
[ -n "$hdr" ] && cat "$STUB_HDR" >"$hdr"
exit 0
`

// The restart stub counts its calls, and fails while RESTART_FAIL=1.
const restartStub = `#!/bin/sh
echo x >>"$RESTART_CALLS"
[ "${RESTART_FAIL:-0}" = 1 ] && exit 7
touch "$RESTART_MARK"
`

// workingJQ finds a jq that runs: a broken one earlier on PATH (a Homebrew jq missing its
// library) is skipped for the system's.
func workingJQ(t *testing.T) string {
	t.Helper()
	cands := []string{}
	if p, err := exec.LookPath("jq"); err == nil {
		cands = append(cands, p)
	}
	cands = append(cands, "/usr/bin/jq", "/opt/homebrew/bin/jq")
	for _, p := range cands {
		if exec.Command(p, "--version").Run() == nil {
			return p
		}
	}
	t.Skip("no working jq; gtmux-authsync needs one")
	return ""
}

type server struct {
	t                 *testing.T
	dir, stubs, jqDir string
}

func device(n string) string { return "u" + n + ":p" + n }

// newServer is a Direct server's /etc/gtmux-tunnel with the given device accounts (plus
// the sentinel) and, when pin is set, the server id recorded at the last complete sync.
func newServer(t *testing.T, devices []string, pin string) *server {
	t.Helper()
	s := &server{t: t, dir: t.TempDir(), stubs: t.TempDir(), jqDir: filepath.Dir(workingJQ(t))}
	write := func(path, body string, mode os.FileMode) {
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(s.dir, "sync.env"), "SYNC_URL=https://provisioner.invalid/direct/authfile\nSYNC_TOKEN=stub\n", 0o600)
	write(filepath.Join(s.dir, "sentinel"), sentinel+"\n", 0o600)
	users := map[string][]string{sentinel: {"^$"}}
	for _, d := range devices {
		users[d] = []string{`^R:127\.0\.0\.1:20001$`}
	}
	b, _ := json.Marshal(users)
	write(filepath.Join(s.dir, "users.json"), string(b), 0o600)
	if pin != "" {
		write(filepath.Join(s.dir, "server-id"), pin+"\n", 0o600)
	}
	write(filepath.Join(s.stubs, "curl"), curlStub, 0o755)
	write(filepath.Join(s.stubs, "restart"), restartStub, 0o755)
	return s
}

type result struct {
	code      int
	out       string
	restarted bool
}

// sync runs the real script once against a provisioner answering body with header (a
// raw header line, or "" for none), and extra environment.
func (s *server) sync(body, header string, env ...string) result {
	s.t.Helper()
	bodyPath := filepath.Join(s.stubs, "body.json")
	hdrPath := filepath.Join(s.stubs, "headers.txt")
	_ = os.WriteFile(bodyPath, []byte(body), 0o600)
	h := "HTTP/2 200\r\ncontent-type: application/json\r\n"
	if header != "" {
		h += header + "\r\n"
	}
	_ = os.WriteFile(hdrPath, []byte(h+"\r\n"), 0o600)
	marker := filepath.Join(s.stubs, "restarted")
	_ = os.Remove(marker)
	me, err := user.Current()
	if err != nil {
		s.t.Fatal(err)
	}
	cmd := exec.Command("sh", "gtmux-authsync")
	cmd.Env = append([]string{
		"PATH=" + s.stubs + ":" + s.jqDir + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"GTMUX_TUNNEL_DIR=" + s.dir,
		"GTMUX_TUNNEL_OWNER=" + me.Username,
		"RESTART_CMD=" + filepath.Join(s.stubs, "restart"),
		"RESTART_MARK=" + marker,
		"RESTART_CALLS=" + filepath.Join(s.stubs, "restart-calls"),
		"STUB_BODY=" + bodyPath,
		"STUB_HDR=" + hdrPath,
	}, env...)
	out, err := cmd.CombinedOutput()
	r := result{out: string(out)}
	if ee, ok := err.(*exec.ExitError); ok {
		r.code = ee.ExitCode()
	} else if err != nil {
		s.t.Fatal(err)
	}
	_, statErr := os.Stat(marker)
	r.restarted = statErr == nil
	return r
}

// devices lists the device accounts in users.json, the sentinel checked and left out.
func (s *server) devices() []string {
	s.t.Helper()
	b, err := os.ReadFile(filepath.Join(s.dir, "users.json"))
	if err != nil {
		s.t.Fatal(err)
	}
	var users map[string][]string
	if err := json.Unmarshal(b, &users); err != nil {
		s.t.Fatal(err)
	}
	if _, ok := users[sentinel]; !ok {
		s.t.Fatalf("users.json lost the sentinel: %s", b)
	}
	out := []string{}
	for k := range users {
		if k != sentinel {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func (s *server) pin() string {
	b, _ := os.ReadFile(filepath.Join(s.dir, "server-id"))
	return strings.TrimSpace(string(b))
}

func body(devices ...string) string {
	m := map[string][]string{}
	for _, d := range devices {
		m[d] = []string{`^R:127\.0\.0\.1:20001$`}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

const claimLA0 = "x-gtmux-authfile: complete; server=la; accounts=0"

// The last account revoked, or the last device moved away: the provisioner's complete,
// empty answer for the server this one has been syncing as removes it, and chisel is
// restarted so the device's live session ends. Before, the empty set was refused and the
// revoked device stayed connected (%12, 2026-10-06).
func TestTheLastAccountIsRemovedWhenTheProvisionerSaysSo(t *testing.T) {
	s := newServer(t, []string{device("1")}, "la")
	r := s.sync(body(), claimLA0)
	if r.code != 0 || len(s.devices()) != 0 || !r.restarted {
		t.Fatalf("complete empty answer: exit %d, devices %v, restarted %v\n%s", r.code, s.devices(), r.restarted, r.out)
	}
	// Synced again, the same answer changes nothing.
	if r := s.sync(body(), claimLA0); r.code != 0 || r.restarted {
		t.Fatalf("repeat: exit %d restarted %v\n%s", r.code, r.restarted, r.out)
	}
}

// Every empty answer the provisioner does not vouch for still keeps the current file:
// that guard is what stops a fault from cutting off every device at once.
func TestAnEmptyAnswerWithoutAValidClaimKeepsTheFile(t *testing.T) {
	const refusal = "refusing to replace 1 device accounts with none"
	cases := []struct {
		name, header, pin, says string
	}{
		{"no header: an older provisioner, or a registry that is missing", "", "la", refusal},
		{"no server recorded yet", claimLA0, "", refusal},
		// Refused before the emptiness is even weighed: another server's answer is never
		// taken (TestAClaimForAnotherServerKeepsTheFileAndThePin).
		{"a different server than last time", "x-gtmux-authfile: complete; server=sh; accounts=0", "la", "pinned to la"},
		{"a claim that is not complete", "x-gtmux-authfile: partial; server=la; accounts=0", "la", refusal},
		{"a malformed server id", "x-gtmux-authfile: complete; server=l a; accounts=0", "la", refusal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newServer(t, []string{device("1")}, c.pin)
			r := s.sync(body(), c.header)
			if r.code == 0 || len(s.devices()) != 1 || r.restarted {
				t.Fatalf("exit %d, devices %v, restarted %v\n%s", r.code, s.devices(), r.restarted, r.out)
			}
			if !strings.Contains(r.out, c.says) {
				t.Fatalf("does not say %q:\n%s", c.says, r.out)
			}
		})
	}
}

// The operator's explicit override still empties a server with no claim at all.
func TestAllowEmptyStillOverrides(t *testing.T) {
	s := newServer(t, []string{device("1")}, "")
	if r := s.sync(body(), "", "ALLOW_EMPTY=1"); r.code != 0 || len(s.devices()) != 0 || !r.restarted {
		t.Fatalf("exit %d devices %v restarted %v\n%s", r.code, s.devices(), r.restarted, r.out)
	}
}

// A complete answer records the server id it was for; one whose count disagrees with
// its body is not trusted at all; a failed or malformed fetch keeps the file.
func TestClaimsAreRecordedCheckedAndFaultsKeepTheFile(t *testing.T) {
	s := newServer(t, []string{device("1"), device("2")}, "")
	r := s.sync(body(device("1")), "X-Gtmux-Authfile: complete; server=la; accounts=1")
	if r.code != 0 || len(s.devices()) != 1 || !r.restarted || s.pin() != "la" {
		t.Fatalf("2 -> 1: exit %d devices %v restarted %v pin %q\n%s", r.code, s.devices(), r.restarted, s.pin(), r.out)
	}
	// Recorded, the next complete empty answer is applied.
	if r := s.sync(body(), claimLA0); r.code != 0 || len(s.devices()) != 0 {
		t.Fatalf("then empty: exit %d devices %v\n%s", r.code, s.devices(), r.out)
	}

	m := newServer(t, []string{device("1"), device("2")}, "la")
	if r := m.sync(body(device("1")), "x-gtmux-authfile: complete; server=la; accounts=2"); r.code == 0 || len(m.devices()) != 2 {
		t.Fatalf("count mismatch applied: exit %d devices %v\n%s", r.code, m.devices(), r.out)
	}
	if r := m.sync("not json", claimLA0); r.code == 0 || len(m.devices()) != 2 {
		t.Fatalf("malformed applied: exit %d devices %v\n%s", r.code, m.devices(), r.out)
	}
	if r := m.sync(body(), claimLA0, "STUB_FAIL=1"); r.code == 0 || len(m.devices()) != 2 {
		t.Fatalf("failed fetch applied: exit %d devices %v\n%s", r.code, m.devices(), r.out)
	}
}

// A claim for another server than the pinned one is a misconfigured list, empty or not.
// A non-empty one used to be applied and to move the pin without a word, so the wrong
// server's next empty answer then removed every account (%6, review of a3cfc497). Now
// nothing is taken, the pin included, until a person removes server-id.
func TestAClaimForAnotherServerKeepsTheFileAndThePin(t *testing.T) {
	s := newServer(t, []string{device("1"), device("2")}, "la")
	r := s.sync(body(device("9")), "X-Gtmux-Authfile: complete; server=sf; accounts=1")
	if r.code == 0 || len(s.devices()) != 2 || r.restarted || s.pin() != "la" {
		t.Fatalf("another server's answer: exit %d devices %v restarted %v pin %q\n%s", r.code, s.devices(), r.restarted, s.pin(), r.out)
	}
	if !strings.Contains(r.out, "pinned to la") {
		t.Errorf("the refusal does not say why:\n%s", r.out)
	}
	if r := s.sync(body(), "X-Gtmux-Authfile: complete; server=sf; accounts=0"); r.code == 0 || len(s.devices()) != 2 || s.pin() != "la" {
		t.Fatalf("its empty answer next: exit %d devices %v pin %q\n%s", r.code, s.devices(), s.pin(), r.out)
	}
	// A deliberate move: the person removes server-id, and the next complete answer is
	// taken and recorded.
	if err := os.Remove(filepath.Join(s.dir, "server-id")); err != nil {
		t.Fatal(err)
	}
	if r := s.sync(body(device("9")), "X-Gtmux-Authfile: complete; server=sf; accounts=1"); r.code != 0 || len(s.devices()) != 1 || s.pin() != "sf" {
		t.Fatalf("after the pin was removed: exit %d devices %v pin %q\n%s", r.code, s.devices(), s.pin(), r.out)
	}
}

// A provisioner without the header still syncs a non-empty set exactly as before.
func TestAnOlderProvisionerStillSyncs(t *testing.T) {
	s := newServer(t, []string{device("1")}, "")
	if r := s.sync(body(device("1"), device("2")), ""); r.code != 0 || len(s.devices()) != 2 || r.restarted || s.pin() != "" {
		t.Fatalf("exit %d devices %v restarted %v pin %q\n%s", r.code, s.devices(), r.restarted, s.pin(), r.out)
	}
}

// A removal is done only when chisel has restarted. A restart that failed used to be
// tried once: the next sync found the file already matching and did nothing, so the
// removed device's session could live on (%12, 2026-10-06). It is now owed until it
// succeeds, and paid at the next sync even when nothing else changed.
func TestAFailedRestartIsRetriedUntilItSucceeds(t *testing.T) {
	s := newServer(t, []string{device("1"), device("2")}, "")
	r := s.sync(body(device("1")), "", "RESTART_FAIL=1")
	if r.code == 0 || len(s.devices()) != 1 || !s.owes() {
		t.Fatalf("failed restart: exit %d devices %v owes %v\n%s", r.code, s.devices(), s.owes(), r.out)
	}
	// Still failing: still owed, and the sync says it failed, even with nothing new.
	if r := s.sync(body(device("1")), "", "RESTART_FAIL=1"); !s.owes() || r.restarted || r.code == 0 {
		t.Fatalf("second failure: exit %d owes %v\n%s", r.code, s.owes(), r.out)
	}
	// Still failing while an account is added: the addition applies, the sync still fails.
	if r := s.sync(body(device("1"), device("3")), "", "RESTART_FAIL=1"); r.code == 0 || len(s.devices()) != 2 || !s.owes() {
		t.Fatalf("addition under a debt: exit %d devices %v owes %v\n%s", r.code, s.devices(), s.owes(), r.out)
	}
	if r := s.sync(body(device("1"), device("3")), ""); r.code != 0 || !r.restarted || s.owes() {
		t.Fatalf("paid after the addition: exit %d restarted %v owes %v\n%s", r.code, r.restarted, s.owes(), r.out)
	}
	// Back to the one device: a removal again, restart working.
	if r := s.sync(body(device("1")), ""); r.code != 0 || !r.restarted || s.owes() {
		t.Fatalf("second removal: exit %d restarted %v owes %v\n%s", r.code, r.restarted, s.owes(), r.out)
	}
	s.sync(body(device("1")), "") // settle
	// A sync that died after swapping the file in, before restarting: the debt was
	// written before the swap, so the next sync of the same set pays it.
	if err := os.WriteFile(filepath.Join(s.dir, "restart-pending"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The same set again, restart working: paid, and the debt is gone.
	if r := s.sync(body(device("1")), ""); r.code != 0 || !r.restarted || s.owes() {
		t.Fatalf("retry: exit %d restarted %v owes %v\n%s", r.code, r.restarted, s.owes(), r.out)
	}
	// Nothing owed and nothing removed: no restart.
	if r := s.sync(body(device("1")), ""); r.restarted {
		t.Fatalf("restarted with nothing owed:\n%s", r.out)
	}
	// A failed fetch does not stop a debt from being paid.
	s2 := newServer(t, []string{device("1"), device("2")}, "")
	s2.sync(body(device("1")), "", "RESTART_FAIL=1")
	if r := s2.sync(body(), "", "STUB_FAIL=1"); !r.restarted || s2.owes() || len(s2.devices()) != 1 {
		t.Fatalf("debt with a failed fetch: restarted %v owes %v devices %v\n%s", r.restarted, s2.owes(), s2.devices(), r.out)
	}
}

func (s *server) owes() bool {
	_, err := os.Stat(filepath.Join(s.dir, "restart-pending"))
	return err == nil
}
