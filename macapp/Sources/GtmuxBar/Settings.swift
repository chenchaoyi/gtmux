import AppKit
import Combine
import ServiceManagement

/// Settings is the persisted preferences store (DESIGN §8), UserDefaults-backed.
/// AppDelegate observes it to repaint the status item and re-time polling.
final class AppSettings: ObservableObject {
    static let shared = AppSettings()
    private let d = UserDefaults.standard

    @Published var displayMode: DisplayMode {
        didSet { d.set(displayMode.rawValue, forKey: "statusbar.mode") }
    }
    @Published var refreshInterval: Double {
        didSet { d.set(refreshInterval, forKey: "refresh.interval") }
    }
    @Published var notifications: Bool {
        didSet { d.set(notifications, forKey: "notifications") }
    }
    @Published var launchAtLogin: Bool {
        didSet { applyLaunchAtLogin() }
    }

    private init() {
        displayMode = DisplayMode(rawValue: d.string(forKey: "statusbar.mode") ?? "") ?? .dotCount
        refreshInterval = (d.object(forKey: "refresh.interval") as? Double) ?? 1.5
        notifications = (d.object(forKey: "notifications") as? Bool) ?? true
        launchAtLogin = (SMAppService.mainApp.status == .enabled)
    }

    private func applyLaunchAtLogin() {
        do {
            if launchAtLogin { try SMAppService.mainApp.register() }
            else { try SMAppService.mainApp.unregister() }
        } catch {
            dbg("launch-at-login toggle failed: \(error)")
        }
    }
}
