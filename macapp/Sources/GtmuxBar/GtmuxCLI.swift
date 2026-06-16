import Foundation

/// GtmuxCLI locates and runs the cgo-free `gtmux` binary the app consumes.
/// The app is a pure consumer of the CLI's contract (`agents --json`, `focus`),
/// so gtmux-core stays the single data source.
enum GtmuxCLI {
    /// Resolved path to `gtmux`. Lookup order: $GTMUX_BIN → a sibling inside the
    /// app bundle (version-matched) → ~/.local/bin → PATH → Homebrew dirs.
    static let path: String = resolve()

    private static func resolve() -> String {
        let fm = FileManager.default
        if let env = ProcessInfo.processInfo.environment["GTMUX_BIN"], !env.isEmpty {
            return env
        }
        if let exe = Bundle.main.executablePath {
            let sibling = (exe as NSString).deletingLastPathComponent + "/gtmux"
            if fm.isExecutableFile(atPath: sibling) { return sibling }
        }
        let home = fm.homeDirectoryForCurrentUser.path
        for candidate in [
            "\(home)/.local/bin/gtmux",
            "/opt/homebrew/bin/gtmux",
            "/usr/local/bin/gtmux",
        ] where fm.isExecutableFile(atPath: candidate) {
            return candidate
        }
        return "/usr/bin/env" // last resort handled by callers (env gtmux …)
    }

    private static func makeProcess(_ args: [String]) -> Process {
        let proc = Process()
        if path == "/usr/bin/env" {
            proc.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            proc.arguments = ["gtmux"] + args
        } else {
            proc.executableURL = URL(fileURLWithPath: path)
            proc.arguments = args
        }
        return proc
    }

    /// Run gtmux and return its stdout (nil on failure). Blocking — call off-main.
    static func capture(_ args: [String]) -> Data? {
        let proc = makeProcess(args)
        let out = Pipe()
        proc.standardOutput = out
        proc.standardError = FileHandle.nullDevice
        do { try proc.run() } catch { return nil }
        let data = out.fileHandleForReading.readDataToEndOfFile()
        proc.waitUntilExit()
        return data
    }

    /// Fire-and-forget (focus / restore / new) — don't block the UI on it.
    static func spawn(_ args: [String]) {
        let proc = makeProcess(args)
        proc.standardOutput = FileHandle.nullDevice
        proc.standardError = FileHandle.nullDevice
        try? proc.run()
    }

    /// A shell-quoted invocation string for embedding in an AppleScript command.
    static func shellInvocation(_ args: [String]) -> String {
        let quoted = ([path == "/usr/bin/env" ? "gtmux" : path] + args)
            .map { "'" + $0.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        return quoted.joined(separator: " ")
    }
}
