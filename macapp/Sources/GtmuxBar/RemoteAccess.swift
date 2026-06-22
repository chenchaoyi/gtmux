import Combine
import Foundation

/// RemoteAccess reflects + drives the opt-in "always-on" tunnel (the CLI's
/// `gtmux tunnel --service`). State is the presence of the two LaunchAgents (same
/// signal the CLI uses), so it stays truthful even if toggled from the terminal.
/// Enabling is a STANDING public exposure, so the Preferences toggle confirms
/// first and the popover shows a visible indicator while it's on.
final class RemoteAccess: ObservableObject {
    static let shared = RemoteAccess()

    @Published private(set) var isOn = false
    @Published private(set) var busy = false

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
        let on = fm.fileExists(atPath: servePlist) && fm.fileExists(atPath: tunnelPlist)
        DispatchQueue.main.async { self.isOn = on }
    }

    /// Enable always-on (the toggle confirms first). Runs the CLI off-main; the
    /// `--yes` flag skips the CLI's own prompt since the UI already confirmed.
    func enable() { run(["tunnel", "--service", "--yes"]) }
    func disable() { run(["tunnel", "--unservice"]) }

    private func run(_ args: [String]) {
        guard !busy else { return }
        busy = true
        DispatchQueue.global().async {
            _ = GtmuxCLI.capture(args)
            DispatchQueue.main.async {
                self.busy = false
                self.refresh()
            }
        }
    }
}
