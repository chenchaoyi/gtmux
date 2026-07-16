package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DeviceToken is a registered push target. The Mac (gtmux serve) keeps these so
// the relay can stay stateless — every push intent carries its own token.
type DeviceToken struct {
	Token    string `json:"token"`
	Platform string `json:"platform"` // "ios" | "android" | "harmony"
	// APNs environment this token belongs to: "sandbox" (dev-signed build) or
	// "production" (App Store / TestFlight). Forwarded on each intent so ONE relay
	// serves both. Empty = let the relay fall back to its default.
	Env string `json:"env,omitempty"`
	// Kinds the device wants ("waiting"/"done"). Empty = all (backward compat),
	// so a device can opt out of e.g. "done" notifications.
	Kinds []string `json:"kinds,omitempty"`
}

// wants reports whether this device wants a notification of the given kind.
func (d DeviceToken) wants(kind string) bool {
	if len(d.Kinds) == 0 {
		return true
	}
	for _, k := range d.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// PushIntent is one notification to deliver, sent from gtmux serve to the relay.
// It is intentionally minimal (no conversation content) — title/body are short
// status lines, so a dumb relay only ever sees a device token + a one-liner.
type PushIntent struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
	Env      string `json:"env,omitempty"` // APNs env of this token — relay routes on it
	Title    string `json:"title"`
	Body     string `json:"body"`
	Pane     string `json:"pane"`               // jump target for the notification tap
	Kind     string `json:"kind"`               // "waiting" | "done"
	Subtitle string `json:"subtitle,omitempty"` // this Mac's name — WHICH server, for multi-server
	// Live Activity push-to-update: when set, Token is the activity push token and
	// ContentState replaces the lock-screen activity's state (even app-killed).
	LiveActivity bool           `json:"liveActivity,omitempty"`
	Event        string         `json:"event,omitempty"` // "update" | "end"
	ContentState map[string]any `json:"contentState,omitempty"`
	// Silent badge/dismiss sync (6a): a content-available push (no alert) that keeps
	// the app-icon badge correct on every device and collapses a resolved agent's
	// banner. Badge is the absolute waiting count; CollapseID is the agent's pane.
	Silent     bool   `json:"silent,omitempty"`
	Badge      *int   `json:"badge,omitempty"`
	CollapseID string `json:"collapseId,omitempty"`
}

// Relay forwards a push intent onward to the push gateway (APNs/FCM/HMS). The
// HTTP implementation talks to the gtmux push relay service; tests use a fake.
type Relay interface {
	Send(PushIntent) error
}

// PushManager holds registered device tokens and, on each agent alert, forwards
// one intent per token to the relay. It owns no APNs credentials (those live in
// the relay), so the Mac only ever makes an outbound HTTPS call.
type PushManager struct {
	mu     sync.Mutex
	tokens map[string]DeviceToken
	// activity push tokens for Live Activity push-to-update (in-memory; the app
	// re-registers on each launch / activity start). token → APNs env (for routing).
	activityTokens map[string]string
	relay          Relay
	save           func([]DeviceToken)              // optional persistence hook
	format         func(Alert) (title, body string) // optional copy formatter (i18n lives in app)
	serverName     string                           // this Mac's name → notification subtitle (WHICH server)
}

// NewPushManager builds a manager seeded with any persisted tokens. relay may be
// a no-op (empty relay URL) — registration still works, forwarding is just off.
func NewPushManager(relay Relay, initial []DeviceToken, save func([]DeviceToken), serverName string, format func(Alert) (string, string)) *PushManager {
	m := &PushManager{tokens: map[string]DeviceToken{}, activityTokens: map[string]string{}, relay: relay, save: save, format: format, serverName: serverName}
	for _, d := range initial {
		if d.Token != "" {
			m.tokens[d.Token] = d
		}
	}
	return m
}

// Register adds or refreshes a device token and persists the set.
func (p *PushManager) Register(d DeviceToken) {
	if d.Token == "" {
		return
	}
	if d.Platform == "" {
		d.Platform = "ios"
	}
	p.mu.Lock()
	p.tokens[d.Token] = d
	snap := p.snapshotLocked()
	p.mu.Unlock()
	if p.save != nil {
		p.save(snap)
	}
}

// Unregister drops a device token — the phone has unpaired this Mac (removed the
// server), so this Mac must stop forwarding alerts and silent-badge pushes to it.
// Idempotent; persists only when the token was actually present. Each Mac keeps its
// own token set, so removing one paired server never affects the others.
func (p *PushManager) Unregister(token string) {
	if token == "" {
		return
	}
	p.mu.Lock()
	_, had := p.tokens[token]
	delete(p.tokens, token)
	snap := p.snapshotLocked()
	p.mu.Unlock()
	if had && p.save != nil {
		p.save(snap)
	}
}

// UnregisterActivity drops a Live Activity push token (the device removed this
// server) so this Mac stops pushing lock-screen tally updates to it — and, when the
// token was actually held, best-effort pushes an END so a card this Mac was keeping
// alive disappears even if the app couldn't end it locally (e.g. it was force-killed).
// Activity tokens are in-memory (re-registered each launch), so nothing is persisted.
// Idempotent.
func (p *PushManager) UnregisterActivity(token string) {
	if token == "" {
		return
	}
	p.mu.Lock()
	env, had := p.activityTokens[token]
	delete(p.activityTokens, token)
	relay := p.relay
	p.mu.Unlock()
	if had && relay != nil {
		go func() {
			_ = relay.Send(PushIntent{Token: token, Env: env, LiveActivity: true, Event: "end"})
		}()
	}
}

func (p *PushManager) snapshotLocked() []DeviceToken {
	out := make([]DeviceToken, 0, len(p.tokens))
	for _, d := range p.tokens {
		out = append(out, d)
	}
	return out
}

// Tokens returns a snapshot of the registered device tokens.
func (p *PushManager) Tokens() []DeviceToken {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snapshotLocked()
}

// RegisterActivity records a Live Activity push token so the tally can be pushed
// to the lock screen even when the app is closed.
func (p *PushManager) RegisterActivity(token, env string) {
	if token == "" {
		return
	}
	p.mu.Lock()
	if p.activityTokens == nil {
		p.activityTokens = map[string]string{}
	}
	p.activityTokens[token] = env
	p.mu.Unlock()
}

// PushLiveActivity forwards the current tally to every registered Live Activity
// push token (best-effort, async). No-op when no relay or no activity is
// registered. Wired as the hub's onTally hook.
func (p *PushManager) PushLiveActivity(t Tally) {
	if p == nil || p.relay == nil {
		return
	}
	type actTok struct{ token, env string }
	p.mu.Lock()
	toks := make([]actTok, 0, len(p.activityTokens))
	for tok, env := range p.activityTokens {
		toks = append(toks, actTok{tok, env})
	}
	p.mu.Unlock()
	if len(toks) == 0 {
		return
	}
	items := make([]map[string]any, 0, len(t.Items))
	for _, it := range t.Items {
		items = append(items, map[string]any{"title": it.Title, "status": it.Status, "since": it.Since})
	}
	cs := map[string]any{
		"waiting": t.Waiting, "working": t.Working, "idle": t.Idle,
		"waitingTitle": t.WaitingTitle, "waitingSession": t.WaitingSession,
		"items": items, "more": t.More,
	}
	go func() {
		for _, tk := range toks {
			_ = p.relay.Send(PushIntent{Token: tk.token, Env: tk.env, LiveActivity: true, Event: "update", ContentState: cs})
		}
	}()
}

// OnAlert is wired as the hub's alert hook. It dispatches asynchronously so a
// slow relay never stalls the SSE diff loop.
func (p *PushManager) OnAlert(a Alert) {
	if p.relay == nil {
		return
	}
	go p.dispatch(a)
}

// dispatch forwards one intent per registered token that wants this kind
// (best-effort: a dead token must not stop the others). Synchronous — OnAlert
// runs it in a goroutine.
func (p *PushManager) dispatch(a Alert) {
	title, body := p.copy(a)
	for _, d := range p.Tokens() {
		if !d.wants(a.Kind) {
			continue
		}
		_ = p.relay.Send(PushIntent{
			Token: d.Token, Platform: d.Platform, Env: d.Env,
			Title: title, Body: body, Subtitle: p.serverName, Pane: a.Pane, Kind: a.Kind,
			// Collapse an agent's banners into one: a re-nudge (#89) replaces the
			// prior "needs you" instead of stacking a second banner per agent.
			CollapseID: a.Pane,
		})
	}
}

// OnTally is the hub's tally hook: it pushes the Live Activity update AND a silent
// badge sync, so the app-icon badge tracks the live waiting count on every device.
func (p *PushManager) OnTally(t Tally) {
	p.PushLiveActivity(t)
	p.pushBadge(t.Waiting)
}

// pushBadge fans a silent, absolute-badge push out to every registered device so a
// second (even offline-until-now) phone clears its red dot to the true count.
func (p *PushManager) pushBadge(waiting int) {
	if p == nil || p.relay == nil {
		return
	}
	n := waiting
	for _, d := range p.Tokens() {
		_ = p.relay.Send(PushIntent{
			Token: d.Token, Platform: d.Platform, Env: d.Env, Silent: true, Badge: &n,
		})
	}
}

// Test sends a REALISTIC sample notification to every registered device so the
// settings screen shows the ACTUAL banner — same kind ("waiting" → the "needs you"
// style + kind badge via the Notification Service Extension), server-name subtitle,
// and localized title/body as a genuine alert — not a bare "test" banner. Ignores
// kind prefs. Returns the number of devices it tried.
func (p *PushManager) Test() int {
	a := Alert{Kind: "waiting", Agent: "Claude Code", Task: "npm test · Bash", Pane: "gtmux-test"}
	title, body := p.copy(a)
	toks := p.Tokens()
	for _, d := range toks {
		_ = p.relay.Send(PushIntent{
			Token: d.Token, Platform: d.Platform, Env: d.Env,
			Title: title, Body: body, Subtitle: p.serverName,
			Pane: a.Pane, Kind: a.Kind, CollapseID: a.Pane,
		})
	}
	return len(toks)
}

// copy builds the notification title/body, via the injected formatter (i18n) or
// a plain English fallback.
func (p *PushManager) copy(a Alert) (string, string) {
	if p.format != nil {
		return p.format(a)
	}
	name := a.Agent
	if name == "" {
		name = "agent"
	}
	if a.Kind == "waiting" {
		if a.Repeat {
			return name + " still needs you", a.Task
		}
		return name + " needs you", a.Task
	}
	return name + " finished", a.Task
}

// handleRegister implements POST /api/push/register. 503 when push is unconfigured
// (no PushManager), so the app can detect remote push isn't available.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("push not configured"))
		return
	}
	var d DeviceToken
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil || d.Token == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid token"))
		return
	}
	s.deps.Push.Register(d)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleUnregister implements POST /api/push/unregister — drop a device's APNs token
// and/or Live Activity token so this Mac stops pushing (alerts, silent badge, and
// lock-screen updates) to a phone that has removed it as a server. Idempotent: 200
// even if a token was never registered (the caller is best-effort on removal). At
// least one of token / activityToken must be present.
func (s *Server) handleUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("push not configured"))
		return
	}
	var d struct {
		Token         string `json:"token"`
		ActivityToken string `json:"activityToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil || (d.Token == "" && d.ActivityToken == "") {
		writeJSON(w, http.StatusBadRequest, errBody("invalid token"))
		return
	}
	if d.Token != "" {
		s.deps.Push.Unregister(d.Token) // stop alerts + silent-badge pushes
	}
	if d.ActivityToken != "" {
		s.deps.Push.UnregisterActivity(d.ActivityToken) // stop Live Activity updates + end the card
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleActivityRegister implements POST /api/push/activity — store a Live
// Activity push token so the tally can be pushed to the lock screen app-killed.
func (s *Server) handleActivityRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("push not configured"))
		return
	}
	var body struct {
		Token string `json:"token"`
		Env   string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid token"))
		return
	}
	s.deps.Push.RegisterActivity(body.Token, body.Env)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HTTPRelay is the production Relay: it POSTs intents to the gtmux push relay
// service. A blank URL makes Send a no-op (push forwarding disabled).
type HTTPRelay struct {
	url    string
	token  string // optional bearer to authenticate this Mac to the relay
	client *http.Client
}

// NewHTTPRelay returns an HTTPRelay. url "" → Send is a no-op.
func NewHTTPRelay(url, token string) *HTTPRelay {
	return &HTTPRelay{url: url, token: token, client: &http.Client{Timeout: 10 * time.Second}}
}

// Send POSTs the intent as JSON to the relay. Non-2xx is an error.
func (r *HTTPRelay) Send(intent PushIntent) error {
	if r.url == "" {
		return nil
	}
	b, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("relay status %d", resp.StatusCode)
	}
	return nil
}
