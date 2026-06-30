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
            // v2 carries the Mac's name so the phone shows the computer name (e.g.
            // "ccy's MacBook Pro") rather than deriving a label from the URL host —
            // which, over an Anywhere tunnel, would be the opaque `gtmux-<id>`. The
            // name is ~20 bytes; negligible for the QR's capacity.
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
        filter.correctionLevel = "H" // level H tolerates the center logo occlusion (A5)
        guard let ci = filter.outputImage else { return nil }
        let scale = size / ci.extent.width
        let scaled = ci.transformed(by: CGAffineTransform(scaleX: scale, y: scale))
        let rep = NSCIImageRep(ciImage: scaled)
        let img = NSImage(size: NSSize(width: size, height: size))
        img.addRepresentation(rep)

        // Brand QR (A5): center the gtmux pane-grid mark on a white rounded
        // quiet-zone so the code still reads (EC level H covers the occlusion).
        img.lockFocus()
        let badge = size * 0.26
        let r = CGRect(x: (size - badge) / 2, y: (size - badge) / 2, width: badge, height: badge)
        NSColor.white.setFill()
        NSBezierPath(roundedRect: r, xRadius: badge * 0.22, yRadius: badge * 0.22).fill()
        drawPaneGrid(in: r.insetBy(dx: badge * 0.20, dy: badge * 0.20))
        img.unlockFocus()
        return img
    }

    /// The gtmux pane-grid mark (2×2, one cyan cell), for the brand QR center.
    private static func drawPaneGrid(in r: CGRect) {
        let gap = r.width * 0.12
        let cell = (r.width - gap) / 2
        let neutral = NSColor.black.withAlphaComponent(0.32)
        func tile(_ c: NSColor, _ x: CGFloat, _ y: CGFloat) {
            c.setFill()
            NSBezierPath(roundedRect: CGRect(x: x, y: y, width: cell, height: cell),
                         xRadius: cell * 0.28, yRadius: cell * 0.28).fill()
        }
        let x0 = r.minX, x1 = r.minX + cell + gap
        let y0 = r.minY, y1 = r.minY + cell + gap
        tile(Theme.Status.workingNS, x0, y1) // top-left cyan (matches GtmuxLogo)
        tile(neutral, x1, y1)
        tile(neutral, x0, y0)
        tile(neutral, x1, y0)
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
    @ObservedObject private var ent = Entitlements.shared
    @State private var info: PairingInfo?
    @State private var reachable: Bool? // nil = checking, true = reachable, false = couldn't verify
    @State private var dnsBlocked = false // reach failed because the host resolves to a private IP (corp-DNS interception)
    @State private var enrollCode: String? // minted short-lived code (v2 QR)
    @State private var codeReady = false // mint attempt finished (success or fallback)
    @State private var showPaywall = false

    var body: some View {
        VStack(spacing: 13) {
            modeChooser
            if !ent.isPro { proHint }
            if let err = remote.lastError { errorLine(err) }

            if remote.busy {
                switchingLine
            } else if let p = info, codeReady,
                      let qr = Pairing.qrImage(Pairing.payload(p, enrollCode: enrollCode)) {
                Image(nsImage: qr)
                    .interpolation(.none).resizable()
                    .frame(width: 220, height: 220)
                    .background(Color.white).cornerRadius(10)
                wrap(l10n.tr("Scan in the gtmux phone app → Pair → Scan",
                             "在 gtmux 手机 app 里：配对 → 扫一扫"), size: 12, color: .secondary)
                Text(p.url)
                    .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                    .textSelection(.enabled).lineLimit(1).truncationMode(.middle)
                reachLine
                wrap(p.anywhere
                        ? l10n.tr("Reachable from anywhere (a tunnel is up).", "任意网络可达（隧道在运行）。")
                        : l10n.tr("Same Wi-Fi only.", "仅同一 Wi-Fi 可达。"),
                     size: 11, color: .tertiary)
            } else if remote.mode == .off {
                Image(systemName: "qrcode").font(.system(size: 44)).foregroundStyle(.tertiary)
                    .frame(height: 130)
                wrap(l10n.tr("Pick how your phone reaches this Mac — Wi-Fi (same network) or Anywhere.",
                             "选择手机如何连到这台 Mac —— 局域网（同一网络）或任意网络。"),
                     size: 12, color: .secondary)
            } else {
                ProgressView().controlSize(.large).frame(width: 220, height: 220)
                wrap(l10n.tr("Preparing a one-time pairing code…", "正在准备一次性配对码…"),
                     size: 12, color: .secondary)
            }
        }
        .padding(22)
        .frame(width: 340)
        .onAppear { remote.refresh(); reload() }
        .onChange(of: remote.mode) { _ in reload() }
        .sheet(isPresented: $showPaywall) {
            PaywallView(l10n: l10n,
                        onUnlock: { ent.unlockFree(); showPaywall = false; confirmAnywhere() },
                        onClose: { showPaywall = false })
        }
    }

    // modeChooser — the merged remote-access control: Off / Wi-Fi (free LAN serve)
    // / Anywhere (the Pro always-on tunnel). Selecting Anywhere without Pro opens
    // the paywall instead of switching.
    @ViewBuilder private var modeChooser: some View {
        Picker("", selection: modeBinding) {
            Text(l10n.tr("Off", "关闭")).tag(RemoteMode.off)
            Text(l10n.tr("Wi-Fi", "局域网")).tag(RemoteMode.lan)
            Text(l10n.tr("Anywhere", "任意网络")).tag(RemoteMode.anywhere)
        }
        .labelsHidden()
        .pickerStyle(.segmented)
        .frame(width: 290)
        .disabled(remote.busy)
    }

    private var modeBinding: Binding<RemoteMode> {
        Binding(
            get: { remote.mode },
            set: { m in
                switch m {
                case .off: remote.turnOff()
                case .lan: remote.enableLan()
                case .anywhere: ent.isPro ? confirmAnywhere() : (showPaywall = true)
                }
            })
    }

    // errorLine — why the last switch didn't take (e.g. Anywhere needs a hosted
    // build). Shown instead of silently reverting the chooser to the old mode.
    private func errorLine(_ text: String) -> some View {
        HStack(alignment: .top, spacing: 5) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.system(size: 10)).foregroundStyle(.orange)
            Text(text)
                .font(.system(size: 10)).foregroundStyle(.secondary)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.horizontal, 8).padding(.vertical, 6)
        .background(Color.orange.opacity(0.12)).cornerRadius(6)
        .frame(width: 290)
    }

    private var proHint: some View {
        HStack(spacing: 4) {
            Image(systemName: "lock.fill").font(.system(size: 9))
            Text(l10n.tr("“Anywhere” is a Pro feature", "“任意网络”为 Pro 功能"))
        }
        .font(.system(size: 10)).foregroundStyle(.tertiary)
    }

    private var switchingLine: some View {
        VStack(spacing: 8) {
            ProgressView().controlSize(.small)
            Text(l10n.tr("Switching remote access…", "正在切换远程访问…"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
        }
        .frame(width: 220, height: 220)
    }

    @ViewBuilder private var reachLine: some View {
        switch reachable {
        case .some(true):
            label("checkmark.circle.fill", .green, l10n.tr("Reachable now", "现在可达"))
        case .some(false) where dnsBlocked:
            // The tunnel host resolves to a private IP → this network is hijacking
            // DNS (common on corporate Wi-Fi). The tunnel itself is fine; a phone on
            // cellular reaches it. Inform calmly (blue), don't alarm (orange).
            label("wifi.exclamationmark", .blue,
                  l10n.tr("This network blocks the address · a phone on cellular connects fine",
                          "本机网络拦截了该地址 · 手机用蜂窝可正常连接"))
        case .some(false):
            label("exclamationmark.triangle.fill", .orange,
                  l10n.tr("Can't reach it yet", "暂时连不上"))
        case .none:
            label("clock", .secondary, l10n.tr("Checking…", "检查中…"))
        }
    }

    private func label(_ symbol: String, _ color: Color, _ text: String) -> some View {
        HStack(alignment: .top, spacing: 5) {
            Image(systemName: symbol).font(.system(size: 10)).foregroundStyle(color)
            Text(text).font(.system(size: 11)).foregroundStyle(.secondary)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: 220)
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
        dnsBlocked = false
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

    // confirmAnywhere — confirm the standing exposure, then enable the always-on
    // tunnel (Pro). Reached only when Pro is unlocked (else the paywall shows).
    private func confirmAnywhere() {
        let a = NSAlert()
        a.messageText = l10n.tr("Turn on Anywhere access?", "开启任意网络访问？")
        a.informativeText = l10n.tr(
            "Your Mac becomes reachable from anywhere at a stable URL (token-gated) until you switch it off. It's a standing exposure.",
            "开启后，你的 Mac 会在一个固定地址上从任何网络可达（有 token 把关），直到你关闭。这是个长期敞口。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enableAnywhere()
        }
    }

    private func probe(_ url: String) {
        guard let u = URL(string: url + "/api/health") else { reachable = false; dnsBlocked = false; return }
        let host = u.host
        var req = URLRequest(url: u)
        req.timeoutInterval = 6
        URLSession.shared.dataTask(with: req) { _, resp, _ in
            let ok = (resp as? HTTPURLResponse)?.statusCode == 200
            // On failure, check whether the host resolves to a private (RFC1918) IP —
            // i.e. this network is intercepting DNS (the tunnel host should map to a
            // public Cloudflare edge). If so we surface the calmer "blocked" message.
            let blocked = !ok && (host.map(PairingView.resolvesToPrivateIP) ?? false)
            DispatchQueue.main.async { reachable = ok; dnsBlocked = blocked }
        }.resume()
    }

    // resolvesToPrivateIP — true if `host` resolves (IPv4) to an RFC1918 / loopback
    // address. Used to tell "corp-DNS hijack" apart from a genuinely down tunnel.
    private static func resolvesToPrivateIP(_ host: String) -> Bool {
        var hints = addrinfo(ai_flags: 0, ai_family: AF_INET, ai_socktype: SOCK_STREAM,
                             ai_protocol: 0, ai_addrlen: 0, ai_canonname: nil, ai_addr: nil, ai_next: nil)
        var res: UnsafeMutablePointer<addrinfo>?
        guard getaddrinfo(host, nil, &hints, &res) == 0 else { return false }
        defer { freeaddrinfo(res) }
        var ptr = res
        while let p = ptr {
            if p.pointee.ai_family == AF_INET, let sa = p.pointee.ai_addr {
                let s = sa.withMemoryRebound(to: sockaddr_in.self, capacity: 1) { $0.pointee }
                let v = UInt32(bigEndian: s.sin_addr.s_addr)
                let a = (v >> 24) & 0xff, b = (v >> 16) & 0xff
                if a == 10 || a == 127 || (a == 172 && (16...31).contains(b)) || (a == 192 && b == 168) {
                    return true
                }
            }
            ptr = p.pointee.ai_next
        }
        return false
    }
}
