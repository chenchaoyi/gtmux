import SwiftUI

struct HQBoardView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @Environment(\.colorScheme) private var scheme
    var body: some View { boardBody(Theme.Palette.of(scheme)) }

    /// How long ago HQ last wrote the board, in the words the phone uses.
    ///
    /// The phone's board sheet says "updated 3h ago" under its title; the Mac said nothing
    /// at all, and `updatedAt` was already on the model (2026-09-09). A board read as
    /// current when it is hours stale is a failure that costs something real: it is HQ's
    /// picture of the fleet, and the whole reason to open it is that the picture is
    /// trustworthy.
    private func boardAge(_ at: Int64?) -> String? {
        guard let at, at > 0 else { return nil }
        return boardAgeText(Int64(Date().timeIntervalSince1970) - at, zh: l10n.lang == "zh")
    }

    @ViewBuilder private func boardBody(_ p: Theme.Palette) -> some View {
        if let b = store.board, b.exists, let text = b.text, !text.isEmpty {
            // An OUTLINE, not the whole document: 74 KB on this machine, and reaching any
            // one entry meant dragging through the rest. Same reader as the phone.
            VStack(alignment: .leading, spacing: 0) {
                if let age = boardAge(b.updatedAt) {
                    HStack(spacing: 5) {
                        Circle()
                            .fill(Theme.Status.idle)
                            .frame(width: 6, height: 6)
                        Text(age).font(.system(size: 11)).foregroundStyle(p.fg3)
                        Spacer()
                    }
                    .padding(.horizontal, 14)
                    .padding(.top, 8)
                    .padding(.bottom, 2)
                }
                BoardOutlineView(markdown: text, p: p, l10n: l10n)
            }
        } else {
            // A supervisor that has written no board is ordinary, not broken.
            HQResourceEmpty(
                l10n.tr(
                    "No situation board yet. HQ writes one as it works",
                    "还没有态势板，HQ 干着干着就会写一份"), p)
        }
    }

}
