//go:build !darwin && !linux

package resume

func platformNativeRestoreBoot() (string, error) { return "", errNativeBootUnavailable }
