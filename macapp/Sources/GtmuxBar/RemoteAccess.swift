import Combine
import Foundation

/// RemoteAccess reflects + drives the opt-in "always-on" tunnel (the CLI's
/// `gtmux tunnel --service`). State is the presence of the two LaunchAgents (same
/// signal the CLI uses), so it stays truthful even if toggled from the terminal.
/// Enabling is a STANDING public exposure, so the Preferences toggle confirms
/// first and the popover shows a visible indicator while it's on.
/// The three remote-access modes, derived from which LaunchAgents exist:
/// off (neither) / lan (serve only — same Wi-Fi, free) / anywhere (serve + tunnel
/// — the Pro always-on tunnel). They're mutually exclusive: one selectable control.
enum RemoteMode {
    case off, lan, anywhere
}

final class RemoteAccess: ObservableObject {
    static let shared = RemoteAccess()

    @Published private(set) var mode: RemoteMode = .off
    @Published private(set) var busy = false
    /// A human-readable reason the last switch didn't take (e.g. "Anywhere" needs
    /// a hosted build), or nil. Surfaced by the UI so a failed enable explains
    /// itself instead of silently snapping back to the previous mode.
    @Published var lastError: String?

    /// Back-compat: callers that only care about the always-on tunnel being up.
    var isOn: Bool { mode == .anywhere }

    private var agentsDir: String { "\(NSHomeDirectory())/Library/LaunchAgents" }
    private var servePlist: String { "\(agentsDir)/com.gtmux.serve.plist" }
    private var tunnelPlist: String { "\(agentsDir)/com.gtmux.tunnel.plist" }
    private var urlPath: String { "\(NSHomeDirectory())/.config/gtmux/tunnel-url" }

    /// The stable public URL, when always-on is set up (for display).
    var url: String? {
        guard let s = try? String(contentsOfFile: urlPath, encoding: .utf8) else { return nil }
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        return t.isEmpty ? nil : t
    }

    func refresh() {
        let fm = FileManager.default
        let serveOn = fm.fileExists(atPath: servePlist)
        let tunnelOn = fm.fileExists(atPath: tunnelPlist)
        let m: RemoteMode = tunnelOn ? .anywhere : (serveOn ? .lan : .off)
        DispatchQueue.main.async { self.mode = m }
    }

    /// Enable LAN (same Wi-Fi) access — the free mode. Removes the tunnel if any.
    func enableLan() { run(["serve", "--service"], expect: .lan) }

    /// Enable the always-on tunnel (Pro). The UI confirms the standing exposure
    /// first; `--yes` skips the CLI's own prompt.
    func enableAnywhere() { run(["tunnel", "--service", "--yes"], expect: .anywhere) }

    /// Turn remote access off from any mode (removes whichever agents exist).
    func turnOff() { run(["serve", "--unservice"], expect: .off) }

    // Back-compat shims (Pairing/Preferences migrated to the explicit methods).
    func enable() { enableAnywhere() }
    func disable() { turnOff() }

    /// Run a state-changing CLI command, then settle `mode` to ground truth. If
    /// the resulting mode isn't what we asked for, publish `lastError` (the CLI's
    /// own stderr when it gave one) so the UI can explain the silent revert.
    private func run(_ args: [String], expect: RemoteMode) {
        guard !busy else { return }
        busy = true
        lastError = nil
        DispatchQueue.global().async {
            let res = GtmuxCLI.captureResult(args)
            let fm = FileManager.default
            let serveOn = fm.fileExists(atPath: self.servePlist)
            let tunnelOn = fm.fileExists(atPath: self.tunnelPlist)
            let m: RemoteMode = tunnelOn ? .anywhere : (serveOn ? .lan : .off)
            DispatchQueue.main.async {
                self.mode = m
                self.busy = false
                if m != expect {
                    self.lastError = res.stderr.isEmpty
                        ? self.genericFailure(expect)
                        : res.stderr
                }
            }
        }
    }

    private func genericFailure(_ expect: RemoteMode) -> String {
        switch expect {
        case .anywhere:
            return "Couldn't turn on Anywhere access. / 无法开启任意网络访问。"
        case .lan:
            return "Couldn't turn on Wi-Fi access. / 无法开启局域网访问。"
        case .off:
            return "Couldn't turn off remote access. / 无法关闭远程访问。"
        }
    }
}
