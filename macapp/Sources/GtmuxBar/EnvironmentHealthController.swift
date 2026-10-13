import AppKit
import SwiftUI

final class EnvironmentHealthController: NSObject, NSWindowDelegate {
    static let shared = EnvironmentHealthController()
    private var window: NSWindow?
    func show(l10n: L10n) {
        if let window { NSApp.activate(ignoringOtherApps: true); window.makeKeyAndOrderFront(nil); return }
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 720, height: 600),
                         styleMask: [.titled, .closable, .resizable, .miniaturizable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false; w.minSize = NSSize(width: 560, height: 420)
        w.title = l10n.tr("Environment check", "运行环境检查")
        w.contentView = NSHostingView(rootView: EnvironmentHealthView(model: .shared, l10n: l10n))
        w.delegate = self; window = w; w.center()
        NSApp.activate(ignoringOtherApps: true); w.makeKeyAndOrderFront(nil)
        if EnvironmentHealth.shared.report == nil && !EnvironmentHealth.shared.failed { EnvironmentHealth.shared.check() }
    }
    func windowWillClose(_ notification: Notification) { window = nil }
}

struct EnvironmentHealthView: View {
    @ObservedObject var model: EnvironmentHealth
    @ObservedObject var l10n: L10n
    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                if model.checking { ProgressView().controlSize(.small) }
                VStack(alignment: .leading, spacing: 4) {
                    Text(model.checking ? l10n.tr("Checking environment…", "正在检查运行环境…") : model.failed ? l10n.tr("Check did not finish", "检查未完成") : l10n.tr("Environment check", "运行环境检查")).font(Theme.Font.title)
                    if let report = model.report {
                        Text(report.summary(lang: l10n.lang)).font(Theme.Font.task).foregroundStyle(.secondary)
                        Text(l10n.tr("Checked ", "检查时间：") + report.checkedTime(lang: l10n.lang) + " · gtmux " + report.version)
                            .font(Theme.Font.task).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                Button(l10n.tr("Check again", "重新检查")) { model.check() }.disabled(model.checking)
            }.padding(20).frame(maxWidth: 760)
            if model.failed {
                Text(l10n.tr("Retry the check; any report below is the last successful result.", "请重试检查；下方如有报告，是上次成功检查的结果。"))
                    .font(Theme.Font.task).foregroundStyle(.secondary).padding(.horizontal, 20).padding(.bottom, 12)
            }
            Divider()
            ScrollView {
                if let report = model.report {
                    LazyVStack(alignment: .leading, spacing: 24) {
                        ForEach(report.displaySections) { section in
                            EnvironmentHealthSection(section: section, l10n: l10n)
                        }
                    }
                    .padding(20)
                    .frame(maxWidth: 760)
                    .frame(maxWidth: .infinity)
                }
            }
            Divider()
            HStack(alignment: .center, spacing: 16) {
                Text(model.repairFailed ? l10n.tr("Could not open the terminal. Run gtmux doctor --fix manually.", "无法打开终端，请手动运行 gtmux doctor --fix。") : model.repairOpened ? l10n.tr("Complete the steps in the terminal, then check again.", "请在终端完成修复，然后重新检查。") : l10n.tr("Review each repair in the terminal. Some findings require manual action.", "在终端逐项确认修复；部分检查项需要手动处理。"))
                    .font(Theme.Font.task).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
                Button(l10n.tr("Review repairs…", "查看修复方案…")) { model.repair() }
                    .disabled(model.checking || model.repairOpening || model.report?.findings == 0)
            }.padding(20).frame(maxWidth: 760)
        }
        .font(Theme.Font.task)
        .frame(maxWidth: .infinity)
    }
}
