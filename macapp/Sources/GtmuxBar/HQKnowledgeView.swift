import AppKit
import SwiftUI

/// Which pane of the knowledge tab is showing. The index is a list; opening an entry
/// replaces it, exactly as the phone's sheet does — the acts belong next to the prose they
/// are a judgment on, not on a row a reader is scanning past.
/// The mark on the selected knowledge row: a fill AND a bar down the leading edge.
///
/// The fill alone is a few percent of grey on a list of grey rows, which is the reading
/// that failed — the commander could not tell which entry was filling the pane beside it
/// (2026-09-21). Its own view so the drawing is testable, rather than a property nothing
/// can see.
struct KBSelectionMark: View {
    let selected: Bool
    let p: Theme.Palette

    var body: some View {
        if selected {
            ZStack(alignment: .leading) {
                Rectangle().fill(p.rowSelected)
                Rectangle().fill(Theme.Status.working).frame(width: 3)
            }
        }
    }
}

enum KnowledgePane: Equatable {
    case index
    case entry(id: String)
    case candidate(key: String)
}

/// One act waiting on a reason and a confirmation.
struct PendingAct: Equatable {
    let act: KnowledgeAct
    /// What the reader is about to act on, shown back to them in the sheet.
    let subject: String
}

final class HQKnowledgeNavigation: ObservableObject {
    @Published var pane: KnowledgePane = .index
    @Published var query = ""
    @Published var openTopics: Set<String> = []
}

struct HQKnowledgeView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @ObservedObject var navigation: HQKnowledgeNavigation
    @Environment(\.colorScheme) private var scheme
    @State private var pendingAct: PendingAct?
    @State private var draft = ""
    @State private var whyOpen = false
    @State private var actError: String?
    @State private var busy = false
    @State private var audience: KnowledgeAudience = .machine
    @State private var repoPath = ""

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            HStack {
                TextField(l10n.tr("Search knowledge", "搜索知识库"), text: $navigation.query)
                    .textFieldStyle(.roundedBorder)
                    .accessibilityIdentifier("hq-knowledge-search")
                if !navigation.query.isEmpty {
                    Button(l10n.tr("Clear search", "清除搜索"), systemImage: "xmark.circle.fill") {
                        navigation.query = ""
                    }.labelStyle(.iconOnly).buttonStyle(.borderless)
                }
            }.padding(12)
            HQMemoryBar(l10n: l10n, store: store)
            Divider()
            knowledgeBody(p)
        }
        .background(p.bg)
        .sheet(
            item: Binding(
                get: { pendingAct.map { ActSheetItem(pending: $0) } },
                set: { if $0 == nil { closeSheet() } })
        ) { item in
            actSheet(item.pending, p)
        }
    }

    @ViewBuilder private func knowledgeBody(_ p: Theme.Palette) -> some View {
        // LIST BESIDE DETAIL. This is a window with 640pt of width and the content was one
        // 12pt-padded column, so reading an entry meant leaving the list and coming back —
        // a screen change to answer "what does this one say?" (2026-09-09).
        //
        // The phone keeps its drill-in: it has no width to spare, and its answer to the
        // same problem is returning you to the list at the place you left.
        GeometryReader { geometry in
            HStack(spacing: 0) {
                knowledgeList(p)
                    .frame(width: min(280, max(220, geometry.size.width * 0.36)))
                Divider()
                knowledgeDetail(p)
                    .frame(maxWidth: .infinity)
            }
        }
    }

    /// Entries and unfiled leads use the same list/detail navigation.
    @ViewBuilder private func knowledgeDetail(_ p: Theme.Palette) -> some View {
        switch navigation.pane {
        case .index:
            HQResourceEmpty(l10n.tr("Pick an entry on the left", "在左边选一条"), p)
        case .entry(let id):
            if let e = store.entries.first(where: { $0.id == id }) {
                entryDetail(e, p)
            } else {
                // The entry went away under the reader (a retire, or HQ superseded it).
                // Saying so beats an empty pane with a back button.
                VStack(spacing: 0) {
                    topicLine(l10n.tr("Knowledge", "知识库"), p)
                    HQResourceEmpty(l10n.tr("That entry is no longer live", "这条已不在有效集里"), p)
                }
            }
        case .candidate(let key):
            if let c = store.candidates.first(where: { $0.key == key }) {
                KBCandidateDetail(candidate: c, l10n: l10n, p: p) {
                    actButton(.dismiss(key: c.key), subject: c.lesson, p)
                }
            } else {
                HQResourceEmpty(
                    l10n.tr(
                        "This lead is no longer awaiting review",
                        "这条线索已不在待整理列表中"), p)
            }
        }
    }

    @ViewBuilder private func knowledgeList(_ p: Theme.Palette) -> some View {
        let q = navigation.query.trimmingCharacters(in: .whitespaces)
        if !q.isEmpty {
            // Results REPLACE the index. The index is the browse answer; showing both is
            // two answers to one question.
            let hits = matchKB(store.entries, q)
            if hits.isEmpty {
                HQResourceEmpty(l10n.tr("Nothing matches “\(q)”", "没有匹配「\(q)」的条目"), p)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        sectionHead(l10n.tr("results", "结果"), hits.count, p, accent: false)
                        ForEach(hits) { e in row(e, p, showWhy: false) }
                    }
                    .padding(.bottom, 10)
                }
            }
        } else if store.entries.isEmpty && store.candidates.isEmpty {
            HQResourceEmpty(l10n.tr("Nothing recorded yet", "还没有记录"), p)
        } else {
            ScrollView {
                // Lazy for the same reason the board is: rows are built as they are
                // needed, not all of them on every switch back to this tab.
                LazyVStack(alignment: .leading, spacing: 0) {
                    if !store.pending.isEmpty {
                        // What the commander owes leads, exactly as it does on the phone:
                        // carrying a promoted lesson somewhere durable is the only step of
                        // this lifecycle that waits on a person.
                        sectionHead(l10n.tr("waiting on you", "待你带走"), store.pending.count, p, accent: true)
                        // What a promotion IS, said where it matters. This base is one of
                        // THREE places a rule can live — gtmux's shipped charter, the
                        // operator's own LOCAL.md, and this machine's ledger — and a
                        // reader who takes it for all three files a lesson everyone needed
                        // on one machine. Same sentence as the phone's; the long form is
                        // docs/design/knowledge-layers.md.
                        Button {
                            whyOpen.toggle()
                        } label: {
                            Text(
                                whyOpen
                                    ? l10n.tr("What this is ⌃", "这是什么 ⌃")
                                    : l10n.tr("What this is ⌄", "这是什么 ⌄")
                            )
                            .font(HQTypography.caption)
                            .foregroundStyle(p.fg2)
                        }
                        .buttonStyle(.plain)
                        .padding(.horizontal, 12)
                        .padding(.bottom, 6)
                        if whyOpen {
                            Text(
                                l10n.tr(
                                    "HQ marked these entries for use beyond this machine. Add each to the appropriate LOCAL.md, project instructions, team runbook or gtmux repository, then record where it went.",
                                    "这些知识需要在本机以外使用。请将它们写入对应的 LOCAL.md、项目指令、团队手册或 gtmux 仓库，再记录实际位置。")
                            )
                            .font(HQTypography.caption)
                            .foregroundStyle(p.fg3)
                            .fixedSize(horizontal: false, vertical: true)
                            .padding(.horizontal, 12)
                            .padding(.bottom, 6)
                        }
                        ForEach(store.pending) { e in row(e, p, showWhy: true) }
                    }
                    if !store.candidates.isEmpty {
                        // These are raw leads for HQ's distillation pass, not entries
                        // waiting for the user to approve. Dismissal is an optional
                        // correction when a lead is wrong or already covered.
                        sectionHead(
                            l10n.tr("for HQ to review", "待 HQ 整理的线索"), store.candidates.count, p, accent: false)
                        Text(
                            l10n.tr(
                                "HQ reviews these leads. Select one to read the original.",
                                "由 HQ 核实和整理，点击查看原文。")
                        )
                        .font(HQTypography.caption)
                        .foregroundStyle(p.fg3)
                        .fixedSize(horizontal: false, vertical: true)
                        .padding(.horizontal, 12)
                        .padding(.bottom, 6)
                        ForEach(store.candidates) { c in candidateRow(c, p) }
                    }
                    if !store.entries.isEmpty {
                        // What just landed, at a glance. A lesson recorded wrong is not
                        // inert — it is echoed into every dispatch — so spot-checking the
                        // recent writes is the cheapest way to catch one, and it is the
                        // question this list is opened with.
                        // The count is what this section SHOWS, not the size of the base.
                        // "NEWEST 396" over a list of twelve names the wrong thing; the
                        // total is already in the window's own header.
                        sectionHead(
                            l10n.tr("newest", "最近"), min(KBRecentCount, store.entries.count), p, accent: false)
                        // What "newest" is, relative to the topics below it. These entries
                        // are ALSO in their topic — the topic counts add up to the whole
                        // base — and a reader looking at both lists asked, fairly, whether
                        // the recent ones were in a topic at all (2026-09-07). Same
                        // sentence as the phone's.
                        Text(
                            l10n.tr(
                                "across every topic; each one also sits under its topic below",
                                "跨全部主题 · 这几条同时也在下面各自的主题里")
                        )
                        .font(HQTypography.caption)
                        .foregroundStyle(p.fg3)
                        .padding(.horizontal, 12)
                        .padding(.bottom, 6)
                        ForEach(store.entries.prefix(KBRecentCount)) { e in row(e, p, showWhy: false) }

                        // And the whole base, BY TOPIC, folded. This list used to be all
                        // 386 entries in one run — deliberately uncapped, because "a cap
                        // is an entry the commander cannot retire". That reasoning still
                        // holds and is why the cap above is safe: every entry is here,
                        // inside its topic, so none has become unreachable. Seven rows
                        // instead of 386.
                        sectionHead(l10n.tr("topics", "主题"), store.topics.count, p, accent: false)
                        ForEach(store.topics) { t in
                            topicGroup(t, p)
                        }
                    }
                }
                .padding(.bottom, 10)
            }
        }
    }

    /// One topic, folded. Closed by default: the point of grouping is to see the whole
    /// vocabulary at once, and nothing here is the row you came for.
    @ViewBuilder private func topicGroup(_ t: KBTopic, _ p: Theme.Palette) -> some View {
        let shown = navigation.openTopics.contains(t.name)
        Button {
            if shown {
                navigation.openTopics.remove(t.name)
            } else {
                navigation.openTopics.insert(t.name)
            }
        } label: {
            HStack(spacing: 8) {
                Text(shown ? "▾" : "▸")
                    .font(HQTypography.caption).foregroundStyle(p.fg3).frame(width: 11, alignment: .leading)
                Text(t.name).font(HQTypography.rowTitle).foregroundStyle(p.fg)
                Spacer(minLength: 6)
                Text("\(t.entries.count)")
                    .font(HQTypography.caption.monospacedDigit()).foregroundStyle(p.fg3)
            }
            .contentShape(Rectangle())
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
        }
        .buttonStyle(.plain)
        Divider().overlay(p.divider)
        if shown {
            ForEach(t.entries) { e in row(e, p, showWhy: false) }
        }
    }

    @ViewBuilder private func sectionHead(_ text: String, _ n: Int, _ p: Theme.Palette, accent: Bool)
        -> some View
    {
        HStack(spacing: 6) {
            Text(text).font(HQTypography.sidebarGroup).tracking(0.8)
            Text("\(n)").font(HQTypography.caption)
            Spacer()
        }
        .foregroundStyle(accent ? Theme.Status.waiting : p.fg2)
        .padding(.horizontal, 12).padding(.top, 14).padding(.bottom, 6)
    }

    /// An index row. It OPENS the entry; it does not act on it. The list stays a reading
    /// surface, and a judgment is made looking at the thing being judged.
    @ViewBuilder private func row(_ e: KBEntry, _ p: Theme.Palette, showWhy: Bool) -> some View {
        let parts = e.titleParts(l10n.lang)
        // WHICH ONE AM I READING. The list and the detail sit side by side, and the row
        // carried no mark of being the selected one — so the answer to "where is the entry
        // filling the right half" was to read both and match the titles (2026-09-21).
        let picked = navigation.pane == .entry(id: e.id)
        Button {
            navigation.pane = .entry(id: e.id)
        } label: {
            VStack(alignment: .leading, spacing: 3) {
                // Identifier over sentence, the phone's anatomy (#1099). Inline, the key
                // reads as the opening words of the prose and pushes the rest into an odd
                // wrap, which in a rail this narrow costs a whole line of the sentence.
                VStack(alignment: .leading, spacing: 1) {
                    if let key = parts.key {
                        Text(key)
                            .font(HQTypography.code)
                            .foregroundStyle(p.fg3)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                    Text(parts.rest)
                        .font(HQTypography.caption)
                        .foregroundStyle(p.fg)
                        .lineLimit(2)
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                HStack(spacing: 6) {
                    if e.sensitive ?? false {
                        // The lock says "yours, kept here" — the same mark on every surface.
                        Image(systemName: "lock.fill").font(HQTypography.caption).foregroundStyle(p.fg3)
                            .help(l10n.tr("Sensitive: stays on this Mac", "敏感：只留本机"))
                    }
                    Text(e.topic).font(HQTypography.caption).foregroundStyle(p.fg3)
                    if showWhy, let why = e.promoteWhy, !why.isEmpty {
                        Text(why).font(HQTypography.caption).foregroundStyle(p.fg2).lineLimit(1)
                    }
                    if showWhy, let target = e.promoteTarget, !target.isEmpty {
                        Text("→ \(target)").font(HQTypography.caption).foregroundStyle(p.fg3).lineLimit(1)
                    }
                    Spacer(minLength: 4)
                    Text("›").font(HQTypography.caption).foregroundStyle(p.fg3)
                }
            }
            .padding(.horizontal, 12).padding(.vertical, 7)
            .frame(maxWidth: .infinity, alignment: .leading)
            // A fill AND a bar down the leading edge: the fill alone is a few percent of
            // grey on a list of grey rows, which is the reading that failed.
            .background(alignment: .leading) { KBSelectionMark(selected: picked, p: p) }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .overlay(alignment: .bottom) {
            Rectangle().fill(p.divider).frame(height: 0.5).padding(.leading, 12)
        }
    }

    @ViewBuilder private func candidateRow(_ c: KBCandidateGroup, _ p: Theme.Palette) -> some View {
        KBCandidateRow(candidate: c, selected: navigation.pane == .candidate(key: c.key), p: p) {
            navigation.pane = .candidate(key: c.key)
        }
    }

    /// One entry, opened: its prose, its lifecycle, and the judgments available on it.
    @ViewBuilder private func entryDetail(_ e: KBEntry, _ p: Theme.Palette) -> some View {
        VStack(spacing: 0) {
            // No back control: the list is beside this, not behind it. The topic line
            // stays because it says WHERE this entry lives, which the list only implies.
            topicLine(e.topic, p)
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    // The heading is the SENTENCE. Its key is not dropped: the id line
                    // directly below carries it, with the topic and the ledger's suffix,
                    // and setting the same words twice in two faces reads as two facts.
                    Text(e.titleParts(l10n.lang).rest)
                        .font(HQTypography.rowTitle)
                        .foregroundStyle(p.fg)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Text(e.id).font(HQTypography.code).foregroundStyle(p.fg3)
                        .textSelection(.enabled)
                    // The three axes, where the reader judges: what it is, where it came
                    // from and how often, who must know it.
                    let axes = e.axesLine(l10n)
                    if !axes.isEmpty {
                        Text(axes).font(HQTypography.caption).foregroundStyle(p.fg2)
                    }

                    if e.pending {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(l10n.tr("Promoted · waiting on you", "已晋升 · 待你带走"))
                                .font(HQTypography.rowTitle).tracking(0.8)
                                .foregroundStyle(Theme.Status.waiting)
                            if let why = e.promoteWhy, !why.isEmpty {
                                Text(why).font(HQTypography.caption).foregroundStyle(p.fg2)
                            }
                            let aud = audienceShort(e.audience, l10n)
                            if !aud.isEmpty {
                                Text(
                                    l10n.tr("for: ", "给：") + aud
                                        + ((e.audienceRepo ?? "").isEmpty ? "" : " · " + (e.audienceRepo ?? ""))
                                )
                                .font(HQTypography.caption).foregroundStyle(p.fg3)
                            } else if let target = e.promoteTarget, !target.isEmpty {
                                Text("→ \(target)").font(HQTypography.caption).foregroundStyle(p.fg3)
                            } else {
                                Text(
                                    l10n.tr(
                                        "no audience chosen; withdraw, then promote again saying who must know it",
                                        "没选读者，撤回后重新晋升，说清给谁看")
                                )
                                .font(HQTypography.caption).foregroundStyle(p.fg3)
                            }
                        }
                    } else if let ref = e.landedRef, !ref.isEmpty {
                        Text(l10n.tr("✓ landed \(ref)", "✓ 已落地 \(ref)"))
                            .font(HQTypography.rowTitle)
                            .foregroundStyle(Theme.Status.idle)
                    }

                    if let body = e.resolved(l10n.lang).body, !body.isEmpty {
                        // Entries are written in markdown, the same as the board: tables of
                        // evidence, `code` for identifiers, and [[links]] to sibling
                        // entries. Printed raw it was the reader's job to parse them —
                        // and the links, which are the base's structure, read as noise.
                        MarkdownBody(markdown: body, p: p)
                    }

                    Divider().padding(.vertical, 2)

                    HStack(spacing: 8) {
                        ForEach(knowledgeActs(for: e), id: \.self) { act in
                            actButton(act, subject: e.title, p)
                        }
                        Spacer(minLength: 0)
                    }
                    // Authoring stays off this screen on purpose, and saying so beats
                    // leaving a reader hunting for a button that is not coming.
                    Text(
                        l10n.tr(
                            "Writing an entry stays in the CLI: `gtmux knowledge add`",
                            "写入新条目仍在 CLI：`gtmux knowledge add`")
                    )
                    .font(HQTypography.caption).foregroundStyle(p.fg3)
                }
                .padding(14)
            }
        }
    }

    /// The detail pane's own heading — where this entry lives.
    @ViewBuilder private func topicLine(_ title: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 6) {
            Text(title).font(HQTypography.rowTitle).foregroundStyle(p.fg2)
            Spacer()
        }
        .padding(.horizontal, 14).padding(.top, 10).padding(.bottom, 6)
    }

    @ViewBuilder private func backBar(_ title: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            Button {
                navigation.pane = .index
            } label: {
                HStack(spacing: 3) {
                    Text("‹").font(.system(size: 14))
                    Text(l10n.tr("Knowledge", "知识库")).font(HQTypography.caption)
                }
                .foregroundStyle(p.fg2)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            Text(title).font(HQTypography.caption).foregroundStyle(p.fg3).lineLimit(1)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 12).padding(.vertical, 8)
        .overlay(alignment: .bottom) { Rectangle().fill(p.divider).frame(height: 0.5) }
    }

    @ViewBuilder private func actButton(_ act: KnowledgeAct, subject: String, _ p: Theme.Palette)
        -> some View
    {
        let c = act.copy(l10n)
        Button {
            if case .feedback(let url) = act {
                // The everyone audience's exit is a browser, not a process: the issue
                // page is already prefilled from the brief.
                if let u = URL(string: url) { NSWorkspace.shared.open(u) }
                return
            }
            draft = ""
            actError = nil
            audience = .machine
            repoPath = ""
            pendingAct = PendingAct(act: act, subject: subject)
        } label: {
            Text(c.button)
                .font(HQTypography.caption)
                .foregroundStyle(act.removes ? Theme.Status.waiting : p.fg)
                .padding(.horizontal, 10).padding(.vertical, 5)
                .overlay(
                    RoundedRectangle(cornerRadius: 7, style: .continuous)
                        .strokeBorder(p.divider, lineWidth: 1)
                )
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // MARK: the confirm sheet

    /// The one confirmation before anything is written. It names the verb and the subject,
    /// and its confirm button stays disabled until the reason exists — the same shape as
    /// the phone's, for the same reason: every one of these verbs is refused by the CLI
    /// without one anyway, and a reason typed to satisfy a dialog after the fact is worth
    /// less than one typed while looking at the entry.
    @ViewBuilder private func actSheet(_ pending: PendingAct, _ p: Theme.Palette) -> some View {
        let c = pending.act.copy(l10n)
        VStack(alignment: .leading, spacing: 10) {
            Text(c.title).font(HQTypography.rowTitle).foregroundStyle(p.fg)
            Text(pending.subject)
                .font(HQTypography.caption).foregroundStyle(p.fg2).lineLimit(3)
                .frame(maxWidth: .infinity, alignment: .leading)
            Text(c.hint).font(HQTypography.caption).foregroundStyle(p.fg3)
                .fixedSize(horizontal: false, vertical: true)
            if case .promote = pending.act {
                // Who must know it — the one question promote asks. Four answers, each
                // with an exit a person can actually take (D4).
                Picker(l10n.tr("Who must know it", "这条给谁看"), selection: $audience) {
                    ForEach(KnowledgeAudience.allCases, id: \.self) { a in
                        Text(a.word(l10n)).tag(a)
                    }
                }
                .pickerStyle(.radioGroup)
                .font(HQTypography.caption)
                .disabled(busy)
                if audience == .repo {
                    TextField(
                        l10n.tr("repository path, e.g. ~/work/api", "仓库路径，例如 ~/work/api"), text: $repoPath
                    )
                    .textFieldStyle(.roundedBorder)
                    .font(HQTypography.caption)
                    .disabled(busy)
                }
            }
            if pending.act.needsReason {
                TextField(c.placeholder, text: $draft, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                    .lineLimit(2...5)
                    .font(HQTypography.caption)
                    .disabled(busy)
            }

            if let err = actError {
                // The CLI's own words, unedited. "has no pending promotion to land (gtmux
                // knowledge promotions)" says what happened AND what to do next.
                Text(err)
                    .font(HQTypography.code)
                    .foregroundStyle(Theme.Status.waiting)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 8) {
                Text(
                    ("gtmux knowledge \(verbWord(pending.act)) \(c.field)").trimmingCharacters(
                        in: .whitespaces)
                )
                .font(HQTypography.code).foregroundStyle(p.fg3)
                Spacer(minLength: 8)
                Button(l10n.tr("Cancel", "取消")) { closeSheet() }
                    .disabled(busy)
                Button(l10n.tr("Confirm", "确认")) { run(pending) }
                    .keyboardShortcut(.defaultAction)
                    .disabled(busy || !canConfirm(pending))
            }
        }
        .padding(16)
        .frame(width: 420)
        .background(p.bg)
    }

    private func verbWord(_ act: KnowledgeAct) -> String {
        switch act {
        case .promote: return "promote"
        case .land, .carry: return "land"
        case .withdraw: return "withdraw"
        case .retire: return "retire"
        case .dismiss: return "dismiss"
        case .feedback: return ""
        }
    }

    /// The reason exists when one is needed, and a repository audience has its path.
    private func canConfirm(_ pending: PendingAct) -> Bool {
        if pending.act.needsReason && draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return false
        }
        if case .promote = pending.act, audience == .repo,
            repoPath.trimmingCharacters(in: .whitespaces).isEmpty
        {
            return false
        }
        return true
    }

    /// The act as it will run: a promote picks up the audience chosen in the sheet.
    private func resolved(_ act: KnowledgeAct) -> KnowledgeAct {
        guard case .promote(let id, _) = act else { return act }
        let path = repoPath.trimmingCharacters(in: .whitespaces)
        let forValue =
            audience == .repo ? "repo:" + (path as NSString).expandingTildeInPath : audience.rawValue
        return .promote(id: id, audience: forValue)
    }

    private func run(_ pending: PendingAct) {
        busy = true
        actError = nil
        store.perform(resolved(pending.act), reason: draft, l10n: l10n) { err in
            busy = false
            if let err {
                actError = err
                return
            }
            pendingAct = nil
            draft = ""
            // A retired entry is gone from the live set and a landed one has left the
            // debt, so there is nothing behind this sheet worth returning to.
            navigation.pane = .index
        }
    }

    private func closeSheet() {
        guard !busy else { return }
        pendingAct = nil
        draft = ""
        actError = nil
    }

}

/// `.sheet(item:)` wants an Identifiable; the pending act is identified by what it is
/// about to do to what.
private struct ActSheetItem: Identifiable {
    let pending: PendingAct
    var id: String { "\(pending.act)" }
}
