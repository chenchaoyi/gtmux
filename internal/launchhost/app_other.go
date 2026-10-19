//go:build !darwin

package launchhost

import "context"

func appName(context.Context, string) string { return "" }
