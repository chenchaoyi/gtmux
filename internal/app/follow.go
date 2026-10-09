package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func serveSessionFollow(id string, next *sessionpolicy.Settings) (sessionpolicy.Settings, error) {
	return serveSessionFollowAs(id, next, diag.Caller())
}

func serveSessionFollowAs(id string, next *sessionpolicy.Settings, actor string) (sessionpolicy.Settings, error) {
	if !sessionpolicy.Desktop(id) {
		return sessionpolicy.Settings{}, sessionpolicy.ErrUnverified
	}
	if next == nil {
		return sessionpolicy.Get(id), nil
	}
	saved, err := sessionpolicy.Save(id, *next)
	if err != nil {
		diag.For("session-follow").Act("act.session.follow", actor, id, diag.Failed, "could not save conversation permissions", "error", err)
		return saved, err
	}
	diag.For("session-follow").Act("act.session.follow", actor, id, diag.OK, "conversation permissions saved", "hq", saved.HQ, "notify", saved.Notify, "knowledge", saved.Knowledge, "revision", saved.Revision)
	// No conversation text in this receipt; permissions are reviewable after the row disappears.
	events.Append(events.Record{Event: "gtmux:audit:session-follow", Agent: "Codex", AgentSession: id, Client: "chatgpt_desktop", Actor: actor, Outcome: "saved", Summary: fmt.Sprintf("hq=%t notify=%t knowledge=%t revision=%d", saved.HQ, saved.Notify, saved.Knowledge, saved.Revision)})
	return saved, nil
}

func cmdFollow(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		commandHelp("follow")
		return 2
	}
	id := args[0]
	next, err := serveSessionFollow(id, nil)
	if err != nil {
		i18n.Sae("gtmux follow: "+err.Error(), "gtmux follow：未找到可确认身份的桌面 Codex 会话")
		return 1
	}
	writing, jsonOut := false, false
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--json" {
			jsonOut = true
			continue
		}
		if a == "--help" || a == "-h" {
			commandHelp("follow")
			return 0
		}
		if i+1 >= len(args) {
			commandHelp("follow")
			return 2
		}
		i++
		value := args[i]
		if a == "--revision" {
			n, e := strconv.ParseInt(value, 10, 64)
			if e != nil || n < 0 {
				return 2
			}
			next.Revision = n
			continue
		}
		if value != "on" && value != "off" {
			commandHelp("follow")
			return 2
		}
		switch a {
		case "--hq":
			next.HQ = value == "on"
		case "--notify":
			next.Notify = value == "on"
		case "--knowledge":
			next.Knowledge = value == "on"
		default:
			commandHelp("follow")
			return 2
		}
		writing = true
	}
	if writing {
		next, err = serveSessionFollow(id, &next)
	}
	if err != nil {
		i18n.Sae("gtmux follow: "+err.Error(), "gtmux follow：保存失败，请重新读取设置后重试（"+err.Error()+"）")
		return 1
	}
	if jsonOut {
		b, _ := json.Marshal(next)
		fmt.Println(string(b))
	} else {
		fmt.Println(i18n.Tr("HQ follow", "HQ 跟进") + ": " + strconv.FormatBool(next.HQ) + " · " + i18n.Tr("notifications", "通知") + ": " + strconv.FormatBool(next.Notify) + " · " + i18n.Tr("knowledge", "知识沉淀") + ": " + strconv.FormatBool(next.Knowledge))
	}
	return 0
}
