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

    private static func makeProcess(_ args: [String], cwd: String? = nil) -> Process {
        let proc = Process()
        proc.environment = childEnvironment()
        // A cwd only matters for the few commands that are JUDGED by it. `gtmux knowledge`
        // accepts a mutation only from the HQ home (the cwd-keyed role rule that keeps
        // workers out of the quality gate), so a caller acting on the knowledge base runs
        // the verb FROM there rather than asking for an exemption.
        if let cwd { proc.currentDirectoryURL = URL(fileURLWithPath: cwd) }
        if path == "/usr/bin/env" {
            proc.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            proc.arguments = ["gtmux"] + args
        } else {
            proc.executableURL = URL(fileURLWithPath: path)
            proc.arguments = args
        }
        return proc
    }

    /// The environment gtmux runs in. Identical to ours except PATH, which gets the
    /// usual install locations PREPENDED.
    ///
    /// A GUI app inherits launchd's PATH — `/usr/bin:/bin:/usr/sbin:/sbin` — which has
    /// neither Homebrew prefix on it. gtmux shells out to real tools (cloudflared, brew,
    /// tmux, git), so with that PATH it reported cloudflared as "not installed" and
    /// Homebrew as "not installed to fetch it" on a Mac holding both in /usr/local/bin:
    /// switching to Anywhere from the menu bar silently could not work, while the exact
    /// same command from a terminal did. We already resolve gtmux ITSELF across these
    /// same directories; anything gtmux then calls deserves the same PATH.
    private static func childEnvironment() -> [String: String] {
        var env = ProcessInfo.processInfo.environment
        let extras = ["/opt/homebrew/bin", "/usr/local/bin", NSHomeDirectory() + "/.local/bin"]
        let current = env["PATH"] ?? "/usr/bin:/bin:/usr/sbin:/sbin"
        let have = Set(current.split(separator: ":").map(String.init))
        let prefix = extras.filter { !have.contains($0) }
        env["PATH"] = prefix.isEmpty ? current : prefix.joined(separator: ":") + ":" + current
        return env
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

    /// Run gtmux and return its exit status + trimmed stderr. Blocking — call
    /// off-main. status is -1 if the process couldn't be launched. Used by
    /// state-changing commands (e.g. `tunnel --service`) that need to surface a
    /// failure reason instead of failing silently.
    static func captureResult(_ args: [String]) -> (status: Int32, stderr: String) {
        let proc = makeProcess(args)
        let err = Pipe()
        proc.standardOutput = FileHandle.nullDevice
        proc.standardError = err
        do { try proc.run() } catch { return (-1, "") }
        let data = err.fileHandleForReading.readDataToEndOfFile()
        proc.waitUntilExit()
        let msg = String(data: data, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return (proc.terminationStatus, msg)
    }

    /// Run gtmux and return its exit status + BOTH streams, trimmed. Blocking — call
    /// off-main. status is -1 (with an empty stderr) if the process could not be launched
    /// at all, which is the case a caller has to word itself: there is no CLI output to
    /// quote when there was no CLI.
    ///
    /// Both pipes are drained concurrently. Reading one to the end and then the other
    /// deadlocks the moment the writer fills the pipe buffer of the stream nobody is
    /// reading yet — which the other two helpers dodge by pointing one stream at
    /// /dev/null, an option a caller that needs both does not have.
    static func captureFull(_ args: [String], cwd: String? = nil)
        -> (status: Int32, stdout: String, stderr: String) {
        let proc = makeProcess(args, cwd: cwd)
        let out = Pipe(), err = Pipe()
        proc.standardOutput = out
        proc.standardError = err
        do { try proc.run() } catch { return (-1, "", "") }
        var outData = Data(), errData = Data()
        let group = DispatchGroup()
        let q = DispatchQueue.global(qos: .userInitiated)
        group.enter()
        q.async {
            outData = out.fileHandleForReading.readDataToEndOfFile()
            group.leave()
        }
        group.enter()
        q.async {
            errData = err.fileHandleForReading.readDataToEndOfFile()
            group.leave()
        }
        group.wait()
        proc.waitUntilExit()
        let text = { (d: Data) in
            String(data: d, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        }
        return (proc.terminationStatus, text(outData), text(errData))
    }

    /// Fire-and-forget (focus / restore / new) — don't block the UI on it.
    static func spawn(_ args: [String]) { spawn(args, onExit: nil) }

    /// spawn, with a completion callback on the main queue.
    ///
    /// The fire-and-forget form is right for a jump — it is over before you look. It is
    /// wrong for anything SLOW: the restore row spawned and returned, so the menu bar had
    /// no idea the work was running, showed no change, and invited the second click that
    /// used to restore everything twice.
    static func spawn(_ args: [String], onExit: (() -> Void)?) {
        let proc = makeProcess(args)
        proc.standardOutput = FileHandle.nullDevice
        proc.standardError = FileHandle.nullDevice
        if let onExit {
            proc.terminationHandler = { _ in DispatchQueue.main.async(execute: onExit) }
        }
        do {
            try proc.run()
        } catch {
            // Never leave a caller waiting on a process that never started.
            if let onExit { DispatchQueue.main.async(execute: onExit) }
        }
    }

    /// A shell-quoted invocation string for embedding in an AppleScript command.
    static func shellInvocation(_ args: [String]) -> String {
        let quoted = ([path == "/usr/bin/env" ? "gtmux" : path] + args)
            .map { "'" + $0.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        return quoted.joined(separator: " ")
    }
}
