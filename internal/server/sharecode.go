package server

import (
	"crypto/rand"
	"strings"
	"time"
)

// A share link's ONE-TIME CODE: the door for a guest who cannot paste.
//
// A share link hands over its credential in the URL (`#g=<64 hex>`), which is fine
// wherever the guest can paste and unusable where they cannot — a TV browser, a
// locked-down machine, someone else's laptop you are reading your own screen into. gtmux
// already solved that for its OWNER devices and the two halves never met: pairing hands
// over a single-use code with a short life (`Mint`), a guest was handed the token itself.
//
// So a link can also be handed over as a short code bound to THAT link. Redeeming it
// creates nothing — it returns the token the link already has, with its panes and its
// expiry — so a code can never widen a scope, and revoking the link ends every code
// minted for it.

// shareCodeTTL bounds a share code's life. Longer than a pairing code's five minutes,
// because this one is usually read out or sent in a message before it is typed, and
// shorter than anything a person would call "later".
const shareCodeTTL = 10 * time.Minute

// shareCodeLen is the number of characters minted, in two groups. 6 × 5 bits = 30 bits.
//
// 30 bits is only safe because guessing is RATE LIMITED (see redeemLimiter): a billion
// combinations against at most a few hundred tries inside the code's ten minutes is about
// one chance in two million, and the code is single-use besides. Without that limiter the
// same length would be a poor bet, so the two belong together — lengthening this constant
// is the fix if the limiter ever has to go, and 50 bits is what it was before.
const shareCodeLen = 6

// shareCodeAlphabet is Crockford base32: no I, L, O or U, so nothing in a code can be
// misheard as something else, and the one letter that could be a digit is not in it.
const shareCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newShareCode returns a fresh code in its grouped display form ("4F7KQ-9X2TM").
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

// shareCode is a minted code's binding: which link it opens, and until when.
type shareCode struct {
	deviceID string
	exp      int64
}

// MintShareCode issues a one-time code for an existing GUEST device. It refuses anything
// that is not a live share link, so a code can only ever hand over a credential the owner
// has already decided to hand over.
func (m *EnrollManager) MintShareCode(deviceID string) (string, bool) {
	code := newShareCode()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	var found *EnrolledDevice
	for tok, d := range m.devices {
		if d.ID == deviceID && d.Scope == "guest" {
			dd := m.devices[tok]
			found = &dd
			break
		}
	}
	if found == nil {
		return "", false
	}
	if found.ExpiresAt > 0 && m.now().Unix() > found.ExpiresAt {
		return "", false
	}
	if m.shareCodes == nil {
		m.shareCodes = map[string]shareCode{}
	}
	m.shareCodes[normalizeShareCode(code)] = shareCode{
		deviceID: deviceID,
		exp:      m.now().Add(shareCodeTTL).Unix(),
	}
	return code, true
}

// redeemShareCodeLocked answers a typed code with the link's own device. The caller holds
// the lock. An unknown code returns ok=false and the caller falls through to the owner
// pairing codes, so one endpoint keeps taking both kinds.
func (m *EnrollManager) redeemShareCodeLocked(typed string) (EnrolledDevice, string, bool) {
	key := normalizeShareCode(typed)
	sc, ok := m.shareCodes[key]
	if !ok {
		if sp, seen := m.spent[key]; seen {
			return EnrolledDevice{}, sp.why, true
		}
		return EnrolledDevice{}, "", false
	}
	if m.now().Unix() > sc.exp {
		delete(m.shareCodes, key)
		m.spent[key] = spentCode{why: RedeemExpired, at: m.now().Unix()}
		return EnrolledDevice{}, RedeemExpired, true
	}
	for _, d := range m.devices {
		if d.ID != sc.deviceID {
			continue
		}
		// A link that expired or was revoked between minting and typing hands over
		// nothing: the code is a door to a room, not a key of its own.
		if d.ExpiresAt > 0 && m.now().Unix() > d.ExpiresAt {
			delete(m.shareCodes, key)
			m.spent[key] = spentCode{why: RedeemExpired, at: m.now().Unix()}
			return EnrolledDevice{}, RedeemExpired, true
		}
		delete(m.shareCodes, key) // single use
		m.spent[key] = spentCode{why: RedeemUsed, at: m.now().Unix()}
		return d, "", true
	}
	// The link is gone (revoked). Say unknown: there is nothing left to describe.
	delete(m.shareCodes, key)
	m.spent[key] = spentCode{why: RedeemUnknown, at: m.now().Unix()}
	return EnrolledDevice{}, RedeemUnknown, true
}

// pruneShareCodesLocked drops codes that have run out. Called from pruneLocked.
func (m *EnrollManager) pruneShareCodesLocked(now int64) {
	for k, sc := range m.shareCodes {
		if now > sc.exp {
			delete(m.shareCodes, k)
			m.spent[k] = spentCode{why: RedeemExpired, at: now}
		}
	}
}
