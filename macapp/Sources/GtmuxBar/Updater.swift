import Combine
import Foundation

/// Updater backs the menu-bar "check for updates". It reuses the CLI's OWN updater
/// so a menu-bar update is the same as the user typing `gtmux update`: fetch +
/// SHA-verify the CLI tarball AND the menu-bar app (with the CN mirror fallback),
/// install the CLI to ~/.local/bin and the app to ~/Applications.
///
/// The install step `pkill`s + relaunches THIS app (to swap the bundle), so the
/// update is spawned DETACHED to outlive us — the installer reopens the new app
/// when it finishes. Because it's detached we can't wait on it, so the installer
/// records its exit code to a status file and a watchdog polls it: a FAILED update
/// (non-zero — a network blip, a SHA mismatch) flips to `.updateFailed` with a
/// retry, instead of sitting on the "Updating…" spinner forever.
final class Updater: ObservableObject {
    static let shared = Updater()

    enum State: Equatable {
        case idle
        case checking
        case upToDate(String) // current version
        case available(String) // latest version (newer than current)
        case updating
        case failed // a CHECK for updates failed (couldn't reach the release API)
        case updateFailed // an INSTALL failed — offer a retry; detail in lastError
    }

    @Published private(set) var state: State = .idle
    /// A short human reason for the last install failure, shown next to the retry.
    @Published private(set) var lastError: String?
    private var lastCheck: Date?

    // Watchdog for a detached install (see the type doc). Nil unless an install is in
    // flight. On success the installer kills us before the timeout, so it never fires.
    private var watchdog: DispatchSourceTimer?
    private var updateStartedAt: Date?
    private static let statusPath = NSTemporaryDirectory() + "gtmux-update.status"
    private static let logPath = NSTemporaryDirectory() + "gtmux-update.log"
    // A successful update kills+relaunches us in well under this; still alive past it
    // means the installer wedged (a stalled download with no progress) — surface it.
    // Real failures (no network, DNS/refused, SHA mismatch) exit non-zero and are
    // caught by the status file in ~2s, so this only backstops a truly hung download;
    // kept generous so a slow-but-working fetch of the app zip isn't failed early.
    private static let updateTimeout: TimeInterval = 180

    /// `available` carries the latest version when an update is waiting (for the UI).
    var newVersion: String? { if case .available(let v) = state { return v }; return nil }

    /// Auto-check shortly after launch, at most once a day. Silent: only flips to
    /// `.available` (which the popover surfaces) — never shows "up to date" noise.
    /// Throttled to a few checks a day: Sparkle's default is daily, but since we
    /// also call this each time the popover opens, a 6h floor keeps it fresh when
    /// you actually look without hitting the release API on every menu click.
    private let autoCheckInterval: TimeInterval = 6 * 3600
    func autoCheck() {
        if let last = lastCheck, Date().timeIntervalSince(last) < autoCheckInterval { return }
        check(silent: true)
    }

    /// User-initiated check. `silent` returns to `.idle` (not `.upToDate`/`.failed`)
    /// when nothing's new — used by the background path.
    func check(silent: Bool = false) {
        switch state {
        case .checking, .updating: return // already in flight
        default: break
        }
        setState(.checking)
        DispatchQueue.global(qos: .utility).async {
            let r = Self.runCheck()
            DispatchQueue.main.async {
                self.lastCheck = Date()
                if let r = r, r.update, !r.latest.isEmpty {
                    self.state = .available(r.latest)
                } else if let r = r, r.error.isEmpty {
                    self.state = silent ? .idle : .upToDate(r.current)
                } else {
                    self.state = silent ? .idle : .failed
                }
            }
        }
    }

    /// Install the latest release (CLI + app), detached so it survives the app being
    /// killed + relaunched by the installer. A watchdog turns a failed/wedged install
    /// into `.updateFailed` (with a retry) rather than a stuck "Updating…" spinner.
    func install() {
        if case .updating = state { return }
        lastError = nil
        // Drop any status from a prior attempt so the watchdog can't read a stale
        // non-zero as this run's result and fail us instantly.
        try? FileManager.default.removeItem(atPath: Self.statusPath)
        setState(.updating)
        DispatchQueue.global(qos: .utility).async { Self.spawnDetachedUpdate() }
        startWatchdog()
    }

    private func setState(_ s: State) {
        if Thread.isMainThread { state = s } else { DispatchQueue.main.async { self.state = s } }
    }

    // MARK: - Watchdog

    private func startWatchdog() {
        updateStartedAt = Date()
        watchdog?.cancel()
        let t = DispatchSource.makeTimerSource(queue: .main)
        t.schedule(deadline: .now() + 2, repeating: 2)
        t.setEventHandler { [weak self] in self?.pollUpdate() }
        watchdog = t
        t.resume()
    }

    private func stopWatchdog() {
        watchdog?.cancel()
        watchdog = nil
    }

    private func pollUpdate() {
        guard case .updating = state else { stopWatchdog(); return }
        // The installer recorded its exit code → the detached `gtmux update` finished.
        if let code = Self.recordedExit() {
            if code != 0 { failInstall(Self.failureReason()) }
            // exit 0 → it succeeded and is pkill'ing us; wait to be killed.
            return
        }
        // No result yet. A successful update kills us well within the timeout; still
        // alive past it → the installer wedged. Surface it (the detached job may still
        // finish and relaunch us — that just supersedes this failed state).
        if let started = updateStartedAt, Date().timeIntervalSince(started) > Self.updateTimeout {
            failInstall(nil)
        }
    }

    private func failInstall(_ reason: String?) {
        lastError = reason
        setState(.updateFailed)
        stopWatchdog()
    }

    /// The exit code the detached installer wrote on finishing ("exit:<n>"), or nil
    /// while it's still running (no file / not yet a full line).
    private static func recordedExit() -> Int? {
        guard let s = try? String(contentsOfFile: statusPath, encoding: .utf8) else { return nil }
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        guard t.hasPrefix("exit:") else { return nil }
        return Int(t.dropFirst("exit:".count))
    }

    /// A short reason from the tail of the installer log (its last non-empty line),
    /// for the retry row. Nil when there's nothing useful to show.
    private static func failureReason() -> String? {
        guard let s = try? String(contentsOfFile: logPath, encoding: .utf8) else { return nil }
        let last = s.split(whereSeparator: \.isNewline)
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .last(where: { !$0.isEmpty })
        guard let last = last, !last.isEmpty else { return nil }
        return last.count > 140 ? String(last.suffix(140)) : last
    }

    // MARK: - CLI plumbing

    private struct CheckResult {
        let current: String
        let latest: String
        let update: Bool
        let error: String
    }

    private static func runCheck() -> CheckResult? {
        guard let data = GtmuxCLI.capture(["update", "--check", "--json"]),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return nil }
        return CheckResult(
            current: obj["current"] as? String ?? "",
            latest: obj["latest"] as? String ?? "",
            update: obj["update"] as? Bool ?? false,
            error: obj["error"] as? String ?? "")
    }

    /// Resolve the user's installed CLI (NOT the in-bundle copy) so `gtmux update`
    /// targets the same dir the user's own `gtmux update` would (its in-place logic
    /// installs over the running binary's dir). If only the bundled CLI exists,
    /// drive it but pin BIN_DIR to the standard ~/.local/bin so the CLI lands there
    /// rather than inside the app bundle.
    private static func updateTarget() -> (cli: String, binDir: String?) {
        let fm = FileManager.default
        let home = fm.homeDirectoryForCurrentUser.path
        for c in ["\(home)/.local/bin/gtmux", "/opt/homebrew/bin/gtmux", "/usr/local/bin/gtmux"]
        where fm.isExecutableFile(atPath: c) {
            return (c, nil)
        }
        return (GtmuxCLI.path, "\(home)/.local/bin")
    }

    private static func spawnDetachedUpdate() {
        let (cli, binDir) = updateTarget()
        func shq(_ s: String) -> String { "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        var inner = ""
        if let binDir = binDir { inner += "GTMUX_BIN_DIR=\(shq(binDir)) " }
        inner += "\(shq(cli)) update"
        // Record the installer's exit code so the still-running app can tell a FAILED
        // update (non-zero) from a successful one (which pkills+relaunches us before we
        // ever read it). Written after `gtmux update` returns, whatever the outcome.
        let wrapped = "\(inner); echo \"exit:$?\" > \(shq(Self.statusPath))"
        // nohup (ignore SIGHUP) + background + </dev/null: when the launching /bin/sh
        // exits the job reparents to launchd and keeps running after the installer
        // kills GtmuxBar. The installer reopens the freshly-installed app.
        let script = "nohup sh -c \(shq(wrapped)) >\(shq(Self.logPath)) 2>&1 </dev/null &"
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sh")
        p.arguments = ["-c", script]
        try? p.run()
        p.waitUntilExit() // returns at once: sh backgrounds the job, then exits
    }
}
