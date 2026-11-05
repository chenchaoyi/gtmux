import Foundation

protocol HQWorkspaceServing {
    func summary() throws -> HQWorkspaceSnapshot
    func snapshot() throws -> HQWorkspaceSnapshot
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail
    func act(_ action: HQWorkspaceAction) throws
    func drain() throws
}

extension HQWorkspaceServing {
    func summary() throws -> HQWorkspaceSnapshot { try snapshot() }
}

struct HQWorkspaceClient: HQWorkspaceServing {
    var execute: ([String], Data?) -> (status: Int32, stdout: String, stderr: String) = {
        GtmuxCLI.captureBounded($0, timeout: 30, stdin: $1, outputLimit: 8_388_608)
    }
    private func call(_ args: [String], input: Data? = nil) throws -> Data {
        let r = execute(["hq", "workspace"] + args, input)
        return try response(r)
    }
    private func response(_ r: (status: Int32, stdout: String, stderr: String)) throws -> Data {
        if r.status == -2 { throw HQWorkspaceError.timeout }
        guard r.status == 0 else {
            throw HQWorkspaceError.command(
                r.stderr.isEmpty ? "HQ workspace unavailable (\(r.status))" : r.stderr)
        }
        guard let data = r.stdout.data(using: .utf8), !data.isEmpty else {
            throw HQWorkspaceError.command("HQ workspace returned no result")
        }
        return data
    }
    private func decode<T: Decodable>(_ data: Data) throws -> T {
        let d = JSONDecoder()
        d.keyDecodingStrategy = .convertFromSnakeCase
        return try d.decode(T.self, from: data)
    }
    func snapshot() throws -> HQWorkspaceSnapshot {
        let value: HQWorkspaceSnapshot = try decode(call(["list", "--overview", "--json"]))
        guard value.schema == 1 else { throw HQWorkspaceError.command("Unsupported HQ workspace version") }
        return value
    }
    func summary() throws -> HQWorkspaceSnapshot {
        let result = execute(["hq", "workspace", "list", "--overview", "--stored", "--json"], nil)
        // Older CLIs return usage status 2 for an unknown read-only flag. Do not
        // retry timeouts, permission errors or malformed successful responses.
        let legacyUsage = result.status == 2 && result.stdout.isEmpty && (
            result.stderr.isEmpty || (result.stderr.contains("--stored") &&
                ["unknown", "unrecognized", "unexpected"].contains { result.stderr.localizedCaseInsensitiveContains($0) }))
        if legacyUsage { return try snapshot() }
        let value: HQWorkspaceSnapshot = try decode(response(result))
        guard value.schema == 1 else { throw HQWorkspaceError.command("Unsupported HQ workspace version") }
        return value
    }
    func detail(_ scope: String, before: UInt64? = nil, earlier: Int = 0) throws -> HQWorkspaceDetail {
        var args = ["show", scope, "--json", "--earlier", "\(earlier)"]
        if let before { args += ["--before", "\(before)"] }
        let value: HQWorkspaceDetail = try decode(call(args))
        guard value.schema == 1 else { throw HQWorkspaceError.command("Unsupported HQ workspace version") }
        return value
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        try decode(call(["participant", matter, association, "--json"]))
    }
    func act(_ action: HQWorkspaceAction) throws {
        let e = JSONEncoder()
        e.keyEncodingStrategy = .convertToSnakeCase
        _ = try call(["act", "--stdin", "--json"], input: e.encode(action))
    }
    func drain() throws { _ = try call(["drain", "--json"]) }
}

enum HQWorkspaceError: LocalizedError {
    case command(String)
    case timeout
    var errorDescription: String? {
        if case .timeout = self {
            return L10n.shared.tr("The HQ request timed out. Your draft is kept.", "HQ 请求超时，草稿已保留。")
        }
        if case .command(let text) = self { return text }
        return nil
    }
}
