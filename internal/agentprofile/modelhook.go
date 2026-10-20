package agentprofile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ReadModelHook streams past Gemini's request messages rather than buffering the
// model context in a short-lived hook process. Only selected scalar metadata survives.
func ReadModelHook(input io.Reader) ([]byte, error) {
	r := bufio.NewReaderSize(io.LimitReader(input, 128<<20), 16<<10)
	wanted := map[string]bool{"session_id": true, "cwd": true, "llm_request.model": true, "llm_request.config.thinkingConfig": true}
	var walk func(string) (map[string]json.RawMessage, error)
	walk = func(prefix string) (map[string]json.RawMessage, error) {
		if e := expectByte(r, '{'); e != nil {
			return nil, e
		}
		out := map[string]json.RawMessage{}
		for {
			b, e := nextByte(r)
			if e != nil {
				return nil, e
			}
			if b == '}' {
				return out, nil
			}
			_ = r.UnreadByte()
			keyRaw, e := jsonValue(r, true)
			if e != nil {
				return nil, e
			}
			var key string
			if json.Unmarshal(keyRaw, &key) != nil {
				return nil, errors.New("invalid metadata key")
			}
			if e = expectByte(r, ':'); e != nil {
				return nil, e
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			parent := false
			for k := range wanted {
				if strings.HasPrefix(k, path+".") {
					parent = true
					break
				}
			}
			if wanted[path] {
				v, e := jsonValue(r, true)
				if e != nil || !json.Valid(v) {
					return nil, errors.New("invalid metadata value")
				}
				out[key] = v
			} else if parent {
				m, e := walk(path)
				if e != nil {
					return nil, e
				}
				out[key], _ = json.Marshal(m)
			} else {
				if _, e := jsonValue(r, false); e != nil {
					return nil, e
				}
			}
			b, e = nextByte(r)
			if e != nil {
				return nil, e
			}
			if b == '}' {
				return out, nil
			}
			if b != ',' {
				return nil, errors.New("invalid metadata delimiter")
			}
		}
	}
	v, e := walk("")
	if e != nil {
		return nil, e
	}
	if _, e = nextByte(r); e != io.EOF {
		return nil, errors.New("trailing hook payload")
	}
	return json.Marshal(v)
}

func nextByte(r *bufio.Reader) (byte, error) {
	for {
		b, e := r.ReadByte()
		if e != nil {
			return 0, e
		}
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			return b, nil
		}
	}
}
func expectByte(r *bufio.Reader, want byte) error {
	b, e := nextByte(r)
	if e != nil {
		return e
	}
	if b != want {
		return errors.New("unexpected metadata structure")
	}
	return nil
}

// jsonValue recognizes a complete value without allocating discarded strings or
// objects. Selected values have a separate small bound and are JSON-validated.
func jsonValue(r *bufio.Reader, keep bool) ([]byte, error) {
	b, e := nextByte(r)
	if e != nil {
		return nil, e
	}
	var out bytes.Buffer
	add := func(b byte) error {
		if !keep {
			return nil
		}
		if out.Len() >= 4096 {
			return errors.New("metadata exceeds limit")
		}
		return out.WriteByte(b)
	}
	if e = add(b); e != nil {
		return nil, e
	}
	quoted := b == '"'
	escaped := false
	depth := 0
	if b == '{' || b == '[' {
		depth = 1
	}
	if !quoted && depth == 0 {
		for {
			b, e = r.ReadByte()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
			if b == ',' || b == '}' || b == ']' || b == ' ' || b == '\n' || b == '\t' || b == '\r' {
				_ = r.UnreadByte()
				break
			}
			if e = add(b); e != nil {
				return nil, e
			}
		}
		return out.Bytes(), nil
	}
	for {
		b, e = r.ReadByte()
		if e != nil {
			return nil, e
		}
		if e = add(b); e != nil {
			return nil, e
		}
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				quoted = false
				if depth == 0 {
					break
				}
			}
			continue
		}
		switch b {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > 64 {
				return nil, errors.New("metadata nesting exceeds limit")
			}
		case '}', ']':
			depth--
			if depth == 0 {
				return out.Bytes(), nil
			}
		}
	}
	return out.Bytes(), nil
}
