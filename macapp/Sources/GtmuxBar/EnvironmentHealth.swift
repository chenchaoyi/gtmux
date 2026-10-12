import Foundation

struct EnvironmentReport: Codable, Equatable {
    struct Row: Codable, Equatable { let status: String; let label: String; let value: String; let note: String }
    struct Section: Codable, Equatable { let title: String; let rows: [Row] }
    let schema: Int
    let version: String
    let checkedAt: Int64
    let sections: [Section]
    let counts: [String: Int]
    enum CodingKeys: String, CodingKey { case schema, version, sections, counts; case checkedAt = "checked_at" }
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
