//go:build !darwin

package hostplatform

func Current() Capabilities { return Capabilities{} }
