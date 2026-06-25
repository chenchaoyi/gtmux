import AppKit
import CoreImage
import CoreImage.CIFilterBuiltins
import SwiftUI

// "Allow phone access" — produce the pairing QR the gtmux phone app scans
// (matching mobileapp/src/pairing/qr.ts). Prefer the SECURE v2 shape
// {v,url,enrollCode,name}: a short-lived single-use code minted from the local
// radar, so the QR isn't a lasting credential. Fall back to legacy v1
// {v,url,token,name} when the radar can't mint (not running on :8765 / too old).
// The URL is the always-on tunnel address when set up (reachable from anywhere),
// else the Mac's LAN IP (same Wi-Fi).

struct PairingInfo {
    let url: String
    let token: String
    let name: String
    let anywhere: Bool // true when via the always-on tunnel
}

enum Pairing {
    private static var home: String { NSHomeDirectory() }

    /// current returns the pairing info, or nil when there's no serve token yet
    /// (i.e. remote access was never set up — the caller shows guidance instead).
    static func current() -> PairingInfo? {
        guard let token = readTrimmed("\(home)/.config/gtmux/serve-token"), !token.isEmpty else {
            return nil
        }
        let name = Host.current().localizedName ?? "Mac"
        // Prefer the recorded tunnel URL — written by `gtmux tunnel` (foreground)
        // and the always-on service. serve binds loopback under a tunnel, so a LAN
        // IP wouldn't actually be reachable; the tunnel URL is what works.
        if let turl = readTrimmed("\(home)/.config/gtmux/tunnel-url"), !turl.isEmpty {
            return PairingInfo(url: turl, token: token, name: name, anywhere: true)
        }
        let host = primaryIPv4() ?? "localhost"
        return PairingInfo(url: "http://\(host):8765", token: token, name: name, anywhere: false)
    }

    /// payload is the JSON the QR encodes: secure v2 when an enroll code was
    /// minted, else the legacy v1 token shape so pairing still works.
    static func payload(_ p: PairingInfo, enrollCode: String? = nil) -> String {
        let dict: [String: Any]
        if let code = enrollCode, !code.isEmpty {
            dict = ["v": 2, "url": p.url, "enrollCode": code, "name": p.name]
        } else {
            dict = ["v": 1, "url": p.url, "token": p.token, "name": p.name]
        }
        guard let data = try? JSONSerialization.data(withJSONObject: dict),
              let s = String(data: data, encoding: .utf8) else { return "" }
        return s
    }

    /// mintEnrollCode asks the local radar (loopback :8765, the default serve port)
    /// for a short-lived single-use pairing code. completion(nil) when it can't —
    /// callers then fall back to the legacy token QR.
    static func mintEnrollCode(token: String, completion: @escaping (String?) -> Void) {
        guard let u = URL(string: "http://127.0.0.1:8765/api/enroll/mint") else {
            completion(nil)
            return
        }
        var req = URLRequest(url: u)
        req.httpMethod = "POST"
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        req.timeoutInterval = 4
        URLSession.shared.dataTask(with: req) { data, resp, _ in
            guard (resp as? HTTPURLResponse)?.statusCode == 200, let data = data,
                  let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let code = obj["enrollCode"] as? String, !code.isEmpty else {
                completion(nil)
                return
            }
            completion(code)
        }.resume()
    }

    /// qrImage renders `text` as a crisp QR (nearest-neighbor upscaled).
    static func qrImage(_ text: String, size: CGFloat = 240) -> NSImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(text.utf8)
        filter.correctionLevel = "M"
        guard let ci = filter.outputImage else { return nil }
        let scale = size / ci.extent.width
        let scaled = ci.transformed(by: CGAffineTransform(scaleX: scale, y: scale))
        let rep = NSCIImageRep(ciImage: scaled)
        let img = NSImage(size: rep.size)
        img.addRepresentation(rep)
        return img
    }

    private static func readTrimmed(_ path: String) -> String? {
        guard let s = try? String(contentsOfFile: path, encoding: .utf8) else { return nil }
        return s.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// primaryIPv4 returns the Mac's Wi-Fi/Ethernet IPv4 (en0/en1), or nil.
    private static func primaryIPv4() -> String? {
        var result: String?
        var ifaddr: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&ifaddr) == 0, let first = ifaddr else { return nil }
        defer { freeifaddrs(ifaddr) }
        for ptr in sequence(first: first, next: { $0.pointee.ifa_next }) {
            let flags = Int32(ptr.pointee.ifa_flags)
            guard let sa = ptr.pointee.ifa_addr, sa.pointee.sa_family == UInt8(AF_INET),
                  (flags & IFF_UP) == IFF_UP, (flags & IFF_LOOPBACK) == 0 else { continue }
            let name = String(cString: ptr.pointee.ifa_name)
            guard name == "en0" || name == "en1" else { continue }
            var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
            if getnameinfo(sa, socklen_t(sa.pointee.sa_len), &host, socklen_t(host.count),
                           nil, 0, NI_NUMERICHOST) == 0 {
                result = String(cString: host)
            }
        }
        return result
    }
}

/// PairingController owns the "Pair your phone" window (the QR panel).
final class PairingController {
    static let shared = PairingController()
    private var window: NSWindow?

    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 340, height: 500),
                styleMask: [.titled, .closable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(rootView: PairingView(l10n: l10n))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
        }
        window?.title = l10n.tr("Pair your phone", "配对手机")
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

/// PairingView — the QR + reachable address. When only the LAN is reachable, a
/// one-tap "Turn on remote access" enables the always-on tunnel right here (no
/// terminal needed), and the QR updates to the anywhere-reachable address.
struct PairingView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject private var remote = RemoteAccess.shared
    @State private var info: PairingInfo?
    @State private var reachable: Bool? // nil = checking, true = reachable, false = couldn't verify
    @State private var enrollCode: String? // minted short-lived code (v2 QR)
    @State private var codeReady = false // mint attempt finished (success or fallback)

    var body: some View {
        VStack(spacing: 13) {
            if let p = info {
                if codeReady, let qr = Pairing.qrImage(Pairing.payload(p, enrollCode: enrollCode)) {
                    Image(nsImage: qr)
                        .interpolation(.none).resizable()
                        .frame(width: 230, height: 230)
                        .background(Color.white).cornerRadius(10)
                    wrap(l10n.tr("Scan in the gtmux phone app → Pair → Scan",
                                 "在 gtmux 手机 app 里：配对 → 扫一扫"), size: 12, color: .secondary)
                    Text(p.url)
                        .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                        .textSelection(.enabled).lineLimit(1).truncationMode(.middle)
                    reachLine
                    if p.anywhere {
                        wrap(l10n.tr("Reachable from anywhere (a tunnel is up).",
                                     "任意网络可达（隧道在运行）。"), size: 11, color: .tertiary)
                    } else {
                        wrap(l10n.tr("Same Wi-Fi only.", "仅同一 Wi-Fi 可达。"), size: 11, color: .tertiary)
                        remoteButton
                    }
                } else {
                    ProgressView().controlSize(.large).frame(width: 230, height: 230)
                    wrap(l10n.tr("Preparing a one-time pairing code…", "正在准备一次性配对码…"),
                         size: 12, color: .secondary)
                }
            } else {
                Image(systemName: "qrcode").font(.system(size: 44)).foregroundStyle(.tertiary)
                wrap(l10n.tr("Remote access isn't set up yet.", "还没设置远程访问。"), size: 13, color: .primary)
                remoteButton
            }
        }
        .padding(22)
        .frame(width: 340)
        .onAppear { reload() }
        .onChange(of: remote.isOn) { _ in reload() }
    }

    // remoteButton — enable the always-on tunnel from here, so pairing works from
    // anywhere without running `gtmux tunnel` in a terminal.
    @ViewBuilder private var remoteButton: some View {
        if remote.busy {
            HStack(spacing: 6) {
                ProgressView().controlSize(.small)
                Text(l10n.tr("Setting up remote access…", "正在开启远程访问…"))
                    .font(.system(size: 11)).foregroundStyle(.secondary)
            }
        } else {
            Button(action: enableRemote) {
                Label(l10n.tr("Turn on remote access (anywhere)", "开启远程访问（任意网络）"),
                      systemImage: "globe")
            }
            .controlSize(.regular)
        }
    }

    @ViewBuilder private var reachLine: some View {
        switch reachable {
        case .some(true):
            label("checkmark.circle.fill", .green, l10n.tr("Reachable now", "现在可达"))
        case .some(false):
            label("exclamationmark.triangle.fill", .orange,
                  l10n.tr("Can't reach it yet", "暂时连不上"))
        case .none:
            label("clock", .secondary, l10n.tr("Checking…", "检查中…"))
        }
    }

    private func label(_ symbol: String, _ color: Color, _ text: String) -> some View {
        HStack(spacing: 5) {
            Image(systemName: symbol).font(.system(size: 10)).foregroundStyle(color)
            Text(text).font(.system(size: 11)).foregroundStyle(.secondary)
        }
    }

    // wrap — a centered, wrapping text (fixedSize vertical so long lines never get
    // truncated to "…").
    private func wrap(_ text: String, size: CGFloat, color: HierarchicalShapeStyle) -> some View {
        Text(text)
            .font(.system(size: size, weight: size >= 13 ? .semibold : .regular))
            .foregroundStyle(color)
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
    }

    private func reload() {
        let i = Pairing.current()
        info = i
        reachable = nil
        enrollCode = nil
        codeReady = false
        if let i = i {
            probe(i.url)
            // Mint a short-lived code for the secure v2 QR; on failure codeReady
            // still flips so we render the legacy v1 token QR (enrollCode == nil).
            Pairing.mintEnrollCode(token: i.token) { code in
                DispatchQueue.main.async {
                    enrollCode = code
                    codeReady = true
                }
            }
        }
    }

    private func enableRemote() {
        let a = NSAlert()
        a.messageText = l10n.tr("Turn on remote access?", "开启远程访问？")
        a.informativeText = l10n.tr(
            "Your Mac becomes reachable from anywhere at a stable URL (token-gated) until you turn it off in Preferences. It's a standing exposure.",
            "开启后，你的 Mac 会在一个固定地址上从任何网络可达（有 token 把关），直到你在偏好设置里关闭。这是个长期敞口。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enable()
        }
    }

    private func probe(_ url: String) {
        guard let u = URL(string: url + "/api/health") else { reachable = false; return }
        var req = URLRequest(url: u)
        req.timeoutInterval = 6
        URLSession.shared.dataTask(with: req) { _, resp, _ in
            let ok = (resp as? HTTPURLResponse)?.statusCode == 200
            DispatchQueue.main.async { reachable = ok }
        }.resume()
    }
}
