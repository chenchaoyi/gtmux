package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Agent is the subset of `gtmux agents --json` the attach client needs to pick a pane.
type Agent struct {
	PaneID  string `json:"pane_id"`
	Session string `json:"session"`
	Agent   string `json:"agent"`
	Status  string `json:"status"`
	Task    string `json:"task"`
	Loc     string `json:"loc"`
	Source  string `json:"source,omitempty"` // "tmux" | "native"
}

// Cap mirrors GET /api/share — the caller's own scope. All ⇒ owner (full); else a
// guest scoped to ViewPanes/Panes.
type Cap struct {
	Input     bool     `json:"input"`
	All       bool     `json:"all"`
	Panes     []string `json:"panes"`
	ViewPanes []string `json:"view_panes"`
}

// Client is a small consumer of the serve HTTP contract (reachability + scope + the
// pane list) used to set up an attach. The attach stream itself is a WebSocket (see
// attach.go), not one of these request/response calls.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// NewClient builds a client for a base URL + bearer token.
func NewClient(base, token string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), token: token, http: &http.Client{Timeout: 8 * time.Second}}
}

// Base is the resolved server URL (for building the WS attach URL).
func (c *Client) Base() string { return c.base }

// Token is the bearer (for the WS Authorization header).
func (c *Client) Token() string { return c.token }

// Health is a reachability check (unauthenticated).
func (c *Client) Health(ctx context.Context) bool {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(r)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Share reads the caller's scope (GET /api/share): All ⇒ owner, else guest.
func (c *Client) Share(ctx context.Context) (Cap, error) {
	var cap Cap
	if err := c.getJSON(ctx, "/api/share", &cap); err != nil {
		return Cap{}, err
	}
	return cap, nil
}

// Agents fetches the (scope-filtered) radar so `gtmux attach` can pick a pane.
func (c *Client) Agents(ctx context.Context) ([]Agent, error) {
	var a []Agent
	if err := c.getJSON(ctx, "/api/agents", &a); err != nil {
		return nil, err
	}
	return a, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// RedeemEnrollCode exchanges a one-time pair code for THIS terminal's own owner
// device token (POST /api/enroll — unauthenticated; the code IS the credential).
// name labels the device in the host's roster (`gtmux pair list`).
func RedeemEnrollCode(ctx context.Context, base, code, name string) (string, error) {
	body, _ := json.Marshal(map[string]string{"enrollCode": code, "name": name})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(base, "/")+"/api/enroll", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("enroll: HTTP %d (code expired? mint a fresh one with gtmux pair)", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		return "", fmt.Errorf("enroll: bad response")
	}
	return out.Token, nil
}
