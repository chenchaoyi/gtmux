import AppKit
import SwiftUI

/// PreferencesController owns the Preferences window (DESIGN §8).
final class PreferencesController {
    static let shared = PreferencesController()
    private var window: NSWindow?

    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 460, height: 380),
                styleMask: [.titled, .closable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(
                rootView: PreferencesView(l10n: l10n, settings: AppSettings.shared))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
        }
        window?.title = l10n.tr("gtmux Preferences", "gtmux 偏好设置")
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

/// PreferencesView — a standard macOS settings grid (labels right, controls left).
struct PreferencesView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var settings: AppSettings

    var body: some View {
        Grid(alignment: .trailing, horizontalSpacing: 14, verticalSpacing: 18) {
            GridRow {
                label(l10n.tr("Language", "语言 Language"))
                Picker("", selection: $l10n.mode) {
                    Text(l10n.tr("System", "跟随系统")).tag(LangMode.system)
                    Text("English").tag(LangMode.en)
                    Text("中文").tag(LangMode.zh)
                }
                .pickerStyle(.segmented).labelsHidden()
                .frame(width: 260).gridColumnAlignment(.leading)
            }

            GridRow {
                label(l10n.tr("Refresh", "刷新间隔"))
                HStack {
                    Slider(value: $settings.refreshInterval, in: 0.5...5.0, step: 0.5).frame(width: 200)
                    Text(String(format: "%.1fs", settings.refreshInterval))
                        .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                }.gridColumnAlignment(.leading)
            }

            GridRow {
                label(l10n.tr("Launch at login", "开机自启"))
                Toggle(isOn: $settings.launchAtLogin) {
                    Text(l10n.tr("Start gtmux at login", "登录时启动 gtmux"))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                }.toggleStyle(.switch).gridColumnAlignment(.leading)
            }

            GridRow {
                label(l10n.tr("Status bar", "状态栏显示"))
                Picker("", selection: $settings.displayMode) {
                    Text(l10n.tr("Dot + count", "点 + 数字")).tag(DisplayMode.dotCount)
                    Text(l10n.tr("Dot only", "仅圆点")).tag(DisplayMode.dot)
                    Text(l10n.tr("Hide when idle", "空闲时隐藏")).tag(DisplayMode.hideWhenIdle)
                }
                .pickerStyle(.segmented).labelsHidden()
                .frame(width: 300).gridColumnAlignment(.leading)
            }

            GridRow {
                label(l10n.tr("Global hotkey", "全局热键"))
                HStack(spacing: 8) {
                    Text("⌘⌥G").font(.system(size: 12, weight: .medium))
                        .padding(.horizontal, 10).padding(.vertical, 4)
                        .background(RoundedRectangle(cornerRadius: 6).stroke(.secondary.opacity(0.4)))
                    Text(l10n.tr("opens the popover", "打开 popover"))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                }.gridColumnAlignment(.leading)
            }

            GridRow {
                label(l10n.tr("Notifications", "通知"))
                Toggle(isOn: $settings.notifications) {
                    Text(l10n.tr("Notify when an agent waits / finishes", "agent 开始等你 / 完成时提醒"))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                }.toggleStyle(.switch).gridColumnAlignment(.leading)
            }
        }
        .padding(24)
        .frame(width: 460, alignment: .topLeading)
    }

    private func label(_ text: String) -> some View {
        Text(text).font(.system(size: 12)).foregroundStyle(.secondary)
    }
}
