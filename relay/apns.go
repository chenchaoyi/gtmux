package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// apnsProd / apnsSandbox are Apple's push endpoints. Sandbox is for development
// builds (Xcode/TestFlight dev), production for App Store / TestFlight.
const (
	apnsProd    = "https://api.push.apple.com"
	apnsSandbox = "https://api.sandbox.push.apple.com"
)

// apnsPusher delivers notifications to APNs over HTTP/2 using token-based (.p8)
// auth. The provider JWT is cached and refreshed (Apple requires 20–60 min).
type apnsPusher struct {
	baseURL string // injectable so tests can point at a fake Apple
	topic   string // the app's bundle id (apns-topic)
	keyID   string
	teamID  string
	key     *ecdsa.PrivateKey
	client  *http.Client

	mu    sync.Mutex
	jwt   string
	jwtAt time.Time
}

// newAPNSPusher builds a pusher from a PKCS#8 .p8 key (PEM bytes) and ids.
func newAPNSPusher(baseURL, topic, keyID, teamID string, p8 []byte) (*apnsPusher, error) {
	key, err := parseP8(p8)
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		baseURL = apnsProd
	}
	return &apnsPusher{
		baseURL: baseURL,
		topic:   topic,
		keyID:   keyID,
		teamID:  teamID,
		key:     key,
		// APNs requires HTTP/2; net/http negotiates h2 over TLS automatically.
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Push sends one alert notification to APNs for req.Token.
func (a *apnsPusher) Push(ctx context.Context, req pushRequest) error {
	jwt, err := a.bearer()
	if err != nil {
		return err
	}
	body, err := json.Marshal(apnsPayload(req))
	if err != nil {
		return err
	}
	url := a.baseURL + "/3/device/" + req.Token
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("authorization", "bearer "+jwt)
	httpReq.Header.Set("apns-topic", a.topic)
	httpReq.Header.Set("apns-push-type", "alert")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// APNs returns a JSON {"reason":"..."} on failure (e.g. BadDeviceToken).
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<12))
	reason := ""
	var e struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(rb, &e) == nil {
		reason = e.Reason
	}
	return fmt.Errorf("apns status %d %s", resp.StatusCode, reason)
}

// apnsPayload builds the aps dictionary. Pane rides along as a custom key so the
// app can deep-link the notification tap to the right pane.
func apnsPayload(req pushRequest) map[string]any {
	aps := map[string]any{
		"alert": map[string]any{
			"title": req.Title,
			"body":  req.Body,
		},
		"sound": "default",
	}
	// `waiting` pushes carry the AGENT_WAITING category so iOS shows the
	// quick-reply actions (1 Yes / 2 Always / 3 No) the app answers in-background.
	if req.Kind == "waiting" {
		aps["category"] = "AGENT_WAITING"
	}
	return map[string]any{
		"aps":  aps,
		"pane": req.Pane,
		"kind": req.Kind,
	}
}

// bearer returns a cached provider JWT, minting a fresh one when older than
// ~50 minutes (inside Apple's 60-minute ceiling).
func (a *apnsPusher) bearer() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.jwt != "" && time.Since(a.jwtAt) < 50*time.Minute {
		return a.jwt, nil
	}
	tok, err := signProviderJWT(a.key, a.teamID, a.keyID, time.Now())
	if err != nil {
		return "", err
	}
	a.jwt, a.jwtAt = tok, time.Now()
	return tok, nil
}

// signProviderJWT produces an APNs provider token: ES256-signed JWT with header
// {alg:ES256,kid} and claims {iss:teamID,iat}.
func signProviderJWT(key *ecdsa.PrivateKey, teamID, keyID string, now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "ES256", "kid": keyID})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{"iss": teamID, "iat": now.Unix()})
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(claims)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	// JWS ES256 signature is R||S, each left-padded to the curve size (32 bytes).
	sig := append(leftPad(r.Bytes(), 32), leftPad(s.Bytes(), 32)...)
	return signingInput + "." + b64(sig), nil
}

// parseP8 parses an Apple AuthKey .p8 (PKCS#8, EC P-256) PEM into a private key.
func parseP8(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("apns: no PEM block in key")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: parse PKCS8: %w", err)
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns: key is not ECDSA")
	}
	return ec, nil
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// leftPad returns b left-padded with zero bytes to size (APNs ES256 needs fixed
// 32-byte R and S even when a value's big-endian form is shorter).
func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}
