package transcript

import (
	"encoding/json"
	"strings"
)

type questionReply struct {
	Answer         *string `json:"answer"`
	Question       *string `json:"question"`
	QuestionItemID *string `json:"questionItemId"`
}

// unwrapQuestionReplies keeps user answers, not the Desktop question transport.
// Only a leading, complete envelope with the observed schema is interpreted;
// quoted examples, partial records and unknown schemas remain ordinary text.
func unwrapQuestionReplies(s string) string {
	const open = "<send_user_message_question_reply>"
	const close = "</send_user_message_question_reply>"
	var parts []string
	for strings.HasPrefix(s, open) {
		body := s[len(open):]
		var replies []questionReply
		decoder := json.NewDecoder(strings.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&replies) != nil || len(replies) == 0 {
			break
		}
		rest := strings.TrimSpace(body[decoder.InputOffset():])
		if !strings.HasPrefix(rest, close) {
			break
		}
		valid, answered := true, 0
		for _, reply := range replies {
			if reply.Answer == nil || reply.Question == nil || reply.QuestionItemID == nil || !isQuestionReplyID(*reply.QuestionItemID) {
				valid = false
				break
			}
			if strings.TrimSpace(*reply.Answer) != "" {
				answered++
			}
		}
		if !valid {
			break
		}
		for _, reply := range replies {
			answer := strings.TrimSpace(*reply.Answer)
			if answer == "" {
				continue
			}
			if question := strings.TrimSpace(*reply.Question); answered > 1 && question != "" {
				answer = question + "\n" + answer
			}
			parts = append(parts, answer)
		}
		s = strings.TrimSpace(rest[len(close):])
	}
	if s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

func isQuestionReplyID(raw string) bool {
	var fields []json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil || len(fields) != 3 {
		return false
	}
	var tool, call string
	var index *int
	if json.Unmarshal(fields[0], &tool) != nil || json.Unmarshal(fields[1], &call) != nil || json.Unmarshal(fields[2], &index) != nil {
		return false
	}
	return (tool == "request_user_input_async" || tool == "request_user_input") && call != "" && index != nil && *index >= 0
}
