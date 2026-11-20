package app

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/server"
)

// sendFailureForResult translates typed transport facts for both send surfaces.
// A screen capture, original payload or transport error string stays out of the
// response. Unknown takes precedence over any more specific refusal reason.
func sendFailureForResult(result dispatch.Result) (code dispatch.InputFailureCode, en, zh string) {
	code = result.FailureCode
	if result.Submission == dispatch.SubmissionUnknown {
		code = dispatch.InputOutcomeUnconfirmed
	}
	switch code {
	case dispatch.InputSpoolFull:
		return code, "not sent: the Mac has reached its terminal input record limit", "未发送：Mac 的终端输入记录已达上限。"
	case dispatch.InputQuestion:
		return code, "not sent: the agent is asking something (a permission prompt or a question); answer it first", "未发送：请先回应终端中的权限提示或提问。"
	case dispatch.InputDraftPresent:
		return code, "not sent: that pane has unsent text in its input box; clear it, or send from the Mac", "未发送：这个终端的输入框有草稿，请先处理草稿。"
	case dispatch.InputAssociationMissing:
		return code, "not sent: conversation association is unavailable; no input was submitted", "未发送：当前会话关联不可用，内容未提交。"
	case dispatch.InputTargetChanged:
		return code, "not sent: the target conversation changed; select the current session before sending", "未发送：目标会话已变化，请重新选择当前会话。"
	case dispatch.InputSessionNotLoaded:
		return code, "not sent: the session is not loaded; open it on the Mac first", "未发送：会话尚未载入，请先在 Mac 上打开。"
	case dispatch.InputUnavailable:
		return code, "not sent: the session's input state could not be verified; inspect the session and try again", "未发送：无法确认会话当前可接收输入，请检查会话。"
	case dispatch.InputOutcomeUnconfirmed:
		return code, "send outcome unconfirmed; inspect the conversation before retrying", "发送结果未确认，请先查看会话再决定是否重试。"
	default:
		return dispatch.InputUnconfirmed, "not sent: the pane's input box did not settle on the full message", "未发送：无法确认完整内容已进入终端输入框。"
	}
}

func phoneSendFailure(result dispatch.Result) error {
	if result.FailureCode == dispatch.InputQuestion && !result.NativeQuestion {
		result.FailureCode = dispatch.InputUnavailable
	}
	code, message, _ := sendFailureForResult(result)
	return &server.SendError{Code: string(code), Message: message, Submission: string(result.Submission)}
}
