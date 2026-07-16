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
    /// the phone app labels itself with the device model; the attach pair flow
    /// names entries after the hostname; browsers enroll via the web page.
    var kind: String {
        let n = name.lowercased()
        if n.contains("iphone") || n.contains("ipad") || n == "phone" { return "iphone" }
        if n.contains("safari") || n.contains("chrome") || n.contains("browser") { return "globe" }
        return "laptopcomputer"
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
