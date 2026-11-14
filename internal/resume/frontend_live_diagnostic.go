package resume

import "errors"

// LoadLiveDiagnostic reports the existing live predicate with value-free reasons.
// A reason grants no authority, repairs no record and performs no retry.
func LoadLiveDiagnostic(loc string) (Record, bool, string) {
	r, ok := Load(loc)
	if !ok {
		return r, false, "resume-record-unavailable"
	}
	if r.Frontend == "" {
		return r, true, ""
	} // preserve legacy LoadLive semantics
	f, exists := readFrontend(r.Frontend)
	if !exists {
		return r, false, "frontend-record-unavailable"
	}
	if f.Loc != loc || f.HomeID != r.HomeID || f.SessionID != r.SessionID {
		return r, false, "frontend-index-mismatch"
	}
	if err := frontendProof(f, false); err != nil {
		var evidence *FrontendBindingError
		if errors.As(err, &evidence) {
			return r, false, evidence.Code
		}
		return r, false, "frontend-proof-unknown"
	}
	return r, true, ""
}
