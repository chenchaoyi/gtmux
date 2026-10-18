package main

import "fmt"

// The client uses the gateway's status to retire dead tokens, not the relay's 502.
type apnsFailure struct {
	Status int
	Detail string
}

func (e *apnsFailure) Error() string { return fmt.Sprintf("apns status %d %s", e.Status, e.Detail) }

func (a *apnsPusher) endpoint(env string) string {
	// Keep explicitly injected gateway URLs for tests/self-hosted transports.
	if a.baseURL != apnsProd && a.baseURL != apnsSandbox {
		return a.baseURL
	}
	if env == "" {
		return a.baseURL
	}
	if env == "sandbox" {
		return apnsSandbox
	}
	return apnsProd
}

func apnsRequest(req pushRequest, jwt, topic string, now int64) (map[string]any, map[string]string) {
	headers := map[string]string{"authorization": "bearer " + jwt, "apns-topic": topic, "apns-push-type": "alert", "content-type": "application/json"}
	if req.LiveActivity {
		event := "update"
		if req.Event != nil {
			event = *req.Event
		}
		content := req.ContentState
		if content == nil {
			content = map[string]any{}
		}
		aps := map[string]any{"timestamp": now, "event": event, "content-state": content}
		if req.StaleDate > 0 {
			aps["stale-date"] = req.StaleDate
		}
		headers["apns-topic"] = topic + ".push-type.liveactivity"
		headers["apns-push-type"] = "liveactivity"
		headers["apns-priority"] = "10"
		return map[string]any{"aps": aps}, headers
	}
	if req.Silent {
		headers["apns-push-type"] = "background"
		headers["apns-priority"] = "5"
	}
	if req.CollapseID != "" {
		headers["apns-collapse-id"] = req.CollapseID
	}
	return apnsPayload(req), headers
}
