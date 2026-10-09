package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// This user-initiated read never enrolls the conversation for HQ observation.
func transcriptForDesktop(id string) ([]byte, server.TranscriptMeta, error) {
	return desktopTranscriptRead(id, "")
}

func desktopTranscriptRead(id, etag string) ([]byte, server.TranscriptMeta, error) {
	var meta server.TranscriptMeta
	if id == "" || len(id) > 256 || strings.ContainsAny(id, `*/?[]\`) || !sessionpolicy.Desktop(id) {
		return nil, meta, sessionpolicy.ErrUnverified
	}
	// Read the validator before content: an append during parsing must force another
	// read, not mark older content with a newer revision.
	if revision := transcript.LogRevision("codex", id); revision != "" {
		meta.Etag = fmt.Sprintf("W/%q", id+"-"+revision)
	}
	if etag != "" && meta.Etag == etag {
		return nil, meta, nil
	}
	turns, err := transcript.Load("codex", id, maxTranscriptTurns)
	if err != nil {
		return nil, meta, err
	}
	for i := range turns {
		turns[i].Agent = "codex"
	}
	kept, dropped := turnsWithinBudget(turns, transcriptByteBudget)
	if kept == nil {
		kept = []transcript.Turn{}
	}
	meta.Dropped = dropped
	b, err := json.Marshal(kept)
	return b, meta, err
}

func cmdTranscript(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		commandHelp("transcript")
		return 2
	}
	jsonOut, etag := false, ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--etag":
			if i+1 >= len(args) {
				commandHelp("transcript")
				return 2
			}
			i++
			etag = args[i]
		default:
			commandHelp("transcript")
			return 2
		}
	}
	if etag != "" && !jsonOut {
		commandHelp("transcript")
		return 2
	}
	b, meta, err := desktopTranscriptRead(args[0], etag)
	if err != nil {
		i18n.Sae("Could not read this desktop conversation: "+err.Error(), "无法读取此桌面会话："+err.Error())
		return 1
	}
	if jsonOut {
		out := struct {
			Turns     json.RawMessage `json:"turns"`
			Dropped   int             `json:"dropped"`
			Etag      string          `json:"etag,omitempty"`
			Unchanged bool            `json:"unchanged,omitempty"`
		}{b, meta.Dropped, meta.Etag, b == nil}
		data, _ := json.Marshal(out)
		fmt.Println(string(data))
		return 0
	}
	var turns []transcript.Turn
	if json.Unmarshal(b, &turns) != nil {
		return 1
	}
	for _, turn := range turns {
		if turn.Prompt != "" {
			fmt.Println("› " + turn.Prompt)
		}
		if turn.Response != "" {
			fmt.Println(turn.Response)
		}
	}
	if len(turns) == 0 {
		i18n.Say("No messages have been recorded yet.", "尚无已记录的对话。")
	}
	if meta.Dropped > 0 {
		i18n.Sae("Earlier messages omitted from this bounded view.", "此视图省略了较早的部分对话。")
	}
	return 0
}
