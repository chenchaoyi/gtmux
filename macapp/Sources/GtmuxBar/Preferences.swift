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
    @ObservedObject var remote = RemoteAccess.shared

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

            GridRow {
                label(l10n.tr("Remote access", "远程访问"))
                Toggle(isOn: Binding(
                    get: { remote.isOn },
                    set: { want in want ? confirmEnable() : remote.disable() })) {
                    Text(remote.isOn
                        ? (remote.url ?? l10n.tr("on — reachable from anywhere", "已开启，任意网络可达"))
                        : l10n.tr("Keep reachable from anywhere (always-on)", "保持任意网络可达（常驻）"))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                }
                .toggleStyle(.switch).disabled(remote.busy).gridColumnAlignment(.leading)
            }
        }
        .padding(24)
        .frame(width: 460, alignment: .topLeading)
        .onAppear { remote.refresh() }
    }

    private func label(_ text: String) -> some View {
        Text(text).font(.system(size: 12)).foregroundStyle(.secondary)
    }

    // confirmEnable shows the standing-exposure warning before turning always-on
    // on (the CLI's own prompt is skipped via --yes since we confirm here).
    private func confirmEnable() {
        let a = NSAlert()
        a.messageText = l10n.tr("Keep remote access on?", "保持远程访问开启？")
        a.informativeText = l10n.tr(
            "Your Mac stays reachable at a public URL (token-gated) across reboots until you turn this off. It's a standing exposure — enable it consciously.",
            "开启后，你的 Mac 会一直在一个公网地址可达（有 token 把关），重启也不会停，直到你手动关闭。这是个长期敞口，请想清楚再开。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enable()
        } else {
            remote.objectWillChange.send() // snap the toggle back to off
        }
    }
}
