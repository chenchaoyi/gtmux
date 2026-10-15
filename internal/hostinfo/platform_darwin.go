package hostinfo

import "strconv"

func gatherPlatform(in *Info) {
	in.OS = "macOS"
	in.ComputerName = run("scutil", "--get", "ComputerName")
	in.OSVersion = run("sw_vers", "-productVersion")
	in.OSBuild = run("sw_vers", "-buildVersion")
	in.CPU = run("sysctl", "-n", "machdep.cpu.brand_string")
	in.MemoryBytes, _ = strconv.ParseInt(run("sysctl", "-n", "hw.memsize"), 10, 64)
	in.BootTime = parseBootTime(run("sysctl", "-n", "kern.boottime"))
}
