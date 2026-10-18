package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/server"
)

func TestSharedAPNSContract(t *testing.T) {
	var fixtures []struct {
		Name    string
		Now     int64
		Intent  pushRequest
		Payload map[string]any
		Headers map[string]string
	}
	body, err := os.ReadFile("../api/testdata/apns-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			payload, headers := apnsRequest(f.Intent, "jwt-abc", "com.gtmux.app", f.Now)
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var normalized map[string]any
			if err := json.Unmarshal(body, &normalized); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalized, f.Payload) || !reflect.DeepEqual(headers, f.Headers) {
				t.Fatalf("wire contract differs: %s %#v", body, headers)
			}
		})
	}
}

func TestGoRelayRetiresAPNSGoneThroughRealClient(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(410)
		io.WriteString(w, `{"reason":"Unregistered"}`)
	}))
	defer apple.Close()
	p, err := newAPNSPusher(apple.URL, "com.gtmux.app", "K", "T", mustP8(t, key))
	if err != nil {
		t.Fatal(err)
	}
	relay := httptest.NewServer((&relayServer{pushers: map[string]Pusher{"ios": p}}).handler())
	defer relay.Close()
	err = server.NewHTTPRelay(relay.URL+"/push", "").Send(server.PushIntent{Token: "fixture", Platform: "ios"})
	var gone *server.APNsError
	if !errors.As(err, &gone) || !gone.Gone() || gone.Status != 410 {
		t.Fatalf("dead-token verdict lost: %v", err)
	}
}

func TestAPNSLiveActivityUsesDedicatedWireType(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var topic, kind, priority string
	var payload map[string]any
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic = r.Header.Get("apns-topic")
		kind = r.Header.Get("apns-push-type")
		priority = r.Header.Get("apns-priority")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		w.WriteHeader(200)
	}))
	defer apple.Close()
	p, err := newAPNSPusher(apple.URL, "com.gtmux.app", "K", "T", mustP8(t, key))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Push(context.Background(), pushRequest{Token: "fixture", LiveActivity: true, ContentState: map[string]any{"waiting": 2}, StaleDate: 1700002400}); err != nil {
		t.Fatal(err)
	}
	aps, _ := payload["aps"].(map[string]any)
	if topic != "com.gtmux.app.push-type.liveactivity" || kind != "liveactivity" || priority != "10" || aps["event"] != "update" || aps["alert"] != nil {
		t.Fatalf("live activity degraded to an alert: %s %s %s %#v", topic, kind, priority, payload)
	}
	for _, base := range []string{apnsProd, apnsSandbox} {
		p.baseURL = base
		if p.endpoint("sandbox") != apnsSandbox || p.endpoint("production") != apnsProd || p.endpoint("") != base {
			t.Fatal("per-token environment was ignored")
		}
	}
}
