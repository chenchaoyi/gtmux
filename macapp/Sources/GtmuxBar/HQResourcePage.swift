import SwiftUI

enum HQResourcePage: String, CaseIterable, Hashable {
    case board, knowledge
    /// The machine (menubar-hq-report): `gtmux resource` laid out to read — the destination the
    /// card's machine row opens, so a red tier has somewhere on the Mac to say what it is.
    case machine
    /// Usage (menubar-hq-usage): the quotas, who is burning them, the phone's UsageSheet
    /// on the Mac — the door the card's usage row opens.
    case usage

    /// Current work opens Overview; the authored board remains a secondary source.
    var workspaceShortcut: HQWorkspaceSelection {
        self == .board ? .overview : .resource(self)
    }
    var shortcutSymbol: String { self == .board ? "square.grid.2x2" : symbol }
    static var sidebarPages: [HQResourcePage] { allCases.filter { $0 != .board } }

    func title(_ l10n: L10n) -> String {
        switch self {
        case .board: return l10n.tr("Handoff snapshot", "交接快照")
        case .knowledge: return l10n.tr("Knowledge", "知识库")
        case .machine: return l10n.tr("Machine", "机器")
        case .usage: return l10n.tr("Usage", "用量")
        }
    }

    /// Same meaning as the phone/iPad shortcuts; use native SF Symbols on the Mac.
    var symbol: String {
        switch self {
        case .board: return "doc.plaintext"
        case .knowledge: return "book"
        case .machine: return "desktopcomputer"
        case .usage: return "chart.bar"
        }
    }
}
