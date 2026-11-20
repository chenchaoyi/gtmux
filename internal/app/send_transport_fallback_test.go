package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestSPL019HTTPProtectsNativeIDAfterRouteLoss(t *testing.T) {
	for _, route := range []string{"missing-record", "missing-frontend", "bare-shell"} {
		t.Run(route, func(t *testing.T) {
			const pane, location, payload = "%19", "fallback:0.0", "synthetic dispatch text"
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GTMUX_FIXTURE_FG", "codex")
			mutations := filepath.Join(home, "mutations")
			t.Setenv("GTMUX_FIXTURE_MUTATIONS", mutations)
			t.Setenv("GTMUX_FIXTURE_BUFFER", filepath.Join(home, "buffer"))
			t.Setenv("GTMUX_FIXTURE_FRAME", filepath.Join(home, "frame"))
			fixture := filepath.Join(home, "tmux-fixture")
			if err := os.WriteFile(fixture, []byte(`#!/bin/sh
shift
for last do :; done
case "$1" in
  display-message)
    case "$last" in
      '#{pane_id}') printf '%%19\n';;
      '#{pane_current_command}') printf '%s\n' "$GTMUX_FIXTURE_FG";;
      '#{session_name}:#{window_index}.#{pane_index}') printf 'fallback:0.0\n';;
      '#{pane_in_mode}') printf '0\n';;
    esac;;
  load-buffer)
    printf 'load-buffer\n' >> "$GTMUX_FIXTURE_MUTATIONS"
    cat > "$GTMUX_FIXTURE_BUFFER";;
  paste-buffer)
    printf 'paste-buffer\n' >> "$GTMUX_FIXTURE_MUTATIONS"
    { printf '⌸ ❯ '; cat "$GTMUX_FIXTURE_BUFFER"; printf '\n'; } > "$GTMUX_FIXTURE_FRAME";;
  capture-pane)
    if [ -f "$GTMUX_FIXTURE_FRAME" ]; then cat "$GTMUX_FIXTURE_FRAME"; else printf '⌸ ❯ \n'; fi;;
  send-keys) printf 'send-keys %s\n' "$last" >> "$GTMUX_FIXTURE_MUTATIONS";;
esac
`), 0700); err != nil {
				t.Fatal(err)
			}
			oldBin, oldIO := tmux.Bin, sendDispatchIO
			tmux.Bin, sendDispatchIO = fixture, dispatchbridge.DispatchIO
			t.Cleanup(func() { tmux.Bin, sendDispatchIO = oldBin, oldIO })

			record := resume.Record{Agent: "codex", SessionID: "thread", HomeID: "home", Frontend: strings.Repeat("a", 32)}
			if err := resume.Save(location, record); err != nil {
				t.Fatal(err)
			}
			if dispatchbridge.DispatchIO(pane, "codex").SubmitNative == nil {
				t.Fatal("fixture did not initially select the native route")
			}
			oldID, freshID := "old-"+home, "fresh-"+home
			a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: record.Agent, Client: "codex",
				Pane: pane, Instance: strings.Repeat("a", 32), PID: 123, HomeID: record.HomeID,
				SessionID: record.SessionID, ServerEpoch: "19", BindingEpoch: 9, InputReady: true}
			request, err := agentclient.Prepare(a, payload, oldID, false, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := agentclient.ClaimNative(request); err != nil {
				t.Fatal(err)
			}
			claimPath := filepath.Join(agentclient.InputDir(), request.ID+".claimed.json")
			before, err := os.ReadFile(claimPath)
			if err != nil {
				t.Fatal(err)
			}

			resume.Remove(location)
			if route == "missing-frontend" {
				record.Frontend = ""
				if err := resume.Save(location, record); err != nil {
					t.Fatal(err)
				}
			} else if route == "bare-shell" {
				t.Setenv("GTMUX_FIXTURE_FG", "bash")
			}
			agent, plain := resolvePaneAgent(pane)
			if plain != (route == "bare-shell") || dispatchbridge.DispatchIO(pane, agent).SubmitNative != nil {
				t.Fatalf("fixture routing: agent=%q plain=%v", agent, plain)
			}

			handler := server.New(server.Config{Token: "fixture-owner"}, server.Deps{SendWithReceipt: sendToPaneWithReceipt}).Handler()
			doSend := func(id string) *httptest.ResponseRecorder {
				t.Helper()
				data, err := json.Marshal(map[string]any{"id": pane, "text": payload, "enter": true, "send_id": id})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(data))
				req.Header.Set("Authorization", "Bearer fixture-owner")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				return response
			}
			for range 2 {
				response := doSend(oldID)
				var failure struct {
					Code       string               `json:"code"`
					Submission string               `json:"submission"`
					SendID     string               `json:"send_id"`
					Delivery   *server.InputReceipt `json:"delivery"`
				}
				_, cached := readPhoneReceipt(pane + "\x00" + oldID)
				if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil ||
					response.Code != http.StatusBadRequest || failure.Code != "outcome-unconfirmed" ||
					failure.Submission != "unknown" || failure.SendID != oldID || failure.Delivery != nil || cached {
					journal, _ := os.ReadFile(mutations)
					t.Fatalf("retry reply=%d %s; cached=%v; terminal writes=%q", response.Code, response.Body.String(), cached, journal)
				}
			}
			if journal, err := os.ReadFile(mutations); !os.IsNotExist(err) {
				t.Fatalf("the old ID reached tmux: %q, read error=%v", journal, err)
			}
			if after, err := os.ReadFile(claimPath); err != nil || !bytes.Equal(before, after) {
				t.Fatalf("the old claim changed: %v", err)
			}
			if receipt, ok := agentclient.ReadReceipt(request); ok {
				t.Fatalf("the old claim acquired a receipt: %+v", receipt)
			}

			for range 2 {
				response := doSend(freshID)
				var result struct {
					Delivery *server.InputReceipt `json:"delivery"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil ||
					response.Code != http.StatusOK || result.Delivery == nil ||
					result.Delivery.State != "submitted" || result.Delivery.SendID != freshID {
					t.Fatalf("fresh reply=%d %s", response.Code, response.Body.String())
				}
			}
			want := "load-buffer\npaste-buffer\nsend-keys Enter\n"
			if plain {
				want = "send-keys " + payload + "\nsend-keys Enter\n"
			}
			if journal, err := os.ReadFile(mutations); err != nil || string(journal) != want {
				t.Fatalf("wrong fresh route writes: %q, want %q (error=%v)", journal, want, err)
			}
		})
	}
}
