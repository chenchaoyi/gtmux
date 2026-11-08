package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ToolCall describes recorded evidence, never an execution capability. Body fields
// are omitted from polls and materialized only by the scoped ToolInput reader.
type ToolCall struct {
	Provider     string `json:"provider,omitempty"`
	Name         string `json:"name,omitempty"`
	Status       string `json:"status,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	HasArguments bool   `json:"has_arguments,omitempty"`
	HasResult    bool   `json:"has_result,omitempty"`
	Ref          string `json:"ref,omitempty"`
	Loaded       bool   `json:"loaded,omitempty"`
	Arguments    string `json:"arguments,omitempty"`
	Result       string `json:"result,omitempty"`
	Redacted     bool   `json:"redacted,omitempty"`
}

type recordedValue struct {
	capture bool
	present bool
	digest  string
	raw     []byte
}

func (v *recordedValue) UnmarshalJSON(data []byte) error {
	v.present = true
	sum := sha256.Sum256(data)
	v.digest = hex.EncodeToString(sum[:])
	if v.capture {
		v.raw = append([]byte(nil), data...)
	}
	return nil
}

func recordedCall(provider, name, status string, secs, nanos int64, args json.RawMessage, result recordedValue, full bool) (*ToolCall, string) {
	sum := sha256.Sum256(args)
	identity := hex.EncodeToString(sum[:]) + result.digest + provider + "\x00" + name + "\x00" + status
	c := &ToolCall{Provider: clip(provider, 80), Name: clip(name, 160), Status: clip(status, 40), HasArguments: len(args) > 0, HasResult: result.present, Loaded: full}
	if secs >= 0 && secs < 86400*365 && nanos >= 0 && nanos < 1000000000 {
		c.DurationMS = secs*1000 + nanos/1000000
	}
	if full {
		c.Arguments, c.Redacted = safeRecordedJSON(args)
		var redacted bool
		c.Result, redacted = safeRecordedJSON(result.raw)
		c.Redacted = c.Redacted || redacted
	}
	return c, identity
}

// Full call records can contain service credentials alongside ordinary arguments.
// Remove recognized credential fields recursively; do not treat token counts or
// user code as credentials. Omit binary MCP blocks rather than rendering base64.
var recordedCredentialKey = strings.NewReplacer("_", "", "-", "", " ", "")

func safeRecordedJSON(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value any
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return "", false
	}
	redacted := false
	var clean func(any, int) any
	clean = func(value any, depth int) any {
		if depth > 128 {
			redacted = true
			return "[nesting limit]"
		}
		switch v := value.(type) {
		case map[string]any:
			binary := v["type"] == "image" || v["type"] == "audio"
			for k, item := range v {
				key := strings.ToLower(recordedCredentialKey.Replace(k))
				switch key {
				case "token", "accesstoken", "refreshtoken", "idtoken", "apikey", "password", "passwd", "authorization", "cookie", "setcookie", "secret", "clientsecret", "privatekey", "credential", "credentials":
					v[k] = "[redacted]"
					redacted = true
					continue
				}
				if binary && k == "data" {
					v[k] = "[binary content omitted]"
					redacted = true
					continue
				}
				v[k] = clean(item, depth+1)
			}
		case []any:
			for i, item := range v {
				v[i] = clean(item, depth+1)
			}
		}
		return value
	}
	value = clean(value, 0)
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(encoded), redacted
}
