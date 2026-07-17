package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ShareState is the host's shared-input policy: whether guests may type at all
// (Enabled, default false), the per-pane allowlist they may TYPE into (Panes), and
// the separate per-pane allowlist they may SEE (ViewPanes). Invariant: input ⊆ view
// (a pane a guest may type into is always one it may see). Both default empty, so a
// fresh guest sees nothing and types nowhere.
type ShareState struct {
	Enabled   bool     `json:"enabled"`
	Panes     []string `json:"panes"`
	ViewPanes []string `json:"view_panes"`
}

// ShareManager holds the shared-input policy for the running serve. A guest token's
// view and input are gated by it; the master/devices are unaffected. Persisted via
// save so the host's consent + allowlists survive a serve restart.
type ShareManager struct {
	broadcast func(view, input []string) // fan-out hook (legacy global edits → every link)
	mu        sync.Mutex
	enabled   bool
	panes     map[string]bool // input allowlist
	viewPanes map[string]bool // view allowlist (⊇ panes)
	save      func(ShareState)
}

// NewShareManager seeds from persisted state (default: off, no panes). A pane on the
// input allowlist is always viewable (input ⊆ view), so we union Panes into the view
// set — this also migrates an old share.json that had only `panes`: those previously
// shared panes stay viewable, everything else becomes invisible (secure default).
func NewShareManager(initial ShareState, save func(ShareState)) *ShareManager {
	m := &ShareManager{panes: map[string]bool{}, viewPanes: map[string]bool{}, save: save, enabled: initial.Enabled}
	for _, p := range initial.ViewPanes {
		if p != "" {
			m.viewPanes[p] = true
		}
	}
	for _, p := range initial.Panes {
		if p != "" {
			m.panes[p] = true
			m.viewPanes[p] = true // input implies view
		}
	}
	return m
}

// State returns the current policy (panes sorted for a stable display).
func (m *ShareManager) State() ShareState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateLocked()
}

func (m *ShareManager) stateLocked() ShareState {
	return ShareState{Enabled: m.enabled, Panes: sortedKeys(m.panes), ViewPanes: sortedKeys(m.viewPanes)}
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Allowed reports whether a guest may type into pane — consent on AND pane on the
// input allowlist.
func (m *ShareManager) Allowed(pane string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled && m.panes[pane]
}

// CanView reports whether a guest may SEE pane — pane on the view allowlist. View is
// independent of the consent toggle (that gates typing); a host can let a guest watch
// a pane without letting them type.
func (m *ShareManager) CanView(pane string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.viewPanes[pane]
}

// SetConfig applies partial updates (nil = leave unchanged), enforces input ⊆ view
// (every input pane is also viewable), and persists. A pane-list change also fans
// out to every existing guest link via the broadcast hook (pair-share-model: the
// legacy global forms keep their exact observed behavior — the lists double as the
// TEMPLATE for new links and a BROADCAST edit of existing ones).
func (m *ShareManager) SetConfig(enabled *bool, panes, viewPanes *[]string) ShareState {
	m.mu.Lock()
	if enabled != nil {
		m.enabled = *enabled
	}
	if viewPanes != nil {
		m.viewPanes = toSet(*viewPanes)
	}
	if panes != nil {
		m.panes = toSet(*panes)
	}
	// Invariant: input ⊆ view. An input-allowed pane is always viewable.
	for p := range m.panes {
		m.viewPanes[p] = true
	}
	st := m.stateLocked()
	broadcast := m.broadcast
	listsChanged := panes != nil || viewPanes != nil
	m.mu.Unlock()
	if m.save != nil {
		m.save(st)
	}
	if listsChanged && broadcast != nil {
		broadcast(st.ViewPanes, st.Panes)
	}
	return st
}

// OnBroadcast wires the fan-out hook (serve wiring points it at
// EnrollManager.BroadcastGuestScopes).
func (m *ShareManager) OnBroadcast(fn func(view, input []string)) {
	m.mu.Lock()
	m.broadcast = fn
	m.mu.Unlock()
}

// InputEnabled reports the host-level consent gate for ALL guest typing.
func (m *ShareManager) InputEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func toSet(items []string) map[string]bool {
	set := map[string]bool{}
	for _, p := range items {
		if p != "" {
			set[p] = true
		}
	}
	return set
}

// shareCapability is GET /api/share's reply — the CALLER's view+input capability, so
// a surface shows each pane only where allowed. A full caller can see and type
// anywhere (All); a guest is scoped to its view/input allowlists.
type shareCapability struct {
	Input     bool     `json:"input"`         // may this caller type at all
	All       bool     `json:"all,omitempty"` // full caller: any pane (view + input)
	Panes     []string `json:"panes"`         // guest: the input-allowed panes ([] when off)
	ViewPanes []string `json:"view_panes"`    // guest: the view-allowed panes
}

// handleShare implements GET /api/share — AUTHENTICATED, any scope. Returns the
// caller's own capability so the web UI mirrors (never widens) the server gate.
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) != scopeGuest {
		writeJSON(w, http.StatusOK, shareCapability{Input: true, All: true, Panes: []string{}, ViewPanes: []string{}})
		return
	}
	// A guest's capability resolves from ITS OWN link scope (pair-share-model),
	// gated by the host-level consent for input. Input=true only when the caller
	// can actually type somewhere (consent on AND a non-empty input list).
	cap := shareCapability{Panes: []string{}, ViewPanes: []string{}}
	if dev, ok := callerDevice(r.Context()); ok {
		if dev.ViewPanes != nil {
			cap.ViewPanes = dev.ViewPanes
		}
		if s.deps.Share != nil && s.deps.Share.InputEnabled() && len(dev.InputPanes) > 0 {
			cap.Input, cap.Panes = true, dev.InputPanes
		}
	}
	writeJSON(w, http.StatusOK, cap)
}

// handleShareConfig implements GET/POST /api/share/config — MASTER only. GET returns
// the current policy ({enabled, panes}); POST sets consent and/or the allowlist.
// Guests and devices cannot read the full policy, enable sharing, or edit the list.
func (s *Server) handleShareConfig(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if s.deps.Share == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("sharing not configured"))
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.deps.Share.State())
		return
	}
	var body struct {
		Enabled   *bool     `json:"enabled"`
		Panes     *[]string `json:"panes"`
		ViewPanes *[]string `json:"view_panes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("bad json"))
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Share.SetConfig(body.Enabled, body.Panes, body.ViewPanes))
}

// handleShareNew implements POST /api/share/new {label, view?, input?,
// expiresInSec?} — MASTER only. Mints a guest share token with its OWN per-link
// scope (pair-share-model). Omitted scope falls back to the current global lists
// (the TEMPLATE — preserves the legacy "mint, then tick" flow's semantics).
func (s *Server) handleShareNew(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	var body struct {
		Label        string    `json:"label"`
		View         *[]string `json:"view"`
		Input        *[]string `json:"input"`
		ExpiresInSec int64     `json:"expiresInSec"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body)
	// Template default: no explicit scope → copy the current global lists.
	view, input := []string{}, []string{}
	if s.deps.Share != nil {
		st := s.deps.Share.State()
		view, input = st.ViewPanes, st.Panes
	}
	if body.View != nil {
		view = *body.View
	}
	if body.Input != nil {
		input = *body.Input
	}
	expiresAt := int64(0)
	if body.ExpiresInSec > 0 {
		expiresAt = time.Now().Unix() + body.ExpiresInSec
	}
	d := s.deps.Enroll.MintGuest(body.Label, view, input, expiresAt)
	writeJSON(w, http.StatusOK, map[string]string{"token": d.Token, "id": d.ID, "name": d.Name})
}

// handleShareSet implements POST /api/share/set {id, view?, input?, expiresInSec?,
// clearExpiry?} — MASTER only. Edits ONE guest link's scope (pair-share-model):
// an omitted facet is untouched; expiresInSec>0 sets a fresh expiry from now;
// clearExpiry removes it.
func (s *Server) handleShareSet(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	var body struct {
		ID           string    `json:"id"`
		View         *[]string `json:"view"`
		Input        *[]string `json:"input"`
		ExpiresInSec int64     `json:"expiresInSec"`
		ClearExpiry  bool      `json:"clearExpiry"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, errBody("bad json"))
		return
	}
	var expiresAt *int64
	switch {
	case body.ClearExpiry:
		zero := int64(0)
		expiresAt = &zero
	case body.ExpiresInSec > 0:
		at := time.Now().Unix() + body.ExpiresInSec
		expiresAt = &at
	}
	d, ok := s.deps.Enroll.SetGuestScope(body.ID, body.View, body.Input, expiresAt)
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("unknown share link"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": d.ID, "name": d.Name,
		"viewPanes": d.ViewPanes, "inputPanes": d.InputPanes, "expiresAt": d.ExpiresAt,
	})
}

// handleShareLink implements GET /api/share/link?id=<id> — FULL only. Re-hands the
// share TOKEN for an existing guest link so an owner can re-copy the URL after
// minting (owner-remote-admin: a link is no longer view-once). Only guest links;
// a paired device's token is never returned.
func (s *Server) handleShareLink(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	token, label, ok := s.deps.Enroll.TokenByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("unknown share link"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "label": label, "token": token})
}

// masterOnly writes 403 and returns false unless the caller holds the master token.
func (s *Server) masterOnly(w http.ResponseWriter, r *http.Request) bool {
	if callerScope(r.Context()) != scopeMaster {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: host-only"))
		return false
	}
	return true
}

// fullOnly writes 403 and returns false unless the caller is a FULL surface — the
// master token OR an owner device (a paired phone/browser/terminal). A guest is
// refused. This is the gate for SHARE management (owner-remote-admin, decision B):
// an owner device manages sharing remotely, while the device roster + the door
// stay Mac-scoped (handled separately). It also closes the prior unguarded
// device-list path (a guest could list the roster).
func (s *Server) fullOnly(w http.ResponseWriter, r *http.Request) bool {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return false
	}
	return true
}
