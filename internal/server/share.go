package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
)

// ShareState is the host's shared-input policy: whether guests may type at all
// (Enabled, default false) and the per-pane allowlist they may type into (Panes).
type ShareState struct {
	Enabled bool     `json:"enabled"`
	Panes   []string `json:"panes"`
}

// ShareManager holds the shared-input policy for the running serve. A guest token's
// input is gated by it; the master/devices are unaffected. Persisted via save so the
// host's consent + allowlist survive a serve restart.
type ShareManager struct {
	mu      sync.Mutex
	enabled bool
	panes   map[string]bool
	save    func(ShareState)
}

// NewShareManager seeds from persisted state (default: off, no panes).
func NewShareManager(initial ShareState, save func(ShareState)) *ShareManager {
	m := &ShareManager{panes: map[string]bool{}, save: save, enabled: initial.Enabled}
	for _, p := range initial.Panes {
		if p != "" {
			m.panes[p] = true
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
	panes := make([]string, 0, len(m.panes))
	for p := range m.panes {
		panes = append(panes, p)
	}
	sort.Strings(panes)
	return ShareState{Enabled: m.enabled, Panes: panes}
}

// Allowed reports whether a guest may type into pane — consent on AND pane allowlisted.
func (m *ShareManager) Allowed(pane string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled && m.panes[pane]
}

// SetConfig applies partial updates (nil = leave unchanged) and persists.
func (m *ShareManager) SetConfig(enabled *bool, panes *[]string) ShareState {
	m.mu.Lock()
	if enabled != nil {
		m.enabled = *enabled
	}
	if panes != nil {
		m.panes = map[string]bool{}
		for _, p := range *panes {
			if p != "" {
				m.panes[p] = true
			}
		}
	}
	st := m.stateLocked()
	m.mu.Unlock()
	if m.save != nil {
		m.save(st)
	}
	return st
}

// shareCapability is GET /api/share's reply — the CALLER's input capability, so a
// surface shows an input control only where allowed. A full caller can type anywhere.
type shareCapability struct {
	Input bool     `json:"input"`         // may this caller type at all
	All   bool     `json:"all,omitempty"` // full caller: any pane
	Panes []string `json:"panes"`         // guest: the allowed panes ([] when off)
}

// handleShare implements GET /api/share — AUTHENTICATED, any scope. Returns the
// caller's own capability so the web UI mirrors (never widens) the server gate.
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) != scopeGuest {
		writeJSON(w, http.StatusOK, shareCapability{Input: true, All: true, Panes: []string{}})
		return
	}
	cap := shareCapability{Panes: []string{}}
	if s.deps.Share != nil {
		if st := s.deps.Share.State(); st.Enabled {
			cap.Input, cap.Panes = true, st.Panes
		}
	}
	writeJSON(w, http.StatusOK, cap)
}

// handleShareConfig implements GET/POST /api/share/config — MASTER only. GET returns
// the current policy ({enabled, panes}); POST sets consent and/or the allowlist.
// Guests and devices cannot read the full policy, enable sharing, or edit the list.
func (s *Server) handleShareConfig(w http.ResponseWriter, r *http.Request) {
	if !s.masterOnly(w, r) {
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
		Enabled *bool     `json:"enabled"`
		Panes   *[]string `json:"panes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("bad json"))
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Share.SetConfig(body.Enabled, body.Panes))
}

// handleShareNew implements POST /api/share/new {label} — MASTER only. Mints a guest
// share token (input-restricted by the gate) and returns it for a share link.
func (s *Server) handleShareNew(w http.ResponseWriter, r *http.Request) {
	if !s.masterOnly(w, r) {
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body)
	d := s.deps.Enroll.MintGuest(body.Label)
	writeJSON(w, http.StatusOK, map[string]string{"token": d.Token, "id": d.ID, "name": d.Name})
}

// masterOnly writes 403 and returns false unless the caller holds the master token.
func (s *Server) masterOnly(w http.ResponseWriter, r *http.Request) bool {
	if callerScope(r.Context()) != scopeMaster {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: host-only"))
		return false
	}
	return true
}
