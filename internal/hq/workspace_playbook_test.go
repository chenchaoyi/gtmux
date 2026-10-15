package hq

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

func TestBothChartersTeachUsableExistingGoalRegistration(t *testing.T) {
	for lang, charter := range map[string]string{"en": hqWorkspaceInstructionsEN, "zh": hqWorkspaceInstructionsZH} {
		t.Run(lang, func(t *testing.T) {
			start := strings.Index(charter, `{"op":"register"`)
			if start < 0 {
				t.Fatal("HQ cannot enroll a previously assigned goal")
			}
			var action struct {
				Op, ID, Title, Body, State string
				Origin                     hqworkspace.Origin
			}
			if err := json.NewDecoder(strings.NewReader(charter[start:])).Decode(&action); err != nil {
				t.Fatal("charter's registration example cannot be read", err)
			}
			s := hqworkspace.New(t.TempDir())
			if _, err := s.Register(action.ID, action.Title, action.Body, action.State, action.Origin, 100); err != nil {
				t.Fatal("charter teaches an invalid registration", err)
			}
			d, _ := s.Read()
			for _, msg := range d.Messages {
				if msg.Speaker != "hq" || msg.Delivery != "" {
					t.Fatal("charter's enrollment example reissues a task")
				}
			}
		})
	}
}
