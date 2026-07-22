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

    /// kind guesses a display icon from the device name (best-effort chrome only):
    /// the phone app labels itself with its idiom + OS version; the attach pair flow
    /// names entries after the hostname; browsers enroll via the web page.
    var kind: String {
        let n = name.lowercased()
        if n.contains("iphone") || n.contains("ipad") || n == "phone" { return "iphone" }
        if n.contains("safari") || n.contains("chrome") || n.contains("browser") { return "globe" }
        return "laptopcomputer"
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

    // The pairing sheet's ONE-TIME code lives here (not in the sheet's @State) so it
    // survives the Preferences view's frequent re-renders (the agent poll re-evaluates
    // the body every ~1.5s). Kept in a singleton + minted idempotently → the QR/code
    // stay STABLE while the sheet is open, instead of re-minting (and visibly changing)
    // on every re-render.
    @Published var pairInfo: PairingInfo?
    @Published var pairCode: String?
    @Published var pairFailed = false
    private var mintingPair = false

    /// Mint the one-time pair code ONCE per sheet presentation. Idempotent: a repeat
    /// call (e.g. a re-rendered sheet's onAppear firing again) is a no-op while a code
    /// is already held or a mint is in flight.
    func mintPairCodeIfNeeded() {
        if pairCode != nil || mintingPair { return }
        guard let p = Pairing.current() else {
            pairFailed = true
            return
        }
        mintingPair = true
        pairInfo = p
        pairFailed = false
        Pairing.mintEnrollCode(token: p.token) { c in
            DispatchQueue.main.async {
                self.mintingPair = false
                if let c = c, !c.isEmpty { self.pairCode = c } else { self.pairFailed = true }
            }
        }
    }

    /// Clear the held code when the sheet closes, so reopening mints a fresh one (the
    /// prior code is single-use / may be spent).
    func clearPairCode() {
        pairCode = nil
        pairInfo = nil
        pairFailed = false
        mintingPair = false
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
                                lastSeen: r["lastSeen"] as? Int ?? 0)
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
