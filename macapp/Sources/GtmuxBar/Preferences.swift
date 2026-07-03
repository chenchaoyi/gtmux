import AppKit
import SwiftUI

/// PreferencesController owns the Preferences window (DESIGN §8).
final class PreferencesController {
    static let shared = PreferencesController()
    private var window: NSWindow?

    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 460, height: 560),
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

/// PreferencesView — a grouped macOS settings Form (sectioned cards, the native
/// System-Settings idiom), so preferences read at a glance like Moshi's settings.
struct PreferencesView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var settings: AppSettings
    @ObservedObject var remote = RemoteAccess.shared
    @ObservedObject var ent = Entitlements.shared
    @ObservedObject var updater = Updater.shared
    @State private var showPaywall = false

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "dev"
    }

    // A grouped Form (macOS System-Settings idiom) — sectioned cards instead of a
    // flat grid, so the preferences read at a glance like Moshi's settings.
    var body: some View {
        Form {
            Section(l10n.tr("General", "通用")) {
                Picker(l10n.tr("Language", "语言"), selection: $l10n.mode) {
                    Text(l10n.tr("System", "跟随系统")).tag(LangMode.system)
                    Text("English").tag(LangMode.en)
                    Text("中文").tag(LangMode.zh)
                }
                Toggle(l10n.tr("Launch at login", "开机自启"), isOn: $settings.launchAtLogin)
                LabeledContent(l10n.tr("Global hotkey", "全局热键")) {
                    HStack(spacing: 8) {
                        Text("⌘⌥G").font(.system(size: 12, weight: .medium))
                            .padding(.horizontal, 8).padding(.vertical, 3)
                            .background(RoundedRectangle(cornerRadius: 6).stroke(.secondary.opacity(0.4)))
                        Text(l10n.tr("opens the popover", "打开 popover"))
                            .font(.system(size: 11)).foregroundStyle(.secondary)
                    }
                }
            }

            Section(l10n.tr("Status bar", "状态栏")) {
                Picker(l10n.tr("Display", "显示"), selection: $settings.displayMode) {
                    Text(l10n.tr("Dot + count", "点 + 数字")).tag(DisplayMode.dotCount)
                    Text(l10n.tr("Dot only", "仅圆点")).tag(DisplayMode.dot)
                    Text(l10n.tr("Hide when idle", "空闲时隐藏")).tag(DisplayMode.hideWhenIdle)
                }
                LabeledContent(l10n.tr("Refresh", "刷新间隔")) {
                    HStack {
                        Slider(value: $settings.refreshInterval, in: 0.5...5.0, step: 0.5).frame(width: 170)
                        Text(String(format: "%.1fs", settings.refreshInterval))
                            .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                    }
                }
            }

            Section(l10n.tr("Notifications", "通知")) {
                Toggle(isOn: $settings.notifications) {
                    Text(l10n.tr("Notify when an agent waits / finishes", "agent 开始等你 / 完成时提醒"))
                }
            }

            Section(l10n.tr("Remote access", "远程访问")) {
                // Merged Off / Wi-Fi (free LAN) / Anywhere (Pro tunnel) control.
                Picker("", selection: remoteModeBinding) {
                    Text(l10n.tr("Off", "关闭")).tag(RemoteMode.off)
                    Text(l10n.tr("Wi-Fi", "局域网")).tag(RemoteMode.lan)
                    Text(l10n.tr("Anywhere", "任意网络")).tag(RemoteMode.anywhere)
                }
                .pickerStyle(.segmented).labelsHidden().disabled(remote.busy)
                Text(remoteSubtitle)
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                connectedDevices
            }

            Section(l10n.tr("Software update", "软件更新")) {
                LabeledContent(l10n.tr("Current version", "当前版本")) {
                    Text(appVersion).font(.system(size: 12, design: .monospaced)).foregroundStyle(.secondary)
                }
                updateRow
            }
        }
        .formStyle(.grouped)
        .frame(width: 460, height: 640)
        .onAppear { remote.refresh(); updater.autoCheck() }
        .sheet(isPresented: $showPaywall) {
            PaywallView(l10n: l10n,
                        onUnlock: { ent.unlockFree(); showPaywall = false; confirmAnywhere() },
                        onClose: { showPaywall = false })
        }
    }

    // WHO is connected right now — paired phones by name, browsers as anonymous
    // "Safari · macOS" rows. Hidden entirely when nobody's viewing, so the section
    // stays quiet at rest (matches the "idle 静" ethos). A phone icon vs a globe
    // reuses the phone/browser distinction from the popover indicator.
    @ViewBuilder private var connectedDevices: some View {
        let list = remote.remoteClientList
        if !list.isEmpty {
            Divider()
            VStack(alignment: .leading, spacing: 6) {
                Text(l10n.tr("Connected now", "当前已连接"))
                    .font(.system(size: 11, weight: .medium)).foregroundStyle(.secondary)
                ForEach(list) { c in
                    HStack(spacing: 8) {
                        Image(systemName: c.isPhone ? "iphone" : "globe")
                            .font(.system(size: 12)).foregroundStyle(Theme.Status.idle)
                            .frame(width: 16)
                        Text(c.title(l10n.tr)).font(.system(size: 12))
                        let dur = c.connectedFor(l10n.tr)
                        if !dur.isEmpty {
                            Text(dur).font(.system(size: 10)).foregroundStyle(.tertiary)
                        }
                        Spacer(minLength: 0)
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // Explicit check-for-updates row (reuses Updater — same effect as `gtmux
    // update`): a Check button, or a "new version available → Update now" row, or a
    // progress line while updating.
    @ViewBuilder private var updateRow: some View {
        switch updater.state {
        case .available(let v):
            HStack(spacing: 8) {
                Image(systemName: "arrow.down.circle.fill").foregroundStyle(Theme.Status.working)
                Text(l10n.tr("New version \(v) available", "有新版本 \(v)")).font(.system(size: 12))
                Spacer()
                Button(l10n.tr("Update now", "立即更新")) { updater.install() }
                    .buttonStyle(.borderedProminent)
            }
        case .updating:
            HStack(spacing: 8) {
                ProgressView().controlSize(.small)
                Text(l10n.tr("Updating… the app will relaunch when done", "正在更新…完成后会自动重启"))
                    .font(.system(size: 12)).foregroundStyle(.secondary)
            }
        default:
            HStack(spacing: 8) {
                Button(l10n.tr("Check for updates", "检查更新")) { updater.check() }
                    .disabled(isChecking)
                if isChecking { ProgressView().controlSize(.small) }
                Spacer()
                if let s = checkStatusText {
                    Text(s).font(.system(size: 11)).foregroundStyle(.secondary)
                }
            }
        }
    }

    private var isChecking: Bool { if case .checking = updater.state { return true }; return false }
    private var checkStatusText: String? {
        switch updater.state {
        case .upToDate: return l10n.tr("Up to date", "已是最新")
        case .failed: return l10n.tr("Check failed — try again", "检查失败，请重试")
        default: return nil
        }
    }

    private var remoteModeBinding: Binding<RemoteMode> {
        Binding(
            get: { remote.mode },
            set: { m in
                switch m {
                case .off: remote.turnOff()
                case .lan: remote.enableLan()
                case .anywhere: ent.isPro ? confirmAnywhere() : (showPaywall = true)
                }
            })
    }

    private var remoteSubtitle: String {
        switch remote.mode {
        case .off:
            return ent.isPro
                ? l10n.tr("Phone access is off.", "手机访问已关闭。")
                : l10n.tr("Off. “Anywhere” is a Pro feature.", "已关闭。“任意网络”为 Pro 功能。")
        case .lan:
            return l10n.tr("Reachable on the same Wi-Fi.", "同一 Wi-Fi 下可达。")
        case .anywhere:
            return remote.url ?? l10n.tr("Reachable from anywhere (always-on).", "任意网络可达（常驻）。")
        }
    }

    // confirmAnywhere shows the standing-exposure warning before enabling the
    // always-on tunnel (the CLI's own prompt is skipped via --yes since we confirm
    // here). Reached only when Pro is unlocked.
    private func confirmAnywhere() {
        let a = NSAlert()
        a.messageText = l10n.tr("Keep Anywhere access on?", "保持任意网络访问开启？")
        a.informativeText = l10n.tr(
            "Your Mac stays reachable at a public URL (token-gated) across reboots until you turn this off. It's a standing exposure — enable it consciously.",
            "开启后，你的 Mac 会一直在一个公网地址可达（有 token 把关），重启也不会停，直到你手动关闭。这是个长期敞口，请想清楚再开。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enableAnywhere()
        } else {
            remote.objectWillChange.send() // snap the picker back
        }
    }
}
