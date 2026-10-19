import SwiftUI

/// SectionCollapse remembers which popover sections the user folded (A4), so the
/// choice survives reopen. The count stays visible when a section is collapsed.
final class SectionCollapse: ObservableObject {
    static let shared = SectionCollapse()
    @Published private var collapsed: Set<String>
    private let key = "popover.collapsedSections"

    private let defaults: UserDefaults

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        let raw = defaults.string(forKey: key) ?? ""
        collapsed = Set(raw.split(separator: ",").map(String.init))
    }

    /// The errored section folds on its OWN key: folding "finished" must not take the
    /// failures with it, which is what a shared key would do (both are idle sessions).
    private func key(_ s: Status, errored: Bool) -> String { errored ? "errored" : s.rawValue }

    func isCollapsed(_ s: Status, errored: Bool = false) -> Bool { collapsed.contains(key(s, errored: errored)) }

    enum ExternalGroup: String { case elsewhere, external }
    func isCollapsed(_ group: ExternalGroup) -> Bool { collapsed.contains(group.rawValue) }
    func toggle(_ group: ExternalGroup) { toggleKey(group.rawValue) }

    func toggle(_ s: Status, errored: Bool = false) { toggleKey(key(s, errored: errored)) }

    private func toggleKey(_ k: String) {
        if collapsed.contains(k) { collapsed.remove(k) } else { collapsed.insert(k) }
        defaults.set(collapsed.sorted().joined(separator: ","), forKey: key)
    }
}
