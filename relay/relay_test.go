package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakePusher records pushes and can be made to fail.
type fakePusher struct {
	got []pushRequest
	err error
}

func (f *fakePusher) Push(_ context.Context, req pushRequest) error {
	f.got = append(f.got, req)
	return f.err
}

func post(t *testing.T, h http.Handler, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRelayPush(t *testing.T) {
	fp := &fakePusher{}
	s := &relayServer{token: "secret", pushers: map[string]Pusher{"ios": fp}}
	h := s.handler()

	// health is open
	hr := httptest.NewRequest(http.MethodGet, "/health", nil)
	hrr := httptest.NewRecorder()
	h.ServeHTTP(hrr, hr)
	if hrr.Code != http.StatusOK {
		t.Fatalf("health = %d", hrr.Code)
	}

	if rr := post(t, h, `{"token":"d","platform":"ios","title":"hi"}`, ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
	if rr := post(t, h, `{"platform":"ios"}`, "secret"); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing device token = %d, want 400", rr.Code)
	}
	if rr := post(t, h, `{"token":"d","platform":"android"}`, "secret"); rr.Code != http.StatusBadRequest {
		t.Fatalf("unsupported platform = %d, want 400", rr.Code)
	}

	rr := post(t, h, `{"token":"d","platform":"ios","title":"hi","pane":"%1"}`, "secret")
	if rr.Code != http.StatusOK {
		t.Fatalf("push = %d, want 200", rr.Code)
	}
	if len(fp.got) != 1 || fp.got[0].Token != "d" || fp.got[0].Pane != "%1" {
		t.Fatalf("pusher got %+v", fp.got)
	}

	// platform defaults to ios when omitted
	if rr := post(t, h, `{"token":"d2","title":"hi"}`, "secret"); rr.Code != http.StatusOK {
		t.Fatalf("default platform push = %d, want 200", rr.Code)
	}
}

func TestRelayPushGatewayError(t *testing.T) {
	fp := &fakePusher{err: errors.New("BadDeviceToken")}
	s := &relayServer{pushers: map[string]Pusher{"ios": fp}} // no token → open
	rr := post(t, s.handler(), `{"token":"d","platform":"ios"}`, "")
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("gateway error = %d, want 502", rr.Code)
	}
}

// TestSignProviderJWT verifies the ES256 JWT is well-formed and its signature
// validates against the signing key.
func TestSignProviderJWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signProviderJWT(key, "TEAMID", "KEYID", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt has %d parts, want 3", len(parts))
	}
	// header carries alg/kid
	hdr, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if !strings.Contains(string(hdr), `"alg":"ES256"`) || !strings.Contains(string(hdr), `"kid":"KEYID"`) {
		t.Fatalf("header = %s", hdr)
	}
	// signature (R||S, 64 bytes) verifies over header.claims
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("sig len = %d (err %v), want 64", len(sig), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	sNum := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, sNum) {
		t.Fatalf("signature does not verify")
	}
}

// TestAPNSPush drives the real apnsPusher against a fake Apple endpoint.
func TestAPNSPush(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	p8 := mustP8(t, key)

	var gotPath, gotTopic, gotAuth, gotType string
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTopic = r.Header.Get("apns-topic")
		gotAuth = r.Header.Get("authorization")
		gotType = r.Header.Get("apns-push-type")
		w.WriteHeader(http.StatusOK)
	}))
	defer apple.Close()

	p, err := newAPNSPusher(apple.URL, "com.gtmux.app", "KID", "TID", p8)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Push(context.Background(), pushRequest{Token: "devtok", Title: "hi", Body: "b", Pane: "%2"}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if gotPath != "/3/device/devtok" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotTopic != "com.gtmux.app" || gotType != "alert" || !strings.HasPrefix(gotAuth, "bearer ") {
		t.Fatalf("headers topic=%q type=%q auth=%q", gotTopic, gotType, gotAuth)
	}
}

// TestAPNSSilentBadge: a silent badge-sync push goes out as a background push with
// a collapse-id, an absolute badge, and content-available — no alert.
func TestAPNSSilentBadge(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var gotType, gotCollapse, gotPrio, gotBody string
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("apns-push-type")
		gotCollapse = r.Header.Get("apns-collapse-id")
		gotPrio = r.Header.Get("apns-priority")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer apple.Close()

	p, _ := newAPNSPusher(apple.URL, "t", "k", "tm", mustP8(t, key))
	badge := 3
	if err := p.Push(context.Background(), pushRequest{
		Token: "x", Silent: true, Badge: &badge, CollapseID: "%5", Pane: "%5",
	}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if gotType != "background" || gotPrio != "5" {
		t.Errorf("silent push type=%q prio=%q, want background/5", gotType, gotPrio)
	}
	if gotCollapse != "%5" {
		t.Errorf("collapse-id = %q, want %%5", gotCollapse)
	}
	if !strings.Contains(gotBody, `"content-available":1`) || !strings.Contains(gotBody, `"badge":3`) {
		t.Errorf("silent payload = %s", gotBody)
	}
	if strings.Contains(gotBody, `"alert"`) || strings.Contains(gotBody, `"sound"`) {
		t.Errorf("silent push must carry no alert/sound: %s", gotBody)
	}
}

func TestAPNSPushError(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"BadDeviceToken"}`))
	}))
	defer apple.Close()
	p, _ := newAPNSPusher(apple.URL, "t", "k", "tm", mustP8(t, key))
	err := p.Push(context.Background(), pushRequest{Token: "x"})
	if err == nil || !strings.Contains(err.Error(), "BadDeviceToken") {
		t.Fatalf("err = %v, want BadDeviceToken", err)
	}
}

// mustP8 marshals an EC key into PKCS#8 PEM (the .p8 format Apple issues).
func mustP8(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
