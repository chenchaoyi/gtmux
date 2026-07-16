import AppKit
import Combine
import Foundation

/// What a still-`.updating` app should do once the detached installer has recorded
/// exit code 0. The installer's job is to pkill + relaunch us; on success we're
/// already dead and this never runs. Still `.updating` past the grace ⇒ the relaunch
/// did NOT take (a bare-`open` re-activate of the dying old instance, or the app step
/// silently skipped) — decide by comparing the on-disk bundle version to ours. Pure +
/// unit-tested, mirroring the single-instance guard's testable decision.
enum PostExitZeroAction: Equatable {
    case wait      // within grace — the relaunch is imminent; keep waiting to be killed
    case relaunch  // grace elapsed, on-disk bundle differs (⇒ newer) — swap ok, relaunch missed
    case fail      // grace elapsed, on-disk == running (or unreadable) — the swap never happened
}

/// `installedVersion != runningVersion` ⇒ newer here: a user-triggered update only ever
/// installs the LATEST release, so the only way the on-disk bundle differs from the
/// running one after `exit:0` is that the swap landed the new version but we weren't
/// relaunched. Equal (or unreadable) ⇒ the app was never swapped → offer a retry.
func postExitZeroAction(
    secondsSinceExitZero: TimeInterval,
    grace: TimeInterval,
    runningVersion: String,
    installedVersion: String?
) -> PostExitZeroAction {
    if secondsSinceExitZero < grace { return .wait }
    guard let installed = installedVersion, !installed.isEmpty, installed != runningVersion
    else { return .fail }
    return .relaunch
}

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
/// retry, instead of sitting on the "Updating…" spinner forever. A recorded exit:0
/// that DOESN'T kill us within a short grace (the installer's relaunch was missed —
/// a bare-`open` re-activate, or the app step skipped) self-heals: if the on-disk
/// bundle is the newer version we `open -n` it and quit; if not, we flip to
/// `.updateFailed`. Net: no path can sit on "Updating…" forever.
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
    // When the installer first recorded exit:0. On success it pkills + relaunches us
    // within ~1–2s of this; still `.updating` past `graceAfterExit` ⇒ the relaunch was
    // missed, and we self-heal rather than spin forever (see pollUpdate).
    private var exitZeroAt: Date?
    private static let statusPath = NSTemporaryDirectory() + "gtmux-update.status"
    private static let logPath = NSTemporaryDirectory() + "gtmux-update.log"
    // Grace after a recorded exit:0 before we conclude the relaunch didn't take. A
    // real relaunch kills us in ~1–2s; kept short so a stuck spinner self-heals fast.
    private static let graceAfterExit: TimeInterval = 12
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
        exitZeroAt = nil // fresh attempt (incl. a retry) restarts the exit:0 grace clock
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
            if code != 0 { failInstall(Self.failureReason()); return }
            // exit 0 → it should be pkill'ing + relaunching us. Give the relaunch a
            // short grace; if we're STILL alive past it, the relaunch was missed — a
            // bare-`open` re-activate of the dying old instance, or the app step
            // silently skipped. Decide via the on-disk bundle version so we never spin
            // forever on "Updating…".
            if exitZeroAt == nil { exitZeroAt = Date() }
            switch postExitZeroAction(
                secondsSinceExitZero: Date().timeIntervalSince(exitZeroAt!),
                grace: Self.graceAfterExit,
                runningVersion: Self.runningVersion(),
                installedVersion: Self.installedAppVersion()) {
            case .wait:
                return // relaunch imminent — keep waiting to be killed
            case .relaunch:
                relaunchInstalledApp() // swap landed the new version; we just weren't relaunched
            case .fail:
                failInstall(Self.failureReason() ?? Self.appNotInstalledReason())
            }
            return
        }
        // No result yet. A successful update kills us well within the timeout; still
        // alive past it → the installer wedged before it even recorded an exit. Surface
        // it (the detached job may still finish and relaunch us — that just supersedes
        // this failed state).
        if let started = updateStartedAt, Date().timeIntervalSince(started) > Self.updateTimeout {
            failInstall(nil)
        }
    }

    /// Self-heal a missed relaunch (see pollUpdate): force-launch the freshly-swapped
    /// bundle as a NEW instance (`open -n`) and terminate ourselves. The new (newer)
    /// instance's single-instance guard finishes the handover; terminating is the belt.
    private func relaunchInstalledApp() {
        stopWatchdog()
        let app = Self.installedAppPath()
        guard !app.isEmpty else { failInstall(Self.appNotInstalledReason()); return }
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        p.arguments = ["-n", app]
        try? p.run()
        // Give the new instance a beat to come up (and its guard to fire), then exit.
        DispatchQueue.main.asyncAfter(deadline: .now() + 1) { NSApp.terminate(nil) }
    }

    private static func appNotInstalledReason() -> String {
        L10n.shared.tr("the app update didn't install — retry", "app 更新未安装成功 —— 请重试")
    }

    /// Where the installed menu-bar bundle lives — install.sh targets ~/Applications;
    /// the Homebrew-cask default /Applications is the fallback. "" if neither exists.
    private static func installedAppPath() -> String {
        let fm = FileManager.default
        let home = fm.homeDirectoryForCurrentUser.path
        for p in ["\(home)/Applications/Gtmux.app", "/Applications/Gtmux.app"]
        where fm.fileExists(atPath: p) { return p }
        return ""
    }

    /// The on-disk installed bundle's version, read straight from its Info.plist — NOT
    /// Bundle.main, which keeps the OLD version in memory after a swap. nil if the
    /// bundle or the key is unreadable.
    private static func installedAppVersion() -> String? {
        let app = installedAppPath()
        guard !app.isEmpty,
              let info = NSDictionary(contentsOfFile: app + "/Contents/Info.plist"),
              let v = info["CFBundleShortVersionString"] as? String, !v.isEmpty
        else { return nil }
        return v
    }

    private static func runningVersion() -> String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? ""
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

    /// Build the `gtmux update` invocation run inside the detached shell. Pure (given
    /// the resolved CLI + bin dir) so a test can pin its shape.
    ///
    /// `env -u GTMUX_VERSION`: installer runs leak GTMUX_VERSION into the app's
    /// environment (see install.sh's `open -n`), and if the app forwarded that pin to
    /// `gtmux update`, Go would honor it and reinstall the CURRENT version instead of
    /// resolving the latest — the self-update loops and the banner never clears. Strip
    /// it so a menu-bar update always targets the newest release.
    static func updateCommand(cli: String, binDir: String?) -> String {
        func shq(_ s: String) -> String { "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        var inner = "env -u GTMUX_VERSION "
        if let binDir = binDir { inner += "GTMUX_BIN_DIR=\(shq(binDir)) " }
        inner += "\(shq(cli)) update"
        return inner
    }

    private static func spawnDetachedUpdate() {
        let (cli, binDir) = updateTarget()
        func shq(_ s: String) -> String { "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        let inner = updateCommand(cli: cli, binDir: binDir)
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
