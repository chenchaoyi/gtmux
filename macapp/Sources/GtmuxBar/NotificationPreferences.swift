import SwiftUI

/// Presentation only. The parent owns settings and all notification/tmux actions.
struct NotificationPreferences: View {
    @ObservedObject var l10n: L10n
    @Binding var enabled: Bool
    var authorization: DesktopNotificationAuthorization
    var testState: NotificationManager.TestState
    @Binding var tabAlert: Bool
    var tabAlertBusy: Bool
    var sendTest: () -> Void
    var openSystemSettings: () -> Void

    var body: some View {
        Section {
            VStack(alignment: .leading, spacing: 12) {
                PreferenceToggleRow(
                    title: l10n.tr("Desktop notifications", "桌面通知"),
                    detail: l10n.tr("Alerts for input requests and turn completion.", "会话需要处理或本轮结束时通知。"),
                    symbol: "bell", isOn: $enabled)
                Divider()
                VStack(alignment: .leading, spacing: 10) {
                    HStack(alignment: .firstTextBaseline, spacing: 8) {
                        Text(authorization.summary(l10n))
                            .font(.system(size: 11)).foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                        Spacer(minLength: 4)
                        Button(l10n.tr("System settings…", "系统设置…"), action: openSystemSettings)
                            .buttonStyle(.link).font(.system(size: 11)).fixedSize()
                    }
                    HStack(alignment: .firstTextBaseline, spacing: 10) {
                        Button(l10n.tr("Test notification", "测试通知"), action: sendTest)
                            .buttonStyle(.bordered).controlSize(.small)
                            .disabled(!enabled || testState == .sending)
                        if let feedback = testFeedback {
                            Text(feedback).font(.system(size: 11))
                                .foregroundStyle(testState == .failed || testState == .blocked
                                    ? Theme.Status.errored : .secondary)
                                .fixedSize(horizontal: false, vertical: true)
                                .accessibilityAddTraits(.updatesFrequently)
                        }
                    }
                }
                .padding(.leading, 28)
            }
            .padding(.vertical, 4)
        } header: {
            Text(l10n.tr("Notifications", "通知"))
        } footer: {
            Text(l10n.tr("Input alerts may play a sound; turn-end alerts are silent. System and Focus settings apply.",
                         "待处理通知可播放提示音，结束通知静音；遵循系统通知与专注模式设置。"))
                .font(.system(size: 10)).foregroundStyle(.secondary)
        }

        Section(l10n.tr("Terminal alerts", "终端提示")) {
            PreferenceToggleRow(
                title: l10n.tr("Tab marker", "标签标记"),
                detail: l10n.tr("Mark tabs needing input with ●. Disabling restores the original title format.",
                               "在终端标签前显示 ●，标记待处理会话；关闭后恢复原格式。"),
                symbol: "macwindow.badge.plus", isOn: $tabAlert)
                .disabled(tabAlertBusy)
        }
    }

    var testFeedback: String? {
        switch testState {
        case .idle: return nil
        case .sending: return l10n.tr("Submitting…", "正在提交…")
        case .accepted: return l10n.tr("Submitted. Check banners or Notification Center.", "已提交，请查看横幅或通知中心。")
        case .blocked: return l10n.tr("Enable app and system notifications to retry.", "开启桌面通知和系统权限后重试。")
        case .failed: return l10n.tr("Submission failed. Check system settings and retry.", "提交失败，请检查系统设置后重试。")
        }
    }
}
