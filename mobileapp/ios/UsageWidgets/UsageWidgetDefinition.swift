import SwiftUI
import WidgetKit
import AppIntents

private let usageKind = "com.gtmux.app.usage"
private var usageChinese: Bool { Locale.preferredLanguages.first?.hasPrefix("zh") ?? false }

struct UsageEntry: TimelineEntry {
  let date: Date
  let snapshot: UsageWidgetSnapshot
  let provider: String
}

private func usageTimeline(provider: String) -> Timeline<UsageEntry> {
  let now = Date()
  let snapshot = UsageSnapshotStore.shared.read() ?? .empty
  let entries = ([now] + snapshot.transitionDates(after: now)).map {
    UsageEntry(date: $0, snapshot: snapshot, provider: provider)
  }
  return Timeline(entries: entries, policy: .after(now.addingTimeInterval(15 * 60)))
}

struct UsageEntryView: View {
  let entry: UsageEntry
  @Environment(\.widgetFamily) private var family
  var body: some View {
    if #available(iOS 17.0, *) {
      content.containerBackground(.background, for: .widget)
    } else { content.padding() }
  }
  private var content: some View {
    UsageWidgetContent(snapshot: entry.snapshot, provider: entry.provider, date: entry.date,
                       family: family, chinese: usageChinese)
  }
}

@available(iOS 17.0, *)
struct UsageAgentOptions: DynamicOptionsProvider {
  func results() async throws -> IntentItemCollection<String> {
    let choices = UsageWidgetSelection.choices(in: UsageSnapshotStore.shared.read() ?? .empty,
                                               automaticName: usageChinese ? "自动" : "Automatic")
    return IntentItemCollection(sections: [IntentItemSection(items: choices.map {
      IntentItem($0.id, title: "\($0.name)")
    })])
  }
  func defaultResult() async -> String? { UsageWidgetSelection.automaticID }
}

@available(iOS 17.0, *)
struct UsageConfiguration: WidgetConfigurationIntent {
  static var title: LocalizedStringResource = "Agent usage"
  static var description = IntentDescription("Choose an agent from the current server.")
  @Parameter(title: "Agent", optionsProvider: UsageAgentOptions())
  var agent: String?
  static var parameterSummary: some ParameterSummary { Summary { \.$agent } }
}

@available(iOS 17.0, *)
struct ConfigurableUsageProvider: AppIntentTimelineProvider {
  func placeholder(in context: Context) -> UsageEntry { UsageEntry(date: Date(), snapshot: .empty, provider: "") }
  func snapshot(for configuration: UsageConfiguration, in context: Context) async -> UsageEntry {
    UsageEntry(date: Date(), snapshot: UsageSnapshotStore.shared.read() ?? .empty,
               provider: UsageWidgetSelection.provider(for: configuration.agent))
  }
  func timeline(for configuration: UsageConfiguration, in context: Context) async -> Timeline<UsageEntry> {
    usageTimeline(provider: UsageWidgetSelection.provider(for: configuration.agent))
  }
}

@available(iOS 17.0, *)
struct GtmuxUsageWidget: Widget {
  var body: some WidgetConfiguration {
    AppIntentConfiguration(kind: usageKind, intent: UsageConfiguration.self, provider: ConfigurableUsageProvider()) { UsageEntryView(entry: $0) }
      .configurationDisplayName(usageChinese ? "Agent 用量" : "Agent usage")
      .description(usageChinese ? "额度或已记录的输出用量。" : "Quota or recorded output usage.")
      .supportedFamilies([.systemSmall, .systemMedium, .accessoryCircular, .accessoryRectangular, .accessoryInline])
  }
}
