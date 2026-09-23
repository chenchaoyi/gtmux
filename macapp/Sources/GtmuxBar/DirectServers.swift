import SwiftUI

// Choosing a Direct server (openspec/changes/direct-server-choice).
//
// Direct was one server, so a user in another part of the world paid for the distance and
// an outage had nowhere to go. The servers are configuration the provisioner serves, and
// this screen renders whatever it returns: a server the operator adds today shows up here
// without a new version of this app.
//
// This surface stays a CONSUMER of the CLI, as every other part of it is: it reads
// `gtmux tunnel --servers --json` and calls `gtmux tunnel --server <id>`.

struct DirectServer: Identifiable, Decodable, Equatable {
    let id: String
    let url: String
    let region: String?
    let name: String
    let current: Bool
    let accepting: Bool
    let answering: Bool
    let rttMS: Int?

    enum CodingKeys: String, CodingKey {
        case id, url, region, name, current, accepting, answering
        case rttMS = "rtt_ms"
    }

    /// What the reader is told about the round trip. A server that did not answer says so:
    /// it is why their phone cannot reach this Mac, and the reason to move.
    func roundTrip(_ l10n: L10n) -> String {
        guard answering, let ms = rttMS else { return l10n.tr("no answer", "没有回应") }
        return "\(ms) ms"
    }
}

/// ServerStore loads the list and performs a move, both through the CLI.
final class DirectServerStore: ObservableObject {
    @Published var servers: [DirectServer] = []
    @Published var loading = false
    /// When the round trips were taken. A number with no time on it is not a measurement,
    /// and before this the figures were frozen at whenever the window opened.
    @Published var measuredAt: Date?
    @Published var moving: String? // the id being moved to
    @Published var lastError: String?

    func load() {
        guard !loading else { return }
        loading = true
        lastError = nil
        DispatchQueue.global().async {
            let out = GtmuxCLI.capture(["tunnel", "--servers", "--json"])
            let parsed = out.flatMap(Self.decode)
            DispatchQueue.main.async {
                self.loading = false
                if let list = parsed {
                    self.servers = list
                    self.measuredAt = Date()
                } else {
                    // The CLI says why on stderr; a second run for it is not worth a Mac's
                    // time, so this surface only says the list is not available.
                    self.lastError = self.l10nFallback
                }
            }
        }
    }

    func move(to id: String, done: @escaping (Bool) -> Void) {
        guard moving == nil else { return }
        moving = id
        lastError = nil
        DispatchQueue.global().async {
            let res = GtmuxCLI.captureResult(["tunnel", "--server", id])
            DispatchQueue.main.async {
                self.moving = nil
                if res.status != 0 {
                    self.lastError = res.stderr
                    done(false)
                    return
                }
                self.load()
                done(true)
            }
        }
    }

    /// The message shown when the list could not be read at all. Set by the view, which is
    /// where the reader's language lives.
    var l10nFallback = "Could not read the Direct servers."

    /// The name of the server in use, as the reader reads it ("" when unknown). The
    /// pairing window says where it just moved TO, and an id would not be that place.
    func currentName(_ l10n: L10n) -> String {
        servers.first(where: { $0.current })?.name ?? ""
    }

    static func decode(_ data: Data) -> [DirectServer]? {
        struct Reply: Decodable { let servers: [DirectServer] }
        guard let reply = try? JSONDecoder().decode(Reply.self, from: data) else { return nil }
        return reply.servers
    }
}

/// The list itself: region, round trip, and which one is in use. No address is printed
/// here — an address belongs to one server, so it belongs on that server's own row when it
/// is opened, not in a list of places.
struct DirectServerList: View {
    @ObservedObject var store: DirectServerStore
    let l10n: L10n
    /// Asked before a move, because a move has consequences for already-paired devices.
    let confirm: (DirectServer) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(store.servers.enumerated()), id: \.element.id) { i, s in
                if i > 0 { Divider().padding(.leading, 8) }
                row(s)
            }
            if let e = store.lastError {
                Text(e).font(.system(size: 10)).foregroundStyle(.secondary).lineLimit(3)
                    .padding(.horizontal, 8).padding(.top, 6)
            } else if store.servers.isEmpty {
                Text(store.loading
                        ? l10n.tr("Measuring…", "正在测…")
                        : l10n.tr("No Direct server is configured.", "没有配置任何 Direct 服务器。"))
                    .font(.system(size: 10)).foregroundStyle(.secondary)
                    .padding(.horizontal, 8).padding(.vertical, 6)
            }
            if !store.servers.isEmpty || store.loading {
                Divider().padding(.leading, 8)
                measuredLine.padding(.horizontal, 8).padding(.vertical, 6)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder private func row(_ s: DirectServer) -> some View {
        Button {
            guard pickableRoute(s), store.moving == nil else { return }
            confirm(s)
        } label: {
            HStack(spacing: 8) {
                // Colour says one thing only: whether the server answered.
                Circle()
                    .fill(s.answering ? Theme.Status.idle : Theme.Status.waiting)
                    .frame(width: 6, height: 6)
                // The name is the place; an id like cn-shanghai is not one.
                Text(s.name)
                    .font(.system(size: 12, weight: s.current ? .semibold : .regular))
                    .foregroundStyle(.primary)
                Spacer(minLength: 8)
                Text(s.roundTrip(l10n))
                    .font(.system(size: 11).monospacedDigit())
                    .foregroundStyle(.secondary)
                if s.current {
                    // A check, not a dimming. The row you are on is the one that must read
                    // loudest; `.disabled` faded it instead, which said "broken".
                    Image(systemName: "checkmark")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundStyle(Color.accentColor)
                } else if store.moving == s.id {
                    ProgressView().controlSize(.small)
                } else if !s.accepting {
                    Text(l10n.tr("closed", "不接新设备")).font(.system(size: 10)).foregroundStyle(.tertiary)
                }
            }
            .padding(.vertical, 7).padding(.horizontal, 8)
            .frame(maxWidth: .infinity)
            .background(s.current ? Color.accentColor.opacity(0.10) : Color.clear)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        // Not `.disabled`: that greys the row out. The row in use simply does nothing when
        // clicked, and a move in flight takes the clicks away without fading anything.
        .allowsHitTesting(pickableRoute(s) && store.moving == nil)
    }

    /// The line under the rows: when these figures were taken, and a way to take them
    /// again. They are measurements, and a measurement with no time on it says nothing.
    @ViewBuilder var measuredLine: some View {
        HStack(spacing: 8) {
            Text(measuredText)
                .font(.system(size: 10)).foregroundStyle(.tertiary)
            Spacer(minLength: 0)
            Button(l10n.tr("Measure again", "重新测")) { store.load() }
                .buttonStyle(.plain)
                .font(.system(size: 11))
                .foregroundStyle(Color.accentColor)
                .disabled(store.loading)
        }
    }

    private var measuredText: String {
        if store.loading { return l10n.tr("Measuring from this Mac…", "正在从这台 Mac 测…") }
        guard let at = store.measuredAt else {
            return l10n.tr("Measured from this Mac", "从这台 Mac 测")
        }
        let s = Int(Date().timeIntervalSince(at))
        if s < 10 { return l10n.tr("Measured from this Mac, just now", "从这台 Mac 测，刚刚测的") }
        if s < 60 { return l10n.tr("Measured from this Mac, \(s)s ago", "从这台 Mac 测，\(s) 秒前") }
        return l10n.tr("Measured from this Mac, \(s / 60)m ago", "从这台 Mac 测，\(s / 60) 分钟前")
    }
}

/// pickableRoute: whether a row is a choice. The row in use is not — you are already
/// there — and that is expressed by taking its clicks away, never by fading it out: the
/// row that reads loudest must be the one you are on (2026-09-23).
func pickableRoute(_ s: DirectServer) -> Bool { !s.current }

/// codeLeft is the pairing code's countdown, m:ss, or nil once there is none left to
/// show. "The code expires in 5 minutes" was the same sentence a second before it died.
func codeLeft(_ expiresAt: Date, now: Date) -> String? {
    let left = Int(expiresAt.timeIntervalSince(now).rounded())
    guard left > 0 else { return nil }
    return String(format: "%d:%02d", left / 60, left % 60)
}

/// What a move costs, said BEFORE it happens: a device that has connected before follows on
/// its own, one that only ever scanned has to scan again, and links minted before the move
/// stop working. Discovering any of that afterwards is what this exists to prevent.
func directMoveConfirmation(_ s: DirectServer, l10n: L10n) -> NSAlert {
    let a = NSAlert()
    a.messageText = l10n.tr("Move this Mac to \(s.name)?", "把这台 Mac 换到「\(s.name)」？")
    a.informativeText = l10n.tr(
        """
        A phone that has connected to this Mac before follows on its own.

        A device that paired but never connected has to scan the pairing code again, and \
        guest links made before the move stop working.
        """,
        """
        之前连上来过的手机会自己跟过来。

        只扫过码、还没连上来过的设备要重新扫一次；换之前发出的分享链接会失效。
        """)
    a.addButton(withTitle: l10n.tr("Move", "换过去"))
    a.addButton(withTitle: l10n.tr("Cancel", "取消"))
    return a
}
