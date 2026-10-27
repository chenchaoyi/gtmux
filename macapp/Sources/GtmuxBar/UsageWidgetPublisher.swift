import Foundation
import WidgetKit
import GtmuxUsageWidgets
import MacUsageWidgetSupport

/// Only the owning app invokes the CLI. The extension reads a typed display
/// projection and has no network, terminal, credentials or owner-token access.
final class UsageWidgetPublisher {
    private var timer: Timer?
    private var policy = MacUsageRefreshPolicy()
    private var checkingConfiguration = false

    func start() {
        guard timer == nil else { return }
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in self?.refresh() }
    }

    func stop() { timer?.invalidate(); timer = nil }

    private func refresh() {
        guard !checkingConfiguration else { return }
        checkingConfiguration = true
        WidgetCenter.shared.getCurrentConfigurations { [weak self] result in
            DispatchQueue.main.async {
                guard let self else { return }
                self.checkingConfiguration = false
                let configured = (try? result.get().contains { $0.kind == MacUsageWidgetContainer.kind }) ?? false
                guard self.policy.begin(configured: configured, at: Date()) else { return }
                DispatchQueue.global(qos: .utility).async { [weak self] in
                    let report = GtmuxCLI.capturePoll(["usage", "--json"])
                    var published = false
                    if let report, let snapshot = try? UsageWidgetSnapshot.project(report: report, server: "", at: Date()) {
                        do { try MacUsageWidgetContainer.store.write(snapshot); published = true }
                        catch { DiagLog.info("widget.publish", "usage snapshot unavailable") }
                    }
                    DispatchQueue.main.async {
                        guard let self else { return }
                        if self.policy.finish(published: published, at: Date()) {
                            WidgetCenter.shared.reloadTimelines(ofKind: MacUsageWidgetContainer.kind)
                        }
                    }
                }
            }
        }
    }
}
