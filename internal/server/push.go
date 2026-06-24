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
	Title    string `json:"title"`
	Body     string `json:"body"`
	Pane     string `json:"pane"` // jump target for the notification tap
	Kind     string `json:"kind"` // "waiting" | "done"
	// Live Activity push-to-update: when set, Token is the activity push token and
	// ContentState replaces the lock-screen activity's state (even app-killed).
	LiveActivity bool           `json:"liveActivity,omitempty"`
	Event        string         `json:"event,omitempty"` // "update" | "end"
	ContentState map[string]any `json:"contentState,omitempty"`
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
	// re-registers on each launch / activity start).
	activityTokens map[string]struct{}
	relay          Relay
	save           func([]DeviceToken)              // optional persistence hook
	format         func(Alert) (title, body string) // optional copy formatter (i18n lives in app)
}

// NewPushManager builds a manager seeded with any persisted tokens. relay may be
// a no-op (empty relay URL) — registration still works, forwarding is just off.
func NewPushManager(relay Relay, initial []DeviceToken, save func([]DeviceToken), format func(Alert) (string, string)) *PushManager {
	m := &PushManager{tokens: map[string]DeviceToken{}, activityTokens: map[string]struct{}{}, relay: relay, save: save, format: format}
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
func (p *PushManager) RegisterActivity(token string) {
	if token == "" {
		return
	}
	p.mu.Lock()
	if p.activityTokens == nil {
		p.activityTokens = map[string]struct{}{}
	}
	p.activityTokens[token] = struct{}{}
	p.mu.Unlock()
}

// PushLiveActivity forwards the current tally to every registered Live Activity
// push token (best-effort, async). No-op when no relay or no activity is
// registered. Wired as the hub's onTally hook.
func (p *PushManager) PushLiveActivity(t Tally) {
	if p == nil || p.relay == nil {
		return
	}
	p.mu.Lock()
	toks := make([]string, 0, len(p.activityTokens))
	for tok := range p.activityTokens {
		toks = append(toks, tok)
	}
	p.mu.Unlock()
	if len(toks) == 0 {
		return
	}
	cs := map[string]any{
		"waiting": t.Waiting, "working": t.Working, "idle": t.Idle,
		"waitingTitle": t.WaitingTitle, "waitingSession": t.WaitingSession,
	}
	go func() {
		for _, tok := range toks {
			_ = p.relay.Send(PushIntent{Token: tok, LiveActivity: true, Event: "update", ContentState: cs})
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
			Token: d.Token, Platform: d.Platform,
			Title: title, Body: body, Pane: a.Pane, Kind: a.Kind,
		})
	}
}

// Test sends a test notification to every registered device (ignores kind prefs).
// Returns the number of devices it tried.
func (p *PushManager) Test(title, body string) int {
	toks := p.Tokens()
	for _, d := range toks {
		_ = p.relay.Send(PushIntent{
			Token: d.Token, Platform: d.Platform,
			Title: title, Body: body, Pane: "", Kind: "test",
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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid token"))
		return
	}
	s.deps.Push.RegisterActivity(body.Token)
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
