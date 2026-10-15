package userservice

func Current(home string) Manager { return Launchd{Home: home, Run: Command} }
