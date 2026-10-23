import Foundation
import UserNotifications

struct DesktopNotificationAuthorization: Equatable {
    enum Permission { case checking, notRequested, denied, allowed }
    var permission: Permission
    var banners = false
    var sounds = false

    var allowsDelivery: Bool { permission == .allowed }

    func summary(_ l10n: L10n) -> String {
        switch permission {
        case .checking: return l10n.tr("Checking macOS permission…", "正在检查系统通知权限…")
        case .notRequested: return l10n.tr("Send a test to allow notifications.", "发送测试通知以申请系统权限。")
        case .denied: return l10n.tr("Notifications are off in macOS Settings.", "macOS 已关闭 gtmux 通知。")
        case .allowed:
            return banners ? l10n.tr("Allowed by macOS. Focus may silence alerts.", "系统已允许通知；专注模式可能静音。")
                : l10n.tr("Allowed, but banners are off in macOS Settings.", "系统已允许通知，但横幅未开启。")
        }
    }
}

// Keep native authorization and posting injectable. Tests never register an app,
// request real permission or submit a notification to the user's notification center.
protocol DesktopNotificationCenter: AnyObject {
    func setDelegate(_ delegate: UNUserNotificationCenterDelegate)
    func settings(_ completion: @escaping (DesktopNotificationAuthorization) -> Void)
    func requestAuthorization(_ completion: @escaping (Error?) -> Void)
    func setCategories(_ categories: Set<UNNotificationCategory>)
    func add(_ request: UNNotificationRequest, completion: @escaping (Error?) -> Void)
    func removeDelivered(_ identifiers: [String])
}

final class SystemDesktopNotificationCenter: DesktopNotificationCenter {
    private var center: UNUserNotificationCenter { .current() }
    func setDelegate(_ delegate: UNUserNotificationCenterDelegate) { center.delegate = delegate }
    func settings(_ completion: @escaping (DesktopNotificationAuthorization) -> Void) {
        center.getNotificationSettings { settings in
            let permission: DesktopNotificationAuthorization.Permission
            switch settings.authorizationStatus {
            case .authorized, .provisional, .ephemeral: permission = .allowed
            case .notDetermined: permission = .notRequested
            case .denied: permission = .denied
            @unknown default: permission = .denied
            }
            completion(DesktopNotificationAuthorization(permission: permission,
                banners: settings.alertSetting == .enabled && settings.alertStyle != .none,
                sounds: settings.soundSetting == .enabled))
        }
    }
    func requestAuthorization(_ completion: @escaping (Error?) -> Void) {
        center.requestAuthorization(options: [.alert, .sound]) { _, error in completion(error) }
    }
    func setCategories(_ categories: Set<UNNotificationCategory>) { center.setNotificationCategories(categories) }
    func add(_ request: UNNotificationRequest, completion: @escaping (Error?) -> Void) {
        center.add(request, withCompletionHandler: completion)
    }
    func removeDelivered(_ identifiers: [String]) { center.removeDeliveredNotifications(withIdentifiers: identifiers) }
}
