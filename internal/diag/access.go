package diag

import (
	"fmt"
	"net"
	"strings"
)

// AccessEntry is metadata only. Device identity is set after bearer verification;
// names/platform remain descriptive client metadata, not authentication factors.
type AccessEntry struct {
	ID         string `json:"id"`
	At         string `json:"at"`
	Actor      string `json:"actor"`
	DeviceID   string `json:"device_id,omitempty"`
	Name       string `json:"name"`
	Platform   string `json:"platform,omitempty"`
	SourceIP   string `json:"source_ip,omitempty"`
	Scope      string `json:"scope"`
	Operation  string `json:"operation"`
	LabelEN    string `json:"label_en"`
	LabelZH    string `json:"label_zh"`
	Method     string `json:"method,omitempty"`
	Target     string `json:"target,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Status     int    `json:"status,omitempty"`
	Outcome    string `json:"outcome"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

// The vocabulary is deliberately limited: arbitrary paths, query strings and
// request/response bodies never enter the access store or its remote projection.
var accessOperations = map[string][2]string{
	"sessions":          {"View sessions", "查看会话列表"},
	"terminal":          {"Read terminal", "读取终端画面"},
	"conversation":      {"Read conversation", "读取对话"},
	"hq":                {"View HQ", "查看 HQ"},
	"knowledge":         {"Read knowledge base", "读取知识库"},
	"archive_export":    {"Download HQ archive", "下载 HQ 档案"},
	"notification_read": {"View notification registrations", "查看通知注册"},
	"usage":             {"View usage", "查看用量"},
	"settings":          {"View server settings", "查看服务器设置"},
	"send":              {"Send terminal input", "发送终端输入"},
	"focus":             {"Focus a pane", "切换窗格"},
	"create_session":    {"Create a session", "创建会话"},
	"upload":            {"Upload a file", "上传文件"},
	"sharing":           {"Change sharing", "更改共享设置"},
	"devices":           {"Manage paired devices", "管理配对设备"},
	"notifications":     {"Synchronize notifications", "同步通知设置"},
	"routes":            {"Change connection route", "切换连接线路"},
	"server_mode":       {"Change server mode", "更改服务器模式"},
	"follow":            {"Change session follow-up", "更改会话跟进"},
	"knowledge_write":   {"Update knowledge base", "更新知识库"},
	"live_open":         {"Live connection opened", "实时连接已建立"},
	"live_close":        {"Live connection closed", "实时连接已断开"},
	"attach_open":       {"Remote terminal opened", "远程终端已打开"},
	"attach_close":      {"Remote terminal closed", "远程终端已关闭"},
	"history":           {"View access history", "查看访问历史"},
	"pair":              {"Pair a device or open a share code", "配对设备或兑换共享码"},
	"rejected":          {"Access rejected", "访问已拒绝"},
	"other":             {"Server request", "服务器请求"},
}

// Access records HTTP acceptance, refusal or failure, not eventual agent results.
// Keep it informational so an HTTP failure already logged by its handler does not
// manufacture a second diagnostic error. History filters outcome separately.
func (l *Logger) Access(e AccessEntry) {
	labels, ok := accessOperations[e.Operation]
	if !ok {
		e.Operation = "other"
		labels = accessOperations[e.Operation]
	}
	ip := net.ParseIP(e.SourceIP)
	if ip == nil {
		e.SourceIP = ""
	}
	if len(e.Target) < 2 || len(e.Target) > 16 || !strings.HasPrefix(e.Target, "%") || strings.Trim(e.Target[1:], "0123456789") != "" {
		e.Target = ""
	}
	e.SessionID = AccessSessionID(e.SessionID)
	switch e.Method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "":
	default:
		e.Method = "OTHER"
	}
	if e.Name == "" {
		e.Name = "Owner credential"
	}
	write(Entry{Level: Info.String(), Component: l.component, Kind: KindAct,
		Event: "access.request", Actor: e.Actor, Target: e.Target, Outcome: e.Outcome,
		Msg: labels[0], Attrs: map[string]any{
			"op_id": NewOpID(), "operation": e.Operation, "device_id": e.DeviceID,
			"device_name": boundedAccessText(e.Name), "platform": boundedAccessText(e.Platform),
			"source_ip": e.SourceIP, "scope": e.Scope, "method": e.Method,
			"status": e.Status, "duration_ms": e.DurationMS,
			"session_id": e.SessionID,
		}})
}

// AccessSessionID accepts only Codex's structured UUID locator, never arbitrary
// query/body text. A home has already been checked by the authenticated handler.
func AccessSessionID(raw string) string {
	if len(raw) != 36 {
		return ""
	}
	for i, c := range strings.ToLower(raw) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return ""
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return ""
		}
	}
	return strings.ToLower(raw)
}
func boundedAccessText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 128 {
		return string(r[:128])
	}
	return s
}

func accessFromEntry(e Entry) (AccessEntry, bool) {
	if e.Component != "serve" || e.Event != "access.request" || e.Kind != KindAct {
		return AccessEntry{}, false
	}
	text := func(k string) string { s, _ := e.Attrs[k].(string); return s }
	number := func(k string) int64 {
		switch n := e.Attrs[k].(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
		return 0
	}
	operation := text("operation")
	labels, ok := accessOperations[operation]
	if !ok {
		operation = "other"
		labels = accessOperations[operation]
	}
	id := e.OpID
	if id == "" {
		id = fmt.Sprintf("%s/%s", e.TS, e.Actor)
	}
	return AccessEntry{ID: id, At: e.TS, Actor: e.Actor, DeviceID: text("device_id"), Name: text("device_name"),
		Platform: text("platform"), SourceIP: text("source_ip"), Scope: text("scope"), Operation: operation,
		LabelEN: labels[0], LabelZH: labels[1], Method: text("method"), Target: e.Target,
		Status: int(number("status")), Outcome: e.Outcome, DurationMS: number("duration_ms"), SessionID: AccessSessionID(text("session_id"))}, true
}
