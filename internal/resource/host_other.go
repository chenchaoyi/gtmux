//go:build !darwin && !linux

package resource

func sampleMachine() Machine { return Machine{} }
