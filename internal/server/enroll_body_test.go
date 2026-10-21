package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

type enrollCountingBody struct {
	io.Reader
	read int
}

func (b *enrollCountingBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}

func (*enrollCountingBody) Close() error { return nil }

func TestEnrollBodySizeBoundary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Pin the public 4 KiB contract independently of the implementation constant.
	for _, framing := range []string{"known", "unknown", "chunked", "understated"} {
		for _, size := range []int{4095, 4096, 4097} {
			t.Run(framing+"/"+strconv.Itoa(size), func(t *testing.T) {
				en := NewEnrollManager(nil, nil)
				code := en.Mint()
				h := New(Config{Token: "master"}, Deps{Enroll: en}).Handler()
				// Forty astral Unicode characters, escaped as surrogate pairs, still
				// fit comfortably. Padding follows a complete JSON value on purpose.
				payload := `{"enrollCode":"` + code + `","name":"` + strings.Repeat(`\ud83d\udcf1`, 40) + `"}`
				payload += strings.Repeat(" ", size-len(payload))
				body := &enrollCountingBody{Reader: strings.NewReader(payload)}
				r := httptest.NewRequest(http.MethodPost, "/api/enroll", body)
				switch framing {
				case "known":
					r.ContentLength = int64(size)
				case "unknown":
					r.ContentLength = -1
				case "chunked":
					r.ContentLength = -1
					r.TransferEncoding = []string{"chunked"}
				case "understated":
					r.ContentLength = 1
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if size > 4096 {
					if w.Code != http.StatusRequestEntityTooLarge || strings.TrimSpace(w.Body.String()) != `{"error":"request too large"}` {
						t.Fatalf("oversized enrollment = %d %s", w.Code, w.Body.String())
					}
					if len(en.Devices()) != 0 {
						t.Fatal("oversized request created a device")
					}
					// The rejected request must not spend its valid code.
					w = post(t, h, "/api/enroll", "", `{"enrollCode":"`+code+`"}`)
				}
				var got struct{ Token, Scope string }
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != http.StatusOK || got.Scope != "owner" || !en.ValidToken(got.Token) {
					t.Fatalf("valid enrollment = %d %s (%v)", w.Code, w.Body.String(), err)
				}
				if size <= 4096 && en.Devices()[0].Name != strings.Repeat("📱", 40) {
					t.Fatal("valid escaped device label changed")
				}
			})
		}
	}
}

func TestEnrollOversizedStreamStopsReading(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, shape := range []string{"code", "name", "unknown-field", "trailing-space"} {
		t.Run(shape, func(t *testing.T) {
			en := NewEnrollManager(nil, nil)
			code := en.Mint()
			prefix := `{"enrollCode":"` + code + `"`
			padding := strings.Repeat("x", 64*1024)
			var payload string
			switch shape {
			case "code":
				payload = `{"enrollCode":"` + padding + `"}`
			case "name":
				payload = prefix + `,"name":"` + padding + `"}`
			case "unknown-field":
				payload = prefix + `,"extra":"` + padding + `"}`
			case "trailing-space":
				payload = prefix + `}` + strings.Repeat(" ", len(padding))
			}
			body := &enrollCountingBody{Reader: strings.NewReader(payload)}
			r := httptest.NewRequest(http.MethodPost, "/api/enroll", body)
			r.ContentLength = -1
			r.TransferEncoding = []string{"chunked"}
			w := httptest.NewRecorder()
			New(Config{Token: "master"}, Deps{Enroll: en}).Handler().ServeHTTP(w, r)
			if w.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("oversized %s = %d %s", shape, w.Code, w.Body.String())
			}
			if body.read > 4097 {
				t.Errorf("read %d bytes; must stop after the limit plus one overflow byte", body.read)
			}
			if len(en.Devices()) != 0 {
				t.Fatal("oversized request created a device")
			}
		})
	}
}

func TestEnrollBodyLimitPreservesThrottle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	en := NewEnrollManager(nil, nil)
	s := New(Config{Token: "master"}, Deps{Enroll: en})
	s.redeem.now = func() time.Time { return now }
	h := s.Handler()
	code := en.Mint()
	call := func(payload string, want int) {
		t.Helper()
		w := post(t, h, "/api/enroll", "", payload)
		if w.Code != want {
			t.Fatalf("enrollment = %d %s, want %d", w.Code, w.Body.String(), want)
		}
	}
	oversized := `{"enrollCode":"` + code + `"}` + strings.Repeat(" ", 4096)
	// Invalid bodies do not count as failed code guesses.
	for i := 0; i < 11; i++ {
		call(oversized, http.StatusRequestEntityTooLarge)
	}
	call(`{"enrollCode":"`+code+`"}`, http.StatusOK)
	for i := 0; i < 10; i++ {
		call(`{"enrollCode":"wrong"}`, http.StatusUnauthorized)
	}
	code = en.Mint()
	call(`{"enrollCode":"`+code+`"}`, http.StatusTooManyRequests)
	call(oversized, http.StatusRequestEntityTooLarge)
	call(`{`, http.StatusBadRequest)
	now = now.Add(time.Minute + time.Second)
	call(`{"enrollCode":"`+code+`"}`, http.StatusOK)
}

func TestEnrollRejectsIncompleteOrInvalidBodyBeforeRedemption(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	en := NewEnrollManager(nil, nil)
	code := en.Mint()
	payload := `{"enrollCode":"` + code + `"}`
	h := New(Config{Token: "master"}, Deps{Enroll: en}).Handler()
	for _, body := range []io.Reader{
		strings.NewReader(""),
		strings.NewReader(`{`),
		strings.NewReader(`{"name":"phone"}`),
		strings.NewReader(payload + `{}`),
		io.MultiReader(strings.NewReader(payload), iotest.ErrReader(io.ErrUnexpectedEOF)),
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/enroll", body))
		if w.Code != http.StatusBadRequest || strings.TrimSpace(w.Body.String()) != `{"error":"invalid request"}` {
			t.Fatalf("invalid body = %d %s", w.Code, w.Body.String())
		}
		if len(en.Devices()) != 0 {
			t.Fatal("invalid or incomplete body spent the code")
		}
	}
	if w := post(t, h, "/api/enroll", "", payload); w.Code != http.StatusOK {
		t.Fatalf("valid code was lost: %d %s", w.Code, w.Body.String())
	}
}
