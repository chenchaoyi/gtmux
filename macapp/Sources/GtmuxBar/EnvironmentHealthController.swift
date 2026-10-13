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
    @State private var showClasses = false
    @State private var focusedRow: EnvironmentReport.RowID?
    @State private var focusRevision = 0
    private let findingsAnchor = "doctor-findings-summary"

    var body: some View {
        ScrollViewReader { proxy in
            VStack(spacing: 0) {
                HStack(spacing: 10) {
                    if model.checking { ProgressView().controlSize(.small) }
                    VStack(alignment: .leading, spacing: 4) {
                        Text(model.checking ? l10n.tr("Checking environment…", "正在检查运行环境…") : model.failed ? l10n.tr("Check did not finish", "检查未完成") : model.report?.summary(lang: l10n.lang) ?? l10n.tr("Environment check", "运行环境检查"))
                            .font(Theme.Font.session).fixedSize(horizontal: false, vertical: true)
                        if let report = model.report {
                            Text(l10n.tr("Checked ", "检查时间：") + report.checkedTime(lang: l10n.lang) + " · gtmux " + report.version)
                                .font(Theme.Font.task).foregroundStyle(.secondary)
                        }
                    }
                    Spacer(minLength: 0)
                    Button { showClasses.toggle() } label: {
                        Image(systemName: "info.circle").font(.system(size: 14))
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(l10n.tr("Check result classes", "检查结果分类"))
                    .help(l10n.tr("Check result classes", "检查结果分类"))
                    .popover(isPresented: $showClasses) { EnvironmentResultClasses(l10n: l10n) }
                    if let report = model.report, report.findings > 0 {
                        Button(l10n.tr("Review \(report.findings)", "查看 \(report.findings) 项")) {
                            withAnimation { proxy.scrollTo(findingsAnchor, anchor: .top) }
                        }.buttonStyle(.link)
                    }
                    Button(l10n.tr("Check again", "重新检查")) { model.check() }.disabled(model.checking)
                }.padding(.horizontal, 16).padding(.vertical, 12).frame(maxWidth: 760)
                if model.failed {
                    Text(l10n.tr("Retry the check; any report below is the last successful result.", "请重试检查；下方如有报告，是上次成功检查的结果。"))
                        .font(Theme.Font.task).foregroundStyle(.secondary).padding(.horizontal, 20).padding(.bottom, 12)
                }
                Divider()
                ScrollView {
                    if let report = model.report {
                        VStack(alignment: .leading, spacing: 16) {
                            ForEach(report.displaySections) { section in
                                EnvironmentHealthSection(section: section, l10n: l10n, focusedRow: focusedRow, focusRevision: focusRevision)
                            }
                            if !report.attentionItems.isEmpty {
                                EnvironmentFindingsSummary(findings: report.attentionItems, l10n: l10n) { target in
                                    // A repeated selection reopens manually collapsed details.
                                    focusRevision += 1
                                    focusedRow = target
                                    withAnimation { proxy.scrollTo(target, anchor: .center) }
                                }.id(findingsAnchor)
                            }
                        }
                        .padding(.horizontal, 16).padding(.vertical, 12)
                        .frame(maxWidth: 760).frame(maxWidth: .infinity)
                    }
                }
                Divider()
                HStack(alignment: .center, spacing: 16) {
                    Text(model.repairFailed ? l10n.tr("Could not open the terminal. Run gtmux doctor --fix manually.", "无法打开终端，请手动运行 gtmux doctor --fix。") : model.repairOpened ? l10n.tr("Complete the steps in the terminal, then check again.", "请在终端完成修复，然后重新检查。") : l10n.tr("Review available repairs in the terminal; some findings need a separate action.", "在终端查看可修复项；部分事项需另行处理。"))
                        .font(Theme.Font.task).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    Spacer(minLength: 0)
                    if let report = model.report, report.findings > 0 {
                        Button(l10n.tr("Findings (\(report.findings))", "需关注（\(report.findings)）")) {
                            withAnimation { proxy.scrollTo(findingsAnchor, anchor: .top) }
                        }.buttonStyle(.link)
                    }
                    Button(l10n.tr("Review repairs…", "查看修复方案…")) { model.repair() }
                        .disabled(model.checking || model.repairOpening || model.report?.findings == 0)
                }.padding(.horizontal, 16).padding(.vertical, 12).frame(maxWidth: 760)
            }
            .font(Theme.Font.task).frame(maxWidth: .infinity)
            .onChange(of: model.report) { _, _ in focusedRow = nil }
        }
    }
}
