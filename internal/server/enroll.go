package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// enrollCodeTTL bounds how long a pairing code is valid. Short, because the code
// is shown in a QR and is the only thing standing between a scanner and a device
// token — so a leaked screenshot goes stale fast.
const enrollCodeTTL = 5 * time.Minute

// EnrolledDevice is one paired phone in the roster. It authenticates with its OWN
// Token (never the shared master token), so a device can be revoked individually
// and the pairing QR can carry a throwaway code instead of a lasting credential.
type EnrolledDevice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Token      string `json:"token"`
	EnrolledAt int64  `json:"enrolledAt"`
	LastSeen   int64  `json:"lastSeen,omitempty"`
	// Scope is "" (a paired device — the owner's own surface, unrestricted input) or
	// "guest" (a share link — input-restricted by the share gate). Additive/back-compat.
	Scope string `json:"scope,omitempty"`
	// Per-link guest scope (pair-share-model): the panes THIS link may see / type
	// into (input ⊆ view, normalized on every write), and an optional expiry (unix
	// seconds; 0 = never — past it the token stops authenticating). nil/0 on owner
	// devices (unrestricted). ScopeSet marks that the per-link lists are
	// authoritative — it distinguishes an explicitly-emptied scope from a legacy
	// entry awaiting the one-time global-list migration (empty slices are dropped
	// by omitempty on persist, so the flag carries the difference).
	ViewPanes  []string `json:"viewPanes,omitempty"`
	InputPanes []string `json:"inputPanes,omitempty"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
	ScopeSet   bool     `json:"scopeSet,omitempty"`
}

// Expired reports whether the entry has an expiry in the past.
func (d EnrolledDevice) Expired(now int64) bool {
	return d.ExpiresAt > 0 && now > d.ExpiresAt
}

// MayView reports whether THIS link may see pane (per-link view allowlist).
func (d EnrolledDevice) MayView(pane string) bool {
	for _, p := range d.ViewPanes {
		if p == pane {
			return true
		}
	}
	return false
}

// MayInput reports whether THIS link may type into pane (per-link input allowlist —
// the host-level consent gate is checked separately by the ShareManager).
func (d EnrolledDevice) MayInput(pane string) bool {
	for _, p := range d.InputPanes {
		if p == pane {
			return true
		}
	}
	return false
}

// normalizeScope dedups + sorts both lists and enforces input ⊆ view (every
// input-allowed pane is unioned into view). Returns non-nil slices.
func normalizeScope(view, input []string) (v, in []string) {
	vs, is := map[string]bool{}, map[string]bool{}
	for _, p := range view {
		if p != "" {
			vs[p] = true
		}
	}
	for _, p := range input {
		if p != "" {
			is[p] = true
			vs[p] = true // input implies view
		}
	}
	v, in = make([]string, 0, len(vs)), make([]string, 0, len(is))
	for p := range vs {
		v = append(v, p)
	}
	for p := range is {
		in = append(in, p)
	}
	sort.Strings(v)
	sort.Strings(in)
	return v, in
}

// EnrollManager holds the device roster plus short-lived, single-use enroll codes.
// Pairing flow: a trusted surface (the running serve, via the authenticated mint
// endpoint) issues a code into the QR; the phone redeems it ONCE for its own
// device token. Codes expire fast and can't be reused.
type EnrollManager struct {
	mu      sync.Mutex
	devices map[string]EnrolledDevice // keyed by token
	codes   map[string]int64          // code → unix expiry
	save    func([]EnrolledDevice)    // optional persistence hook
	now     func() time.Time          // injectable clock (tests)
}

// NewEnrollManager seeds the roster with any persisted devices.
func NewEnrollManager(initial []EnrolledDevice, save func([]EnrolledDevice)) *EnrollManager {
	m := &EnrollManager{
		devices: map[string]EnrolledDevice{},
		codes:   map[string]int64{},
		save:    save,
		now:     time.Now,
	}
	for _, d := range initial {
		if d.Token != "" {
			m.devices[d.Token] = d
		}
	}
	return m
}

// Mint creates a short-lived, single-use enroll code for a pairing QR. Kept
// SHORT on purpose (8 bytes = 64-bit, single-use, 5-min TTL) so the pairing QR
// stays small/scannable — see the module-count footgun note in app/qr.go.
func (m *EnrollManager) Mint() string {
	code := randHex(8)
	m.mu.Lock()
	m.pruneLocked()
	m.codes[code] = m.now().Add(enrollCodeTTL).Unix()
	m.mu.Unlock()
	return code
}

// Redeem exchanges a valid code for a fresh per-device token, consuming the code.
// ok=false for an unknown/expired code.
func (m *EnrollManager) Redeem(code, name string) (EnrolledDevice, bool) {
	m.mu.Lock()
	m.pruneLocked()
	exp, ok := m.codes[code]
	if !ok || m.now().Unix() > exp {
		m.mu.Unlock()
		return EnrolledDevice{}, false
	}
	delete(m.codes, code) // single use
	d := EnrolledDevice{
		ID:         randHex(8),
		Name:       sanitizeDeviceName(name),
		Token:      randHex(32),
		EnrolledAt: m.now().Unix(),
	}
	m.devices[d.Token] = d
	snap := m.devicesLocked()
	m.mu.Unlock()
	if m.save != nil {
		m.save(snap)
	}
	return d, true
}

// MintGuest creates a GUEST share token (scope "guest") directly — the owner hands
// this out in a share link. It carries its OWN per-link scope (pair-share-model):
// which panes it may see/type into (input ⊆ view, normalized here) and an optional
// expiry. It is persisted and individually revocable by ID like any roster entry.
// No enroll code is involved: the owner is the one minting it.
func (m *EnrollManager) MintGuest(label string, view, input []string, expiresAt int64) EnrolledDevice {
	v, in := normalizeScope(view, input)
	m.mu.Lock()
	d := EnrolledDevice{
		ID:         randHex(8),
		Name:       sanitizeDeviceName(label),
		Token:      randHex(32),
		EnrolledAt: m.now().Unix(),
		Scope:      "guest",
		ViewPanes:  v,
		InputPanes: in,
		ExpiresAt:  expiresAt,
		ScopeSet:   true,
	}
	m.devices[d.Token] = d
	snap := m.devicesLocked()
	m.mu.Unlock()
	if m.save != nil {
		m.save(snap)
	}
	return d
}

// SetGuestScope edits ONE guest link's scope by id: a nil slice/pointer leaves that
// facet untouched (per-flag replace semantics). Returns the updated entry;
// ok=false for an unknown id or a non-guest entry.
func (m *EnrollManager) SetGuestScope(id string, view, input *[]string, expiresAt *int64) (EnrolledDevice, bool) {
	m.mu.Lock()
	var updated EnrolledDevice
	found := false
	for tok, d := range m.devices {
		if d.ID != id || d.Scope != "guest" {
			continue
		}
		v, in := d.ViewPanes, d.InputPanes
		if view != nil {
			v = *view
		}
		if input != nil {
			in = *input
		}
		d.ViewPanes, d.InputPanes = normalizeScope(v, in)
		if expiresAt != nil {
			d.ExpiresAt = *expiresAt
		}
		d.ScopeSet = true
		m.devices[tok] = d
		updated, found = d, true
		break
	}
	snap := m.devicesLocked()
	m.mu.Unlock()
	if found && m.save != nil {
		m.save(snap)
	}
	return updated, found
}

// BroadcastGuestScopes replaces EVERY guest link's allowlists with the given lists —
// the legacy global-mutation semantics (pair-share-model: the old global forms fan
// out, so pre-per-link UIs keep their exact observed behavior).
func (m *EnrollManager) BroadcastGuestScopes(view, input []string) {
	v, in := normalizeScope(view, input)
	m.mu.Lock()
	changed := false
	for tok, d := range m.devices {
		if d.Scope != "guest" {
			continue
		}
		d.ViewPanes, d.InputPanes = append([]string(nil), v...), append([]string(nil), in...)
		d.ScopeSet = true
		m.devices[tok] = d
		changed = true
	}
	snap := m.devicesLocked()
	m.mu.Unlock()
	if changed && m.save != nil {
		m.save(snap)
	}
}

// MigrateGuestScopes gives every legacy guest entry (minted before per-link scope;
// ScopeSet false) a ONE-TIME copy of the global lists, preserving pre-upgrade
// behavior exactly. Explicitly-scoped links are never touched. Called once at serve
// wiring.
func (m *EnrollManager) MigrateGuestScopes(view, input []string) {
	v, in := normalizeScope(view, input)
	m.mu.Lock()
	changed := false
	for tok, d := range m.devices {
		if d.Scope != "guest" || d.ScopeSet {
			continue
		}
		d.ViewPanes, d.InputPanes = append([]string(nil), v...), append([]string(nil), in...)
		d.ScopeSet = true
		m.devices[tok] = d
		changed = true
	}
	snap := m.devicesLocked()
	m.mu.Unlock()
	if changed && m.save != nil {
		m.save(snap)
	}
}

// TokenScope returns an enrolled token's scope — "guest" for a share link, "device"
// for a paired device (scope "") — updating LastSeen, and ok=false for an unknown
// token. An EXPIRED guest link reads as unknown (pair-share-model): past its expiry
// the token stops authenticating exactly like a revoked one. auth() uses it to
// authorize per scope.
func (m *EnrollManager) TokenScope(tok string) (scope string, ok bool) {
	if tok == "" {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[tok]
	if !ok {
		return "", false
	}
	if d.Expired(m.now().Unix()) {
		return "", false
	}
	d.LastSeen = m.now().Unix()
	m.devices[tok] = d
	if d.Scope == "guest" {
		return "guest", true
	}
	return "device", true
}

// TokenByID returns a GUEST link's token by its id, so a FULL caller (owner) can
// re-copy the share URL after minting (owner-remote-admin). Only guest entries are
// returned — a paired device's token is never handed back.
func (m *EnrollManager) TokenByID(id string) (token, label string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if d.ID == id && d.Scope == "guest" {
			return d.Token, d.Name, true
		}
	}
	return "", "", false
}

// DeviceByToken returns the enrolled device a token belongs to (for showing WHO
// is connected). Read-only; ok=false for the master token or any unknown token.
func (m *EnrollManager) DeviceByToken(tok string) (EnrolledDevice, bool) {
	if tok == "" {
		return EnrolledDevice{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[tok]
	return d, ok
}

// ValidToken reports whether tok belongs to an enrolled device (best-effort
// LastSeen update, not persisted — kept cheap so it runs on every request).
func (m *EnrollManager) ValidToken(tok string) bool {
	if tok == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[tok]
	if !ok {
		return false
	}
	d.LastSeen = m.now().Unix()
	m.devices[tok] = d
	return true
}

// Devices returns the roster (without tokens would be nicer for display, but the
// CLI that lists them runs out-of-process off the file; in-process callers get all).
func (m *EnrollManager) Devices() []EnrolledDevice {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.devicesLocked()
}

// Revoke removes a device by id, returning whether one was found.
func (m *EnrollManager) Revoke(id string) bool {
	found, _ := m.RevokeBy(id, true)
	return found
}

// RevokeBy removes a roster entry by id, honoring the caller's scope
// (owner-remote-admin, decision B): allowDevice=true (a master) may revoke ANY
// entry; allowDevice=false (an owner device) may revoke ONLY a `guest` link.
// Returns (removed, refused): refused=true means the entry exists but is a paired
// device the caller may not revoke — the handler maps that to 403.
func (m *EnrollManager) RevokeBy(id string, allowDevice bool) (removed, refused bool) {
	m.mu.Lock()
	for tok, d := range m.devices {
		if d.ID != id {
			continue
		}
		if !allowDevice && d.Scope != "guest" {
			m.mu.Unlock()
			return false, true // a paired device — owner may not revoke it
		}
		delete(m.devices, tok)
		snap := m.devicesLocked()
		m.mu.Unlock()
		if m.save != nil {
			m.save(snap)
		}
		return true, false
	}
	m.mu.Unlock()
	return false, false // no such id
}

func (m *EnrollManager) devicesLocked() []EnrolledDevice {
	out := make([]EnrolledDevice, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, d)
	}
	return out
}

func (m *EnrollManager) pruneLocked() {
	now := m.now().Unix()
	for c, exp := range m.codes {
		if now > exp {
			delete(m.codes, c)
		}
	}
}

// sanitizeDeviceName trims a user-supplied device label to something safe to store
// and show (no control chars, bounded length, a fallback).
func sanitizeDeviceName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		return "phone"
	}
	return s
}

// handleEnroll implements POST /api/enroll — UNAUTHENTICATED, because the
// short-lived single-use code in the body IS the credential. It redeems the code
// for this device's own token, which auth then accepts going forward.
func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	var body struct {
		EnrollCode string `json:"enrollCode"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EnrollCode == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request"))
		return
	}
	d, ok := s.deps.Enroll.Redeem(body.EnrollCode, body.Name)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid or expired enroll code"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": d.Token, "deviceId": d.ID})
}

// deviceInfo is a device roster entry WITHOUT its token (safe to list). Scope is ""
// for a paired device or "guest" for a share link, so a lister can tell them apart.
type deviceInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt int64  `json:"enrolledAt"`
	LastSeen   int64  `json:"lastSeen,omitempty"`
	Scope      string `json:"scope,omitempty"`
	// Per-link guest scope (pair-share-model), additive: absent on owner devices.
	ViewPanes  []string `json:"viewPanes,omitempty"`
	InputPanes []string `json:"inputPanes,omitempty"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
}

// handleDevices implements GET /api/devices — FULL callers only (master or owner
// device; owner-remote-admin). Lists the enrolled devices (no tokens) so the menu
// bar / `gtmux devices` / the phone's manage screen can show them. A guest is
// refused (closing the prior unguarded path).
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	out := make([]deviceInfo, 0)
	for _, d := range s.deps.Enroll.Devices() {
		out = append(out, deviceInfo{ID: d.ID, Name: d.Name, EnrolledAt: d.EnrolledAt, LastSeen: d.LastSeen, Scope: d.Scope,
			ViewPanes: d.ViewPanes, InputPanes: d.InputPanes, ExpiresAt: d.ExpiresAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out})
}

// handleRevoke implements POST /api/devices/revoke {id} — scoped (owner-remote-
// admin, decision B): a guest is refused; an owner device may revoke ONLY a guest
// share link (never a paired device — that stays Mac/master-only); the master may
// revoke anything.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if !s.fullOnly(w, r) { // guest refused
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request"))
		return
	}
	allowDevice := callerScope(r.Context()) == scopeMaster // owner may revoke only guest links
	removed, refused := s.deps.Enroll.RevokeBy(body.ID, allowDevice)
	if refused {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: paired devices are managed on the Mac"))
		return
	}
	// A revoked device must also stop receiving push — the roster and the push-token
	// store are separate, so drop any token bound to this device id (no-op if it never
	// registered one). This is the fix for a removed device that kept getting alerts.
	if removed && s.deps.Push != nil {
		s.deps.Push.UnregisterByDevice(body.ID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": removed})
}

// handleEnrollMint implements POST /api/enroll/mint — AUTHENTICATED (master or an
// existing device). It hands back a fresh short-lived code for a pairing QR, so a
// already-trusted surface can enroll a new phone without ever putting a lasting
// token in the QR.
func (s *Server) handleEnrollMint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	code := s.deps.Enroll.Mint()
	writeJSON(w, http.StatusOK, map[string]any{
		"enrollCode":   code,
		"expiresInSec": int(enrollCodeTTL.Seconds()),
	})
}

func randHex(nbytes int) string {
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
