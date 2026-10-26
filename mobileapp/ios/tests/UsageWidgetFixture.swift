// Standalone synthetic UI fixture, never compiled into the shipped application.
import SwiftUI
import UIKit
import WidgetKit

@main struct UsageWidgetFixture: App {
  var body: some Scene { WindowGroup { UsageFixtureScreen() } }
}

private struct UsageFixtureScreen: View {
  @State private var provider = "dsh"
  @State private var scenario = "Current"
  @State private var message = ""
  private let captured = Date()
  init() {
    let args = ProcessInfo.processInfo.arguments
    func option(_ key: String) -> String? {
      guard let index = args.firstIndex(of: key), index + 1 < args.count else { return nil }
      return args[index + 1]
    }
    _provider = State(initialValue: option("--provider") ?? "dsh")
    _scenario = State(initialValue: option("--scenario") ?? "Current")
  }
  private var sourceDay: String {
    let formatter = DateFormatter(); formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.calendar = Calendar(identifier: .gregorian)
    formatter.dateFormat = "yyyy-MM-dd"
    return formatter.string(from: captured.addingTimeInterval(scenario == "Previous day" ? -86400 : 0))
  }
  private var snapshot: UsageWidgetSnapshot {
    if scenario == "Empty" { return .empty }
    let now = captured.timeIntervalSince1970
    var value = UsageWidgetSnapshot(server: "Demo server", capturedAt: now, accounts: [
      UsageWidgetAccount(id: "codex", agent: "codex", name: "Codex", windows: [
        UsageWidgetWindow(label: "session", kind: "session", percent: 82.5, resetAt: now + 3600, observedAt: now - 60, tier: "warn"),
        UsageWidgetWindow(label: "week", kind: "week", percent: 28, resetAt: now + 86400, observedAt: now - 60)], wallets: []),
      UsageWidgetAccount(id: "claude", agent: "claude", name: "Claude Code", windows: [
        UsageWidgetWindow(label: "5h", kind: "session", percent: 18, resetAt: now + 7200, observedAt: now - 30)], wallets: []),
      UsageWidgetAccount(id: "dsh", agent: "dsh", name: "DeepSeek Harness", windows: [], wallets: [],
        observedAt: now - 90, output: UsageWidgetOutput(day: sourceDay, today: 2400, week: 16800)),
    ])
    if scenario == "Stale" { value.capturedAt = now - 1800 }
    if scenario == "Zero" { value.accounts[2].output?.today = 0 }
    if scenario == "Missing history" { value.accounts.removeLast() }
    if scenario == "Reset" { value.accounts[0].windows[0].resetAt = now - 1 }
    if scenario == "Long" { value.accounts[0].name = "Custom Agent with a long provider name"; value.server = "A very long server name that needs truncation" }
    return value
  }
  var body: some View {
    NavigationStack {
      ScrollView {
        VStack(alignment: .leading, spacing: 20) {
          Text("Synthetic data · no real connection").font(.caption).foregroundStyle(.secondary)
          Picker("Agent", selection: $provider) {
            Text("Automatic").tag(""); Text("Codex").tag("codex"); Text("Claude").tag("claude"); Text("DeepSeek").tag("dsh"); Text("Missing").tag("missing")
          }.pickerStyle(.menu).accessibilityIdentifier("usage-fixture-provider")
          Picker("Reading", selection: $scenario) {
            ForEach(["Current", "Zero", "Previous day", "Missing history", "Stale", "Reset", "Empty", "Long"], id: \.self) { Text($0).tag($0) }
          }.pickerStyle(.menu).accessibilityIdentifier("usage-fixture-scenario")
          Text("Small").font(.headline)
          widget(.systemSmall, width: 170, height: 170)
          Text("Medium").font(.headline)
          widget(.systemMedium, width: 350, height: 170)
          Text("Lock Screen").font(.headline)
          widget(.accessoryRectangular, width: 170, height: 80)
          widget(.accessoryCircular, width: 76, height: 76)
          widget(.accessoryInline, width: 300, height: 30)
          Button("Seed widget gallery") {
            do { try UsageSnapshotStore.shared.write(snapshot); WidgetCenter.shared.reloadTimelines(ofKind: "com.gtmux.app.usage"); message = "Snapshot saved" }
            catch { message = "Snapshot error: \(error.localizedDescription)" }
          }.accessibilityIdentifier("usage-fixture-seed")
          Text(message).accessibilityIdentifier("usage-fixture-result")
        }.padding()
      }.navigationTitle("Usage widget fixture")
        .task {
          let args = ProcessInfo.processInfo.arguments
          if let index = args.firstIndex(of: "--refresh-checks"), index + 1 < args.count {
            let results = await UsageWidgetRefreshFixtureChecks.run(base: args[index + 1])
            let file = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("usage-refresh-checks.json")
            if let data = try? JSONEncoder().encode(results) { try? data.write(to: file, options: .atomic) }
            message = results.values.allSatisfy { $0 } ? "All refresh checks passed" : "Refresh check failed"
          }
        }
    }
  }
  private func widget(_ family: WidgetFamily, width: CGFloat, height: CGFloat) -> some View {
    UsageWidgetContent(snapshot: snapshot, provider: provider, date: captured, family: family,
                       chinese: !ProcessInfo.processInfo.arguments.contains("--english"))
      .padding(family == .systemSmall || family == .systemMedium ? 16 : 4)
      .frame(width: width, height: height).background(Color(uiColor: .secondarySystemGroupedBackground))
      .clipShape(RoundedRectangle(cornerRadius: 20)).frame(maxWidth: .infinity, alignment: .leading)
  }
}
