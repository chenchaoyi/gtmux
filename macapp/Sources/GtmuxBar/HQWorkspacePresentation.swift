import Foundation

/// One grouping rule for both navigation and Overview; no Markdown state inference.
enum HQMatterGroup: String, CaseIterable, Hashable, Identifiable {
    case needsInput, following, ended, other
    var id: Self { self }
    static func of(_ state: String) -> Self {
        switch state {
        case "needs-input": return .needsInput
        case "open": return .following
        case "completed", "closed": return .ended
        default: return .other
        }
    }
    var status: Status {
        switch self {
        case .needsInput: return .waiting
        case .following: return .working
        case .ended: return .idle
        case .other: return .running
        }
    }
    func title(_ l: L10n) -> String {
        switch self {
        case .needsInput: return l.tr("Needs your input", "待你处理")
        case .following: return l.tr("Following", "正在跟进")
        case .ended: return l.tr("Recently ended", "最近结束")
        case .other: return l.tr("Other matters", "其他事项")
        }
    }
    func matters(in snapshot: HQWorkspaceSnapshot?) -> [HQMatter] {
        (snapshot?.matters ?? []).filter { Self.of($0.state) == self }.sorted {
            $0.updatedAt == $1.updatedAt ? $0.id < $1.id : $0.updatedAt > $1.updatedAt
        }
    }
}

enum HQSidebarSection: Hashable {
    case matters(HQMatterGroup)
    case resources
}

/// An unread value must not masquerade as an empty result, even before a task starts.
enum HQReadPhase: Equatable {
    case loading, failed, content
    init(loaded: Bool, failed: Bool) {
        self = loaded ? .content : failed ? .failed : .loading
    }
}
