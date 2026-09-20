package server

import (
	"sync"
	"time"
)

// redeemLimiter bounds how fast a code can be GUESSED.
//
// A share link's code is short enough to read out loud (eight characters, 40 bits), which
// is only a safe trade when nobody can sit there trying codes: a trillion combinations
// means nothing against an attacker allowed a thousand tries a second. So failed redeems
// are counted, per caller and in total, and once either line is crossed the endpoint stops
// answering for a while. The two decisions are one: shortening the code without tightening
// this is not a safe edit (see shareCodeLen).
//
// Only FAILURES count. A person typing a code they were given wrong twice is not what
// this is for, which is why the per-caller allowance is far above a human typo rate and
// the window is short.
type redeemLimiter struct {
	mu     sync.Mutex
	byIP   map[string][]int64
	all    []int64
	now    func() time.Time
	window time.Duration
	perIP  int
	total  int
}

func newRedeemLimiter() *redeemLimiter {
	return &redeemLimiter{
		byIP:   map[string][]int64{},
		now:    time.Now,
		window: time.Minute,
		// A human retyping a code they misheard needs a handful; a thousand a second
		// needs none of these.
		perIP: 10,
		// Behind a tunnel every caller can share one address, so the per-caller line
		// cannot be the only one — this is the line for everyone at once.
		total: 60,
	}
}

// allow reports whether a redeem attempt may proceed, without recording anything.
func (l *redeemLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked()
	return len(l.byIP[ip]) < l.perIP && len(l.all) < l.total
}

// failed records a refused attempt. A successful redeem records nothing: the code is
// spent, and the person who used it should not be closer to a lockout for having used it.
func (l *redeemLimiter) failed(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked()
	now := l.now().UnixMilli()
	l.byIP[ip] = append(l.byIP[ip], now)
	l.all = append(l.all, now)
}

func (l *redeemLimiter) pruneLocked() {
	cut := l.now().Add(-l.window).UnixMilli()
	keep := l.all[:0]
	for _, t := range l.all {
		if t > cut {
			keep = append(keep, t)
		}
	}
	l.all = keep
	for ip, ts := range l.byIP {
		k := ts[:0]
		for _, t := range ts {
			if t > cut {
				k = append(k, t)
			}
		}
		if len(k) == 0 {
			delete(l.byIP, ip)
			continue
		}
		l.byIP[ip] = k
	}
}
