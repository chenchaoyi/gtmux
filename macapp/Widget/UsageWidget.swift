import SwiftUI
import WidgetKit
import AppIntents
import GtmuxUsageWidgets
import MacUsageWidgetSupport

private var chinese: Bool { Locale.preferredLanguages.first?.hasPrefix("zh") ?? false }

struct MacUsageEntry: TimelineEntry {
    let date: Date
    let snapshot: UsageWidgetSnapshot
    let provider: String
}

struct MacUsageProvider: AppIntentTimelineProvider {
    func placeholder(in context: Context) -> MacUsageEntry { MacUsageEntry(date: Date(), snapshot: .empty, provider: "") }
    func snapshot(for configuration: MacUsageConfiguration, in context: Context) async -> MacUsageEntry {
        MacUsageEntry(date: Date(), snapshot: MacUsageWidgetContainer.store.read() ?? .empty,
                      provider: UsageWidgetSelection.provider(for: configuration.agent))
    }
    func timeline(for configuration: MacUsageConfiguration, in context: Context) async -> Timeline<MacUsageEntry> {
        let now = Date(), snapshot = MacUsageWidgetContainer.store.read() ?? .empty
        let provider = UsageWidgetSelection.provider(for: configuration.agent)
        return Timeline(entries: ([now] + snapshot.transitionDates(after: now)).map {
            MacUsageEntry(date: $0, snapshot: snapshot, provider: provider)
        }, policy: .after(now.addingTimeInterval(15 * 60)))
    }
}

struct MacUsageEntryView: View {
    let entry: MacUsageEntry
    @Environment(\.widgetFamily) private var family
    var body: some View {
        Group {
            if entry.snapshot.selected(entry.provider).isEmpty {
                VStack(alignment: .leading, spacing: 6) {
                    Label("gtmux", systemImage: "square.split.2x2").font(.headline)
                    Text(chinese ? "暂无用量数据" : "Usage unavailable").font(.subheadline.weight(.semibold))
                    Text(entry.provider.isEmpty
                         ? (chinese ? "打开 gtmux，更新本机用量。" : "Open gtmux to update usage on this Mac.")
                         : (chinese ? "本机暂无此 Agent 的当前读数。" : "This Mac has no current reading for this agent."))
                        .font(.caption).foregroundStyle(.secondary)
                }.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
                    .accessibilityElement(children: .combine)
                    .dynamicTypeSize(.small ... .large)
            } else {
                UsageWidgetContent(snapshot: entry.snapshot, provider: entry.provider, date: entry.date,
                                   family: family, chinese: chinese)
            }
        }
        .widgetURL(URL(string: "gtmux://usage"))
        .privacySensitive()
        .containerBackground(.background, for: .widget)
    }
}

struct GtmuxMacUsageWidget: Widget {
    var body: some WidgetConfiguration {
        AppIntentConfiguration(kind: MacUsageWidgetContainer.kind, intent: MacUsageConfiguration.self,
                               provider: MacUsageProvider()) { MacUsageEntryView(entry: $0) }
            .configurationDisplayName(chinese ? "Agent 用量" : "Agent usage")
            .description(chinese ? "本机额度或已记录的输出用量。" : "Quota or recorded output on this Mac.")
            .supportedFamilies([.systemSmall, .systemMedium])
    }
}

@main
struct GtmuxMacWidgets: WidgetBundle {
    var body: some Widget { GtmuxMacUsageWidget() }
}
