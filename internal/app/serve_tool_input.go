package app

import (
	"encoding/json"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func fullToolInput(id, home, ref string) ([]byte, error) {
	identity, err := sessionpolicy.Resolve(id, home)
	if err != nil {
		return nil, err
	}
	step, err := transcript.ToolInput(identity.Agent, id, ref)
	if err != nil {
		return nil, err
	}
	return json.Marshal(step)
}
