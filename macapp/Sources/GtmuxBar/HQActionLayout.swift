import SwiftUI

/// Keep action labels intact; narrow readers use a vertical list instead of crushed buttons.
struct HQActionLayout<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 8) { content }.fixedSize(horizontal: true, vertical: false)
            VStack(alignment: .leading, spacing: 8) { content }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
}
