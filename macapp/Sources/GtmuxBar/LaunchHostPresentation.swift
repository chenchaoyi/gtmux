import Foundation

/// Historical display context from the core, never input or process authority.
struct LaunchHost: Decodable, Equatable {
    let name: String
    let kind: String
    let observedAt: Int
    enum CodingKeys: String, CodingKey { case name, kind; case observedAt = "observed_at" }
    var valid: Bool { !name.isEmpty && name.count <= 80 && ["app", "process"].contains(kind) && observedAt > 0 }
}

func nativeOriginLabel(_ agent: Agent, l10n: L10n) -> String {
    if let host = agent.launchHost, host.valid { return "\(agent.agent) · \(host.name)" }
    if agent.currentClient == "unconfirmed" { return "\(agent.agent) · \(l10n.tr("External session", "外部会话"))" }
    switch agent.client {
    case "chatgpt_desktop": return "\(l10n.tr("ChatGPT desktop", "ChatGPT 桌面版")) · \(agent.follow?.hq == true ? l10n.tr("HQ following", "HQ 跟进中") : l10n.tr("HQ off", "未开启 HQ 跟进"))"
    case "terminal": return "\(agent.agent) · \(agent.terminal.isEmpty ? l10n.tr("terminal", "终端") : agent.terminal)"
    default: return agent.terminal.isEmpty ? agent.agent : "\(agent.agent) · \(agent.terminal)"
    }
}

func nativeEvidenceLabel(_ agent: Agent, l10n: L10n) -> String {
    var parts: [String] = []
    if let host = agent.launchHost, host.valid {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: l10n.lang)
        formatter.dateStyle = .short; formatter.timeStyle = .medium
        let date = formatter.string(from: Date(timeIntervalSince1970: Double(host.observedAt)))
        parts.append(l10n.tr("Launch host: \(host.name) · observed \(date)", "启动宿主：\(host.name) · 记录于 \(date)"))
    }
    if agent.currentClient == "unconfirmed" { parts.append(l10n.tr("Current input client unconfirmed", "当前输入客户端未确认")) }
    return parts.joined(separator: "\n")
}
