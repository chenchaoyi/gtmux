import Foundation

struct HQUsageWindowGroup: Identifiable {
    var id: String
    var name: String
    var windows: [HQUsageWindow]
}

func hqUsageShowsLead(_ windows: [HQUsageWindow]) -> Bool {
    let current = windows.filter { $0.stale != true }
    return current.count > 1 || current.contains { $0.tier == "warn" || $0.tier == "full" || $0.pctUsed >= 100 }
}

func hqUsageResetSummary(_ window: HQUsageWindow, zh: Bool, now: Date = Date()) -> String {
    guard let unix = window.resetUnix, unix > 0 else { return window.resetAt ?? "" }
    let format = DateFormatter()
    format.locale = Locale(identifier: zh ? "zh_CN" : "en_US")
    format.dateFormat = zh ? "M月d日 HH:mm" : "MMM d 'at' h:mma"
    let date = format.string(from: Date(timeIntervalSince1970: TimeInterval(unix)))
    guard Double(unix) > now.timeIntervalSince1970 else { return date }
    let minutes = Int(((Double(unix) - now.timeIntervalSince1970) / 60).rounded())
    let days = minutes / 1440
    let hours = minutes % 1440 / 60
    let duration = days > 0 ? (hours == 0 ? (zh ? "\(days) 天后重置" : "resets in \(days)d") : (zh ? "\(days) 天 \(hours) 小时后重置" : "resets in \(days)d \(hours)h"))
        : hours > 0 ? (minutes % 60 == 0 ? (zh ? "\(hours) 小时后重置" : "resets in \(hours)h") : (zh ? "\(hours) 小时 \(minutes % 60) 分钟后重置" : "resets in \(hours)h \(minutes % 60)m"))
        : (zh ? "\(minutes) 分钟后重置" : "resets in \(minutes)m")
    return duration + " · " + date
}

/// The core owns freshness. Older reports without a stale flag keep their meaning.
func hqUsageWindowGroups(_ windows: [HQUsageWindow], historical: Bool) -> [HQUsageWindowGroup] {
    var groups: [HQUsageWindowGroup] = []
    for window in windows where (window.stale == true) == historical {
        let key = window.agent ?? window.label
        if let i = groups.firstIndex(where: { $0.id == key }) {
            groups[i].windows.append(window)
        } else {
            groups.append(HQUsageWindowGroup(id: key, name: window.agentName ?? key, windows: [window]))
        }
    }
    return groups
}

func hqUsageSourceNote(_ status: String?, zh: Bool) -> String {
    switch status {
    case "signed-out": return zh ? "未登录 DeepSeek Account" : "Signed out of DeepSeek Account"
    case "unverified-account": return zh ? "原生服务未提供可核对的账户标识，暂不展示余额。" : "The native service supplied no verifiable account identity; balance is unavailable."
    case "unavailable": return zh ? "账户余额暂时无法读取。" : "Account balance is temporarily unavailable."
    default: return zh ? "尚未收到原生账户余额；API Key 和自定义 Provider 暂不支持。" : "No native account balance received; API-key and custom providers are not supported yet."
    }
}
