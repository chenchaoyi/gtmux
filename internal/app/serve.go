package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

const defaultServePort = 8765

// cmdServe implements `gtmux serve` — a local, read-only HTTP server that
// exposes the agent radar to the remote mobile app over a VPN/tunnel.
//
// It binds an intranet/VPN interface (default 0.0.0.0 so the phone can reach the
// Mac's internal IP), guards every /api/* route with a Bearer token, and serves
// ONLY read-only data plus a local "focus" (no input injection / no RCE).
func cmdServe(args []string) int {
	port := defaultServePort
	bind := "0.0.0.0"
	token := ""
	relayURL := ""
	relayToken := ""
	service := false
	unservice := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, bool) {
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		switch {
		case a == "-h" || a == "--help":
			usage()
			return 0
		case a == "--port", a == "-p":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux serve: invalid --port", "gtmux serve: 无效的 --port")
				return 2
			}
			port = n
		case strings.HasPrefix(a, "--port="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--port="))
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux serve: invalid --port", "gtmux serve: 无效的 --port")
				return 2
			}
			port = n
		case a == "--bind":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			bind = v
		case strings.HasPrefix(a, "--bind="):
			bind = strings.TrimPrefix(a, "--bind=")
		case a == "--token":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			token = v
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		case a == "--relay-url":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			relayURL = v
		case strings.HasPrefix(a, "--relay-url="):
			relayURL = strings.TrimPrefix(a, "--relay-url=")
		case a == "--relay-token":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			relayToken = v
		case strings.HasPrefix(a, "--relay-token="):
			relayToken = strings.TrimPrefix(a, "--relay-token=")
		case a == "--service":
			service = true
		case a == "--unservice":
			unservice = true
		default:
			i18n.Sae("gtmux serve: unknown option '"+a+"'", "gtmux serve: 未知选项 '"+a+"'")
			return 2
		}
	}

	// LAN access as a managed launchd agent — the free "same Wi-Fi" remote mode,
	// the counterpart to the always-on tunnel (`gtmux tunnel --service`).
	if unservice {
		return serviceRemoveAll()
	}
	if service {
		return serveServiceInstall(port)
	}

	token = resolveServeToken(token)
	srv := newServeServer(bind, port, token, relayURL, relayToken)
	printServeBanner(bind, port, token, srv.MintEnroll())
	if err := srv.ListenAndServe(); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	return 0
}

// newServeServer builds the read-only radar HTTP server (shared by `gtmux serve`
// and `gtmux tunnel`, which starts it in-process when one isn't already up).
func newServeServer(bind string, port int, token, relayURL, relayToken string) *server.Server {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	deps := server.Deps{
		AgentsJSON: func() ([]byte, error) {
			if !tmux.ServerUp() { // no tmux → empty array, same as `agents --json`
				return []byte("[]"), nil
			}
			return radar.AgentsJSONBytes()
		},
		PaneText: func(id string) (string, bool) {
			if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
				return "", false
			}
			return tmux.CapturePaneColor(id), true
		},
		// The supervisor's fleet view (goal/last/ask per agent) — same bytes as
		// `gtmux digest --json`; includes native rows even with no tmux server.
		DigestJSON: radar.DigestJSONBytes,
		// usage-watch: token usage + threshold warnings, same bytes as the CLI.
		UsageJSON: radar.UsageJSONBytes,
		// resource-watch + limits-watch: the SINGLE-WRITER warn evaluator (no race).
		// Also backstops the tmux-resurrect save: if continuum's autosave is disarmed,
		// gtmux keeps the save fresh itself (a no-op when the save is already recent).
		OnSlowTick: func() { hq.SlowTickEval(); maybeBackstopSave() },
		// The HQ nudge drain's backstop: a knock queued behind a half-typed draft
		// lands within seconds of the box clearing, not on the sampling cadence.
		OnFastTick: hq.DrainHQNudges,
		// The approval card's options are gated on the hook waiting marker, not screen
		// text (an idle pane showing a numbered list must not surface an approval menu).
		IsWaiting:  func(id string) bool { return state.Exists(state.WaitingPath(id)) },
		PaneCursor: paneCursor,
		// attach-predictive-echo: the pane's cursor CELL + alt-screen flag, streamed as
		// OpCursor frames so the attach client has the authoritative cursor without
		// emulating a terminal. `alternate_on` is the precise full-screen-TUI signal.
		AttachCursor: attachCursor,
		Focus:        func(id string) error { return panefocus.FocusPaneByID(id) },
		Send:         sendToPane,
		// `gtmux attach` bridges a tmux client (spawned in a server-side PTY) to a WS.
		// Resolve the pane's session and attach to it; the handler drops write frames
		// for a read-only (view-only guest) pane. attach-session (not new-session) keeps
		// it leak-free — the multi-client size trade-off is a documented follow-up.
		AttachCommand: func(paneID string) ([]string, bool) {
			if tmux.Bin == "" {
				return nil, false
			}
			session := tmux.Display(paneID, "#{session_name}")
			if session == "" {
				return nil, false
			}
			// `-u` forces UTF-8 so CJK (and other wide chars) render instead of being
			// substituted with placeholder dashes — the serve's launchd env has no
			// UTF-8 locale (see the LC_CTYPE the attach handler also sets). Same fix as
			// internal/tmux uses for the radar.
			return []string{tmux.Bin, "-u", "attach-session", "-t", session}, true
		},
		Upload:     saveUpload,
		Icon:       agentIconPNG,
		Diff:       diffForPane,
		Transcript: transcriptForPane,
		Theme:      terminal.Appearance,
		OnClients:  writeRemoteClients,
		AgentStatuses: func() []server.AgentStatus {
			if !tmux.ServerUp() {
				return nil
			}
			panes := radar.GatherAgents()
			out := make([]server.AgentStatus, 0, len(panes))
			for _, p := range panes {
				out = append(out, server.AgentStatus{
					PaneID: p.PaneID, Agent: p.Agent, Loc: p.Loc, Task: p.Task, Status: p.Status,
					Since: p.Since, Role: p.Role(),
				})
			}
			return out
		},
	}

	// Push: tokens live here (the relay stays stateless); alerts are forwarded
	// out to the relay, which holds the APNs key. With no explicit --relay-url
	// (e.g. the serve that `gtmux tunnel` / always-on starts), fall back to the
	// hosted relay so notifications work by default.
	if relayURL == "" {
		relayURL, relayToken = resolveRelay()
	}
	relay := server.NewHTTPRelay(relayURL, relayToken)
	deps.Push = server.NewPushManager(relay, loadPushTokens(), savePushTokens, serverDisplayName(),
		func(a server.Alert) (string, string) { return pushCopy(a, deps.PaneText) })

	// Per-device enrollment: a phone pairs via a short-lived code for its own
	// token (revocable, and the QR stops being a lasting credential). The master
	// token keeps working, so existing pairings are unaffected.
	deps.Enroll = server.NewEnrollManager(loadDevices(), saveDevices)

	// Shared-input policy (web-shared-input): the host's consent + per-pane allowlist
	// that gates a GUEST share link's input. Default off; persisted like the roster.
	deps.Share = server.NewShareManager(loadShareState(), saveShareState)

	// pair-share-model: per-link guest scope. Legacy links (minted before per-link
	// scope) get a ONE-TIME copy of the global lists so upgrade preserves behavior;
	// the legacy global mutations keep their meaning by fanning out to every link.
	st := deps.Share.State()
	deps.Enroll.MigrateGuestScopes(st.ViewPanes, st.Panes)
	deps.Share.OnBroadcast(deps.Enroll.BroadcastGuestScopes)

	return server.New(server.Config{Addr: addr, Token: token}, deps)
}

func serveUsageErr() int {
	i18n.Sae("usage: gtmux serve [--port N] [--bind ADDR] [--token TOKEN] [--service|--unservice] [--relay-url URL] [--relay-token TOKEN]",
		"用法：gtmux serve [--port N] [--bind ADDR] [--token TOKEN] [--service|--unservice] [--relay-url URL] [--relay-token TOKEN]")
	return 2
}

// paneCursor returns the pane's text cursor for /api/pane: column x (0-based), Up =
// rows above the last captured line (pane_height-1-cursor_y, so it's anchored to the
// bottom and survives the phone having fewer rows than the Mac pane), and whether
// the pane's cursor is visible (hidden by alt-screen TUIs).
// attachCursor reports the pane's cursor CELL (x,y) plus whether it is on the ALTERNATE
// screen — the attach bridge streams this as OpCursor so the client gets the
// authoritative cursor without a terminal emulator (attach-predictive-echo). `alt` is
// what a cursor-visible flag can't tell us: vim shows a cursor, but predicting inside it
// is unsafe.
func attachCursor(id string) (x, y int, alt, ok bool) {
	if tmux.Bin == "" {
		return 0, 0, false, false
	}
	f := strings.Fields(tmux.Display(id, "#{cursor_x} #{cursor_y} #{alternate_on}"))
	if len(f) < 3 {
		return 0, 0, false, false
	}
	cx, err1 := strconv.Atoi(f[0])
	cy, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil || cx < 0 || cy < 0 {
		return 0, 0, false, false
	}
	return cx, cy, f[2] == "1", true
}

func paneCursor(id string) (x, up int, visible, ok bool) {
	if tmux.Bin == "" {
		return 0, 0, false, false
	}
	f := strings.Fields(tmux.Display(id, "#{cursor_x} #{cursor_y} #{pane_height} #{cursor_flag}"))
	return cursorFromFields(f)
}

// cursorFromFields is the pure half of paneCursor: it turns tmux's
// "cursor_x cursor_y pane_height cursor_flag" fields into the bottom-anchored
// cursor (Up = pane_height-1-cursor_y, clamped at 0). Split out so the anchoring
// math is unit-testable without a running tmux.
func cursorFromFields(f []string) (x, up int, visible, ok bool) {
	if len(f) != 4 {
		return 0, 0, false, false
	}
	cx, e1 := strconv.Atoi(f[0])
	cy, e2 := strconv.Atoi(f[1])
	h, e3 := strconv.Atoi(f[2])
	if e1 != nil || e2 != nil || e3 != nil || h <= 0 {
		return 0, 0, false, false
	}
	if up = h - 1 - cy; up < 0 {
		up = 0
	}
	return cx, up, f[3] == "1", true
}

// pushTokensPath is where registered device push tokens persist across restarts.
func pushTokensPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "push-tokens.json")
}

func loadPushTokens() []server.DeviceToken {
	b, err := os.ReadFile(pushTokensPath())
	if err != nil {
		return nil
	}
	var out []server.DeviceToken
	_ = json.Unmarshal(b, &out)
	return out
}

func savePushTokens(toks []server.DeviceToken) {
	path := pushTokensPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(toks); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
}

// devicesPath is where the enrolled-device roster persists (per-device tokens).
func devicesPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "devices.json")
}

func sharePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "share.json")
}

func loadShareState() server.ShareState {
	var st server.ShareState
	if b, err := os.ReadFile(sharePath()); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func saveShareState(st server.ShareState) {
	path := sharePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(st); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
}

func loadDevices() []server.EnrolledDevice {
	b, err := os.ReadFile(devicesPath())
	if err != nil {
		return nil
	}
	var out []server.EnrolledDevice
	_ = json.Unmarshal(b, &out)
	return out
}

func saveDevices(d []server.EnrolledDevice) {
	path := devicesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(d); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
}

// pushCopy renders an alert into a bilingual notification title/body
// (en/zh via GTMUX_LANG). The body is the agent's current task.
// pushCopy builds the notification (title, body). The TITLE is the agent's session
// name (its task/title — the bold line in the app/popover), matching the macOS
// banner, instead of "<Agent> needs you"; the state (needs you / finished) is the
// body. Falls back to the locator then a generic word when the task is empty.
func pushCopy(a server.Alert, paneText func(string) (string, bool)) (string, string) {
	title := a.Task
	if title == "" {
		title = a.Loc
	}
	if title == "" {
		title = i18n.Tr("agent", "agent")
	}
	if a.Kind == "waiting" {
		// Body = this session's ACTUAL choices, so expanding the notification shows
		// what you're being asked (not a generic "needs you"). Falls back to the
		// plain phrase when the pane has no parseable menu.
		body := optionsBody(a.Pane, paneText)
		switch {
		case body != "" && a.Repeat:
			return title, i18n.Tr("Still waiting · ", "仍在等待 · ") + body
		case body != "":
			return title, body
		case a.Repeat:
			return title, i18n.Tr("Still needs you", "仍在等你输入")
		default:
			return title, i18n.Tr("Needs you", "等你输入")
		}
	}
	return title, i18n.Tr("Finished", "已完成")
}

// optionsBody formats a waiting pane's 1/2/3 choices as "1. Yes  2. Always  3. No"
// for the notification body — the same parser the reply UI uses. "" when none.
func optionsBody(pane string, paneText func(string) (string, bool)) string {
	if pane == "" || paneText == nil {
		return ""
	}
	text, ok := paneText(pane)
	if !ok {
		return ""
	}
	opts := prompt.ParseOptions(text)
	if len(opts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(opts))
	for _, o := range opts {
		parts = append(parts, fmt.Sprintf("%d. %s", o.N, o.Label))
	}
	return strings.Join(parts, "   ")
}

// resolveServeToken returns the explicit flag token, or a persistent one from
// ~/.config/gtmux/serve-token (generating + writing it 0600 on first run).
func resolveServeToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	path := filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "serve-token")
	if b, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(b)); tok != "" {
			return tok
		}
	}
	tok := randToken()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(tok+"\n"), 0o600)
	}
	return tok
}

// randToken returns 16 random bytes hex-encoded (128 bits).
func randToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is catastrophic; fail closed with a marker the
		// operator will notice rather than a silent weak token.
		return "INSECURE-RANDOM-FAILED"
	}
	return hex.EncodeToString(b)
}

// allowedSendKeys whitelists the named control keys POST /api/send accepts (a
// tmux key name must never be free-form — the rest goes through as literal text).
var allowedSendKeys = map[string]bool{
	"Enter": true, "C-c": true, "Escape": true, "Tab": true,
	"Up": true, "Down": true, "Left": true, "Right": true,
	"BSpace": true, "C-d": true, "C-z": true, "C-l": true,
}

// maxDiffBytes caps the diff payload so a huge working tree can't bloat the phone
// (the rest is the agent's recent edits; a 400 KB head is plenty to review).
const maxDiffBytes = 400 << 10

// diffForPane returns a unified `git diff` (working tree vs HEAD) plus a list of
// untracked files for the pane's current working directory — "what did the agent
// change". Returns "" (no error) when the cwd isn't a git repo. Read-only; git
// runs with color disabled so the phone renders the raw unified diff.
func diffForPane(id string) (string, error) {
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return "", fmt.Errorf("pane not found")
	}
	cwd := tmux.Display(id, "#{pane_current_path}")
	if cwd == "" {
		return "", fmt.Errorf("pane not found")
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", cwd, "-c", "color.ui=never"}, args...)...)
		out, _ := c.Output()
		return string(out)
	}
	// Not a git repo → empty diff (the app shows a friendly note, not an error).
	c := exec.Command("git", "-C", cwd, "rev-parse", "--is-inside-work-tree")
	if c.Run() != nil {
		return "", nil
	}
	var b strings.Builder
	if branch := strings.TrimSpace(git("rev-parse", "--abbrev-ref", "HEAD")); branch != "" {
		fmt.Fprintf(&b, "# branch %s\n", branch)
	}
	b.WriteString(git("diff", "HEAD"))
	if u := strings.TrimSpace(git("ls-files", "--others", "--exclude-standard")); u != "" {
		b.WriteString("\n# untracked:\n")
		for _, f := range strings.Split(u, "\n") {
			fmt.Fprintf(&b, "+ %s\n", f)
		}
	}
	out := b.String()
	if len(out) > maxDiffBytes {
		out = out[:maxDiffBytes] + "\n… (diff truncated)\n"
	}
	return out, nil
}

// serverDisplayName is this Mac's human name, shown as the push notification's
// subtitle so a phone paired to several Macs can tell WHICH one an alert came from.
// Prefers the macOS ComputerName (matches the pairing QR's Host.localizedName —
// e.g. "Chaoyi's MacBook Pro"); falls back to the network hostname, then "Mac".
// cgo-free (shells out to scutil).
func serverDisplayName() string {
	if out, err := exec.Command("scutil", "--get", "ComputerName").Output(); err == nil {
		if n := strings.TrimSpace(string(out)); n != "" {
			return n
		}
	}
	if h, err := os.Hostname(); err == nil {
		if n := strings.TrimSpace(strings.TrimSuffix(h, ".local")); n != "" {
			return n
		}
	}
	return "Mac"
}

// writeRemoteClients records the live remote-viewer roster + count + timestamp so
// the menu-bar app can show WHO is connected (paired phones by name; browsers as
// anonymous "Safari · macOS"-style entries). Written on every SSE connect/disconnect
// and heartbeated while clients are connected, so a dead serve's file goes stale and
// the app treats it as disconnected. `count` is kept for older readers. Best-effort.
func writeRemoteClients(clients []server.ClientInfo) {
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return
	}
	if clients == nil {
		clients = []server.ClientInfo{}
	}
	b, err := json.Marshal(struct {
		Clients []server.ClientInfo `json:"clients"`
		Count   int                 `json:"count"`
		At      int64               `json:"at"`
	}{Clients: clients, Count: len(clients), At: time.Now().Unix()})
	if err != nil {
		return
	}
	_ = os.WriteFile(state.RemoteClientsPath(), b, 0o644)
}

// maxTranscriptTurns bounds the chat-history payload sent to the phone. Set high
// so the chat view shows as much as the terminal scrollback would — the parser
// tail-reads the log (8 MiB window), so this is the effective ceiling.
const maxTranscriptTurns = 300

// transcriptForPane returns the pane's parsed chat history as a JSON array of
// turns for GET /api/transcript. It maps the pane → its resume record (agent +
// session id, captured by the hooks) → the agent's on-disk conversation log.
// Always returns a valid JSON array — "[]" when the pane has no resumable
// session or the agent's log can't be found — never a hard error for those.
func transcriptForPane(id string) ([]byte, error) {
	empty := []byte("[]")
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return empty, nil
	}
	loc := tmux.Display(id, "#{session_name}:#{window_index}.#{pane_index}")
	if loc == "" {
		return empty, nil
	}
	rec, ok := resume.Load(loc)
	if !ok {
		return empty, nil
	}
	turns, err := transcript.Load(rec.Agent, rec.SessionID, maxTranscriptTurns)
	if err != nil || len(turns) == 0 {
		return empty, nil
	}
	b, err := json.Marshal(turns)
	if err != nil {
		return empty, nil
	}
	return b, nil
}

// sendToPane types into a pane for POST /api/send (a WRITE). A non-empty key must
// be in the allowlist; otherwise the text is pasted (+ Enter when enter).
//
// The API stays FAST — it does not verify the LANDING the way the CLI's default send
// does — but for a text+Enter reply it does confirm the DRAFT before submitting:
// dispatch.PasteAndSubmit pastes, waits (bounded) for the full payload to render in
// the input box, THEN sends Enter. A blind paste-then-Enter raced the composer and
// submitted a multi-line reply truncated (or left an unterminated paste that ate the
// Enter). The pre-submit confirm removes that race without the CLI's post-submit
// verify budget, so the phone path stays fast (a healthy paste confirms in a frame).
func sendToPane(id, text, key string, enter bool) error {
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return fmt.Errorf("pane not found")
	}
	// A pane in copy/view-mode swallows input as mode-nav commands; drop out first so
	// the phone's key/text actually reaches the agent (matches the CLI send path).
	_ = tmux.ExitCopyMode(id)
	if key != "" {
		if !allowedSendKeys[key] {
			return fmt.Errorf("key not allowed")
		}
		return tmux.SendKey(id, key)
	}
	if text != "" && enter {
		// Confirm-then-submit: never race Enter against a still-rendering paste.
		dispatch.PasteAndSubmit(dispatchbridge.DispatchIO(id), dispatch.Opts{Pane: id, PasteRetries: 2}, text)
		return nil
	}
	if text != "" {
		// A SINGLE-LINE send with no submit is a KEYSTROKE, not a paste. An agent's
		// numbered menu (Claude's "❯ 1. Yes …") commits on the digit KEYPRESS; a
		// bracketed paste of "1" is inserted as literal text and never selects the option
		// — the "tapping a number in the approval card does nothing" regression, from when
		// text delivery switched to the paste buffer. Only MULTI-LINE text needs the paste
		// buffer (bracketed, so its newlines don't submit line-by-line).
		if keystrokeText(text) {
			if err := tmux.SendText(id, text, false); err != nil { // send-keys -l
				return err
			}
		} else if err := tmux.Paste(id, text); err != nil {
			return err
		}
	}
	if enter {
		return tmux.SendKey(id, "Enter")
	}
	return nil
}

// keystrokeText reports whether text should be delivered as literal KEYSTROKES
// (`send-keys -l`) rather than the bracketed paste buffer: a non-empty SINGLE line.
// Keystrokes are what an agent's numbered menu commits on (a bracketed paste of a
// digit selects nothing); the paste buffer is only needed for MULTI-LINE text, so its
// newlines don't reach the TUI as bare Returns that submit each line separately.
func keystrokeText(text string) bool {
	return text != "" && !strings.Contains(text, "\n")
}

// saveUpload writes an uploaded file under ~/.local/share/gtmux/uploads with a
// random prefix (no collisions / overwrites) and returns its path, so the phone
// can hand a photo/file to an agent by path. Read by whoever the agent can read.
func saveUpload(name string, data []byte) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		safe = "upload"
	}
	path := filepath.Join(dir, randToken()[:8]+"-"+safe)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// sanitizeFilename keeps just the base name with a conservative charset, so an
// uploaded name can't escape the uploads dir or inject anything.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.TrimLeft(b.String(), ".") // no leading dots
	if len(out) > 80 {
		out = out[len(out)-80:]
	}
	return out
}

// agentIconPNG returns a PNG of the agent's identity icon, extracted from its
// installed .app via sips (cached by app mtime), or nil. Read-only; uses the
// user's installed app — nothing third-party is bundled (DESIGN §6).
func agentIconPNG(name string) []byte {
	hint := radar.IconFor(name, radar.LoadProfiles())
	if hint == "" {
		return nil
	}
	if !strings.HasSuffix(hint, ".app") {
		b, _ := os.ReadFile(hint) // a direct image path
		return b
	}
	cacheDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "icon-cache")
	_ = os.MkdirAll(cacheDir, 0o755)
	cache := filepath.Join(cacheDir, sanitizeFilename(name)+"-"+strconv.FormatInt(radar.FileMtime(hint), 10)+".png")
	if b, err := os.ReadFile(cache); err == nil {
		return b
	}
	icns := appIcns(hint)
	if icns == "" {
		return nil
	}
	if exec.Command("sips", "-s", "format", "png", "-Z", "64", icns, "--out", cache).Run() != nil {
		return nil
	}
	b, _ := os.ReadFile(cache)
	return b
}

// appIcns finds a .app's .icns icon (CFBundleIconFile, else the first *.icns).
func appIcns(app string) string {
	res := filepath.Join(app, "Contents", "Resources")
	if out, err := exec.Command("defaults", "read", filepath.Join(app, "Contents", "Info"), "CFBundleIconFile").Output(); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			if !strings.HasSuffix(name, ".icns") {
				name += ".icns"
			}
			if p := filepath.Join(res, name); fileExists(p) {
				return p
			}
		}
	}
	if m, _ := filepath.Glob(filepath.Join(res, "*.icns")); len(m) > 0 {
		return m[0]
	}
	return ""
}

// printServeBanner tells the user where to point the phone and the token to use.
func printServeBanner(bind string, port int, token, pairCode string) {
	i18n.Say("gtmux serve — read-only remote radar (keep this behind a VPN/tunnel)",
		"gtmux serve — 只读远程雷达（请放在 VPN/隧道之后）")
	fmt.Printf("  token: %s\n", token)
	hosts := reachableHosts(bind)
	for _, host := range hosts {
		fmt.Printf("  http://%s/api/agents\n", net.JoinHostPort(host, strconv.Itoa(port)))
	}
	// Browser mirror: open the web UI on another Mac / your computer. The pairing
	// link carries a short-lived single-use code (NOT the master token).
	i18n.Say("  open in a browser (view-only mirror):", "  在浏览器里打开（只读镜像）：")
	for _, host := range hosts {
		fmt.Printf("    http://%s/\n", net.JoinHostPort(host, strconv.Itoa(port)))
	}
	if pairCode != "" && len(hosts) > 0 {
		base := net.JoinHostPort(hosts[0], strconv.Itoa(port))
		i18n.Say("  one-time pairing link (expires in 5 min — open it promptly):",
			"  一次性配对链接（5 分钟内有效，请尽快打开）：")
		fmt.Printf("    http://%s/#c=%s\n", base, pairCode)
	}
	i18n.Say("  test: curl -H \"Authorization: Bearer <token>\" <url>",
		"  自测：curl -H \"Authorization: Bearer <token>\" <url>")
}

// reachableHosts lists candidate hosts to advertise. A specific --bind is
// returned as-is; a wildcard bind expands to the non-loopback IPv4 interface
// addresses (the intranet/VPN IPs the phone routes to).
func reachableHosts(bind string) []string {
	if bind != "" && bind != "0.0.0.0" && bind != "::" {
		return []string{bind}
	}
	var hosts []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			hosts = append(hosts, ipnet.IP.String())
		}
	}
	if len(hosts) == 0 {
		hosts = []string{"localhost"}
	}
	return hosts
}
