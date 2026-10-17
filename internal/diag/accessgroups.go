package diag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type AccessFilter struct {
	Actor     string `json:"actor,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	Operation string `json:"operation,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	GroupBy   string `json:"group_by,omitempty"`
}

func (q AccessFilter) valid() bool {
	for _, v := range []string{q.Actor, q.DeviceID} {
		if len(v) > 128 || strings.ContainsAny(v, "\r\n") {
			return false
		}
	}
	if q.Operation != "" {
		if _, ok := accessOperations[q.Operation]; !ok {
			return false
		}
	}
	return (q.Outcome == "" || q.Outcome == OK || q.Outcome == Refused || q.Outcome == Failed) && (q.GroupBy == "" || q.GroupBy == "device")
}
func (q AccessFilter) key() string {
	b, _ := json.Marshal(q)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (q AccessFilter) matches(e AccessEntry) bool {
	return (q.Actor == "" || e.Actor == q.Actor || e.DeviceID == q.Actor) && (q.DeviceID == "" || e.DeviceID == q.DeviceID) && (q.Operation == "" || e.Operation == q.Operation) && (q.Outcome == "" || e.Outcome == q.Outcome)
}

type AccessOperation struct {
	Value string `json:"value"`
	EN    string `json:"en"`
	ZH    string `json:"zh"`
}

func AccessOperations() []AccessOperation {
	out := make([]AccessOperation, 0, len(accessOperations))
	for key, labels := range accessOperations {
		out = append(out, AccessOperation{key, labels[0], labels[1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

// Counts are recorded activities, not request totals: successful reads are sampled.
// Metadata is from the most recent record, including revoked/historically renamed devices.
type AccessGroup struct {
	Key      string `json:"key"`
	Actor    string `json:"actor"`
	DeviceID string `json:"device_id,omitempty"`
	Name     string `json:"name"`
	Platform string `json:"platform,omitempty"`
	Scope    string `json:"scope"`
	SourceIP string `json:"source_ip,omitempty"`
	Count    int    `json:"count"`
	Accepted int    `json:"accepted"`
	Refused  int    `json:"refused"`
	Failed   int    `json:"failed"`
	FirstAt  string `json:"first_at"`
	LastAt   string `json:"last_at"`
}

func addAccessGroup(out []AccessGroup, index map[string]int, e AccessEntry) []AccessGroup {
	key := "actor:" + e.Actor
	if e.DeviceID != "" {
		key = "device:" + e.DeviceID
	}
	i, ok := index[key]
	if !ok {
		i = len(out)
		index[key] = i
		out = append(out, AccessGroup{Key: key, Actor: e.Actor, DeviceID: e.DeviceID, Name: e.Name, Platform: e.Platform, Scope: e.Scope, SourceIP: e.SourceIP, FirstAt: e.At, LastAt: e.At})
	}
	g := &out[i]
	g.Count++
	switch e.Outcome {
	case OK:
		g.Accepted++
	case Refused:
		g.Refused++
	case Failed:
		g.Failed++
	}
	at, _ := time.Parse(time.RFC3339Nano, e.At)
	first, _ := time.Parse(time.RFC3339Nano, g.FirstAt)
	last, _ := time.Parse(time.RFC3339Nano, g.LastAt)
	if at.Before(first) {
		g.FirstAt = e.At
	}
	if at.After(last) {
		g.LastAt = e.At
		g.Name = e.Name
		g.Platform = e.Platform
		g.SourceIP = e.SourceIP
		g.Actor = e.Actor
	}
	return out
}
