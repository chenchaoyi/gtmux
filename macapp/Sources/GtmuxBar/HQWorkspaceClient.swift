import Foundation

protocol HQWorkspaceServing {
    func snapshot() throws -> HQWorkspaceSnapshot
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail
    func act(_ action: HQWorkspaceAction) throws
    func drain() throws
}

struct HQWorkspaceClient: HQWorkspaceServing {
    var execute: ([String], Data?) -> (status: Int32, stdout: String, stderr: String) = {
        GtmuxCLI.captureBounded($0, timeout: 30, stdin: $1, outputLimit: 8_388_608)
    }
    private func call(_ args: [String], input: Data? = nil) throws -> Data {
        let r = execute(["hq", "workspace"] + args, input)
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
        let value: HQWorkspaceSnapshot = try decode(call(["list", "--json"]))
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
    var errorDescription: String? {
        if case .command(let text) = self { return text }
        return nil
    }
}
