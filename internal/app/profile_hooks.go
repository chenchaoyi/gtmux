package app

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// Only installation probes a known CLI. Profile reads never spawn executables.
var profileVersionProbe = func(agent string) string {
	bin := lookTool(agent)
	if bin == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--version")
	var prefix versionPrefix
	cmd.Stdout = &prefix
	if cmd.Run() != nil {
		return ""
	}
	return string(prefix.b)
}

type versionPrefix struct{ b []byte }

func (w *versionPrefix) Write(b []byte) (int, error) {
	n := len(b)
	if len(w.b) < 256 {
		w.b = append(w.b, b[:min(len(b), 256-len(w.b))]...)
	}
	return n, nil
}

var versionTriplet = regexp.MustCompile(`(?:^|\s)v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:\s|$|[-+])`)

func supportsProfileHooks(agent, version string) bool {
	m := versionTriplet.FindStringSubmatch(version)
	if len(m) != 4 {
		return false
	}
	v := [3]int{}
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	floor := [3]int{2, 1, 251}
	if agent == "gemini" {
		floor = [3]int{0, 35, 3}
	} else if agent != "claude" {
		return false
	}
	for i := range v {
		if v[i] != floor[i] {
			return v[i] > floor[i]
		}
	}
	return true
}
