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
        case .checking: return l10n.tr("Checking system permission…", "正在检查系统通知权限…")
        case .notRequested: return l10n.tr("System permission not requested", "尚未申请系统通知权限")
        case .denied: return l10n.tr("System notifications off", "系统通知已关闭")
        case .allowed:
            return banners ? l10n.tr("System notifications allowed", "系统通知已允许")
                : l10n.tr("System notifications allowed · banners off", "系统通知已允许 · 横幅已关闭")
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
