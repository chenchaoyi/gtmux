import AppKit
import SwiftUI

struct AccessHistoryLog: View {
    let entries: [AccessHistoryEntry]
    let zh: Bool
    var body: some View {
        LazyVStack(alignment: .leading, spacing: 0) {
            ForEach(AccessHistoryPresentation.lines(entries, zh: zh)) { line in
                if let day = line.day {
                    Text(day).font(Theme.Font.session).foregroundStyle(.secondary)
                        .padding(.top, 12).padding(.bottom, 6)
                        .accessibilityAddTraits(.isHeader)
                }
                AccessHistoryLogRow(line: line, zh: zh)
                Divider()
            }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct AccessHistoryLogRow: View {
    let line: AccessHistoryLine
    let zh: Bool
    private var entry: AccessHistoryEntry { line.entry }
    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(line.clock).font(Theme.Font.task.monospacedDigit()).foregroundStyle(.secondary)
                .frame(width: 60, alignment: .leading).fixedSize(horizontal: false, vertical: true)
            VStack(alignment: .leading, spacing: 4) {
                HStack(alignment: .firstTextBaseline, spacing: 12) {
                    Text(AccessHistoryPresentation.name(entry, zh: zh)).font(Theme.Font.session)
                        .frame(width: 112, alignment: .leading)
                    Text(AccessHistoryPresentation.operation(entry, zh: zh)).font(Theme.Font.task)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .fixedSize(horizontal: false, vertical: true)
                    Text(AccessHistoryPresentation.outcome(entry, zh: zh))
                        .font(Theme.Font.task.monospacedDigit())
                        .foregroundStyle(entry.outcome == "ok" ? Color.secondary : Theme.Status.errored)
                }
                let metadata = AccessHistoryPresentation.metadata(entry, zh: zh)
                if !metadata.isEmpty {
                    Text(metadata).font(Theme.Font.window).foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }.frame(maxWidth: .infinity, alignment: .leading)
        }.padding(.vertical, 8).textSelection(.enabled)
            .contextMenu {
                Button(zh ? "复制记录" : "Copy record") {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(AccessHistoryPresentation.copyText(entry, zh: zh), forType: .string)
                }
            }
    }
}
