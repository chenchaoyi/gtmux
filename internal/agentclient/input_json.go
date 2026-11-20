package agentclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

var errInvalidInputJSON = errors.New("invalid terminal input metadata")

// The DSH bridge and preflight use the same decoded-key and nesting rules.
func checkInputJSON(data []byte) error {
	if !utf8.Valid(data) || !validInputEscapes(data) {
		return errInvalidInputJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := visitInputJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errInvalidInputJSON
	}
	return nil
}

// encoding/json replaces unpaired surrogate escapes. For persisted input,
// reject them so the receivers cannot interpret the same bytes differently.
func validInputEscapes(data []byte) bool {
	for i := 0; i+1 < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if data[i] != 'u' {
			continue
		}
		if i+5 > len(data) {
			return false
		}
		unit, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		i += 4
		if err != nil || unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+7 > len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func visitInputJSON(decoder *json.Decoder, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return errInvalidInputJSON
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	if depth == 64 || (delim != '{' && delim != '[') {
		return errInvalidInputJSON
	}
	seen := make(map[string]bool)
	for decoder.More() {
		if delim == '{' {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return errInvalidInputJSON
			}
			seen[name] = true
		}
		if err := visitInputJSON(decoder, depth+1); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return errInvalidInputJSON
	}
	return nil
}

func readInputBytes(path string, limit int64, private bool) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	return readOpenInputBytes(f, limit, private)
}

func readOpenInputBytes(f *os.File, limit int64, private bool) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() || !info.Mode().IsRegular() || info.Size() > limit || info.Mode().Perm()&0022 != 0 {
		return nil, errInvalidInputJSON
	}
	if private && (info.Mode().Perm()&0077 != 0 || st.Nlink != 1) {
		return nil, errInvalidInputJSON
	}
	return readInputSnapshot(f, info.Size())
}

// Read one extra byte to detect growth, and reject a short snapshot even if
// the bytes still form valid JSON.
func readInputSnapshot(file io.Reader, size int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != size {
		return nil, errInvalidInputJSON
	}
	return data, nil
}

func readInputDiscovery(path string, value any) error {
	data, err := readInputBytes(path, 256<<10, false)
	if err != nil {
		return err
	}
	if err := checkInputJSON(data); err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

type inputRecordFields struct {
	names    []string
	required int
}

var requestFields = inputRecordFields{
	names:    []string{"version", "request_id", "instance", "home_id", "session_id", "pid", "pane", "server_epoch", "binding_epoch", "created_at", "text", "text_hash", "durable_retry", "index_key"},
	required: 11,
}
var receiptFields = inputRecordFields{
	names:    []string{"version", "request_id", "instance", "home_id", "session_id", "pid", "state", "delivery", "error", "created_at"},
	required: 7,
}
var attachmentFields = inputRecordFields{
	names: []string{"version", "agent", "client", "pane", "pid", "instance", "home_id", "session_id", "cwd", "server_epoch", "binding_epoch", "started_at", "updated_at", "input_ready", "native_session", "input_store_generation"},
}

func decodeInputRecord(data []byte, value any, fields inputRecordFields) error {
	if err := checkInputJSON(data); err != nil {
		return err
	}
	var shape map[string]json.RawMessage
	if json.Unmarshal(data, &shape) != nil || shape == nil {
		return errInvalidInputJSON
	}
	for i, name := range fields.names {
		raw, present := shape[name]
		if !present {
			if i < fields.required {
				return errInvalidInputJSON
			}
			continue
		}
		if bytes.Equal(raw, []byte("null")) {
			return errInvalidInputJSON
		}
		switch name {
		case "version", "pid", "binding_epoch", "created_at", "started_at", "updated_at":
			const maxSafeInteger = 1<<53 - 1
			value, err := strconv.ParseInt(string(raw), 10, 64)
			if err != nil || value < -maxSafeInteger || value > maxSafeInteger {
				return errInvalidInputJSON
			}
		}
		delete(shape, name)
	}
	if len(shape) != 0 {
		return errInvalidInputJSON
	}
	return json.Unmarshal(data, value)
}

func readInputRecord(path string, value any, limit int64, fields inputRecordFields) error {
	data, err := readInputBytes(path, limit, true)
	if err != nil {
		return err
	}
	return decodeInputRecord(data, value, fields)
}
