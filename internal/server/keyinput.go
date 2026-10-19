package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
)

const keyContextSeconds = 120
const maxKeyReceipts = 512

// crypto/rand.Text fails closed if the OS cannot provide secure randomness.
func newKeyBoot() string { return rand.Text() }

type keyReceipt struct {
	fingerprint string
	expires     int64
	status      int
	message     string
}
type keyInputs struct {
	mu       sync.Mutex
	boot     string
	secret   string
	receipts map[string]keyReceipt
}

type keyCapabilities struct {
	Version      int                           `json:"version"`
	Context      terminalkeys.Context          `json:"context"`
	CanInput     bool                          `json:"can_input"`
	Keys         []terminalkeys.Key            `json:"keys,omitempty"`
	LegacyEvents map[string]terminalkeys.Event `json:"legacy_events,omitempty"`
	Actions      []terminalkeys.Action         `json:"actions"`
	RetrySeconds int                           `json:"retry_seconds"`
}

func keyCaller(r *http.Request) string {
	sum := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	return hex.EncodeToString(sum[:])
}

func (s *Server) keyProof(r *http.Request, id string, c terminalkeys.Context) string {
	h := hmac.New(sha256.New, []byte(s.keyInputs.secret))
	for _, part := range []string{keyCaller(r), id, c.Boot, c.PaneBinding, strconv.FormatInt(c.ExpiresAt, 10)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Server) handleInputCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, errBody("method not allowed"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, 400, errBody("missing id"))
		return
	}
	if !s.mayReachPane(w, r, id) {
		return
	}
	if s.deps.InputTarget == nil || s.deps.InputKey == nil {
		writeJSON(w, 503, errBody("terminal keys unavailable"))
		return
	}
	t, err := s.deps.InputTarget(id)
	if err != nil || t.Binding == "" {
		writeJSON(w, 404, errBody("pane input route unavailable"))
		return
	}
	c := terminalkeys.Context{Boot: s.keyInputs.boot, PaneBinding: t.Binding, ExpiresAt: time.Now().Unix() + keyContextSeconds}
	c.Proof = s.keyProof(r, id, c)
	actions := t.Actions
	if actions == nil {
		actions = []terminalkeys.Action{}
	}
	w.Header().Set("Cache-Control", "no-store")
	cap := keyCapabilities{Version: terminalkeys.Version, Context: c, CanInput: s.mayInputPane(r, id), Actions: actions, RetrySeconds: keyContextSeconds}
	if r.URL.Query().Get("catalog_version") != strconv.Itoa(terminalkeys.Version) || r.URL.Query().Get("catalog_boot") != s.keyInputs.boot {
		cap.Keys = terminalkeys.Catalog()
		cap.LegacyEvents = terminalkeys.LegacyEvents()
	}
	writeJSON(w, 200, cap)
}

// sendKeyEvent serializes injection and receipt creation so concurrent duplicates
// cannot both land. Receipts are not evicted while their signed context is valid.
func (s *Server) sendKeyEvent(w http.ResponseWriter, r *http.Request, req sendRequest) bool {
	fail := func(status int, message string) bool { writeJSON(w, status, errBody(message)); return false }
	if s.deps.InputTarget == nil || s.deps.InputKey == nil {
		return fail(503, "terminal keys unavailable")
	}
	if req.Text != "" || req.Key != "" || req.Enter || req.KeyEvent != nil && req.Action != nil {
		return fail(400, "provide exactly one input form")
	}
	if len(req.SendID) < 1 || len(req.SendID) > 160 || req.InputContext == nil {
		return fail(400, "key event needs send_id and input_context")
	}
	if req.KeyEvent != nil {
		if _, err := terminalkeys.Encode(*req.KeyEvent); err != nil {
			if errors.Is(err, terminalkeys.ErrUnsupported) {
				return fail(422, err.Error())
			}
			return fail(400, err.Error())
		}
	}
	c := *req.InputContext
	now := time.Now().Unix()
	if c.Boot != s.keyInputs.boot || c.ExpiresAt < now || c.ExpiresAt > now+keyContextSeconds || !hmac.Equal([]byte(c.Proof), []byte(s.keyProof(r, req.ID, c))) {
		return fail(409, "terminal input context expired; inspect the terminal before sending again")
	}
	payload, _ := json.Marshal(req)
	sum := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(sum[:])
	cacheID := keyCaller(r) + ":" + req.SendID
	s.keyInputs.mu.Lock()
	defer s.keyInputs.mu.Unlock()
	// Grants can be revoked while a request waits behind an in-flight key.
	if !s.mayInputPane(r, req.ID) {
		return fail(403, "input not shared for this pane")
	}
	if c.ExpiresAt < time.Now().Unix() {
		return fail(409, "terminal input context expired; no key was sent")
	}
	t, err := s.deps.InputTarget(req.ID)
	if err != nil || t.Binding != c.PaneBinding || t.Binding == "" {
		return fail(409, "pane input route changed; no key was sent")
	}
	if old, ok := s.keyInputs.receipts[cacheID]; ok {
		if old.fingerprint != fingerprint {
			return fail(409, "send_id already belongs to another key event")
		}
		if old.status != 200 {
			return fail(old.status, old.message)
		}
		return true
	}
	for k, v := range s.keyInputs.receipts {
		if v.expires < now {
			delete(s.keyInputs.receipts, k)
		}
	}
	if len(s.keyInputs.receipts) >= maxKeyReceipts {
		return fail(503, "terminal key receipts are busy; no key was sent")
	}
	var event terminalkeys.Event
	if req.KeyEvent != nil {
		event = *req.KeyEvent
	} else {
		found := false
		for _, a := range t.Actions {
			if a.ID == req.Action.ID && a.Revision == req.Action.Revision {
				event = a.Event
				found = true
				break
			}
		}
		if !found {
			return fail(409, "question context changed; no key was sent")
		}
	}
	if _, err := terminalkeys.Encode(event); err != nil {
		return fail(422, err.Error())
	}
	status, message := 200, ""
	if err := s.deps.InputKey(req.ID, t.Binding, event, req.Action); err != nil {
		// Do not retry a failed injection: tmux may have accepted bytes before erroring.
		status, message = 409, "terminal key result unconfirmed; inspect the terminal before continuing"
	}
	outcome := diag.OK
	if status != 200 {
		outcome = diag.Failed
	}
	// Do not log printable shortcut characters or the signed input context.
	lg.Act("act.send-key", actorOf(r.Context()), req.ID, outcome, "terminal key injection attempted", "via", via(r))
	s.keyInputs.receipts[cacheID] = keyReceipt{fingerprint, c.ExpiresAt + keyContextSeconds, status, message}
	if status != 200 {
		return fail(status, message)
	}
	return true
}
