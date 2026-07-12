package limits

import (
	"testing"
	"time"
)

const sample = `You are currently using your subscription to power your Claude Code usage

Current session: 11% used · resets Jul 13 at 1:30am (Asia/Shanghai)
Current week (all models): 58% used · resets Jul 17 at 10:59pm (Asia/Shanghai)
Current week (Fable): 88% used · resets Jul 17 at 10:59pm (Asia/Shanghai)

What's contributing to your limits usage?
  99% of your usage came from subagent-heavy sessions
  93% of your usage was at >150k context
`

func TestParse(t *testing.T) {
	w := parse(sample)
	if len(w) != 3 {
		t.Fatalf("want 3 windows, got %d: %+v", len(w), w)
	}
	if w[0].Label != "session" || w[0].PctUsed != 11 || w[0].ResetAt != "Jul 13 at 1:30am" {
		t.Errorf("session window = %+v", w[0])
	}
	if w[1].Label != "week (all models)" || w[1].PctUsed != 58 {
		t.Errorf("week window = %+v", w[1])
	}
	if w[2].Label != "week (fable)" || w[2].PctUsed != 88 {
		t.Errorf("model window = %+v", w[2])
	}
	// the "99% of your usage" prose must NOT parse as a window
	for _, x := range w {
		if x.PctUsed == 99 || x.PctUsed == 93 {
			t.Errorf("prose leaked into windows: %+v", x)
		}
	}
}

func TestParseGarbled(t *testing.T) {
	if w := parse("total nonsense\nno percentages here"); len(w) != 0 {
		t.Errorf("garbled → %+v, want none", w)
	}
}

func TestWarnOf(t *testing.T) {
	wins := []Window{
		{Label: "session", PctUsed: 95}, // session excluded even at 95%
		{Label: "week (all models)", PctUsed: 58},
		{Label: "week (fable)", PctUsed: 88},
	}
	if got := warnOf(wins, 85); got != "week (fable) 88%" {
		t.Errorf("warnOf = %q", got)
	}
	if got := warnOf(wins, 90); got != "" {
		t.Errorf("warnOf(90) = %q, want empty", got)
	}
}

func TestTTLNearAware(t *testing.T) {
	cfg := DefaultConfig
	calm := Report{Windows: []Window{{Label: "week", PctUsed: 40}}}
	if ttl(calm, cfg) != 15*time.Minute {
		t.Errorf("calm ttl = %v", ttl(calm, cfg))
	}
	near := Report{Windows: []Window{{Label: "week", PctUsed: 72}}}
	if ttl(near, cfg) != 5*time.Minute {
		t.Errorf("near ttl = %v", ttl(near, cfg))
	}
}

func TestFresh(t *testing.T) {
	cfg := DefaultConfig
	now := time.Unix(1_000_000, 0)
	r := Report{Windows: []Window{{PctUsed: 40}}, At: now.Add(-10 * time.Minute).Unix()}
	if !Fresh(r, cfg, now) {
		t.Error("10m-old calm cache should be fresh (15m TTL)")
	}
	r.At = now.Add(-20 * time.Minute).Unix()
	if Fresh(r, cfg, now) {
		t.Error("20m-old cache should be stale")
	}
	// near-cap shortens the TTL to 5m
	rn := Report{Windows: []Window{{PctUsed: 80}}, At: now.Add(-7 * time.Minute).Unix()}
	if Fresh(rn, cfg, now) {
		t.Error("7m-old near-cap cache should be stale (5m TTL)")
	}
}
