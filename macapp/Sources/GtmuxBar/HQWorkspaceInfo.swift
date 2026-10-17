import SwiftUI

/// Shared recording/archive explanations remain available without hover.
struct HQWorkspaceInfo: View {
    let title: String
    let text: String
    var symbol = "info.circle"
    var iconOnly = false
    let done: String
    @State private var presented = false
    var body: some View {
        Button { presented = true } label: {
            if iconOnly { Image(systemName: symbol).frame(width: 28, height: 28) }
            else { Label(title, systemImage: symbol) }
        }
            .buttonStyle(.borderless).font(HQTypography.navigation)
            .accessibilityLabel(title)
            .popover(isPresented: $presented) {
                VStack(alignment: .leading, spacing: 12) {
                    Text(title).font(HQTypography.section).accessibilityAddTraits(.isHeader)
                    Text(text).font(HQTypography.body).fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                    HStack { Spacer(); Button(done) { presented = false }.keyboardShortcut(.cancelAction) }
                }.padding(16).frame(width: 360)
            }
    }
}
