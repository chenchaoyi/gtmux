import SwiftUI

/// Initial positioning must not impose a bottom anchor on later size changes.
/// Older systems use the owner's deferred ScrollViewReader entry request.
struct HQConversationInitialAnchor: ViewModifier {
    @ViewBuilder func body(content: Content) -> some View {
        if #available(macOS 15, *) {
            content.defaultScrollAnchor(.bottom, for: .initialOffset)
        } else {
            content
        }
    }
}
