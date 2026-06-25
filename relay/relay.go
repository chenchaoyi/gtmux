// Command relay is the gtmux push relay: a tiny, stateless, dumb forwarder.
// `gtmux serve` (on the user's Mac) POSTs minimal push intents here; the relay
// holds the platform push credentials (APNs now; FCM/HMS later) — which are tied
// to the app's developer account, NOT the user's — and forwards each intent to
// the right gateway. It stores no device state and no conversation content, so a
// payload is only ever a device token + a one-line status. Open-source and
// self-hostable: run your own with your own APNs key for a pure local-first setup.
//
// Secrets (APNs .p8 key, key id, team id, topic, relay token) come from the
// environment only — never commit them.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// pushRequest is the wire payload from `gtmux serve` (mirrors internal/server
// PushIntent; the two services share a JSON contract, not a Go type).
type pushRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"` // "ios" | "android" | "harmony"
	Title    string `json:"title"`
	Body     string `json:"body"`
	Pane     string `json:"pane"`
	Kind     string `json:"kind"`
	// Silent badge/dismiss sync (6a): a content-available push with no alert, used
	// to keep the app-icon badge correct across ALL devices and to collapse a
	// resolved agent's banner. Badge is the absolute waiting count; CollapseID
	// (apns-collapse-id) merges/replaces an agent's prior banner so re-nudges and
	// cross-device dismissals don't stack.
	Silent     bool   `json:"silent,omitempty"`
	Badge      *int   `json:"badge,omitempty"`
	CollapseID string `json:"collapseId,omitempty"`
}

// Pusher delivers one push to a platform gateway.
type Pusher interface {
	Push(ctx context.Context, req pushRequest) error
}

// relayServer routes intents to a Pusher per platform.
type relayServer struct {
	token   string            // optional bearer to authenticate the calling Mac
	pushers map[string]Pusher // platform → gateway
}

func (s *relayServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "gtmux-relay"})
	})
	mux.HandleFunc("/push", s.handlePush)
	return mux
}

func (s *relayServer) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.token != "" && r.Header.Get("Authorization") != "Bearer "+s.token {
		writeJSON(w, http.StatusUnauthorized, errBody("unauthorized"))
		return
	}
	var req pushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request"))
		return
	}
	platform := req.Platform
	if platform == "" {
		platform = "ios"
	}
	pusher := s.pushers[platform]
	if pusher == nil {
		writeJSON(w, http.StatusBadRequest, errBody("unsupported platform: "+platform))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := pusher.Push(ctx, req); err != nil {
		writeJSON(w, http.StatusBadGateway, errBody("push failed: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
