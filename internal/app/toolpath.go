package app

import "github.com/chenchaoyi/gtmux/internal/toolpath"

// lookTool resolves a helper binary despite launchd's PATH (tool-path-resolution).
// The implementation moved to internal/toolpath — a LEAF — because internal/dispatch
// needs the same lesson for `gh` and cannot import internal/app. This shim keeps app's
// dozen call sites reading as they did.
func lookTool(name string) string { return toolpath.Look(name) }
