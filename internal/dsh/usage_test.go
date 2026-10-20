package dsh

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func usageEvent(kind string, at int64, data string) Event {
	return Event{Type: kind, Time: at, Data: json.RawMessage(data)}
}

func TestUsageRestatementsRetriesAndDisjointBuckets(t *testing.T) {
	var u TokenUsage
	at := time.Now().Add(-time.Minute).UnixMilli()
	data := func(out int) string {
		return fmt.Sprintf(`{"turn":1,"step":0,"usage":{"inputTokens":100,"outputTokens":%d,"cacheReadTokens":200,"cacheWriteTokens":50,"reasoningTokens":7}}`, out)
	}
	u.Fold(usageEvent("assistant/attempt", at, data(10)))
	u.Fold(usageEvent("assistant/message", at+1, data(10)))
	u.Fold(usageEvent("assistant/message", at+2, data(15)))
	if u.Total != (TokenBuckets{100, 15, 200, 50}) {
		t.Fatal("restatement counted twice or reasoning added", u.Total)
	}
	u.Fold(usageEvent("llm/retry-started", at+3, `{"turn":1,"step":0}`))
	u.Fold(usageEvent("assistant/attempt", at+4, data(5)))
	if u.Total.Output != 20 || u.Total.Input != 200 {
		t.Fatal("retry did not contribute independently", u.Total)
	}
	day := time.UnixMilli(at).Local().Format("2006-01-02")
	if u.Days[day] != u.Total {
		t.Fatal("daily fold disagrees", u.Days)
	}
	if u.Rate(time.UnixMilli(at+60000)) != 20 {
		t.Fatal("rate counts restatements", u.Rate(time.Now()))
	}
	u.Fold(usageEvent("request/context", at+5, `{"contextWindow":10000}`))
	if u.Pressure != 350 || u.Window != 10000 {
		t.Fatal("cache pressure or native window lost", u)
	}
	u.Fold(usageEvent("request/context", at+6, `{}`))
	if u.Window != 0 {
		t.Fatal("old context window survived an unspecified new one")
	}
}

func TestUsageUsesLastRawStreamSampleAndRejectsInvalidCounts(t *testing.T) {
	var u TokenUsage
	u.Fold(usageEvent("assistant/attempt", 1000, `{"turn":0,"step":1,"stream":[{"type":"chunk","chunk":{"type":"usage","usage":{"inputTokens":3,"outputTokens":4}}},{"type":"chunk","chunk":{"type":"usage","usage":{"inputTokens":5,"outputTokens":6}}}]}`))
	if u.Total.Input != 5 || u.Total.Output != 6 {
		t.Fatal(u.Total)
	}
	for _, data := range []string{
		`{"turn":0,"step":2,"usage":{"inputTokens":-1,"outputTokens":1}}`,
		`{"turn":0,"step":2,"usage":{"inputTokens":1.5,"outputTokens":1}}`,
		`{"turn":0,"step":2,"usage":{"inputTokens":1,"outputTokens":9007199254740992}}`,
	} {
		u.Fold(usageEvent("assistant/message", 2000, data))
	}
	if u.Total.Output != 6 {
		t.Fatal("invalid record contributed", u.Total)
	}
}

func TestCatalogUsageIsIncrementalScopedAndDetached(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	part := `{"type":"assistant/message","time":3000,"data":{"turn":0,"step":0,"usage":{"inputTokens":12,"outputTokens":8}}}` + "\n"
	path := artifact(t, "workspace", "one", ".zstd", head("one"), promptEvent, part)
	c := &Catalog{}
	u, err := c.Usage("one")
	if err != nil || u.Total.Output != 8 {
		t.Fatal(u, err)
	}
	u.Days[time.UnixMilli(3000).Local().Format("2006-01-02")] = TokenBuckets{Output: 999}
	before := c.entries[path].cursor.Offset
	u, err = c.Usage("one")
	if err != nil || c.entries[path].cursor.Offset != before || u.Total.Output != 8 {
		t.Fatal("unchanged log rescanned or snapshot mutated", u, err)
	}
	for _, v := range u.Days {
		if v.Output == 999 {
			t.Fatal("caller mutated cached projection")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Usage("one"); err == nil {
		t.Fatal("missing source returned cached usage")
	}
	t.Setenv("DSH_HOME", t.TempDir())
	artifact(t, "workspace", "one", "", head("one"), promptEvent)
	u, err = c.Usage("one")
	if err != nil || u.Seen {
		t.Fatal("another home borrowed usage", u, err)
	}
}
