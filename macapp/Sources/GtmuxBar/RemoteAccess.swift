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

/// The tunnel backend behind "anywhere" mode (see RemoteAccess.backend).
enum TunnelBackend {
    case none, cloudflare, selfHosted
}

/// One live remote viewer, mirrored from the serve's `remote-clients.json`
/// roster: a paired phone (`kind == "phone"`, `name` from the enroll roster) or
/// an anonymous browser mirror (`kind == "browser"`, `platform` sniffed from the
/// User-Agent). So the Mac can show WHO is connected, not just how many.
struct RemoteClient: Identifiable {
    let name: String
    let kind: String // "phone" | "browser"
    let platform: String
    let ip: String
    let connectedAt: Double
    // Stable across polls: identity is name/platform/ip, so the SwiftUI list
    // doesn't re-animate rows while a viewer stays connected.
    var id: String { "\(kind)|\(name)|\(platform)|\(ip)" }

    var isPhone: Bool { kind == "phone" }
    /// The row's primary label: a phone's name, else the browser platform, else a
    /// generic fallback (both languages, since callers pick by GTMUX_LANG upstream).
    func title(_ tr: (String, String) -> String) -> String {
        if isPhone { return name.isEmpty ? tr("Phone", "手机") : name }
        return platform.isEmpty ? tr("Browser", "浏览器") : platform
    }

    /// A human-readable "how long connected" label from `connectedAt` — far more
    /// meaningful to a person than the raw client IP the row used to trail with.
    /// Computed at render (the popover re-reads on open), so it's fresh each time.
    func connectedFor(_ tr: (String, String) -> String, now: Double = Date().timeIntervalSince1970) -> String {
        guard connectedAt > 0 else { return "" }
        let s = max(0, now - connectedAt)
        if s < 60 { return tr("just now", "刚刚") }
        let m = Int(s / 60)
        if m < 60 { return tr("connected \(m)m", "已连接 \(m) 分钟") }
        let h = Int(s / 3600)
        if h < 24 { return tr("connected \(h)h", "已连接 \(h) 小时") }
        return tr("connected \(Int(s / 86400))d", "已连接 \(Int(s / 86400)) 天")
    }

    /// The row's trailing detail: a phone's OS tag ("iOS 17.5", sent live) plus how
    /// long it's been connected, joined "iOS 17.5 · connected 5m". A browser already
    /// shows its platform as the title, so it trails with the duration only.
    func subtitle(_ tr: (String, String) -> String, now: Double = Date().timeIntervalSince1970) -> String {
        var parts: [String] = []
        if isPhone && !platform.isEmpty { parts.append(platform) }
        let dur = connectedFor(tr, now: now)
        if !dur.isEmpty { parts.append(dur) }
        return parts.joined(separator: " · ")
    }
}

final class RemoteAccess: ObservableObject {
    static let shared = RemoteAccess()

    @Published private(set) var mode: RemoteMode = .off
    /// Which tunnel backend is providing "anywhere" access, when mode == .anywhere:
    /// the zero-config hosted Cloudflare tunnel, or a self-hosted one on the user's
    /// own VPS+domain (`gtmux tunnel --backend self`). Inferred from which agent exists.
    @Published private(set) var backend: TunnelBackend = .none
    @Published private(set) var busy = false
    /// A human-readable reason the last switch didn't take (e.g. "Anywhere" needs
    /// a hosted build), or nil. Surfaced by the UI so a failed enable explains
    /// itself instead of silently snapping back to the previous mode.
    @Published var lastError: String?

    /// How many remote viewers (phone/browser) are connected RIGHT NOW — derived
    /// from the serve's `remote-clients.json` (live SSE-client count), with a
    /// staleness guard so a dead serve reads as 0. Surfaced as a popover indicator
    /// so you know when someone is looking at this Mac.
    @Published private(set) var remoteClients: Int = 0

    /// WHO is connected right now (paired phones by name; browsers as anonymous
    /// "Safari · macOS"-style rows), same staleness guard as the count. Empty when
    /// nobody's viewing or the serve is dead. Ordered oldest-connection-first by
    /// the serve, so the list is stable.
    @Published private(set) var remoteClientList: [RemoteClient] = []

    /// Back-compat: callers that only care about the always-on tunnel being up.
    var isOn: Bool { mode == .anywhere }

    private var agentsDir: String { "\(NSHomeDirectory())/Library/LaunchAgents" }
    private var servePlist: String { "\(agentsDir)/com.gtmux.serve.plist" }
    private var tunnelPlist: String { "\(agentsDir)/com.gtmux.tunnel.plist" }
    private var selfTunnelPlist: String { "\(agentsDir)/com.gtmux.selftunnel.plist" }
    private var urlPath: String { "\(NSHomeDirectory())/.config/gtmux/tunnel-url" }
    private var clientsPath: String { "\(NSHomeDirectory())/.local/share/gtmux/remote-clients.json" }
    /// remote-clients.json older than this is treated as stale (serve gone) → 0.
    private let clientsStaleAfter: TimeInterval = 8

    /// The stable public URL, when always-on is set up (for display).
    var url: String? {
        guard let s = try? String(contentsOfFile: urlPath, encoding: .utf8) else { return nil }
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        return t.isEmpty ? nil : t
    }

    func refresh() {
        let (m, b) = groundTruth()
        DispatchQueue.main.async { self.mode = m; self.backend = b }
        refreshClients()
    }

    /// The current remote mode + tunnel backend, derived from which LaunchAgents
    /// exist. "anywhere" is either tunnel backend; a self-hosted agent implies the
    /// self backend, else Cloudflare.
    private func groundTruth() -> (RemoteMode, TunnelBackend) {
        let fm = FileManager.default
        let serveOn = fm.fileExists(atPath: servePlist)
        let cfOn = fm.fileExists(atPath: tunnelPlist)
        let selfOn = fm.fileExists(atPath: selfTunnelPlist)
        let mode: RemoteMode = (cfOn || selfOn) ? .anywhere : (serveOn ? .lan : .off)
        let backend: TunnelBackend = selfOn ? .selfHosted : (cfOn ? .cloudflare : .none)
        return (mode, backend)
    }

    /// Re-read the live remote-viewer count from `remote-clients.json`, honoring
    /// staleness (a dead serve stops heartbeating → reads as 0). Cheap; safe to
    /// call on the poll timer.
    func refreshClients() {
        var n = 0
        var list: [RemoteClient] = []
        if let data = try? Data(contentsOf: URL(fileURLWithPath: clientsPath)),
           let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let at = obj["at"] as? Double,
           Date().timeIntervalSince1970 - at <= clientsStaleAfter {
            // Prefer the identified roster; fall back to the bare count for a file
            // written by an older serve (no `clients` key).
            if let raw = obj["clients"] as? [[String: Any]] {
                list = raw.map { c in
                    RemoteClient(
                        name: c["name"] as? String ?? "",
                        kind: c["kind"] as? String ?? "browser",
                        platform: c["platform"] as? String ?? "",
                        ip: c["ip"] as? String ?? "",
                        connectedAt: c["connectedAt"] as? Double ?? 0)
                }
                n = list.count
            } else if let count = obj["count"] as? Int {
                n = count
            }
        }
        if n != remoteClients {
            DispatchQueue.main.async { self.remoteClients = n }
        }
        if list.map(\.id) != remoteClientList.map(\.id) {
            DispatchQueue.main.async { self.remoteClientList = list }
        }
    }

    /// Enable LAN (same Wi-Fi) access — the free mode. Removes the tunnel if any.
    func enableLan() { run(["serve", "--service"], expect: .lan) }

    /// Enable the always-on tunnel (Pro), choosing the backend: the zero-config
    /// hosted Cloudflare tunnel, or a self-hosted one (your own VPS+domain, config in
    /// ~/.config/gtmux/selftunnel.conf). The UI confirms the standing exposure first.
    func enableAnywhere(selfHosted: Bool = false) {
        let args = selfHosted
            ? ["tunnel", "--backend", "self", "--service", "--yes"]
            : ["tunnel", "--service", "--yes"]
        run(args, expect: .anywhere)
    }

    /// Whether Direct is unlocked on this Mac (its server config is present — written
    /// by `--redeem` or by a user pointing at their own server). Reads the shared conf.
    var selfTunnelConfigured: Bool {
        let p = "\(NSHomeDirectory())/.config/gtmux/selftunnel.conf"
        guard let s = try? String(contentsOfFile: p, encoding: .utf8) else { return false }
        return s.contains("url=") && s.contains("secret=")
    }

    /// Redeem a paid Direct access code: the CLI validates it server-side and, on
    /// success, writes selftunnel.conf (→ selfTunnelConfigured flips true). completion
    /// gets nil on success, else a short error. Direct's server + secret are never in
    /// the binary — only a valid code fetches them.
    func redeemDirect(_ code: String, completion: @escaping (String?) -> Void) {
        let c = code.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !c.isEmpty else { completion("Enter your Direct code. / 请输入 Direct 访问码。"); return }
        DispatchQueue.global().async {
            let res = GtmuxCLI.captureResult(["tunnel", "--redeem", c])
            DispatchQueue.main.async {
                if res.status == 0 {
                    completion(nil)
                } else {
                    completion(res.stderr.isEmpty ? "Couldn't unlock Direct. / 无法解锁 Direct。" : res.stderr)
                }
            }
        }
    }

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
            let (m, b) = self.groundTruth()
            DispatchQueue.main.async {
                self.mode = m
                self.backend = b
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
