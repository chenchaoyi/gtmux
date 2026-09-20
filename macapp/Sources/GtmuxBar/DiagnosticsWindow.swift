import AppKit
import SwiftUI

/// The window behind Preferences › Diagnostics › Open.
///
/// Before this, the only way to see what gtmux had recorded was to know the command and
/// have a terminal open — which is exactly the moment a person does not have, since the
/// reason they are looking is that something is already wrong. It shows the last three
/// days, newest first, with one switch: everything, or only what went wrong.
///
/// The entries themselves stay English. The store writes them that way on purpose (they
/// are what a bug report quotes), so the chrome is translated and the lines are not.
final class DiagnosticsController {
    static let shared = DiagnosticsController()
    private var window: NSWindow?

    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 620, height: 520),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(rootView: DiagnosticsView(l10n: l10n))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
        }
        window?.title = l10n.tr("What gtmux recorded", "gtmux 记下了什么")
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

struct DiagnosticsView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject private var diag = DiagnosticsStore.shared
    @State private var problemsOnly = false
    @State private var copied = false

    private var shown: [LogEntry] { diag.entries }

    /// Day headings, in the order the entries come in (newest first).
    private var days: [String] {
        var seen: [String] = []
        for e in shown where !seen.contains(e.day) { seen.append(e.day) }
        return seen
    }

    private func dayTitle(_ day: String) -> String {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        guard let d = f.date(from: day) else { return day }
        let cal = Calendar.current
        if cal.isDateInToday(d) { return l10n.tr("Today", "今天") }
        if cal.isDateInYesterday(d) { return l10n.tr("Yesterday", "昨天") }
        return day
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            Divider()
            if diag.loading && shown.isEmpty {
                Spacer()
                HStack { Spacer(); ProgressView().controlSize(.small); Spacer() }
                Spacer()
            } else if shown.isEmpty {
                empty
            } else {
                list
            }
            Divider()
            footer
        }
        .frame(minWidth: 520, minHeight: 380)
        .onAppear {
            diag.refreshStats()
            diag.load(problemsOnly: problemsOnly)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(summary)
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Picker("", selection: $problemsOnly) {
                Text(l10n.tr("Everything", "全部")).tag(false)
                Text(l10n.tr("Problems only", "只看出问题的")).tag(true)
            }
            .pickerStyle(.segmented).labelsHidden().frame(width: 260)
            .onChange(of: problemsOnly) { v in diag.load(problemsOnly: v) }
        }
        .padding(.horizontal, 16).padding(.top, 14).padding(.bottom, 10)
    }

    /// What is kept, for how long, and that it is not going anywhere. The three days on
    /// screen are said out loud too: a window that silently shows a slice of a 30-day
    /// store is a window that lies about an empty list.
    private var summary: String {
        let s = diag.stats
        let size = ByteCountFormatter.string(fromByteCount: s.bytes, countStyle: .file)
        return l10n.tr(
            "The last three days, newest first. The store holds \(size) and keeps \(s.retainDays) days; it stays on this Mac until you pack a report or copy from here.",
            "最近三天，新的在上面。日志库现在 \(size)，保留 \(s.retainDays) 天；除非你打包问题报告或者从这里拷走，它一直留在这台 Mac 上。")
    }

    private var empty: some View {
        VStack(spacing: 6) {
            Spacer()
            Text(problemsOnly
                 ? l10n.tr("Nothing went wrong in the last three days.", "最近三天没出什么问题。")
                 : l10n.tr("Nothing recorded in the last three days.", "最近三天没有记录。"))
                .font(.system(size: 12)).foregroundStyle(.secondary)
            if problemsOnly {
                Button(l10n.tr("Show everything", "看全部")) { problemsOnly = false }
                    .buttonStyle(.link).font(.system(size: 11))
            }
            Spacer()
        }
        .frame(maxWidth: .infinity)
    }

    private var list: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0, pinnedViews: [.sectionHeaders]) {
                ForEach(days, id: \.self) { day in
                    Section {
                        ForEach(shown.filter { $0.day == day }) { e in
                            row(e)
                            Divider().padding(.leading, 62)
                        }
                    } header: {
                        Text(dayTitle(day))
                            .font(.system(size: 10, weight: .bold))
                            .foregroundStyle(.secondary)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.horizontal, 16).padding(.vertical, 6)
                            .background(.background)
                    }
                }
            }
        }
    }

    private func row(_ e: LogEntry) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Text(e.clock)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(.tertiary)
                .frame(width: 38, alignment: .leading)
            // Level as a dot, not a word: a list where every line carries "info" reads
            // as noise, and the two that matter have to stand out from it.
            Circle()
                .frame(width: 5, height: 5)
                .foregroundStyle(e.level == "error" ? Theme.Status.waiting
                                 : e.level == "warn" ? Theme.Status.errored : Color.secondary.opacity(0.35))
                .padding(.top, 5)
            VStack(alignment: .leading, spacing: 2) {
                Text(e.msg?.isEmpty == false ? e.msg! : e.event)
                    .font(.system(size: 12))
                    .fixedSize(horizontal: false, vertical: true)
                let d = e.detail
                if !d.isEmpty {
                    Text(d)
                        .font(.system(size: 10, design: .monospaced))
                        .foregroundStyle(.tertiary)
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                }
            }
            Spacer(minLength: 0)
            Text(e.component)
                .font(.system(size: 10))
                .foregroundStyle(.tertiary)
        }
        .padding(.horizontal, 16).padding(.vertical, 7)
    }

    private var footer: some View {
        HStack(spacing: 10) {
            Text(l10n.tr("\(shown.count) entries shown", "显示 \(shown.count) 条"))
                .font(.system(size: 11)).foregroundStyle(.tertiary)
            Spacer()
            Button(l10n.tr("Show in Finder", "在访达中显示")) { diag.revealStore() }
            Button(copied ? l10n.tr("Copied", "已拷贝") : l10n.tr("Copy these", "拷贝这些")) {
                diag.copyAll()
                copied = true
                DispatchQueue.main.asyncAfter(deadline: .now() + 2) { copied = false }
            }
            .disabled(shown.isEmpty)
        }
        .padding(.horizontal, 16).padding(.vertical, 10)
    }
}
