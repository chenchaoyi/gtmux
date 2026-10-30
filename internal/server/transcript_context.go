package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Context is opt-in for compatible owner/paired readers. Older clients and guest
// links keep the original public-message shape. A format error fails closed;
// opaque ETags distinguish context-bearing reads from public-message caches.
func traceContextBody(body []byte, meta TranscriptMeta, includeContext bool) ([]byte, TranscriptMeta, error) {
	hasContext := bytes.Contains(body, []byte(`"context"`))
	if meta.Etag != "" && hasContext {
		variant := "context-v1-reader:"
		if !includeContext {
			variant = "context-v1-public:"
		}
		meta.Etag = fmt.Sprintf(`W/"%x"`, sha256.Sum256([]byte(variant+meta.Etag)))
	}
	if includeContext || !hasContext {
		return body, meta, nil
	}
	var turns []map[string]json.RawMessage
	if err := json.Unmarshal(body, &turns); err != nil {
		return nil, meta, err
	}
	for _, turn := range turns {
		raw, exists := turn["segments"]
		if !exists {
			continue
		}
		var segments []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &segments); err != nil {
			return nil, meta, err
		}
		kept := segments[:0]
		for _, segment := range segments {
			_, removed := segment["context"]
			delete(segment, "context")
			if !removed || len(segment) > 0 {
				kept = append(kept, segment)
			}
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return nil, meta, err
		}
		turn["segments"] = encoded
	}
	encoded, err := json.Marshal(turns)
	return encoded, meta, err
}
