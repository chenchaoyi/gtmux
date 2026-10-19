//go:build !darwin && !linux

package codexterminal

import "context"

func stdinTTY(context.Context, int) string { return "" }
