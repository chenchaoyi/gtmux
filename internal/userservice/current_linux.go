package userservice

import "os"

func Current(home string) Manager { return linuxCurrent(home) }

func linuxCurrent(home string) Manager {
	return Systemd{Home: home, Config: os.Getenv("XDG_CONFIG_HOME"), SearchPath: os.Getenv("PATH"), Socket: os.Getenv("TMUX"), Run: Command}
}
