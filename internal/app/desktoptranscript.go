package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// This user-initiated read never enrolls the conversation for HQ observation.
func transcriptForDesktop(id string) ([]byte, server.TranscriptMeta, error) {
	return desktopTranscriptRead(id, "")
}

func desktopTranscriptRead(id, etag string, homes ...string) ([]byte, server.TranscriptMeta, error) {
	var meta server.TranscriptMeta
	home := ""
	if len(homes) > 0 {
		home = homes[0]
	}
	if !sessionpolicy.ValidRequest(id, home) {
		return nil, meta, sessionpolicy.ErrUnverified
	}
	ref, resolveErr := sessionpolicy.Resolve(id, home)
	if resolveErr != nil {
		return nil, meta, sessionpolicy.ErrUnverified
	}
	var senders map[int64]events.MessageSender
	senderRevision := ""
	if ref.Agent == "dsh" {
		senders, senderRevision = events.Sends.MessageSenders(ref.Agent, ref.HomeID, id)
	}
	// Read the validator before content: an append during parsing must force another
	// read, not mark older content with a newer revision.
	if revision := transcript.LogRevision(ref.Agent, id); revision != "" {
		meta.Etag = fmt.Sprintf("W/%q", "profile-v2-"+ref.HomeID+"-"+id+"-"+revision+"-"+senderRevision)
	}
	if etag != "" && meta.Etag == etag {
		return nil, meta, nil
	}
	var turns []transcript.Turn
	var err error
	retained := 0
	if ref.Agent == "dsh" {
		turns, retained, err = transcript.LoadDSH(id, maxTranscriptTurns)
	} else {
		turns, err = transcript.Load(ref.Agent, id, maxTranscriptTurns)
	}
	if err != nil {
		return nil, meta, err
	}
	for i := range turns {
		turns[i].Agent = ref.Agent
		turns[i].SessionID = id
	}
	turns = stampMessageSenders(turns, senders)
	kept, dropped := turnsWithinBudget(turns, transcriptByteBudget)
	if kept == nil {
		kept = []transcript.Turn{}
	}
	meta.Dropped = dropped + retained
	b, err := json.Marshal(kept)
	return b, meta, err
}

func cmdTranscript(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		commandHelp("transcript")
		return 2
	}
	for _, arg := range args[1:] {
		if arg == "--messages" {
			return cmdMessageTranscript(args)
		}
	}
	jsonOut, etag := false, ""
	home := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--home-id":
			if i+1 >= len(args) || !sessionpolicy.ValidHome(args[i+1]) {
				return 2
			}
			i++
			home = args[i]
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
	b, meta, err := desktopTranscriptRead(args[0], etag, home)
	if err != nil {
		i18n.Sae("Could not read this conversation: "+err.Error(), "无法读取此会话："+err.Error())
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
