import SwiftUI

/// Initial entry uses the tail. Later size changes keep SwiftUI's normal reading
/// position; explicit re-entry is handled by the store's navigation signal.
struct HQConversationInitialAnchor: ViewModifier {
    @ViewBuilder func body(content: Content) -> some View {
        if #available(macOS 15, *) {
            content.defaultScrollAnchor(.bottom, for: .initialOffset)
        } else {
            content.defaultScrollAnchor(.bottom)
        }
    }
}
