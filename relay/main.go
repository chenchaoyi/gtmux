package main

import (
	"log"
	"net/http"
	"os"
)

// main wires the relay from the environment and serves. All secrets come from
// env vars (never committed):
//
//	PORT            listen port (default 8080)
//	GTMUX_RELAY_TOKEN  optional bearer the calling Mac must present on /push
//	APNS_KEY_PATH   path to the Apple AuthKey .p8 (PKCS#8 EC P-256)
//	APNS_KEY_ID     the key's Key ID
//	APNS_TEAM_ID    your Apple Developer Team ID
//	APNS_TOPIC      the app's bundle id (apns-topic)
//	APNS_ENV        "sandbox" → dev endpoint; anything else → production
//
// When the APNS_* vars are absent the relay still starts (health works, /push
// for ios returns "unsupported platform"), so it is safe to deploy first and
// add credentials later.
func main() {
	srv := &relayServer{
		token:   os.Getenv("GTMUX_RELAY_TOKEN"),
		pushers: map[string]Pusher{},
	}

	if p, err := apnsFromEnv(); err != nil {
		log.Fatalf("relay: APNs config error: %v", err)
	} else if p != nil {
		srv.pushers["ios"] = p
		log.Printf("relay: APNs enabled (topic=%s, env=%s)", p.topic, p.baseURL)
	} else {
		log.Printf("relay: APNs not configured (set APNS_* to enable iOS push)")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("relay: listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.handler()); err != nil {
		log.Fatalf("relay: %v", err)
	}
}

// apnsFromEnv builds an APNs pusher when the APNS_* vars are set; returns
// (nil, nil) when unconfigured, (nil, err) on a misconfiguration.
func apnsFromEnv() (*apnsPusher, error) {
	keyPath := os.Getenv("APNS_KEY_PATH")
	keyID := os.Getenv("APNS_KEY_ID")
	teamID := os.Getenv("APNS_TEAM_ID")
	topic := os.Getenv("APNS_TOPIC")
	if keyPath == "" && keyID == "" && teamID == "" && topic == "" {
		return nil, nil // wholly unconfigured → disabled, not an error
	}
	p8, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	base := apnsProd
	if os.Getenv("APNS_ENV") == "sandbox" {
		base = apnsSandbox
	}
	return newAPNSPusher(base, topic, keyID, teamID, p8)
}
