package agentprofile

import (
	"encoding/json"
	"strconv"
)

// decode selects only public metadata; raw messages, prompts and options are not retained.
func decode(agent string, raw []byte) point {
	var e struct {
		Type           string          `json:"type"`
		Timestamp      string          `json:"timestamp"`
		Time           int64           `json:"time"`
		Version        string          `json:"version"`
		AgentID        string          `json:"agentId"`
		Kind           string          `json:"kind"`
		Provider       string          `json:"provider"`
		Model          string          `json:"model"`
		ModelAlias     string          `json:"modelAlias"`
		ThinkingEffort string          `json:"thinkingEffort"`
		Mode           string          `json:"mode"`
		Payload        json.RawMessage `json:"payload"`
		Data           json.RawMessage `json:"data"`
		Message        struct {
			Model string `json:"model"`
			Stop  string `json:"stop_reason"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return point{}
	}
	at := e.Time
	if at == 0 {
		at = stamp(e.Timestamp)
	}
	x := point{At: at}
	add := func(k, v string) { x.Facts = append(x.Facts, fact(k, v, "session_log", at)) }
	switch agent {
	case "claude":
		if e.Type != "assistant" {
			return x
		}
		if e.Version != "" {
			add("version", e.Version)
		}
		if e.Message.Model != "" {
			add("model", e.Message.Model)
		}
		x.End = e.Message.Stop == "end_turn"
	case "codex":
		var v struct {
			Type     string `json:"type"`
			Version  string `json:"cli_version"`
			Provider string `json:"model_provider"`
			Model    string `json:"model"`
			Effort   string `json:"effort"`
			Mode     *struct {
				Mode string `json:"mode"`
			} `json:"collaboration_mode"`
			Approval json.RawMessage `json:"approval_policy"`
			Sandbox  *struct {
				Type string `json:"type"`
			} `json:"sandbox_policy"`
			Limits *struct {
				Plan string `json:"plan_type"`
			} `json:"rate_limits"`
		}
		if json.Unmarshal(e.Payload, &v) != nil {
			return x
		}
		switch e.Type {
		case "session_meta":
			add("version", v.Version)
			add("provider", v.Provider)
		case "turn_context":
			add("model", v.Model)
			add("effort", v.Effort)
			mode := ""
			if v.Mode != nil {
				mode = v.Mode.Mode
			}
			add("mode", mode)
			var approval string
			_ = json.Unmarshal(v.Approval, &approval)
			add("permission", approval)
			if v.Sandbox != nil {
				add("sandbox", v.Sandbox.Type)
			}
		case "event_msg":
			x.End = v.Type == "task_complete" || v.Type == "turn_aborted"
			if v.Type == "token_count" && v.Limits != nil && v.Limits.Plan != "" {
				f := fact("subscription", v.Limits.Plan, "session_rate_limits", at)
				x.Facts = append(x.Facts, f)
			}
		}
		// A tier is attributed only to this session record, never the global limits cache.
	case "dsh":
		var v struct {
			Header struct {
				Config struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
					Effort   string `json:"reasoningEffort"`
				} `json:"config"`
			} `json:"header"`
			Active *bool `json:"active"`
		}
		if json.Unmarshal(e.Data, &v) != nil {
			return x
		}
		switch e.Type {
		case "request/header":
			add("provider", v.Header.Config.Provider)
			add("model", v.Header.Config.Model)
			add("effort", v.Header.Config.Effort)
		case "plan/mode":
			if v.Active != nil {
				if *v.Active {
					add("mode", "plan")
				} else {
					add("mode", "normal")
				}
			}
		case "turn/end":
			x.End = true
		}
	case "kimi":
		if e.AgentID != "main" {
			return x
		}
		switch e.Type {
		case "llm.request":
			if e.Kind != "loop" {
				return x
			}
			add("provider", e.Provider)
			add("model", e.Model)
			add("model_alias", e.ModelAlias)
			add("effort", e.ThinkingEffort)
		case "permission.set_mode":
			add("permission", e.Mode)
		case "plan_mode.enter":
			add("mode", "plan")
		case "plan_mode.exit":
			add("mode", "normal")
		case "plan_mode.cancel":
			add("mode", "")
		case "turn.ended":
			x.End = true
		}
	}
	return x
}

// HookFacts consumes each agent's own contract. It never traverses arbitrary options.
func HookFacts(agent string, raw []byte, at int64) []Fact {
	var v struct {
		Model      string `json:"model"`
		ModelID    string `json:"model_id"`
		ToModel    string `json:"to_model"`
		Permission string `json:"permission_mode"`
		Version    string `json:"cursor_version"`
		Effort     *struct {
			Level string `json:"level"`
		} `json:"effort"`
		Params []struct {
			ID    string          `json:"id"`
			Value json.RawMessage `json:"value"`
		} `json:"model_params"`
		Profile *struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
			Role     string `json:"role"`
			Variant  string `json:"variant"`
			Effort   string `json:"effort"`
			Version  string `json:"version"`
		} `json:"gtmux_profile"`
		Request *struct {
			Model  string `json:"model"`
			Config *struct {
				Thinking *struct {
					Level  string `json:"thinkingLevel"`
					Budget *int   `json:"thinkingBudget"`
				} `json:"thinkingConfig"`
			} `json:"config"`
		} `json:"llm_request"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	var out []Fact
	add := func(k, s string) {
		if f := fact(k, s, "native_hook", at); f.Value != "" {
			out = append(out, f)
		}
	}
	switch agent {
	case "claude":
		if v.ToModel != "" {
			add("model", v.ToModel)
		} else {
			add("model", v.Model)
		}
		add("permission", v.Permission)
		if v.Permission == "plan" {
			add("mode", "plan")
		}
		if v.Effort != nil {
			out = append(out, fact("effort", v.Effort.Level, "native_hook", at))
		}
	case "cursor":
		if v.ModelID != "" {
			add("model", v.ModelID)
		} else {
			add("model", v.Model)
		}
		add("cursor_version", v.Version)
		for _, p := range v.Params {
			var value string
			if json.Unmarshal(p.Value, &value) != nil {
				var enabled bool
				if p.ID == "thinking" && (string(p.Value) == "true" || string(p.Value) == "false") && json.Unmarshal(p.Value, &enabled) == nil {
					value = strconv.FormatBool(enabled)
				}
			}
			switch p.ID {
			case "effort":
				add("effort", value)
			case "thinking":
				add("thinking", value)
			case "context":
				add("context", value)
			}
		}
	case "gemini":
		if v.Request != nil {
			// Each request is a complete thinking-options snapshot.
			for _, k := range []string{"thinking_level", "thinking_budget"} {
				out = append(out, Fact{Key: k})
			}
			add("model", v.Request.Model)
			if v.Request.Config != nil && v.Request.Config.Thinking != nil {
				t := v.Request.Config.Thinking
				add("thinking_level", t.Level)
				if t.Budget != nil {
					add("thinking_budget", strconv.Itoa(*t.Budget))
				}
			}
		}
	case "opencode":
		if v.Profile != nil {
			p := v.Profile
			for _, k := range []string{"role", "variant", "effort"} {
				out = append(out, Fact{Key: k})
			}
			add("model", p.Model)
			add("provider", p.Provider)
			add("role", p.Role)
			add("variant", p.Variant)
			add("effort", p.Effort)
			add("version", p.Version)
		}
	}
	return out
}
