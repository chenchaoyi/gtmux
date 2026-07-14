import AppKit
import Combine
import Foundation

/// One guest share link — a `scope:"guest"` entry in the serve roster, surfaced
/// token-free by `gtmux share status --json` (id + label + when it was minted).
struct GuestLink: Identifiable, Equatable {
    let id: String
    let label: String
    let enrolledAt: Int
}

/// ShareStore reflects + drives web-shared input (the CLI's `gtmux share`): the
/// host's consent to let a collaborator on the shared web page type into the
/// terminal, the per-pane allowlist that scopes it, and the guest links that carry
/// it. Like RemoteAccess, state stays truthful even when toggled from the terminal:
/// the cheap poll reads `share.json` directly (no secrets), and the fuller detail
/// (guest links + base URL) comes from the CLI's token-free `--json`, so the app
/// never opens the token roster. Every MUTATION shells out to `gtmux share …`, so
/// gtmux-core stays the single source of the policy.
final class ShareStore: ObservableObject {
    static let shared = ShareStore()

    @Published private(set) var enabled = false
    /// Panes a guest may TYPE into (the input allowlist; ⊆ viewPanes).
    @Published private(set) var allowedPanes: Set<String> = []
    /// Panes a guest may SEE (the view allowlist). Independent of `enabled` (which
    /// gates typing): a host can let a guest watch a pane without letting them type.
    @Published private(set) var viewPanes: Set<String> = []
    @Published private(set) var guests: [GuestLink] = []
    /// The base a share link is built on (tunnel URL when up, else the local
    /// address) — shown so the host knows whether a minted link is publicly
    /// reachable. Loaded with the guest detail.
    @Published private(set) var base = ""
    @Published var busy = false
    /// The most recently minted share link, surfaced + copied so the host can send
    /// it to a collaborator. Cleared when the section reloads.
    @Published var lastMintedLink: String?
    /// A short reason the last action didn't take (the CLI's stderr), or nil.
    @Published var lastError: String?

    /// Shared input is a live, standing exposure ONLY when all three hold: the host
    /// consented, at least one pane is allowlisted, and at least one guest link
    /// exists to carry it. Absent any one, nobody can type in — so the popover
    /// indicator stays quiet (a type-into-terminal exposure is never silent, but it
    /// also never cries wolf).
    var isLive: Bool { enabled && !allowedPanes.isEmpty && !guests.isEmpty }

    private var sharePath: String { "\(NSHomeDirectory())/.config/gtmux/share.json" }

    /// Cheap, poll-safe read of the consent + allowlist straight from `share.json`
    /// (no process spawn, no secrets). Drives the popover indicator and keeps the
    /// Preferences toggles live even when changed from a terminal.
    func refresh() {
        var on = false
        var panes: Set<String> = []
        var view: Set<String> = []
        if let data = try? Data(contentsOf: URL(fileURLWithPath: sharePath)),
           let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
            on = obj["enabled"] as? Bool ?? false
            if let ps = obj["panes"] as? [String] { panes = Set(ps) }
            if let vs = obj["view_panes"] as? [String] { view = Set(vs) }
        }
        if on != enabled { DispatchQueue.main.async { self.enabled = on } }
        if panes != allowedPanes { DispatchQueue.main.async { self.allowedPanes = panes } }
        if view != viewPanes { DispatchQueue.main.async { self.viewPanes = view } }
    }

    /// Load the full detail (guest links + base) via the CLI's token-free `--json`.
    /// Called on the Preferences section appearing and after every mutation — not on
    /// the poll timer — so the token roster is read (by the CLI, never the app) only
    /// when the host is actually looking.
    func loadDetail() {
        DispatchQueue.global().async {
            guard let data = GtmuxCLI.capture(["share", "status", "--json"]),
                  let parsed = ShareStore.parseStatus(data) else { return }
            DispatchQueue.main.async {
                self.enabled = parsed.enabled
                self.allowedPanes = parsed.panes
                self.viewPanes = parsed.viewPanes
                self.guests = parsed.guests
                self.base = parsed.base
            }
        }
    }

    /// Pure parser for `gtmux share status --json` — unit-tested against the wire
    /// shape so a contract drift fails the build.
    static func parseStatus(_ data: Data)
        -> (enabled: Bool, panes: Set<String>, viewPanes: Set<String>, guests: [GuestLink], base: String)? {
        guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        let enabled = obj["enabled"] as? Bool ?? false
        let panes = Set(obj["panes"] as? [String] ?? [])
        let viewPanes = Set(obj["view_panes"] as? [String] ?? [])
        let base = obj["base"] as? String ?? ""
        let guests: [GuestLink] = (obj["guests"] as? [[String: Any]] ?? []).map {
            GuestLink(id: $0["id"] as? String ?? "",
                      label: $0["label"] as? String ?? "",
                      enrolledAt: $0["enrolled_at"] as? Int ?? 0)
        }
        return (enabled, panes, viewPanes, guests, base)
    }

    // MARK: mutations (shell out to the CLI, then reload detail)

    func setEnabled(_ on: Bool) { run(["share", on ? "on" : "off"]) }

    func setPane(_ pane: String, allowed: Bool) {
        guard !pane.isEmpty else { return }
        run(["share", allowed ? "add" : "remove", pane])
    }

    /// Toggle a pane on the VIEW allowlist. `gtmux share view remove` also drops the
    /// pane from the input allowlist (input ⊆ view), so a guest can never type into a
    /// pane it can't see.
    func setView(_ pane: String, visible: Bool) {
        guard !pane.isEmpty else { return }
        run(["share", "view", visible ? "add" : "remove", pane])
    }

    func revoke(_ id: String) { run(["share", "revoke", id]) }

    /// Mint a new guest link, surface + copy its URL. completion gets the URL on
    /// success, else nil (with lastError set).
    func newLink(label: String, completion: ((String?) -> Void)? = nil) {
        guard !busy else { return }
        busy = true
        lastError = nil
        var args = ["share", "new", "--json"]
        let trimmed = label.trimmingCharacters(in: .whitespacesAndNewlines)
        if !trimmed.isEmpty { args += ["--label", trimmed] }
        DispatchQueue.global().async {
            let data = GtmuxCLI.capture(args)
            let url = (data.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] })?["url"] as? String
            DispatchQueue.main.async {
                self.busy = false
                if let url = url, !url.isEmpty {
                    self.lastMintedLink = url
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(url, forType: .string)
                } else {
                    self.lastError = "Couldn't create a share link. / 无法创建分享链接。"
                }
                self.loadDetail()
                completion?(url)
            }
        }
    }

    /// Run a state-changing `gtmux share …`, surface any stderr, then reload detail
    /// so the UI settles to ground truth (never optimistic — the serve owns policy).
    private func run(_ args: [String]) {
        guard !busy else { return }
        busy = true
        lastError = nil
        DispatchQueue.global().async {
            let res = GtmuxCLI.captureResult(args)
            DispatchQueue.main.async {
                self.busy = false
                if res.status != 0 && !res.stderr.isEmpty { self.lastError = res.stderr }
                self.loadDetail()
            }
        }
    }
}
