import Foundation

public enum UsageWidgetWords {
    public static func text(_ en: String, _ zh: String, chinese: Bool) -> String { chinese ? zh : en }

    public static func window(_ value: UsageWidgetWindow, agent: String, chinese: Bool) -> String {
        let known: [String: (String, String)] = [
            "hour": ("Hourly", "小时额度"), "session": ("Session", "会话额度"), "day": ("Daily", "日额度"),
            "week": ("Weekly", "周额度"), "week-all": ("Weekly", "周额度"), "month": ("Monthly", "月额度"),
        ]
        if value.kind == "week-model" {
            return value.model.isEmpty ? text("Weekly", "周额度", chinese: chinese) :
                value.model + " · " + text("Weekly", "周额度", chinese: chinese)
        }
        if let pair = known[value.kind] { return chinese ? pair.1 : pair.0 }
        let prefix = agent + " "
        return value.label.hasPrefix(prefix) ? String(value.label.dropFirst(prefix.count)) : value.label
    }

    public static func percent(_ value: Double) -> String {
        value.formatted(.number.precision(.fractionLength(0...1))) + "%"
    }

    public static func outputDay(_ output: UsageWidgetOutput, at date: Date, chinese: Bool, calendar: Calendar = .current) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.timeZone = calendar.timeZone
        formatter.dateFormat = "yyyy-MM-dd"
        return output.day == formatter.string(from: date) ? text("Today", "今日", chinese: chinese) : output.day
    }

    public static func count(_ value: Int64) -> String {
        let number = Double(value)
        let pair: (Double, String) = value >= 1_000_000 ? (1_000_000, "M") : value >= 1_000 ? (1_000, "K") : (1, "")
        return (number / pair.0).formatted(.number.locale(Locale(identifier: "en_US_POSIX")).precision(.fractionLength(0...1))) + pair.1
    }

    public static func wallet(_ value: UsageWidgetWallet, chinese: Bool) -> String {
        switch value.kind {
        case "bonus": return text("Bonus", "赠送", chinese: chinese)
        case "recharge": return text("Recharge", "充值", chinese: chinese)
        default: return value.kind
        }
    }
}
