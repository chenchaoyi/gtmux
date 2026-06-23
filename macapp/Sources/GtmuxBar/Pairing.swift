import AppKit
import CoreImage
import CoreImage.CIFilterBuiltins
import SwiftUI

// "Allow phone access" — produce the pairing QR the gtmux phone app scans
// (schema v1: {v,url,token,name}, matching mobileapp/src/pairing/qr.ts). The URL
// is the always-on tunnel address when it's set up (reachable from anywhere),
// else the Mac's LAN IP (same Wi-Fi). Token is the persisted serve token.

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
        let tunnelOn = FileManager.default.fileExists(
            atPath: "\(home)/Library/LaunchAgents/com.gtmux.tunnel.plist")
        if tunnelOn, let turl = readTrimmed("\(home)/.config/gtmux/tunnel-url"), !turl.isEmpty {
            return PairingInfo(url: turl, token: token, name: name, anywhere: true)
        }
        let host = primaryIPv4() ?? "localhost"
        return PairingInfo(url: "http://\(host):8765", token: token, name: name, anywhere: false)
    }

    /// payload is the JSON the QR encodes.
    static func payload(_ p: PairingInfo) -> String {
        let dict: [String: Any] = ["v": 1, "url": p.url, "token": p.token, "name": p.name]
        guard let data = try? JSONSerialization.data(withJSONObject: dict),
              let s = String(data: data, encoding: .utf8) else { return "" }
        return s
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
                contentRect: NSRect(x: 0, y: 0, width: 340, height: 440),
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

/// PairingView — the QR + reachable address, or guidance when nothing's set up.
struct PairingView: View {
    @ObservedObject var l10n: L10n
    @State private var info: PairingInfo?

    var body: some View {
        VStack(spacing: 14) {
            if let p = info, let qr = Pairing.qrImage(Pairing.payload(p)) {
                Image(nsImage: qr)
                    .interpolation(.none)
                    .resizable()
                    .frame(width: 240, height: 240)
                    .background(Color.white)
                    .cornerRadius(10)
                Text(l10n.tr("Scan in the gtmux phone app → Pair → Scan",
                             "在 gtmux 手机 app 里:配对 → 扫一扫"))
                    .font(.system(size: 12)).foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                Text(p.url)
                    .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                    .textSelection(.enabled).lineLimit(1).truncationMode(.middle)
                Text(p.anywhere
                     ? l10n.tr("Reachable from anywhere (remote access is on).",
                               "任意网络可达(远程访问已开启)。")
                     : l10n.tr("Same Wi-Fi only. Turn on Remote access (Preferences) for anywhere.",
                               "仅同一 Wi-Fi。想任意网络用,去偏好设置开启远程访问。"))
                    .font(.system(size: 11)).foregroundStyle(.tertiary)
                    .multilineTextAlignment(.center)
            } else {
                Image(systemName: "qrcode").font(.system(size: 44)).foregroundStyle(.tertiary)
                Text(l10n.tr("Remote access isn't set up yet.", "还没设置远程访问。"))
                    .font(.system(size: 13, weight: .semibold))
                Text(l10n.tr("Turn on Remote access in Preferences (or run `gtmux serve`), then reopen this.",
                             "去偏好设置开启远程访问(或跑 `gtmux serve`),再打开这里。"))
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
            }
        }
        .padding(24)
        .frame(width: 340, alignment: .center)
        .onAppear { info = Pairing.current() }
    }
}
