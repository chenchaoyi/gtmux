import AppKit
import SwiftUI

final class EnvironmentHealthController: NSObject, NSWindowDelegate {
    static let shared = EnvironmentHealthController()
    private var window: NSWindow?
    func show(l10n: L10n) {
        if let window { NSApp.activate(ignoringOtherApps: true); window.makeKeyAndOrderFront(nil); return }
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 660, height: 520),
                         styleMask: [.titled, .closable, .resizable, .miniaturizable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false; w.minSize = NSSize(width: 500, height: 380)
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
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 10) {
                if model.checking { ProgressView().controlSize(.small) }
                VStack(alignment: .leading, spacing: 4) {
                    Text(model.checking ? l10n.tr("Checking environment…", "正在检查运行环境…") : model.failed ? l10n.tr("Check did not finish", "检查未完成") : l10n.tr("Environment check", "运行环境检查")).font(.headline)
                    if let report = model.report {
                        Text(l10n.tr("\(report.findings) items to review · gtmux \(report.version)", "\(report.findings) 项待检查 · gtmux \(report.version)"))
                            .font(.subheadline).foregroundStyle(.secondary)
                        Text(Date(timeIntervalSince1970: TimeInterval(report.checkedAt)), style: .date).font(.caption).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                Button(l10n.tr("Check again", "重新检查")) { model.check() }.disabled(model.checking)
            }.padding(20)
            if model.failed {
                Text(l10n.tr("Retry the check; any report below is the last successful result.", "请重试检查；下方如有报告，是上次成功检查的结果。"))
                    .font(.callout).foregroundStyle(.secondary).padding(.horizontal, 20).padding(.bottom, 12)
            }
            Divider()
            List {
                if let report = model.report {
                    ForEach(Array(report.sections.enumerated()), id: \.offset) { _, section in
                        Section(header: Text(section.title)) {
                            ForEach(Array(section.rows.enumerated()), id: \.offset) { _, row in
                                HStack(alignment: .top, spacing: 10) {
                                    Image(systemName: symbol(row.status)).foregroundStyle(color(row.status))
                                        .frame(width: 18).accessibilityLabel(statusLabel(row.status))
                                    VStack(alignment: .leading, spacing: 4) {
                                        HStack(alignment: .firstTextBaseline) {
                                            Text(row.label).fontWeight(.medium)
                                            Spacer(minLength: 12)
                                            Text(row.value).foregroundStyle(.secondary).multilineTextAlignment(.trailing)
                                        }
                                        if !row.note.isEmpty { Text(row.note).font(.caption).foregroundStyle(.secondary).textSelection(.enabled) }
                                    }
                                }.padding(.vertical, 4)
                            }
                        }
                    }
                }
            }.listStyle(.inset)
            Divider()
            HStack(alignment: .center, spacing: 16) {
                Text(model.repairFailed ? l10n.tr("Could not open the terminal. Run gtmux doctor --fix manually.", "无法打开终端，请手动运行 gtmux doctor --fix。") : model.repairOpened ? l10n.tr("Complete the steps in the terminal, then check again.", "请在终端完成修复，然后重新检查。") : l10n.tr("Review each repair in the terminal. Some findings require manual action.", "在终端逐项确认修复；部分检查项需要手动处理。"))
                    .font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
                Button(l10n.tr("Review repairs…", "查看修复方案…")) { model.repair() }
                    .disabled(model.checking || model.repairOpening || model.report?.findings == 0)
            }.padding(16)
        }
    }
    private func symbol(_ status: String) -> String { switch status { case "ok": return "checkmark.circle"; case "blocking": return "xmark.octagon"; case "recommended": return "exclamationmark.triangle"; default: return "info.circle" } }
    private func color(_ status: String) -> Color { switch status { case "ok": return Theme.Status.idle; case "blocking": return Theme.Status.waiting; case "recommended": return .orange; default: return .secondary } }
    private func statusLabel(_ status: String) -> String { switch status { case "ok": return l10n.tr("OK", "正常"); case "blocking": return l10n.tr("Blocking", "阻塞"); case "recommended": return l10n.tr("Recommended", "建议处理"); default: return l10n.tr("Information", "提示") } }
}
