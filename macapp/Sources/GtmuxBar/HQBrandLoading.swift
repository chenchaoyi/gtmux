import SwiftUI

/// Page loading breathes; it does not use the agent-working ring. Unmounting ends it.
struct HQBrandLoading: View {
    var label: String
    var compact = false

    var body: some View {
        Group {
            if compact {
                BrandProgressMark(size: 16)
            } else {
                VStack(spacing: 14) {
                    BrandProgressMark()
                    Text(label).font(Theme.Font.task).foregroundStyle(.secondary)
                }.padding(24)
            }
        }.accessibilityElement(children: .ignore).accessibilityLabel(label)
    }
}
