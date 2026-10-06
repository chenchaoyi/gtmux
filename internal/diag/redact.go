package diag

import (
	"regexp"
	"strings"
	"sync"
)

// Credentials are replaced where an entry is written, not at each call site. A call
// site that logs a request, an error string or a URL cannot be trusted to have looked
// for a token in it first; the writer can. The serve log held the master token 398
// times before this existed.

// Redacted replaces a credential in an entry.
const Redacted = "‹redacted›"

var (
	secretsMu sync.RWMutex
	secrets   = map[string]bool{}
)

// RegisterSecret makes every later entry replace s wherever it appears. Components
// register the serve token, the relay token, the Direct secret and device tokens as
// they are issued. Values shorter than 8 bytes are ignored: they would match ordinary
// text.
func RegisterSecret(s string) {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return
	}
	secretsMu.Lock()
	secrets[s] = true
	secretsMu.Unlock()
}

// credentialKeys are attribute keys whose value is a credential whatever it holds.
var credentialKeys = map[string]bool{
	"token": true, "secret": true, "password": true, "passwd": true, "auth": true,
	"authorization": true, "cookie": true, "code": true, "enroll_code": true,
	"pair_code": true, "share_code": true, "apikey": true, "api_key": true,
}

// credentialSuffixes catch the rest by shape: device_token, relay_token, direct_secret.
// "_code" is deliberately not one: exit_code and status_code are not credentials.
var credentialSuffixes = []string{"_token", "token", "_secret", "_password"}

// IsCredentialKey reports whether an attribute or JSON key names a credential.
func IsCredentialKey(k string) bool { return isCredentialKey(k) }

func isCredentialKey(k string) bool {
	k = strings.ToLower(k)
	if credentialKeys[k] {
		return true
	}
	for _, s := range credentialSuffixes {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	return false
}

// Shapes that are credentials wherever they appear: the fragments of a pairing or share
// link, and an Authorization header's value.
//
// The fragments are every one a link may still carry, whether it leads the fragment (#)
// or follows another parameter in it (&): `c` (a pairing code), `code` (a share link's
// code, which lasts as long as the link), `g` (a guest token) and the legacy `t` (a
// token, still accepted). Only #c and #g used to be here, so a share link written into
// an entry kept its code (%12, 2026-10-06); the writer must not depend on a caller having
// registered it first. The key stays, so a reader can still see a link was there.
var credentialPatterns = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`([#&](?:code|c|g|t)=)[0-9A-Za-z_.~-]+`), "${1}" + Redacted},
	{regexp.MustCompile(`(?i)\bbearer\s+[^\s"']+`), "Bearer " + Redacted},
	// The scheme word is optional and goes too: "Authorization: Basic <creds>" must not
	// leave <creds> behind after redacting "Basic".
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:[A-Za-z]+\s+)?[^\s"']+`), "${1}" + Redacted},
}

// Redact returns s with every registered secret and every credential shape replaced.
func Redact(s string) string {
	if s == "" {
		return s
	}
	secretsMu.RLock()
	for v := range secrets {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, Redacted)
		}
	}
	secretsMu.RUnlock()
	for _, p := range credentialPatterns {
		s = p.re.ReplaceAllString(s, p.with)
	}
	return s
}

func redactEntry(e *Entry) {
	e.Msg = Redact(e.Msg)
	e.Actor = Redact(e.Actor)
	e.Target = Redact(e.Target)
	for k, v := range e.Attrs {
		if isCredentialKey(k) {
			e.Attrs[k] = Redacted
			continue
		}
		if s, ok := v.(string); ok {
			e.Attrs[k] = Redact(s)
		}
	}
}
