import SwiftUI
import WidgetKit
#if os(macOS)
import AppKit
#else
import UIKit
#endif

/// Shared view composition for WidgetKit and isolated native previews. It does
/// not fetch data, know an account credential or infer billing from token output.
@available(iOS 16.0, macOS 14.0, *)
public struct UsageWidgetContent: View {
    public let snapshot: UsageWidgetSnapshot
    public let provider: String
    public let date: Date
    public let family: WidgetFamily
    public let chinese: Bool
    @Environment(\.dynamicTypeSize) private var textSize

    public init(snapshot: UsageWidgetSnapshot, provider: String = "", date: Date, family: WidgetFamily, chinese: Bool) {
        self.snapshot = snapshot; self.provider = provider; self.date = date; self.family = family; self.chinese = chinese
    }

    private var accounts: [UsageWidgetAccount] { snapshot.selected(provider) }
    private func tr(_ en: String, _ zh: String) -> String { UsageWidgetWords.text(en, zh, chinese: chinese) }

    public var body: some View {
        Group {
            if let account = accounts.first {
                switch family {
                #if os(iOS)
                case .accessoryCircular: circle(account)
                case .accessoryInline: inline(account)
                case .accessoryRectangular: accessory(account)
                #endif
                case .systemMedium: medium(account)
                default:
                    if textSize >= .xxLarge { accessibleSmall(account) }
                    else { small(account) }
                }
            } else { empty }
        }
        .widgetURL(URL(string: "gtmux://usage"))
        .privacySensitive()
        // Widget dimensions do not grow with app text size. Keep secondary
        // labels compact; the small-family alternative enlarges its main value.
        // Full values remain available to accessibility and in the usage sheet.
        .dynamicTypeSize(.small ... .large)
    }

    @ViewBuilder private var empty: some View {
        #if os(iOS)
        switch family {
        case .accessoryInline:
            Text(tr("gtmux · Usage unavailable", "gtmux · 暂无用量"))
        case .accessoryCircular:
            VStack(spacing: 3) { Image(systemName: "arrow.clockwise"); Text("gtmux").font(.caption2) }
                .accessibilityLabel(tr("gtmux. Open the app to sync usage.", "gtmux，打开 App 同步用量。"))
        case .accessoryRectangular:
            VStack(alignment: .leading, spacing: 3) {
                Text("gtmux").font(.headline)
                Text(tr("Open the app to sync usage", "打开 App 同步用量")).font(.caption)
            }
        default: emptyHome
        }
        #else
        emptyHome
        #endif
    }

    private var emptyHome: some View {
        VStack(alignment: .leading, spacing: 6) {
            Label("gtmux", systemImage: "square.split.2x2").font(.headline)
            Text(tr("Usage unavailable", "暂无用量数据")).font(.subheadline.weight(.semibold))
            Text(provider.isEmpty ? tr("Open gtmux to sync your server.", "打开 gtmux，同步服务器用量。") :
                tr("This server has no current reading for this agent.", "当前服务器未提供此 Agent 的读数。"))
                .font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }

    private func old(_ account: UsageWidgetAccount) -> Bool {
        snapshot.isStale(at: date) || (account.observedAt.map { date.timeIntervalSince1970 - $0 >= UsageWidgetSnapshot.staleInterval } ?? false)
    }

    private func tint(_ window: UsageWidgetWindow, account: UsageWidgetAccount) -> Color {
        if old(account) || window.expired(at: date) || window.stale(at: date) { return .secondary }
        switch window.tier {
        case "full": return .red
        case "warn": return .orange
        default: return Color(red: 0.231, green: 0.510, blue: 0.965)
        }
    }

    private func heading(_ account: UsageWidgetAccount) -> some View {
        HStack(spacing: 5) {
            UsageWidgetAgentMark(agent: account.agent)
            Text(account.name).font(.caption.weight(.semibold)).lineLimit(1)
        }
    }

    private func small(_ account: UsageWidgetAccount) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            heading(account)
            Text(snapshot.server).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
            Spacer(minLength: 0)
            if let window = account.windows.first {
                quota(window, account: account, large: true)
            } else { nonQuota(account, large: true) }
            Spacer(minLength: 0)
            updated(account)
        }
    }

    private func accessibleSmall(_ account: UsageWidgetAccount) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            heading(account).font(.system(size: 14, weight: .semibold))
            Text(snapshot.server).font(.system(size: 12)).foregroundStyle(.secondary).lineLimit(1)
            Spacer(minLength: 0)
            if let window = account.windows.first {
                Text(UsageWidgetWords.window(window, agent: account.agent, chinese: chinese))
                    .font(.system(size: 12)).foregroundStyle(.secondary).lineLimit(1)
                Text(window.expired(at: date) ? tr("Update needed", "等待更新") : UsageWidgetWords.percent(window.percent))
                    .font(.system(size: window.expired(at: date) ? 20 : 36, weight: .semibold, design: .rounded))
                    .monospacedDigit().foregroundStyle(tint(window, account: account)).lineLimit(1).minimumScaleFactor(0.8)
            } else { nonQuota(account, large: true) }
            Spacer(minLength: 0)
            updated(account)
        }
    }

    private func medium(_ account: UsageWidgetAccount) -> some View {
        VStack(alignment: .leading, spacing: 9) {
            HStack {
                Label("gtmux", systemImage: "square.split.2x2").font(.caption.weight(.semibold))
                Spacer(minLength: 4)
                Text(snapshot.server).font(.caption).foregroundStyle(.secondary).lineLimit(1)
            }
            if provider.isEmpty && accounts.count > 1 {
                HStack(alignment: .top, spacing: 16) {
                    ForEach(Array(accounts.prefix(2))) { item in
                        VStack(alignment: .leading, spacing: 7) {
                            heading(item)
                            if let window = item.windows.first { quota(window, account: item, large: true) }
                            else { nonQuota(item, large: true) }
                        }.frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            } else {
                HStack(alignment: .top, spacing: 16) {
                    VStack(alignment: .leading, spacing: 8) {
                        heading(account)
                        if let window = account.windows.first { quota(window, account: account, large: true) }
                        else { nonQuota(account, large: true) }
                    }.frame(maxWidth: .infinity, alignment: .leading)
                    if account.windows.count > 1 {
                        VStack(alignment: .leading, spacing: 5) {
                            ForEach(Array(account.windows.dropFirst().prefix(2).enumerated()), id: \.offset) { _, window in
                                quota(window, account: account, large: false)
                            }
                        }.frame(maxWidth: .infinity, alignment: .leading)
                    } else if account.wallets.count > 1 {
                        VStack(alignment: .leading, spacing: 6) {
                            ForEach(Array(account.wallets.dropFirst().prefix(2).enumerated()), id: \.offset) { _, wallet in
                                Text(UsageWidgetWords.wallet(wallet, chinese: chinese)).font(.caption2).foregroundStyle(.secondary)
                                Text(wallet.currency + " " + wallet.amount).font(.subheadline.weight(.semibold)).lineLimit(1)
                            }
                        }.frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
            Spacer(minLength: 0)
            HStack {
                updated(account)
                Spacer(minLength: 4)
                if provider.isEmpty && accounts.count > 2 {
                    Text(tr("+\(accounts.count - 2) more", "另有 \(accounts.count - 2) 项"))
                        .font(.caption2).foregroundStyle(.secondary)
                }
            }
        }
    }

    private func quota(_ window: UsageWidgetWindow, account: UsageWidgetAccount, large: Bool) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(UsageWidgetWords.window(window, agent: account.agent, chinese: chinese))
                .font(.caption2).foregroundStyle(.secondary).lineLimit(1)
            if window.expired(at: date) {
                Text(tr("Update needed", "等待更新")).font(.subheadline.weight(.semibold)).lineLimit(1)
            } else {
                Text(UsageWidgetWords.percent(window.percent))
                    .font(large ? .system(size: 30, weight: .semibold, design: .rounded) : .subheadline.weight(.semibold))
                    .monospacedDigit().foregroundStyle(tint(window, account: account)).lineLimit(1).minimumScaleFactor(0.8)
                GeometryReader { geometry in
                    ZStack(alignment: .leading) {
                        Capsule().fill(Color.secondary.opacity(0.18))
                        Capsule().fill(tint(window, account: account))
                            .frame(width: geometry.size.width * min(1, max(0, window.percent / 100)))
                    }
                }.frame(height: 4).accessibilityHidden(true)
            }
            reset(window)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(account.name + ", " + UsageWidgetWords.window(window, agent: account.agent, chinese: chinese) + ", " +
            (window.expired(at: date) ? tr("Update needed", "等待更新") : UsageWidgetWords.percent(window.percent) + tr(" used", " 已用")))
    }

    @ViewBuilder private func reset(_ window: UsageWidgetWindow) -> some View {
        if window.expired(at: date) {
            Text(tr("Open gtmux to sync", "打开 gtmux 同步")).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
        } else if let reset = window.resetAt {
            HStack(spacing: 3) {
                Text(tr("Resets", "重置")).font(.caption2)
                readingTime(reset).font(.caption2).monospacedDigit()
            }.foregroundStyle(.secondary).lineLimit(1)
        } else {
            Text(tr("Used quota", "已用额度")).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
        }
    }

    @ViewBuilder private func nonQuota(_ account: UsageWidgetAccount, large: Bool) -> some View {
        if !account.wallets.isEmpty { balance(account, large: large) }
        else if let output = account.output { recorded(output, account: account, large: large) }
    }

    private func recorded(_ output: UsageWidgetOutput, account: UsageWidgetAccount, large: Bool) -> some View {
        let day = UsageWidgetWords.outputDay(output, at: date, chinese: chinese)
        return VStack(alignment: .leading, spacing: 4) {
            Text(day + tr(" output", "输出")).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
            HStack(alignment: .bottom, spacing: 4) {
                Text(UsageWidgetWords.count(output.today))
                    .font(large ? .system(size: 30, weight: .semibold, design: .rounded) : .headline)
                    .monospacedDigit().lineLimit(1).minimumScaleFactor(0.8)
                VStack(alignment: .leading, spacing: 0) {
                    Text("Token")
                    Text(tr("Local records", "本机记录"))
                }.font(.caption2).foregroundStyle(.secondary).lineLimit(1).minimumScaleFactor(0.8)
            }
            Text(tr("7 days ", "近 7 天 ") + UsageWidgetWords.count(output.week) + " Token")
                .font(.caption2).foregroundStyle(.secondary).lineLimit(1).minimumScaleFactor(0.8)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(account.name + ", " + day + tr(" output ", "输出 ") + String(output.today) +
            tr(" tokens. Past 7 days ", " Token，近 7 天 ") + String(output.week) + tr(" tokens. Local records.", " Token。本机记录。"))
    }

    private func balance(_ account: UsageWidgetAccount, large: Bool) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            if let wallet = account.wallets.first {
                Text(UsageWidgetWords.wallet(wallet, chinese: chinese)).font(.caption2).foregroundStyle(.secondary)
                Text(wallet.currency + " " + wallet.amount)
                    .font(large ? .title3.weight(.semibold) : .subheadline.weight(.semibold))
                    .monospacedDigit().lineLimit(1).minimumScaleFactor(0.8)
                if old(account) { Text(tr("Last reading", "上次读数")).font(.caption2).foregroundStyle(.secondary) }
            }
        }
    }

    private func updated(_ account: UsageWidgetAccount) -> some View {
        let observed = account.observedAt ?? account.windows.compactMap(\.observedAt).min() ?? snapshot.capturedAt
        let stale = old(account) || account.windows.contains { $0.stale(at: date) || $0.expired(at: date) }
        return HStack(spacing: 3) {
            if stale { Image(systemName: "clock").font(.caption2) }
            Text(stale ? tr("Last reading", "上次读数") : tr("Updated", "更新于"))
            readingTime(observed).monospacedDigit()
        }.font(.caption2).foregroundStyle(.secondary).lineLimit(1)
    }

    private func readingTime(_ seconds: TimeInterval) -> Text {
        let reading = Date(timeIntervalSince1970: seconds)
        let sameDay = Calendar.current.isDate(reading, inSameDayAs: date)
        let locale = Locale(identifier: chinese ? "zh-Hans" : "en")
        let format = Date.FormatStyle(date: sameDay ? .omitted : .abbreviated, time: .shortened).locale(locale)
        return Text(reading, format: format)
    }

    #if os(iOS)
    private func circle(_ account: UsageWidgetAccount) -> some View {
        Group {
            if let window = account.windows.first, !window.expired(at: date), !window.stale(at: date), !old(account) {
                Gauge(value: min(100, max(0, window.percent)), in: 0...100) {
                    Text(account.name).lineLimit(1).minimumScaleFactor(0.7)
                } currentValueLabel: { Text(UsageWidgetWords.percent(window.percent)).font(.caption.weight(.semibold)) }
                .gaugeStyle(.accessoryCircular).tint(tint(window, account: account))
            } else if let output = account.output, !old(account) {
                VStack(spacing: 2) {
                    Text(UsageWidgetWords.count(output.today)).font(.headline).monospacedDigit()
                    Text(tr("Local output", "本机输出")).font(.caption2).lineLimit(1).minimumScaleFactor(0.8)
                }
            } else {
                VStack(spacing: 3) {
                    Image(systemName: account.windows.isEmpty && !old(account) ? "creditcard" : "clock.arrow.circlepath")
                    Text(account.name).font(.caption2).lineLimit(1)
                }
            }
        }.accessibilityLabel(account.name + ", " + summary(account))
    }

    private func inline(_ account: UsageWidgetAccount) -> some View {
        Text(account.name + " · " + summary(account))
    }

    private func accessory(_ account: UsageWidgetAccount) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(account.name).font(.headline).lineLimit(1)
            Text(summary(account)).font(.caption).lineLimit(1)
            updated(account)
        }
    }

    private func summary(_ account: UsageWidgetAccount) -> String {
        if let window = account.windows.first {
            return window.expired(at: date) ? tr("Update needed", "等待更新") :
                (old(account) || window.stale(at: date) ? tr("Last reading ", "上次读数 ") : UsageWidgetWords.window(window, agent: account.agent, chinese: chinese) + " ") + UsageWidgetWords.percent(window.percent)
        }
        if let wallet = account.wallets.first {
            return (old(account) ? tr("Last reading ", "上次读数 ") : "") + wallet.currency + " " + wallet.amount
        }
        if let output = account.output {
            return (old(account) ? tr("Last reading · ", "上次读数 · ") : "") + tr("Local output · ", "本机输出 · ") +
                UsageWidgetWords.outputDay(output, at: date, chinese: chinese) + " " +
                UsageWidgetWords.count(output.today) + tr(" tokens", " Token")
        }
        return tr("No reading", "暂无读数")
    }
    #endif
}

@available(iOS 16.0, macOS 14.0, *)
private struct UsageWidgetAgentMark: View {
    let agent: String
    private var mark: Image? {
        let name = "UsageAgent-" + agent
        #if SWIFT_PACKAGE
        let bundle = Bundle.module
        #else
        let bundle = Bundle.main
        #endif
        #if os(macOS)
        let icon = bundle.url(forResource: name, withExtension: "png").flatMap { NSImage(contentsOf: $0) }
            ?? bundle.image(forResource: NSImage.Name(name))
        return icon.map { Image(nsImage: $0) }
        #else
        guard UIImage(named: name, in: bundle, compatibleWith: nil) != nil else { return nil }
        return Image(name, bundle: bundle)
        #endif
    }
    var body: some View {
        // Resource presence selects the official mark. Custom/new providers can
        // add an asset without adding another provider list to the view code.
        Group {
            (mark ?? Image(systemName: "terminal")).resizable().scaledToFit().padding(2)
        }.frame(width: 18, height: 18).accessibilityHidden(true)
    }
}
