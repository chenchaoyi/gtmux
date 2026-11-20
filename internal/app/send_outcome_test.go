package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestSPL009CLIAndHTTPDispatchOutcomes(t *testing.T) {
	const body = "private synthetic payload"
	for _, tc := range []struct {
		name     string
		code     dispatch.InputFailureCode
		boundary dispatch.Submission
		enters   int
	}{
		{"capacity", dispatch.InputSpoolFull, dispatch.SubmissionNotSubmitted, 0},
		{"invalid layout", dispatch.InputUnavailable, dispatch.SubmissionNotSubmitted, 0},
		{"changed target", dispatch.InputTargetChanged, dispatch.SubmissionNotSubmitted, 0},
		{"unknown", dispatch.InputOutcomeUnconfirmed, dispatch.SubmissionUnknown, 1},
		{"accepted", "", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, surface := range []string{"cli", "cli-no-verify", "http"} {
				t.Run(surface, func(t *testing.T) {
					pastes, enters := inputOutcomeFixture(t, tc.name, body)
					if surface != "http" {
						var exit int
						var stdout string
						stderr := captureStderr(t, func() {
							stdout = captureStdout(t, func() {
								args := []string{"%9", "--json"}
								if surface == "cli-no-verify" {
									args = append(args, "--no-verify")
								}
								exit = cmdSend(append(args, body))
							})
						})
						var got sendJSON
						if err := json.Unmarshal([]byte(stdout), &got); err != nil {
							t.Fatalf("invalid CLI JSON: %q, stderr=%q: %v", stdout, stderr, err)
						}
						if got.Code != tc.code || got.Submission != tc.boundary || got.Delivered != (tc.code == "") ||
							(exit == 0) != (tc.code == "") || *enters != tc.enters ||
							strings.Contains(stdout+stderr, body) {
							t.Fatalf("exit=%d result=%+v enters=%d stderr=%q", exit, got, *enters, stderr)
						}
						if tc.code != "" {
							for _, row := range events.Read(60, time.Now().Unix()) {
								if row.Pane == "%9" && row.Event == events.AuditEventSend && strings.HasPrefix(row.Summary, statePlainSent+": ") {
									t.Fatalf("failed CLI send audited as sent: %+v", row)
								}
							}
						}
						return
					}

					sendID := "fixture-" + state.Home()
					handler := server.New(server.Config{Token: "fixture-owner"}, server.Deps{SendWithReceipt: sendToPaneWithReceipt}).Handler()
					do := func() (*httptest.ResponseRecorder, map[string]json.RawMessage) {
						t.Helper()
						payload, err := json.Marshal(map[string]any{"id": "%9", "text": body, "enter": true, "send_id": sendID})
						if err != nil {
							t.Fatal(err)
						}
						req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(string(payload)))
						req.Header.Set("Authorization", "Bearer fixture-owner")
						rr := httptest.NewRecorder()
						handler.ServeHTTP(rr, req)
						var got map[string]json.RawMessage
						if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
							t.Fatalf("invalid HTTP response: %d %s: %v", rr.Code, rr.Body, err)
						}
						return rr, got
					}
					cacheKey := "%9\x00" + sendID
					for i := 0; i < 2; i++ {
						rr, got := do()
						if tc.code == "" {
							var receipt server.InputReceipt
							if rr.Code != http.StatusOK || json.Unmarshal(got["delivery"], &receipt) != nil ||
								receipt.SendID != sendID || receipt.State != "accepted" {
								t.Fatalf("success not remembered: %d %s", rr.Code, rr.Body)
							}
						} else {
							_, cached := readPhoneReceipt(cacheKey)
							var code, boundary, echoedID string
							_ = json.Unmarshal(got["code"], &code)
							_ = json.Unmarshal(got["submission"], &boundary)
							_ = json.Unmarshal(got["send_id"], &echoedID)
							wantCode, wantBoundary := tc.code, tc.boundary
							if tc.name == "invalid layout" {
								wantCode, wantBoundary = dispatch.InputOutcomeUnconfirmed, dispatch.SubmissionUnknown
							}
							if rr.Code != http.StatusBadRequest || code != string(wantCode) || boundary != string(wantBoundary) ||
								echoedID != sendID || got["delivery"] != nil || cached ||
								strings.Contains(rr.Body.String(), body) {
								t.Fatalf("wrong HTTP failure/cache: %d %s", rr.Code, rr.Body)
							}
						}
						if *enters != tc.enters {
							t.Fatalf("same-ID calls=%d Enter=%d", i+1, *enters)
						}
					}
					if (tc.name == "capacity" || tc.name == "invalid layout") && *pastes != 0 {
						t.Fatalf("refusal reached the terminal: paste=%d", *pastes)
					}
				})
			}
		})
	}
}

func inputOutcomeFixture(t *testing.T, scenario, body string) (pastes, enters *int) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "tmux-fixture")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
for last do :; done
case "$last" in
 '#{pane_id}') printf '%%9';;
 '#{pane_current_command}') printf 'dsh';;
 '#{pane_in_mode}') printf '0';;
esac
`), 0700); err != nil {
		t.Fatal(err)
	}
	before, beforeIO := tmux.Bin, sendDispatchIO
	tmux.Bin = bin
	t.Cleanup(func() { tmux.Bin, sendDispatchIO = before, beforeIO })
	pasteCount, enterCount := 0, 0
	a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "dsh", Client: "dst", Pane: "%9",
		PID: 123, Instance: strings.Repeat("a", 32), HomeID: "root", SessionID: "thread",
		ServerEpoch: "17", BindingEpoch: 9, InputReady: true}
	if scenario == "invalid layout" {
		if err := os.MkdirAll(filepath.Join(state.Dir(), "terminal-input-v2"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	sendDispatchIO = func(string, ...string) dispatch.IO {
		var opts dispatch.Opts
		frame := "⌸ ❯ "
		io := dispatch.IO{
			Begin:   func(o dispatch.Opts, _ string) { opts = o },
			Capture: func() string { return frame },
			Now:     func() int64 { return time.Now().Unix() },
			Sleep:   func() {},
			Paste:   func(text string) error { pasteCount++; frame = "⌸ ❯ " + text; return nil },
			Enter: func() error {
				enterCount++
				r, err := agentclient.Prepare(a, body, opts.DeliveryID, false, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				if err := agentclient.ClaimNative(r); err != nil {
					t.Fatal(err)
				}
				if scenario == "unknown" {
					return errors.New("transport lost receipt for " + body)
				}
				frame = "⌸ ❯ "
				return agentclient.FinishNative(r, "accepted", "")
			},
		}
		if scenario == "raw" {
			io.Enter = func() error { enterCount++; return nil }
			return io
		}
		if scenario == "capacity" {
			io.Paste = func(string) error {
				return &dispatch.InputError{Code: dispatch.InputSpoolFull, Submission: dispatch.SubmissionNotSubmitted, Cause: agentclient.ErrInputFull}
			}
			return io
		}
		return agentclient.WrapIO("%9", io, func(string) (agentclient.Attachment, bool) {
			target := a
			if scenario == "changed target" && frame != "⌸ ❯ " {
				target.SessionID = "replacement"
			}
			return target, true
		})
	}
	return &pasteCount, &enterCount
}

func TestSPL014UnavailableDSHDoesNotSettlePriorID(t *testing.T) {
	const text = "fixture"
	_, enters := inputOutcomeFixture(t, "accepted", text)
	a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "dsh", Pane: "%9",
		PID: 123, Instance: strings.Repeat("a", 32), HomeID: "root", SessionID: "thread",
		ServerEpoch: "17", BindingEpoch: 9, InputReady: true}
	r, err := agentclient.Prepare(a, text, "prior-dsh", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := agentclient.ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	readIO := sendDispatchIO
	sendDispatchIO = func(pane string, agent ...string) dispatch.IO {
		base := readIO(pane, agent...)
		// Preparation will fail before any base terminal I/O.
		return agentclient.WrapIO(pane, base, func(string) (agentclient.Attachment, bool) {
			return agentclient.Attachment{}, false
		})
	}
	for _, tc := range []struct{ id, submission string }{{"prior-dsh", "unknown"}, {"fresh-dsh", "not-submitted"}} {
		_, err := sendToPaneWithReceipt("%9", text, "", true, tc.id)
		var e *server.SendError
		if !errors.As(err, &e) || e.Submission != tc.submission || *enters != 0 {
			t.Fatalf("%s: %v (%+v), Enter=%d", tc.id, err, e, *enters)
		}
	}
}

func TestSPL015UnverifiedSuccessAndCacheKeepTheirOriginalGrade(t *testing.T) {
	const text = "fixture"
	_, enters := inputOutcomeFixture(t, "raw", text)
	var exit int
	out := captureStdout(t, func() { exit = cmdSend([]string{"%9", "--no-verify", "--json", text}) })
	var result sendJSON
	if err := json.Unmarshal([]byte(out), &result); err != nil || exit != 0 ||
		result.State != "submitted" || result.Delivered {
		t.Fatalf("exit=%d, reply=%q, %v", exit, out, err)
	}
	id := "plain-id-" + state.Home()
	for i := 0; i < 2; i++ {
		got, err := sendToPaneWithReceipt("%9", text, "", true, id)
		if err != nil || got.State != "submitted" || got.SendID != id || *enters != 2 {
			t.Fatalf("repeat %d: %+v %v, total Enter=%d", i, got, err, *enters)
		}
	}
}
