import AppKit
import Combine
import SwiftUI

/// The same measured reachability and tunnel evidence in both pairing entries.
struct PairingConnectionView: View {
    @ObservedObject var l10n: L10n
    let info: PairingInfo
    var movedAt: Date? = nil
    var recoveredNewAddress: String? = nil
    @ObservedObject private var serverStore = DirectServerStore.shared
    @ObservedObject private var remote = RemoteAccess.shared
    @ObservedObject private var pairStore = PairStore.shared
    @State private var reachable: Bool?
    @State private var tunnelStatus: TunnelStatus?
    @State private var probing = false
    @State private var probeGeneration = 0
    @State private var reachTicks = 0
    @State private var request: URLSessionDataTask?
    @State private var reachTimer = Timer.publish(every: Pairing.reachEvery, on: .main, in: .common).autoconnect()

    private var justMoved: Bool {
        guard let movedAt else { return false }
        return Date().timeIntervalSince(movedAt) < 45 && ReachVerdict.of(probeOK: reachable, status: tunnelStatus).isNotReachable
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            reachLine
            if let note = reachSub(anywhere: info.anywhere) {
                Text(note).font(.system(size: 11)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .onAppear { probe(info.url) }
        .onChange(of: info.url) { _, url in reachable = nil; tunnelStatus = nil; probe(url) }
        .onChange(of: pairStore.pairCode) { _, _ in if !probing { probe(info.url) } }
        .onReceive(reachTimer) { _ in recheckReach() }
        .onDisappear { request?.cancel(); probeGeneration += 1 }
    }

    // The second line under the state: what this address means, or what to do about it.
    private func reachSub(anywhere: Bool) -> String? {
        if let recoveredNewAddress, recoveredNewAddress == info.url {
            return l10n.tr("Your address changed. Scan this new code on your phone.",
                           "连接地址已更新，请用手机扫描这个新二维码。")
        }
        if justMoved {
            return l10n.tr("A phone that has connected before follows on its own, with nothing to scan.",
                           "已经连过的手机会自己跟过来，不用重新扫码。")
        }
        switch ReachVerdict.of(probeOK: reachable, status: tunnelStatus) {
        case .reachable:
            return anywhere
                ? l10n.tr("Reachable from any network.", "任意网络都能连。")
                : l10n.tr("Only on this Wi-Fi.", "只在这个 Wi-Fi 下可达。")
        case .tunnelDown:
            return remote.backend == .cloudflare
                ? l10n.tr("Use Restore connection above. If it still fails, check your network or proxy.",
                          "点击上方“恢复连接”；仍失败时，检查网络或代理线路。")
                : l10n.tr("Pick another server above to recover; phones that have connected follow.",
                          "点上面换一台服务器就能恢复；已经连过的手机会自己跟过来。")
        default:
            return nil
        }
    }

    @ViewBuilder private var reachLine: some View {
        if justMoved {
            // Moving takes a few seconds, and during them the address is genuinely not
            // answering. Saying "can't reach it yet" there sends the reader looking for a
            // fault that is not happening.
            let place = serverStore.currentName(l10n)
            label("arrow.triangle.2.circlepath", .cyan,
                  place.isEmpty
                    ? l10n.tr("Just moved, reconnecting", "刚换了服务器，正在重新连接")
                    : l10n.tr("Just moved to \(place), reconnecting", "刚换到\(place)，正在重新连接"))
        } else {
            verdictLine
        }
    }

    @ViewBuilder private var verdictLine: some View {
        switch ReachVerdict.of(probeOK: reachable, status: tunnelStatus) {
        case .reachable:
            label("checkmark.circle.fill", .green, l10n.tr("Reachable now", "现在可达"))
        case .tunnelDown(let err):
            label("exclamationmark.triangle.fill", .orange,
                  l10n.tr("The tunnel connection could not be verified.", "暂时无法确认隧道连接。")
                  + (err.isEmpty ? "" : " (" + err + ")"))
        case .tunnelConnectedAddressUnverified:
            label("wifi.exclamationmark", .orange,
                  l10n.tr("The tunnel reports a connection, but this address could not be verified. Check your network or proxy.",
                          "隧道报告已连接，但此地址尚未验证可达。请检查网络或代理线路。"))
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
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// recheckReach is the timer's tick. It keeps the last answer on screen while it
    /// asks again, so the line never blinks back to "Checking…".
    private func recheckReach() {
        guard !probing, !remote.busy else { return }
        let url = info.url
        reachTicks += 1
        if Pairing.shouldReprobe(reachable: reachable, tick: reachTicks) { probe(url) }
    }

    private func probe(_ url: String) {
        guard let u = URL(string: url + "/api/health") else { reachable = false; return }
        var req = URLRequest(url: u)
        req.timeoutInterval = 6
        probeGeneration += 1
        let generation = probeGeneration
        request?.cancel()
        probing = true
        request = URLSession.shared.dataTask(with: req) { data, resp, err in
            let code = (resp as? HTTPURLResponse)?.statusCode
            let ok = Pairing.healthOK(data, httpStatus: code)
            let st = ok ? nil : TunnelStatus.read()
            Pairing.logReach(ReachVerdict.of(probeOK: ok, status: st), httpStatus: code, error: err)
            DispatchQueue.main.async {
                guard probeGeneration == generation, info.url == url else { return }
                reachable = ok; tunnelStatus = st; probing = false
            }
        }
        request?.resume()
    }

}
