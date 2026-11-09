import SwiftUI

/// Delivery owns the editor until its receipt, without claiming the agent's task is done.
struct ScreenshotDeliveryOverlay: View {
    var status: ScreenshotStatus
    var target: ScreenshotSendTarget?
    @ObservedObject var l10n: L10n
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency

    var title: String {
        if case .result(.activeTurnAccepted) = status { return l10n.tr("Added to the current task", "已补充到当前任务") }
        if case .result(.delivered(let queued)) = status {
            return queued ? l10n.tr("Screenshot queued", "截图已排队") : l10n.tr("Screenshot sent", "截图已发送")
        }
        return l10n.tr("Sending screenshot…", "正在发送截图…")
    }
    var destination: String {
        guard let target else { return "" }
        var parts: [String] = []
        for value in [target.name, target.session, target.paneID] where !value.isEmpty && !parts.contains(value) {
            parts.append(value)
        }
        return parts.joined(separator: " · ")
    }
    var detail: String {
        switch status {
        case .sending: return l10n.tr("Confirming input delivery", "正在确认投递")
        case .result(.activeTurnAccepted): return l10n.tr("The agent will read it while working", "Agent 将在工作中读取")
        case .result(.delivered(queued: true)): return l10n.tr("Waiting for the agent's next turn", "等待 Agent 下一轮处理")
        default: return l10n.tr("Received by the terminal", "终端已接收")
        }
    }

    var body: some View {
        ZStack {
            if reduceTransparency { Color(nsColor: .windowBackgroundColor) }
            // The editor underneath is already blurred. A full-window material
            // concealed the capture; reserve material for the status card.
            else { Color.primary.opacity(0.035) }
            VStack(spacing: 12) {
                if status == .sending { BrandProgressMark(size: 40) }
                else { GtmuxLogo(size: 40).accessibilityHidden(true) }
                Text(title).font(.system(size: 17, weight: .semibold))
                if !destination.isEmpty {
                    Text(destination).font(.system(size: 13)).lineLimit(2).truncationMode(.middle)
                }
                Text(detail).font(.system(size: 13)).foregroundStyle(.secondary)
            }
            .multilineTextAlignment(.center)
            .padding(28).frame(maxWidth: 360)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16))
            .overlay(RoundedRectangle(cornerRadius: 16).strokeBorder(Color.primary.opacity(0.08)))
            .shadow(color: .black.opacity(0.12), radius: 18, y: 6)
            .padding(24)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .contentShape(Rectangle()).onTapGesture {}
        .accessibilityElement(children: .ignore)
        .accessibilityLabel([title, destination, detail].filter { !$0.isEmpty }.joined(separator: ". "))
        .accessibilityIdentifier("screenshot-delivery-overlay")
    }
}
