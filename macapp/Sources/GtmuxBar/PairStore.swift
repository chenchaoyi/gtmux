import Combine
import Foundation

/// One paired OWNER device (the PAIR track of pair-share-model): the user's own
/// phone / browser / terminal, full control. Guests (share links) never appear
/// here — they live in ShareStore.
struct PairedDevice: Identifiable, Equatable {
    let id: String
    let name: String
    let enrolledAt: Int
    let lastSeen: Int
    /// What the device IS — "iOS 26.6", "Chrome 141 · macOS". The phone reports it per
    /// request; a browser's is sniffed from its User-Agent. Empty for a device that has
    /// not connected since the Mac started recording it.
    let platform: String
    /// Where it last connected from. Shown plainly: this is the user's own Mac, and
    /// "who is connected to it" is only a real answer if an address that should not be
    /// there can be spotted and acted on.
    let lastIP: String

    /// kind guesses a display icon from the device name (best-effort chrome only):
    /// the phone app labels itself with its idiom + OS version; the attach pair flow
    /// names entries after the hostname; browsers enroll via the web page.
    /// kind picks the row's icon from what the device IS, falling back to its NAME.
    ///
    /// The name was the only source before, which meant guessing from a string the device
    /// chose for itself — a phone paired as "ccy" got a laptop icon. The platform tag is
    /// the honest source; the name stays as the fallback for a device that has not
    /// connected since platforms were recorded. Same rule as the phone app's, deliberately.
    var kind: String {
        let p = platform.lowercased()
        if p.contains("ios") || p.contains("iphone") { return "iphone" }
        if p.contains("ipad") { return "ipad" }
        if p.contains("safari") || p.contains("chrome") || p.contains("firefox")
            || p.contains("edge") || p.contains("opera") { return "globe" }
        if !p.isEmpty { return "laptopcomputer" }
        let n = name.lowercased()
        if n.contains("iphone") || n.contains("ipad") || n == "phone" { return "iphone" }
        if n.contains("safari") || n.contains("chrome") || n.contains("browser") { return "globe" }
        return "laptopcomputer"
    }

    /// The row's second line, built the same way on every surface: what it is, where from,
    /// when last seen — the parts that are known, joined by "·".
    func subtitle(now: Int, tr: (String, String) -> String) -> String {
        var parts: [String] = []
        if !platform.isEmpty { parts.append(platform) }
        if !lastIP.isEmpty { parts.append(lastIP) }
        if lastSeen > 0 {
            parts.append(tr("last seen ", "上次连接 ") + relativeTime(lastSeen, now: now) + tr(" ago", "前"))
        } else {
            parts.append(tr("never connected", "从未连接"))
        }
        return parts.joined(separator: " · ")
    }

    /// displayName drops the legacy "gtmux • " prefix the phone app used to register
    /// under. A "gtmux" prefix inside gtmux's OWN roster carried no information — nothing
    /// in this list is not a gtmux device — while pushing the part that identifies the
    /// device out to where it gets truncated. New pairings no longer send it; stripping
    /// it here tidies the entries already on disk without asking anyone to re-pair.
    var displayName: String {
        let cleaned = PairedDevice.stripLegacyPrefix(name)
        return cleaned.isEmpty ? name : cleaned
    }

    /// stripLegacyPrefix removes a leading "gtmux", with or without a bullet separator.
    /// Pure + internal so it can be tested directly.
    static func stripLegacyPrefix(_ raw: String) -> String {
        var s = raw.trimmingCharacters(in: .whitespaces)
        guard s.lowercased().hasPrefix("gtmux") else { return s }
        s = String(s.dropFirst("gtmux".count))
        s = s.trimmingCharacters(in: CharacterSet(charactersIn: " \u{2022}\u{00B7}"))
        return s
    }
}

/// PairStore reflects the owner-device roster (GET /api/devices — it carries NO
/// tokens) and revokes entries. Like Pairing, it authenticates with the serve
/// token from ~/.config/gtmux/serve-token; a missing token / dead serve simply
/// yields an empty list (the section then shows guidance).
final class PairStore: ObservableObject {
    static let shared = PairStore()

    @Published private(set) var devices: [PairedDevice] = []
    @Published var busy = false

    // The pairing QR's ONE-TIME code lives here, not in a view's @State, for two
    // reasons. Preferences re-renders every ~1.5s (the agent poll), and a code held in
    // the view would be re-minted, and the QR would visibly change, on every render.
    // And both pairing surfaces (the "Pair your phone" window and Preferences' "Pair a
    // device" sheet) need the same thing: a code that can still be redeemed.
    //
    // "Stable" used to mean "minted once per presentation". That kept the QR still,
    // and it also kept it dead: a code expires after 5 minutes, works once, and lives
    // only in the serve's memory, so a window left open, a second device, or an update
    // that restarted the serve all left a QR every scan failed on. So a timer now asks
    // the serve every few seconds and replaces the code the moment any of the three
    // has happened (Pairing.codeNeedsRenewal). The QR changes only then.
    @Published var pairInfo: PairingInfo?
    @Published private(set) var pairMinted: Pairing.EnrollCode?
    @Published var pairFailed = false
    var pairCode: String? { pairMinted?.code }
    private var mintingPair = false
    private var pairHolders = 0 // open surfaces showing the code
    private var keepAlive: Timer?
    static let keepAliveEvery: TimeInterval = 5

    /// startPairCode is called by a surface as it starts showing a code. The first
    /// caller mints and starts the keep-alive; later ones share the same code.
    func startPairCode() {
        pairHolders += 1
        if keepAlive == nil {
            let t = Timer(timeInterval: PairStore.keepAliveEvery, repeats: true) { [weak self] _ in
                self?.keepPairCodeLive()
            }
            // .common so it keeps firing while a menu or a slider is being tracked.
            RunLoop.main.add(t, forMode: .common)
            keepAlive = t
        }
        if pairMinted == nil { mintPairCode() }
    }

    /// stopPairCode is the matching call when a surface closes. The last one out
    /// stops the timer and drops the code, so the next open mints a fresh one.
    func stopPairCode() {
        pairHolders = max(0, pairHolders - 1)
        guard pairHolders == 0 else { return }
        keepAlive?.invalidate()
        keepAlive = nil
        pairMinted = nil
        pairInfo = nil
        pairFailed = false
        mintingPair = false
    }

    /// renewPairCode mints a fresh code now: the Refresh button, and a change of
    /// door or tunnel backend. The current code stays on screen until the new one
    /// arrives, so the QR never blinks to a spinner.
    func renewPairCode() { mintPairCode() }

    /// keepPairCodeLive is the timer's tick: ask the serve what changed and mint
    /// again when the code on screen can no longer be trusted to redeem.
    func keepPairCodeLive() {
        if mintingPair { return }
        // Re-read the address too: a backend switch rewrites the tunnel URL, and a
        // QR pointing at the old one fails exactly like an expired code.
        if let p = Pairing.current(), p.url != pairInfo?.url { pairInfo = p }
        guard let tok = token() else { return }
        localServeState(token: tok) { [weak self] state in
            guard let self = self, let state = state else { return } // serve down: next tick
            if Pairing.codeNeedsRenewal(self.pairMinted, boot: state.boot,
                                        newestEnrolledAt: state.newestEnrolledAt, now: Date()) {
                self.mintPairCode()
            }
        }
    }

    private func mintPairCode() {
        if mintingPair { return }
        guard let p = Pairing.current() else {
            pairFailed = pairMinted == nil
            return
        }
        mintingPair = true
        pairInfo = p
        Pairing.mintEnrollCode(token: p.token) { c in
            DispatchQueue.main.async {
                self.mintingPair = false
                if let c = c {
                    self.pairMinted = c
                    self.pairFailed = false
                } else if self.pairMinted == nil {
                    // Keep showing a code that may still work over showing nothing;
                    // the next tick tries again either way.
                    self.pairFailed = true
                }
            }
        }
    }

    /// localServeState reads what decides a code's fate from the local serve: its
    /// boot, and the newest owner enrollment. nil when the serve does not answer,
    /// which is not a reason to mint (that would fail too).
    private func localServeState(token: String,
                                 completion: @escaping ((boot: String?, newestEnrolledAt: Int?)?) -> Void) {
        guard let hu = URL(string: base + "/api/health"),
              let du = URL(string: base + "/api/devices") else { return completion(nil) }
        URLSession.shared.dataTask(with: URLRequest(url: hu, timeoutInterval: 2)) { data, resp, _ in
            guard (resp as? HTTPURLResponse)?.statusCode == 200 else {
                DispatchQueue.main.async { completion(nil) }
                return
            }
            let boot = data.flatMap(Pairing.parseBoot)
            var dreq = URLRequest(url: du, timeoutInterval: 2)
            dreq.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            URLSession.shared.dataTask(with: dreq) { ddata, _, _ in
                let newest = ddata.flatMap(PairStore.parseDevices)?.map(\.enrolledAt).max()
                DispatchQueue.main.async { completion((boot, newest)) }
            }.resume()
        }.resume()
    }

    private var base: String { "http://127.0.0.1:8765" }

    private func token() -> String? {
        let p = NSHomeDirectory() + "/.config/gtmux/serve-token"
        guard let t = try? String(contentsOfFile: p, encoding: .utf8) else { return nil }
        let trimmed = t.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    /// refresh reloads the roster, keeping only OWNER entries (scope != guest).
    func refresh() {
        guard let tok = token(), let url = URL(string: base + "/api/devices") else {
            DispatchQueue.main.async { self.devices = [] }
            return
        }
        var req = URLRequest(url: url, timeoutInterval: 3)
        req.setValue("Bearer \(tok)", forHTTPHeaderField: "Authorization")
        URLSession.shared.dataTask(with: req) { data, _, _ in
            let parsed = data.flatMap { PairStore.parseDevices($0) } ?? []
            DispatchQueue.main.async { self.devices = parsed }
        }.resume()
    }

    /// parseDevices decodes GET /api/devices, dropping guest entries. Pure —
    /// unit-tested against the wire shape.
    static func parseDevices(_ data: Data) -> [PairedDevice]? {
        guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let rows = obj["devices"] as? [[String: Any]] else { return nil }
        return rows.compactMap { r in
            if (r["scope"] as? String) == "guest" { return nil }
            return PairedDevice(id: r["id"] as? String ?? "",
                                name: r["name"] as? String ?? "",
                                enrolledAt: r["enrolledAt"] as? Int ?? 0,
                                lastSeen: r["lastSeen"] as? Int ?? 0,
                                platform: r["platform"] as? String ?? "",
                                lastIP: r["lastIP"] as? String ?? "")
        }
    }

    /// revoke drops one device (effective immediately), then reloads.
    func revoke(_ id: String) {
        guard let tok = token(), let url = URL(string: base + "/api/devices/revoke") else { return }
        busy = true
        var req = URLRequest(url: url, timeoutInterval: 3)
        req.httpMethod = "POST"
        req.setValue("Bearer \(tok)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try? JSONSerialization.data(withJSONObject: ["id": id])
        URLSession.shared.dataTask(with: req) { _, _, _ in
            DispatchQueue.main.async {
                self.busy = false
                self.refresh()
            }
        }.resume()
    }
}
