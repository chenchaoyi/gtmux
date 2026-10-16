import SwiftUI

/// The authored board is continuity context, separate from explicit current matters.
struct HQBoardView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let palette = Theme.Palette.of(scheme)
        if let board = store.board, board.exists, let text = board.text, !text.isEmpty {
            VStack(alignment: .leading, spacing: 0) {
                VStack(alignment: .leading, spacing: 5) {
                    if let at = board.updatedAt, at > 0 {
                        HStack(spacing: 5) {
                            Text(l10n.tr("Saved", "保存于"))
                            Text(Date(timeIntervalSince1970: TimeInterval(at)), format: .dateTime.year().month().day().hour().minute())
                        }.font(HQTypography.caption)
                    }
                    Text(l10n.tr(
                        "Saved context, not live session state. Current work is in Overview.",
                        "这是 HQ 保存的背景与交接记录，当前事项请查看总览。"))
                        .font(HQTypography.body)
                }.foregroundStyle(.secondary).padding(14)
                Divider()
                BoardOutlineView(markdown: text, p: palette, l10n: l10n, snapshot: true)
            }
        } else {
            HQResourceEmpty(l10n.tr("No handoff snapshot yet.", "尚无交接快照。"), palette)
        }
    }
}
