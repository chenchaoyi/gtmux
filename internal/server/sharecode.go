package server

import (
	"crypto/rand"
	"strings"
)

// A share link's CODE: the link itself, short enough to read out loud.
//
// The link and the code used to be two artifacts, and the owner had to choose between
// them: a link that lasts until it is revoked, or a code that lasted ten minutes and once.
// The access behind them was identical, so the only thing that choice bought was how long
// the hand-off stayed valid, and a reader had to hold both in their head to understand
// either (「我还是没理解两个链接的必要性」, 2026-09-20).
//
// They are one thing now. Every guest link is minted with a code beside its token, lasting
// exactly as long as the link does, and the code is the link's public form:
//
//	https://tunnel.example.dev/p35047#code=4F7K-Q9X2   click, or paste
//	https://tunnel.example.dev/p35047 + 4F7K-Q9X2      or say it as two lines
//
// Redeeming trades it for the link's own token, which the browser and the terminal keep,
// so the code is not presented on every request. It creates nothing and can never widen a
// scope, and revoking the link ends it.

// shareCodeLen is how many characters are minted, in two groups of four.
//
// Eight is the shortest length this can defend, and the reason is that the code now LASTS.
// Its length and the bound on guessing are one decision (redeemLimiter): at 60 failed
// tries a minute an attacker gets about 31.5 million tries a year, which against 40 bits
// is one chance in 35,000 per year per live link. Six characters, which is what a
// ten-minute code could afford, would be one in 34.
const shareCodeLen = 8

// shareCodeAlphabet is Crockford base32: no I, L, O or U, so nothing in a code can be
// misheard as something else, and the one letter that could be a digit is not in it.
const shareCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newShareCode returns a fresh code in its grouped display form ("4F7K-Q9X2").
func newShareCode() string {
	b := make([]byte, shareCodeLen)
	_, _ = rand.Read(b)
	out := make([]byte, 0, shareCodeLen+1)
	for i, x := range b {
		if i == shareCodeLen/2 {
			out = append(out, '-')
		}
		out = append(out, shareCodeAlphabet[int(x)%len(shareCodeAlphabet)])
	}
	return string(out)
}

// normalizeShareCode is how a typed code is matched: case is ignored, the separators a
// person adds or drops are ignored, and the three characters Crockford maps are mapped
// (someone reading a code aloud says "oh" for 0 and the listener types the letter).
func normalizeShareCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		switch r {
		case '-', ' ', '\t', '_':
			continue
		case 'I', 'L':
			b.WriteRune('1')
		case 'O':
			b.WriteRune('0')
		case 'U':
			b.WriteRune('V')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ShareCode returns a guest link's code, and whether that link exists and is live. It
// mints nothing for a link that has one: the code was created with the link.
func (m *EnrollManager) ShareCode(deviceID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, d := range m.devices {
		if d.ID != deviceID || d.Scope != "guest" {
			continue
		}
		if d.ExpiresAt > 0 && m.now().Unix() > d.ExpiresAt {
			return "", false
		}
		if d.Code == "" {
			// A link minted before codes existed gets one now, so an old link can be
			// handed over the same way as a new one.
			d.Code = newShareCode()
			m.devices[tok] = d
			snap := m.devicesLocked()
			if m.save != nil {
				go m.save(snap)
			}
		}
		return d.Code, true
	}
	return "", false
}

// redeemShareCodeLocked answers a typed code with the link it belongs to. The caller holds
// the lock. handled=false means the code is not one of these, and the caller falls through
// to the owner pairing codes, so one endpoint keeps taking both kinds.
//
// An expired link answers as expired rather than unknown: the person holding the code did
// have something real, and "it ran out" is the fact they need.
func (m *EnrollManager) redeemShareCodeLocked(typed string) (EnrolledDevice, string, bool) {
	key := normalizeShareCode(typed)
	if key == "" {
		return EnrolledDevice{}, "", false
	}
	for _, d := range m.devices {
		if d.Scope != "guest" || d.Code == "" || normalizeShareCode(d.Code) != key {
			continue
		}
		if d.ExpiresAt > 0 && m.now().Unix() > d.ExpiresAt {
			return EnrolledDevice{}, RedeemExpired, true
		}
		return d, "", true
	}
	return EnrolledDevice{}, "", false
}
