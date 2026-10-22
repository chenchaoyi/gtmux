import Foundation
import CoreFoundation

struct DesktopChatSender: Decodable, Equatable {
    let kind: String
    let label: String
    var agent: String? = nil
    var pane: String? = nil
}

struct DesktopConversationEntry: Identifiable {
    let id: String
    let turn: DesktopChatTurn
}

extension DesktopChatTurn {
    var fullReply: String {
        let text = (segments ?? []).compactMap(\.text).filter { !$0.isEmpty }.joined(separator: "\n\n")
        return text.isEmpty ? response : text
    }
    var preview: String {
        Markdown.parseBlocks(fullReply).flatMap { block -> [String] in
            switch block {
            case .heading(_, let spans), .paragraph(let spans), .quote(let spans): return [mdPlain(spans)]
            case .bullets(let items), .ordered(let items, _): return items.map(mdPlain)
            case .code(let text): return [text]
            case .table(let header, let rows): return [header.map(mdPlain).joined(separator: " · ")] + rows.map { $0.map(mdPlain).joined(separator: " · ") }
            case .rule: return []
            }
        }.joined(separator: " · ").split(whereSeparator: \.isWhitespace).joined(separator: " ")
    }
    func identity(index: Int) -> String {
        "\(sessionID ?? ""):\(time ?? "row-\(index)"):\(prompt.prefix(128))"
    }
    var isHuman: Bool { from == nil || from?.kind == "user" }
    func senderLabel(_ l: L10n) -> String {
        if isHuman { return l.tr("You", "你") }
        if from?.kind == "hq" { return "gtmux HQ" }
        return from?.label ?? l.tr("Agent", "Agent")
    }
}

extension DesktopChatStep {
    var action: String {
        let known = Set(["read", "list", "search", "command", "edit", "web", "code", "wait", "agent", "tool"])
        if let operation, known.contains(operation) { return operation }
        let name = title.components(separatedBy: "__").last?.components(separatedBy: ".").last?.lowercased() ?? ""
        switch name {
        case "read", "readfile", "read_file", "read_text_file": return "read"
        case "ls", "list", "list_files", "list_directory", "glob": return "list"
        case "grep", "rg", "search", "search_query", "find", "find_files": return "search"
        case "bash", "shell", "exec", "exec_command", "local_shell", "run_command": return "command"
        case "edit", "write", "apply_patch", "patch", "write_file", "edit_file": return "edit"
        case "web_search", "webfetch", "web_fetch", "browse", "open_url", "fetch": return "web"
        case "js", "python": return "code"
        case "wait", "wait_for", "waitfor", "sleep", "wait_agent", "taskoutput": return "wait"
        case "agent", "task", "spawn_agent", "delegate", "followup_task", "resume_agent": return "agent"
        default: return "tool"
        }
    }
    var symbol: String {
        switch action {
        case "read": return "doc.text"
        case "list": return "folder"
        case "search": return "magnifyingglass"
        case "command": return "terminal"
        case "edit": return "square.and.pencil"
        case "web": return "globe"
        case "code": return "curlybraces"
        case "wait": return "clock"
        case "agent": return "person.2"
        default: return "wrench.and.screwdriver"
        }
    }
    func actionLabel(_ l: L10n) -> String {
        switch action {
        case "read": return l.tr("Read file", "读取文件")
        case "list": return l.tr("List files", "浏览目录")
        case "search": return l.tr("Search", "搜索")
        case "command": return l.tr("Run command", "运行命令")
        case "edit": return l.tr("Edit file", "编辑文件")
        case "web": return l.tr("Browse web", "访问网页")
        case "code": return l.tr("Run code", "运行代码")
        case "wait": return l.tr("Wait", "等待")
        case "agent": return l.tr("Delegate", "委派任务")
        default: return title.isEmpty ? l.tr("Tool call", "工具调用") : title
        }
    }
    var recordedInput: String { input.flatMap { $0.isEmpty ? nil : $0 } ?? detail ?? "" }
    var hasDetails: Bool {
        !(input ?? "").isEmpty || !(workdir ?? "").isEmpty || detailsTruncated == true ||
            (!(detail ?? "").isEmpty && detail?.lowercased() != title.lowercased())
    }
    func preview(_ l: L10n) -> String {
        func compact(_ text: String) -> String {
            let text = text.split(whereSeparator: \.isWhitespace).joined(separator: " ")
            return text.count > 160 ? String(text.prefix(159)) + "…" : text
        }
        let parameters = inputKind == "parameters" || inputKind == "arguments" && (action == "agent" || action == "wait")
        if parameters, let input, !input.isEmpty {
            if let detail, !detail.isEmpty, !detail.trimmingCharacters(in: .whitespaces).hasPrefix("{"), detail != title { return compact(detail) }
            if input.utf8.count <= 8192, let data = input.data(using: .utf8),
               let values = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] {
                if action == "agent" {
                    for key in ["description", "title", "message", "prompt", "task_name"] {
                        if let text = values[key] as? String, !text.isEmpty { return compact(text) }
                    }
                } else if action == "wait" {
                    for (key, en, zh) in [("duration_ms", "Requested", "设定"), ("yield_time_ms", "Up to", "最多"), ("timeout_ms", "Timeout setting", "超时设置")] {
                        if let number = values[key] as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID(), number.doubleValue.isFinite, number.doubleValue >= 0, number.doubleValue <= 9_007_199_254_740_991 {
                            return l.tr("\(en) \(number.doubleValue / 1000)s", "\(zh) \(number.doubleValue / 1000) 秒")
                        }
                    }
                }
            }
            return l.tr("Recorded parameters", "已记录参数")
        }
        return compact(recordedInput.isEmpty ? title : recordedInput)
    }
}
