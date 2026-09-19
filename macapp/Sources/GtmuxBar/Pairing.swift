import AppKit
import CoreImage
import CoreImage.CIFilterBuiltins
import SwiftUI

// "Allow phone access" — produce the pairing QR the gtmux mobile app scans
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

    /// current returns the pairing info, or nil when there's no serve token yet
    /// (i.e. remote access was never set up — the caller shows guidance instead).
    static func current() -> PairingInfo? {
        guard let token = readTrimmed(Paths.config("serve-token")), !token.isEmpty else {
            return nil
        }
        let name = Host.current().localizedName ?? "Mac"
        // Prefer the recorded tunnel URL — written by `gtmux tunnel` (foreground)
        // and the always-on service. serve binds loopback under a tunnel, so a LAN
        // IP wouldn't actually be reachable; the tunnel URL is what works.
        if let turl = Paths.tunnelURL() {
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
        // .sortedKeys → a DETERMINISTIC key order, so payload(info, code) returns the
        // byte-identical string on every call. Without it the dictionary's per-call
        // iteration order could vary, changing the QR content between renders.
        guard let data = try? JSONSerialization.data(withJSONObject: dict, options: .sortedKeys),
              let s = String(data: data, encoding: .utf8) else { return "" }
        return s
    }

    /// EnrollCode is a minted pairing code with what decides whether it can still be
    /// redeemed: when it runs out, and which serve boot it lives in. A code is valid
    /// until it expires, it is redeemed once, or the serve restarts and drops it from
    /// memory, whichever comes first.
    struct EnrollCode: Equatable {
        let code: String
        let mintedAt: Date
        let expiresAt: Date
        let boot: String? // nil from a serve too old to report one
    }

    /// How long before expiry a displayed code is replaced. The old code keeps working
    /// until its own expiry, so a phone that captured the previous QR a moment before
    /// the swap still pairs.
    static let renewLead: TimeInterval = 60

    /// How often a pairing window re-checks that its address answers.
    static let reachEvery: TimeInterval = 5
    /// Once reachable, it checks one tick in this many (every 30s), so a tunnel that
    /// drops while the window is open is still noticed.
    static let reachSettledEvery = 6

    /// shouldReprobe says whether this tick re-checks the address. The window used to
    /// check once, when it opened: opened in the seconds after an update restarted the
    /// tunnel, it said "Can't reach it yet" and kept saying it long after the tunnel was
    /// back. "Yet" promised a second look that never came. Pure, for the tests.
    static func shouldReprobe(reachable: Bool?, tick: Int) -> Bool {
        reachable == true ? tick % reachSettledEvery == 0 : true
    }

    /// mintEnrollCode asks the local radar (loopback :8765, the default serve port)
    /// for a short-lived single-use pairing code. completion(nil) when it can't —
    /// callers then fall back to the legacy token QR.
    static func mintEnrollCode(token: String, completion: @escaping (EnrollCode?) -> Void) {
        guard let u = URL(string: "http://127.0.0.1:8765/api/enroll/mint") else {
            completion(nil)
            return
        }
        var req = URLRequest(url: u)
        req.httpMethod = "POST"
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        req.timeoutInterval = 4
        URLSession.shared.dataTask(with: req) { data, resp, _ in
            guard (resp as? HTTPURLResponse)?.statusCode == 200, let data = data else {
                completion(nil)
                return
            }
            completion(parseMint(data, now: Date()))
        }.resume()
    }

    /// parseMint decodes POST /api/enroll/mint. A serve that predates expiresInSec is
    /// taken at the 5 minutes every serve has used. Pure, for the tests.
    static func parseMint(_ data: Data, now: Date) -> EnrollCode? {
        guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let code = obj["enrollCode"] as? String, !code.isEmpty else { return nil }
        let ttl = (obj["expiresInSec"] as? Int).map(TimeInterval.init) ?? 300
        return EnrollCode(code: code, mintedAt: now, expiresAt: now.addingTimeInterval(ttl),
                          boot: obj["boot"] as? String)
    }

    /// parseBoot reads the boot id off GET /api/health; nil when the serve reports none.
    static func parseBoot(_ data: Data) -> String? {
        guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let b = obj["boot"] as? String, !b.isEmpty else { return nil }
        return b
    }

    /// codeNeedsRenewal decides whether the code on a pairing QR can no longer be
    /// trusted to redeem, so the window should mint another before someone scans it.
    ///
    /// The window used to mint once when it opened and show that code for as long as it
    /// stayed open. Five minutes later, or after anything restarted the serve (an
    /// update does), or after one device had already used it, the QR still looked fine
    /// and every scan failed. Pure, for the tests.
    ///
    /// - boot: what /api/health reports now; nil when the serve reports none, which
    ///   leaves only the clock to go on.
    /// - newestEnrolledAt: the latest `enrolledAt` on the device roster. A device that
    ///   enrolled after this code was minted most likely used it, and a code works once.
    static func codeNeedsRenewal(_ c: EnrollCode?, boot: String?, newestEnrolledAt: Int?,
                                 now: Date) -> Bool {
        guard let c = c else { return true }
        if let b = boot, let cb = c.boot, b != cb { return true }
        if let e = newestEnrolledAt, e >= Int(c.mintedAt.timeIntervalSince1970) { return true }
        return now >= c.expiresAt.addingTimeInterval(-renewLead)
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
    // What the tunnel reports about itself (status/tunnel.json), read with each probe:
    // the only evidence for why an unreachable address is unreachable.
    @State private var tunnelStatus: TunnelStatus?
    @State private var probing = false // a reachability probe is in flight
    @State private var reachTicks = 0
    // Held in @State so it is created once per window: a publisher built in `body` would
    // be replaced, and its countdown restarted, on every re-render.
    @State private var reachTimer = Timer.publish(every: Pairing.reachEvery, on: .main, in: .common).autoconnect()
    // The code itself lives in PairStore, which keeps it redeemable while this window
    // is open; holdingCode balances this window's start with exactly one stop.
    @ObservedObject private var pairStore = PairStore.shared
    @State private var holdingCode = false
    @State private var showPaywall = false
    @State private var wantSelfHosted = false // which backend the Anywhere toggle uses
    @State private var showDirectCode = false // presents the shared DirectCodeSheet
    @State private var backendRevert = 0 // bumped to snap the backend picker back (see backendChooser)

    var body: some View {
        VStack(spacing: 13) {
            modeChooser
            // Both tunnels are gtmux-provided, so Direct is ALWAYS selectable in
            // Anywhere mode — not gated on a personal self-hosted config (the CLI
            // has a baked-in Direct server). The chooser drives `--backend self`.
            if remote.mode == .anywhere { backendChooser }
            if !ent.isPro { proHint }
            if let err = remote.lastError { errorLine(err) }

            if remote.busy {
                switchingLine
            } else if let p = info, pairStore.pairCode != nil || pairStore.pairFailed,
                      let qr = Pairing.qrImage(Pairing.payload(p, enrollCode: pairStore.pairCode)) {
                Image(nsImage: qr)
                    .interpolation(.none).resizable()
                    .frame(width: 220, height: 220)
                    .background(Color.white).cornerRadius(10)
                wrap(l10n.tr("Scan in the gtmux mobile app → Pair → Scan",
                             "在 gtmux 手机 App 里：配对 → 扫一扫"), size: 12, color: .secondary)
                Text(p.url)
                    .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                    .textSelection(.enabled).lineLimit(1).truncationMode(.middle)
                reachLine
                wrap(p.anywhere ? anywhereBackendNote : l10n.tr("Same Wi-Fi only.", "仅同一 Wi-Fi 可达。"),
                     size: 11, color: .tertiary)
                // The code renews itself (PairStore); this stays as the manual way to
                // get a fresh code and re-check reachability without reopening.
                refreshButton
            } else if remote.mode == .off {
                Image(systemName: "qrcode").font(.system(size: 44)).foregroundStyle(.tertiary)
                    .frame(height: 130)
                wrap(l10n.tr("Pick how your phone reaches this Mac: Wi-Fi (same network) or Anywhere.",
                             "选择手机如何连到这台 Mac：局域网（同一网络）或任意网络。"),
                     size: 12, color: .secondary)
            } else {
                ProgressView().controlSize(.large).frame(width: 220, height: 220)
                wrap(l10n.tr("Preparing a one-time pairing code…", "正在准备一次性配对码…"),
                     size: 12, color: .secondary)
            }
        }
        .padding(22)
        .frame(width: 340)
        .onAppear {
            remote.refresh()
            if !holdingCode { holdingCode = true; pairStore.startPairCode() }
            reload(renewCode: false) // startPairCode already minted
        }
        .onDisappear {
            if holdingCode { pairStore.stopPairCode(); holdingCode = false }
        }
        .onReceive(reachTimer) { _ in recheckReach() }
        .onChange(of: remote.mode) { _, _ in reload() }
        // Switching the tunnel BACKEND (self↔hosted) keeps mode == .anywhere but
        // changes the URL — reload so the QR/URL/reachability follow the new backend.
        .onChange(of: remote.backend) { _, _ in reload() }
        .sheet(isPresented: $showPaywall) {
            PaywallView(l10n: l10n,
                        onUnlock: { ent.unlockFree(); showPaywall = false; confirmAnywhere() },
                        onClose: { showPaywall = false })
        }
        .sheet(isPresented: $showDirectCode) {
            DirectCodeSheet(l10n: l10n, remote: remote, isPresented: $showDirectCode)
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
                .font(.system(size: 12)).foregroundStyle(.orange)
            Text(text)
                .font(.system(size: 10)).foregroundStyle(.secondary)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.horizontal, 8).padding(.vertical, 6)
        .background(Color.orange.opacity(0.12)).cornerRadius(6)
        .frame(width: 290)
    }

    // A manual "mint a fresh code" control — the pairing code times out, so let the
    // user regenerate it (and re-probe reachability) without reopening the window.
    private var refreshButton: some View {
        Button(action: { reload() }) {
            HStack(spacing: 4) {
                Image(systemName: "arrow.clockwise").font(.system(size: 12, weight: .semibold))
                Text(l10n.tr("Refresh code", "刷新配对码")).font(.system(size: 11, weight: .medium))
            }
            .foregroundStyle(Color.accentColor)
        }
        .buttonStyle(.plain)
        .help(l10n.tr("Get a new pairing code now and check the address again. The code also renews on its own while this window is open.",
                      "立刻换一个新的配对码，并重新检查地址能不能连上。窗口开着时配对码也会自己续期。"))
    }

    private var proHint: some View {
        HStack(spacing: 4) {
            Image(systemName: "lock.fill").font(.system(size: 12))
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

    // Which tunnel is providing "anywhere" reachability — self-hosted (your own
    // VPS+domain) or the hosted Cloudflare tunnel.
    private var anywhereBackendNote: String {
        switch remote.backend {
        case .selfHosted:
            return l10n.tr("Reachable from anywhere · direct tunnel.", "任意网络可达 · 直连隧道。")
        case .cloudflare:
            return l10n.tr("Reachable from anywhere · standard tunnel.", "任意网络可达 · 标准隧道。")
        case .none:
            return l10n.tr("Reachable from anywhere (a tunnel is up).", "任意网络可达（隧道在运行）。")
        }
    }

    @ViewBuilder private var reachLine: some View {
        switch ReachVerdict.of(probeOK: reachable, status: tunnelStatus) {
        case .reachable:
            label("checkmark.circle.fill", .green, l10n.tr("Reachable now", "现在可达"))
        case .tunnelDown(let err):
            // The tunnel reports itself down, checked end to end: no device connects, not
            // even on cellular. Orange, with the error it recorded.
            label("exclamationmark.triangle.fill", .orange,
                  l10n.tr("The tunnel is down, so no device can connect, not even on cellular.",
                          "隧道断了，任何设备都连不上，蜂窝也不行。")
                  + (err.isEmpty ? "" : " (" + err + ")"))
        case .tunnelUpMacCannotSee:
            // The tunnel is up but this Mac cannot see its own address (a network that
            // maps the name to a private IP): a phone elsewhere connects. Inform, calmly.
            label("wifi.exclamationmark", .blue,
                  l10n.tr("This Mac can't reach its own address, but the tunnel is up: a phone on cellular connects",
                          "这台 Mac 连不上自己的地址，但隧道是通的，手机用蜂窝可以连上"))
        case .cannotReachYet:
            label("exclamationmark.triangle.fill", .orange,
                  l10n.tr("Can't reach it yet", "暂时连不上"))
        case .checking:
            label("clock", .secondary, l10n.tr("Checking…", "检查中…"))
        }
    }

    private func label(_ symbol: String, _ color: Color, _ text: String) -> some View {
        HStack(alignment: .top, spacing: 5) {
            Image(systemName: symbol).font(.system(size: 12)).foregroundStyle(color)
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

    private func reload(renewCode: Bool = true) {
        let i = Pairing.current()
        info = i
        reachable = nil
        tunnelStatus = nil
        if let i = i {
            probe(i.url)
            if renewCode { pairStore.renewPairCode() }
        }
    }

    // confirmAnywhere — confirm the standing exposure, then enable the always-on
    // tunnel (Pro). Reached only when Pro is unlocked (else the paywall shows).
    private func confirmAnywhere() {
        let a = NSAlert()
        a.messageText = l10n.tr("Turn on Anywhere access?", "开启任意网络访问？")
        a.informativeText = l10n.tr(
            "Your Mac becomes reachable from anywhere at a fixed address until you switch it off. Only a device holding your token gets in, but the address is exposed the whole time.",
            "开启后，你的 Mac 会在一个固定地址上从任何网络可达，直到你关闭。只有持你 token 的设备进得来，但这个地址会一直敞着。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            // Prefer the self-hosted backend when the user has configured one (that's
            // why they set it up); they can switch with the backend chooser.
            wantSelfHosted = remote.selfTunnelConfigured
            remote.enableAnywhere(selfHosted: wantSelfHosted)
        }
    }

    // backendChooser — pick which gtmux tunnel carries "Anywhere": Standard (the
    // zero-config Cloudflare tunnel) or Direct (a chisel tunnel to a gtmux-run VPS,
    // baked into the CLI; a user's own selftunnel.conf overrides it). Always offered
    // in Anywhere mode. Switching re-runs the install (backends are mutually
    // exclusive, so the other is retired).
    @ViewBuilder private var backendChooser: some View {
        Picker("", selection: Binding(
            get: { remote.backend == .selfHosted },
            set: { self_ in
                // Direct is gtmux's paid tunnel: if it isn't unlocked on this Mac yet,
                // ask for an access code instead of switching (redeeming writes the
                // config the CLI needs). Standard, and an already-unlocked Direct, switch
                // straight through.
                if self_ && !remote.selfTunnelConfigured {
                    showDirectCode = true
                    // Snap the segmented control back to Standard: nothing switched, so
                    // the control must not REST on Direct. The control renders its tap
                    // optimistically and nothing else re-publishes here, so without this
                    // bump a canceled unlock leaves "Direct" selected while Standard is
                    // what's actually running — exactly the confusion this fixes.
                    backendRevert += 1
                    return
                }
                wantSelfHosted = self_
                remote.enableAnywhere(selfHosted: self_)
            })) {
            Text(l10n.tr("Standard", "标准")).tag(false)
            Text(l10n.tr("Direct", "直连")).tag(true)
        }
        .id(backendRevert)
        .labelsHidden()
        .pickerStyle(.segmented)
        .frame(width: 290)
        .disabled(remote.busy)
        .help(l10n.tr("Two gtmux tunnels: Standard works on most networks; Direct (an access code unlocks it) also gets through restrictive networks that block the standard one.",
                      "两条 gtmux 隧道：标准隧道在大多数网络可用；直连隧道（凭访问码解锁）在屏蔽标准隧道的受限网络下也能穿透。"))
    }

    // The "Unlock Direct" access-code sheet is now the shared DirectCodeSheet (also used
    // by Preferences), presented from the .sheet(isPresented: $showDirectCode) above.

    /// recheckReach is the timer's tick. It keeps the last answer on screen while it
    /// asks again, so the line never blinks back to "Checking…".
    private func recheckReach() {
        guard let url = info?.url, !probing, !remote.busy else { return }
        reachTicks += 1
        if Pairing.shouldReprobe(reachable: reachable, tick: reachTicks) { probe(url) }
    }

    private func probe(_ url: String) {
        guard let u = URL(string: url + "/api/health") else { reachable = false; return }
        var req = URLRequest(url: u)
        req.timeoutInterval = 6
        probing = true
        URLSession.shared.dataTask(with: req) { _, resp, _ in
            let ok = (resp as? HTTPURLResponse)?.statusCode == 200
            let st = ok ? nil : TunnelStatus.read()
            DispatchQueue.main.async { reachable = ok; tunnelStatus = st; probing = false }
        }.resume()
    }

}
