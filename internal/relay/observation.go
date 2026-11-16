package relay

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

// IsObservationExport recognizes a complete, nonempty relay-list export. It
// never returns records or bodies to a caller. The caller bounds the reader and
// verifies file identity; unknown, partial or ordinary structured notes stay data.
func IsObservationExport(input io.Reader) bool {
	d := json.NewDecoder(input)
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		return false
	}
	count := 0
	for d.More() {
		if count >= 10000 {
			return false
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil || !completeObservation(raw) {
			return false
		}
		count++
	}
	if token, err = d.Token(); err != nil || token != json.Delim(']') || count == 0 {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}

func completeObservation(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	var r Request
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || !validRequestInput(r) || r.CreatedAt <= 0 ||
		r.For != "hq" && r.For != "user" || !completeSerializedObject(raw, reflect.TypeOf(r)) {
		return false
	}
	switch r.State {
	case "pending", "claimed", "replied", "closed":
		return true
	default:
		return false
	}
}

// Required/known keys come from the owning serialized types, including Notice.
// The typed decoder checks values; this pass rejects missing zero-valued fields,
// unknown spelling, duplicate keys and nulls that a struct decode would discard.
func completeSerializedObject(raw []byte, typ reflect.Type) bool {
	fields := make(map[string]reflect.StructField, typ.NumField())
	required := make(map[string]bool, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := strings.Split(f.Tag.Get("json"), ",")
		name := tag[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name], required[name] = f, true
		for _, option := range tag[1:] {
			if option == "omitempty" || option == "omitzero" {
				required[name] = false
			}
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool, len(fields))
	for d.More() {
		key, err := d.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return false
		}
		field, known := fields[name]
		if !known {
			return false
		}
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
		seen[name] = true
		fieldType := field.Type
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !completeSerializedObject(value, fieldType) {
			return false
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return false
	}
	for name, needed := range required {
		if needed && !seen[name] {
			return false
		}
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}
