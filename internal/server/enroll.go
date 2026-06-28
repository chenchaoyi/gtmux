package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
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
	m.mu.Lock()
	found := false
	for tok, d := range m.devices {
		if d.ID == id {
			delete(m.devices, tok)
			found = true
		}
	}
	snap := m.devicesLocked()
	m.mu.Unlock()
	if found && m.save != nil {
		m.save(snap)
	}
	return found
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

// deviceInfo is a device roster entry WITHOUT its token (safe to list).
type deviceInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt int64  `json:"enrolledAt"`
	LastSeen   int64  `json:"lastSeen,omitempty"`
}

// handleDevices implements GET /api/devices — AUTHENTICATED. Lists the enrolled
// devices (no tokens) so `gtmux devices` can show + revoke them.
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if s.deps.Enroll == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("enrollment not configured"))
		return
	}
	out := make([]deviceInfo, 0)
	for _, d := range s.deps.Enroll.Devices() {
		out = append(out, deviceInfo{ID: d.ID, Name: d.Name, EnrolledAt: d.EnrolledAt, LastSeen: d.LastSeen})
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out})
}

// handleRevoke implements POST /api/devices/revoke {id} — AUTHENTICATED. Drops a
// device from the roster (in-memory + persisted) so its token stops working now.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
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
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": s.deps.Enroll.Revoke(body.ID)})
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
