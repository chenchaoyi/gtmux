// Compile this intent into both the app and extension. Xcode extracts the
// metadata for each bundle; the SwiftPM launcher is only a development entry.
import Foundation
import AppIntents
import GtmuxUsageWidgets
import MacUsageWidgetSupport

private var chinese: Bool { Locale.preferredLanguages.first?.hasPrefix("zh") ?? false }

struct MacUsageOptions: DynamicOptionsProvider {
    func results() async throws -> IntentItemCollection<String> {
        let choices = UsageWidgetSelection.choices(in: MacUsageWidgetContainer.store.read() ?? .empty,
                                                   automaticName: chinese ? "自动" : "Automatic")
        return IntentItemCollection(sections: [IntentItemSection(items: choices.map { IntentItem($0.id, title: "\($0.name)") })])
    }
    func defaultResult() async -> String? { UsageWidgetSelection.automaticID }
}

struct MacUsageConfiguration: WidgetConfigurationIntent {
    static var title: LocalizedStringResource = "Agent usage"
    static var description = IntentDescription("Choose an agent on this Mac.")
    @Parameter(title: "Agent", optionsProvider: MacUsageOptions()) var agent: String?
    static var parameterSummary: some ParameterSummary { Summary { \.$agent } }
}
