import SwiftUI

/// Sheet size follows the presenting workspace rather than an unbounded transcript.
/// Loading, long content and errors must use the same envelope and close controls.
enum HQWorkspaceSheet {
    static func size(in parent: CGSize) -> CGSize {
        CGSize(width: min(700, max(1, parent.width - 40)),
               height: min(640, max(1, parent.height - 40)))
    }
}
private struct HQWorkspaceSheetKey: EnvironmentKey {
    static let defaultValue = CGSize(width: 700, height: 600)
}
extension EnvironmentValues {
    var hqWorkspaceSheetSize: CGSize {
        get { self[HQWorkspaceSheetKey.self] }
        set { self[HQWorkspaceSheetKey.self] = newValue }
    }
}

struct HQMatterSource: View {
    let l10n: L10n
    let origin: HQMatterOrigin
    @State private var presented = false
    var body: some View {
        Button { presented.toggle() } label: {
            Label(origin.label(l10n), systemImage: "info.circle")
        }
        .buttonStyle(.borderless).font(Theme.Font.task)
        .popover(isPresented: $presented) {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text(origin.label(l10n)).font(Theme.Font.session)
                    Spacer()
                    Button(l10n.tr("Done", "完成")) { presented = false }
                        .keyboardShortcut(.cancelAction)
                }
                ScrollView {
                    Text(origin.ref).font(Theme.Font.task).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }.padding(16).frame(width: 360, height: 180)
        }
    }
}
