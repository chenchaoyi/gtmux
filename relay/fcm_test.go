package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFCMContract(t *testing.T) {
	data, err := os.ReadFile("../api/testdata/fcm-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name    string         `json:"name"`
		Intent  pushRequest    `json:"intent"`
		Message map[string]any `json:"message"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			message, err := fcmMessage(row.Intent)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(message)
			var normalized map[string]any
			_ = json.Unmarshal(encoded, &normalized)
			if !reflect.DeepEqual(normalized, row.Message) {
				t.Fatalf("message differs: %s", encoded)
			}
		})
	}
}
func TestFCMOAuthAndDelivery(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"type": "service_account", "project_id": "synthetic-project", "client_email": "fixture@synthetic-project.iam.gserviceaccount.com", "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))})
	p, err := newFCMPusher(data)
	if err != nil {
		t.Fatal(err)
	}
	oauth, deliveries := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			oauth++
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			parts := strings.Split(r.Form.Get("assertion"), ".")
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || len(parts) != 3 {
				t.Error("missing signed OAuth assertion")
				w.WriteHeader(400)
				return
			}
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			signature, err := base64.RawURLEncoding.DecodeString(parts[2])
			if err != nil || rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature) != nil {
				t.Error("invalid RS256 assertion")
			}
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				t.Fatal(err)
			}
			var claims struct {
				Iss, Scope, Aud string
				Iat, Exp        int64
			}
			if err := json.Unmarshal(payload, &claims); err != nil {
				t.Fatal(err)
			}
			if claims.Iss != "fixture@synthetic-project.iam.gserviceaccount.com" || claims.Scope != "https://www.googleapis.com/auth/firebase.messaging" || claims.Aud != "https://oauth2.googleapis.com/token" || claims.Exp-claims.Iat != 3600 || claims.Iat < time.Now().Unix()-60 || claims.Iat > time.Now().Unix()+60 {
				t.Error("OAuth claim scope, audience or lifetime differs")
			}
			writeJSON(w, 200, map[string]any{"access_token": "synthetic-oauth-token", "token_type": "Bearer", "expires_in": 3600})
		case "/send":
			deliveries++
			if r.Header.Get("Authorization") != "Bearer synthetic-oauth-token" {
				t.Error("provider token missing")
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["message"] == nil {
				t.Error("missing FCM message")
			}
			writeJSON(w, 200, map[string]string{"name": "synthetic-message"})
		default:
			t.Error("unexpected endpoint")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	p.oauthURL = server.URL + "/token"
	p.sendURL = server.URL + "/send"
	for range 2 {
		if err := p.Push(context.Background(), pushRequest{Token: "synthetic-fcm", Pane: "%12", Kind: "waiting"}); err != nil {
			t.Fatal(err)
		}
	}
	if oauth != 1 || deliveries != 2 {
		t.Fatalf("oauth=%d deliveries=%d", oauth, deliveries)
	}
}
func TestFCMRejectsUnsupportedOrOverlongIntents(t *testing.T) {
	for _, req := range []pushRequest{{Token: "t", LiveActivity: true}, {Token: "t", Body: strings.Repeat("x", 5000)}, {Token: ""}} {
		if _, err := fcmMessage(req); err == nil {
			t.Fatal("invalid FCM intent accepted")
		}
	}
	if _, err := newFCMPusher([]byte(`{"type":"service_account","project_id":"../../elsewhere"}`)); err == nil {
		t.Fatal("invalid provider configuration accepted")
	}
}
