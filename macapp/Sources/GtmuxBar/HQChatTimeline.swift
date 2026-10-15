import Foundation
import SwiftUI

/// Match mobile time.ts: compare successive valid clocks, never successive labels.
enum HQChatTimeline {
    static let gap: TimeInterval = 15 * 60
    static func timestamp(_ iso: String?) -> Int64? {
        guard let iso else { return nil }
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        guard let date = fractional.date(from: iso) ?? ISO8601DateFormatter().date(from: iso),
            date.timeIntervalSince1970 > 0
        else { return nil }
        return Int64(date.timeIntervalSince1970)
    }
    static func separators(_ times: [Int64?], calendar: Calendar = .current) -> [Date?] {
        var previous: Date?
        return times.map { value in
            guard let value, value > 0 else { return nil }
            let date = Date(timeIntervalSince1970: TimeInterval(value))
            let show =
                previous.map {
                    !calendar.isDate(date, inSameDayAs: $0) || date.timeIntervalSince($0) >= gap
                } ?? true
            previous = date
            return show ? date : nil
        }
    }
    static func label(_ date: Date, zh: Bool, now: Date = Date(), calendar: Calendar = .current) -> String {
        let components = calendar.dateComponents([.hour, .minute, .year], from: date)
        let clock = String(format: "%02d:%02d", components.hour ?? 0, components.minute ?? 0)
        if calendar.isDate(date, inSameDayAs: now) { return (zh ? "今天 " : "Today ") + clock }
        if let yesterday = calendar.date(byAdding: .day, value: -1, to: now),
            calendar.isDate(date, inSameDayAs: yesterday)
        {
            return (zh ? "昨天 " : "Yesterday ") + clock
        }
        let formatter = DateFormatter()
        formatter.calendar = calendar
        formatter.timeZone = calendar.timeZone
        formatter.locale = Locale(identifier: zh ? "zh_CN" : "en_US")
        let sameYear = components.year == calendar.component(.year, from: now)
        formatter.dateFormat = zh ? (sameYear ? "M月d日" : "yyyy年M月d日") : (sameYear ? "MMM d," : "MMM d yyyy,")
        return formatter.string(from: date) + " " + clock
    }
}

struct HQChatTimeSeparator: View {
    let date: Date
    let l10n: L10n
    var body: some View {
        HStack(spacing: 12) {
            Rectangle().fill(Color.primary.opacity(0.08)).frame(height: 1)
            Text(HQChatTimeline.label(date, zh: l10n.lang == "zh"))
                .font(Theme.Font.window).foregroundStyle(.secondary).fixedSize()
            Rectangle().fill(Color.primary.opacity(0.08)).frame(height: 1)
        }.padding(.vertical, 4).accessibilityElement(children: .combine)
    }
}
