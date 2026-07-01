import Combine
import Foundation

/// Updater backs the menu-bar "check for updates". It reuses the CLI's OWN updater
/// so a menu-bar update is the same as the user typing `gtmux update`: fetch +
/// SHA-verify the CLI tarball AND the menu-bar app (with the CN mirror fallback),
/// install the CLI to ~/.local/bin and the app to ~/Applications.
///
/// The install step `pkill`s + relaunches THIS app (to swap the bundle), so the
/// update is spawned DETACHED to outlive us — the installer reopens the new app
/// when it finishes.
final class Updater: ObservableObject {
    static let shared = Updater()

    enum State: Equatable {
        case idle
        case checking
        case upToDate(String) // current version
        case available(String) // latest version (newer than current)
        case updating
        case failed
    }

    @Published private(set) var state: State = .idle
    private var lastCheck: Date?

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

    /// Install the latest release (CLI + app), detached so it survives the app
    /// being killed + relaunched by the installer.
    func install() {
        if case .updating = state { return }
        setState(.updating)
        DispatchQueue.global(qos: .utility).async { Self.spawnDetachedUpdate() }
    }

    private func setState(_ s: State) {
        if Thread.isMainThread { state = s } else { DispatchQueue.main.async { self.state = s } }
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
        let log = NSTemporaryDirectory() + "gtmux-update.log"
        func shq(_ s: String) -> String { "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        var inner = ""
        if let binDir = binDir { inner += "GTMUX_BIN_DIR=\(shq(binDir)) " }
        inner += "\(shq(cli)) update"
        // nohup (ignore SIGHUP) + background + </dev/null: when the launching /bin/sh
        // exits the job reparents to launchd and keeps running after the installer
        // kills GtmuxBar. The installer reopens the freshly-installed app.
        let script = "nohup \(inner) >\(shq(log)) 2>&1 </dev/null &"
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sh")
        p.arguments = ["-c", script]
        try? p.run()
        p.waitUntilExit() // returns at once: sh backgrounds the job, then exits
    }
}
