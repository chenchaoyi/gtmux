import AppKit
import SwiftUI

/// The mobile UserAvatar's supplied 40 × 40 geometry, shared without a new identity.
struct HQUserAvatar: View {
    var body: some View {
        Canvas { context, size in
            context.scaleBy(x: size.width / 40, y: size.height / 40)
            context.fill(
                Path(ellipseIn: CGRect(x: 0, y: 0, width: 40, height: 40)),
                with: .linearGradient(
                    Gradient(colors: [
                        Color(red: 34 / 255, green: 211 / 255, blue: 238 / 255),
                        Color(red: 14 / 255, green: 116 / 255, blue: 144 / 255),
                    ]),
                    startPoint: .zero, endPoint: CGPoint(x: 40, y: 40)))
            context.stroke(
                Path(roundedRect: CGRect(x: 9, y: 7, width: 22, height: 28), cornerRadius: 4),
                with: .color(.white), lineWidth: 2.4)
            context.fill(
                Path(roundedRect: CGRect(x: 15.5, y: 4, width: 9, height: 4), cornerRadius: 1.5),
                with: .color(.white))
            context.fill(
                Path(ellipseIn: CGRect(x: 16.6, y: 13.6, width: 6.8, height: 6.8)), with: .color(.white))
            var shoulders = Path()
            shoulders.move(to: CGPoint(x: 13.5, y: 30))
            shoulders.addQuadCurve(to: CGPoint(x: 20, y: 23), control: CGPoint(x: 13.5, y: 23))
            shoulders.addQuadCurve(to: CGPoint(x: 26.5, y: 30), control: CGPoint(x: 26.5, y: 23))
            shoulders.closeSubpath()
            context.fill(shoulders, with: .color(.white))
        }.accessibilityHidden(true)
    }
}

/// The copy action is shared by public HQ messages and linked conversation sources.
@MainActor final class HQChatCopyFeedback: ObservableObject {
    enum State { case ready, copied, failed }
    @Published private(set) var state: State = .ready
    private var reset: Task<Void, Never>?
    private let write: (String) -> Bool
    private let delay: UInt64
    init(
        delay: UInt64 = 2_000_000_000,
        write: @escaping (String) -> Bool = {
            NSPasteboard.general.clearContents()
            return NSPasteboard.general.setString($0, forType: .string)
        }
    ) {
        self.delay = delay
        self.write = write
    }
    deinit { reset?.cancel() }
    func copy(_ text: String) {
        reset?.cancel()
        state = write(text) ? .copied : .failed
        reset = Task { [weak self, delay] in
            do { try await Task.sleep(nanoseconds: delay) } catch { return }
            self?.state = .ready
        }
    }
    func clear() {
        reset?.cancel()
        state = .ready
    }
}

struct HQChatBubble: View {
    let l10n: L10n
    let user: Bool
    let label: String
    let bodyText: String
    var icon: String?
    @Environment(\.colorScheme) private var scheme
    @StateObject private var feedback = HQChatCopyFeedback()

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            if user { Spacer(minLength: 45) } else { avatar }
            VStack(alignment: user ? .trailing : .leading, spacing: 6) {
                Text(label == "HQ" ? "gtmux HQ" : label).font(HQTypography.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                MarkdownBody(markdown: bodyText, p: Theme.Palette.of(scheme))
                    .frame(maxWidth: .infinity, alignment: .leading).padding(12)
                    .background(
                        user ? Theme.Status.working.opacity(0.10) : Color.primary.opacity(0.035),
                        in: RoundedRectangle(cornerRadius: 12))
                Button {
                    feedback.copy(bodyText)
                } label: {
                    Label(copyLabel, systemImage: feedback.state == .copied ? "checkmark" : "doc.on.doc")
                        .font(HQTypography.caption).padding(.vertical, 3)
                }.buttonStyle(.borderless).foregroundStyle(.secondary)
                    .help(l10n.tr("Copy full text", "复制全文"))
                    .accessibilityLabel(l10n.tr("Copy full text from \(label)", "复制\(label)的全文"))
                    .accessibilityValue(copyLabel)
            }
            if user { avatar } else { Spacer(minLength: 45) }
        }.onChange(of: bodyText) { _, _ in feedback.clear() }
            .onDisappear { feedback.clear() }
    }
    private var copyLabel: String {
        switch feedback.state {
        case .ready: return l10n.tr("Copy", "复制")
        case .copied: return l10n.tr("Copied", "已复制")
        case .failed: return l10n.tr("Copy failed · try again", "复制失败 · 请重试")
        }
    }
    @ViewBuilder private var avatar: some View {
        if user {
            HQUserAvatar().frame(width: 28, height: 28)
        } else if label == "HQ" {
            GtmuxLogo(size: 24).accessibilityHidden(true)
        } else {
            HQConversationIcon(agent: label, hint: icon).frame(width: 28, height: 28).accessibilityHidden(true)
        }
    }
}
