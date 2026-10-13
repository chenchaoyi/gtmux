import Foundation

struct EnvironmentReport: Codable, Equatable {
    struct Row: Codable, Equatable {
        let status: String
        let label: String
        let value: String
        let note: String
        var description: String? = nil
        var purpose: String {
            if let description, !description.isEmpty { return description }
            return note
        }
    }
    struct SectionID: Hashable {
        let title: String
        let occurrence: Int
    }
    struct RowID: Hashable {
        let section: SectionID
        let label: String
        let occurrence: Int
    }
    struct DisplayRow: Identifiable {
        let id: RowID
        let row: Row
    }
    struct DisplaySection: Identifiable {
        let id: SectionID
        let title: String
        let rows: [DisplayRow]
    }
    struct FindingID: Hashable { let row: RowID }
    struct Finding: Identifiable {
        let id: FindingID
        let target: RowID
        let section: String
        let row: Row
    }
    struct Section: Codable, Equatable { let title: String; let rows: [Row] }
    let schema: Int
    let version: String
    let checkedAt: Int64
    let sections: [Section]
    let counts: [String: Int]
    enum CodingKeys: String, CodingKey { case schema, version, sections, counts; case checkedAt = "checked_at" }
    // List flattens sections. Qualify row identities with their owning section;
    // use labels so conditional insertions do not move disclosure state to another check.
    var displaySections: [DisplaySection] {
        var groups: [String: Int] = [:]
        return sections.map { section in
            let occurrence = groups[section.title, default: 0]
            groups[section.title] = occurrence + 1
            let key = SectionID(title: section.title, occurrence: occurrence)
            var rows: [String: Int] = [:]
            return DisplaySection(id: key, title: section.title, rows: section.rows.map { row in
                let occurrence = rows[row.label, default: 0]
                rows[row.label] = occurrence + 1
                return DisplayRow(id: RowID(section: key, label: row.label, occurrence: occurrence), row: row)
            })
        }
    }
    var attentionItems: [Finding] {
        let groups = displaySections
        return ["blocking", "recommended"].flatMap { status in
            groups.flatMap { section in
                section.rows.filter { $0.row.status == status }.map { item in
                    Finding(id: FindingID(row: item.id), target: item.id, section: section.title, row: item.row)
                }
            }
        }
    }
    func attentionSummary(lang: String) -> String {
        var parts: [String] = []
        if let n = counts["blocking"], n > 0 { parts.append(lang == "zh" ? "\(n) 项影响功能" : "\(n) feature \(n == 1 ? "issue" : "issues")") }
        if let n = counts["recommended"], n > 0 { parts.append(lang == "zh" ? "\(n) 项建议处理" : "\(n) \(n == 1 ? "recommendation" : "recommendations")") }
        return parts.joined(separator: " · ")
    }
    func summary(lang: String) -> String {
        var parts = [lang == "zh" ? "检查已完成" : "Check complete"]
        let attention = attentionSummary(lang: lang)
        if !attention.isEmpty { parts.append(attention) }
        if let n = counts["ok"], n > 0 { parts.append(lang == "zh" ? "\(n) 项正常" : "\(n) passed") }
        if let n = counts["info"], n > 0 { parts.append(lang == "zh" ? "\(n) 项信息" : "\(n) informational \(n == 1 ? "item" : "items")") }
        return parts.joined(separator: " · ")
    }
    func checkedTime(lang: String) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: lang == "zh" ? "zh_CN" : "en_US")
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        return formatter.string(from: Date(timeIntervalSince1970: TimeInterval(checkedAt)))
    }
    var findings: Int { (counts["recommended"] ?? 0) + (counts["blocking"] ?? 0) }
    var valid: Bool {
        schema == 1 && !version.isEmpty && checkedAt > 0 && !sections.isEmpty &&
        ["ok", "recommended", "blocking", "info"].allSatisfy { name in
            counts[name] == sections.flatMap(\.rows).filter { $0.status == name }.count
        } && sections.flatMap(\.rows).allSatisfy { ["ok", "recommended", "blocking", "info"].contains($0.status) }
    }
}

/// The relaunched app owns checks. Installation success and environment health
/// are independent. Keep the last valid report when a later check fails.
final class EnvironmentHealth: ObservableObject {
    static let shared = EnvironmentHealth()
    static let repairArguments = ["doctor", "--fix", "--terminal"]
    @Published private(set) var report: EnvironmentReport?
    @Published private(set) var checking = false
    @Published private(set) var failed = false
    @Published private(set) var repairOpening = false
    @Published private(set) var repairOpened = false
    @Published private(set) var repairFailed = false
    private let defaults: UserDefaults
    private let run: ([String]) -> (status: Int32, stdout: String, stderr: String)
    private let version: String
    private let key = "environmentHealth.v1"
    init(defaults: UserDefaults = .standard,
         version: String = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "",
         run: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = { GtmuxCLI.captureBounded($0) }) {
        self.defaults = defaults; self.version = version; self.run = run
        if let data = defaults.data(forKey: key + ".report"),
           let saved = try? JSONDecoder().decode(EnvironmentReport.self, from: data), saved.valid { report = saved }
        failed = defaults.bool(forKey: key + ".failed")
    }
    func autoCheck() {
        guard !version.isEmpty, defaults.string(forKey: key + ".attempt") != version else { return }
        check()
    }
    func check() {
        guard !checking else { return }
        checking = true; failed = false; repairOpened = false
        defaults.set(version, forKey: key + ".attempt")
        let run = run
        DispatchQueue.global(qos: .utility).async { [weak self] in
            let result = run(["doctor", "--json", "--no-prompt"])
            DispatchQueue.main.async { self?.finish(result) }
        }
    }
    private func finish(_ result: (status: Int32, stdout: String, stderr: String)) {
        checking = false
        if (result.status == 0 || result.status == 1), let data = result.stdout.data(using: .utf8),
           let parsed = try? JSONDecoder().decode(EnvironmentReport.self, from: data), parsed.valid,
           parsed.version.trimmingCharacters(in: CharacterSet(charactersIn: "v")) == version.trimmingCharacters(in: CharacterSet(charactersIn: "v")),
           (result.status == 1) == ((parsed.counts["blocking"] ?? 0) > 0) {
            report = parsed; defaults.set(data, forKey: key + ".report"); failed = false
        } else { failed = true }
        defaults.set(failed, forKey: key + ".failed")
    }
    func repair() {
        guard !repairOpening else { return }
        repairOpening = true; repairFailed = false
        let run = run
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let result = run(Self.repairArguments)
            DispatchQueue.main.async {
                guard let self else { return }
                self.repairOpening = false; self.repairOpened = result.status == 0
                self.repairFailed = result.status != 0
            }
        }
    }
}
