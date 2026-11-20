import AppKit
import XCTest
@testable import GtmuxBar

final class LocalArtifactPreviewTests: XCTestCase {
    func testLAP005CanonicalFileLinkUsesSeparateArguments() {
        let ref = "gtmux-file:" + String(repeating: "a", count: 32)
        XCTAssertEqual(LocalArtifactPreview.arguments(for: URL(string: ref)!), ["artifact", "open", ref])
    }
    func testLAP005RejectsOtherRoutesAndCommandText() {
        let id = String(repeating: "a", count: 32)
        for value in ["https://example.test/" + id, "gtmux://usage", "gtmux-file://" + id, "gtmux-file:" + id + "?x=y", "gtmux-file:" + id + "#x", "gtmux-file:" + id + "/file", "gtmux-file:" + id.uppercased(), "gtmux-file:echo%20no"] {
            XCTAssertNil(LocalArtifactPreview.arguments(for: URL(string: value)!), value)
        }
    }
    func testLAP005BundleDeclaresFileSchemeAndPreservesUsage() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let data = try Data(contentsOf: root.appendingPathComponent("Info.plist"))
        let info = try XCTUnwrap(PropertyListSerialization.propertyList(from: data, format: nil) as? [String: Any])
        let groups = try XCTUnwrap(info["CFBundleURLTypes"] as? [[String: Any]])
        let schemes = groups.flatMap { $0["CFBundleURLSchemes"] as? [String] ?? [] }
        XCTAssertTrue(schemes.contains("gtmux")); XCTAssertTrue(schemes.contains("gtmux-file"))
    }
}

// The existing macOS CI job builds this exact Xcode host before running XCTest.
// Deliver the Apple event on its disposable desktop, with synthetic CLI state.
extension LocalArtifactPreviewTests {
    @MainActor
    func testLAP005PackagedHostOpensRegisteredArtifact() throws {
        guard ProcessInfo.processInfo.environment["GITHUB_ACTIONS"] == "true" else {
            throw XCTSkip("Requires the disposable desktop and built host from the macOS CI job")
        }
        let mac = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent()
        let host = mac.appendingPathComponent("build/Gtmux.app")
        let fm = FileManager.default
        XCTAssertTrue(fm.fileExists(atPath: host.appendingPathComponent("Contents/MacOS/GtmuxBar").path))
        let tmp = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
        defer { try? fm.removeItem(at: tmp) }
        let cli = host.appendingPathComponent("Contents/MacOS/gtmux")
        XCTAssertTrue(fm.isExecutableFile(atPath: cli.path))
        func run(_ executable: URL, _ arguments: [String], _ environment: [String: String]) throws -> Data {
            let proc = Process(), out = Pipe()
            proc.currentDirectoryURL = mac.deletingLastPathComponent()
            proc.executableURL = executable
            proc.arguments = arguments
            proc.environment = environment
            proc.standardOutput = out
            try proc.run()
            let data = out.fileHandleForReading.readDataToEndOfFile()
            proc.waitUntilExit()
            XCTAssertEqual(proc.terminationStatus, 0)
            return data
        }
        let fixture = tmp.appendingPathComponent("report.html")
        try Data("<p>Desktop preview</p>".utf8).write(to: fixture)
        let environment = ["HOME": tmp.path, "PATH": "/usr/bin:/bin", "TMUX_TMPDIR": tmp.path]
        let workspace = Process()
        let port = Int.random(in: 49152...65535)
        workspace.executableURL = cli
        workspace.arguments = ["serve", "--bind", "127.0.0.1", "--port", String(port)]
        workspace.environment = environment
        workspace.standardOutput = FileHandle.nullDevice
        workspace.standardError = FileHandle.nullDevice
        try workspace.run()
        defer { if workspace.isRunning { workspace.terminate(); workspace.waitUntilExit() } }
        let status = tmp.appendingPathComponent(".local/share/gtmux/status/serve.json")
        let startup = Date().addingTimeInterval(10)
        while workspace.isRunning, !fm.fileExists(atPath: status.path), Date() < startup {
            RunLoop.current.run(until: Date().addingTimeInterval(0.05))
        }
        XCTAssertTrue(workspace.isRunning)
        let receipt = try run(cli, ["artifact", "add", fixture.path, "--session-id", "desktop-fixture", "--json"], environment)
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: receipt) as? [String: Any])
        let ref = try XCTUnwrap(meta["reference"] as? String)
        let target = try XCTUnwrap(meta["local_preview_url"] as? String)
        XCTAssertEqual(URL(string: target)?.port, port)
        func executable(_ name: String, _ source: String) throws -> URL {
            let url = tmp.appendingPathComponent(name)
            try Data(source.utf8).write(to: url)
            try fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: url.path)
            return url
        }
        let adapter = try executable("fixture-cli", """
        #!/bin/sh
        if [ "$1" = artifact ] && [ "$2" = open ]; then exec "$PREVIEW_CLI" "$@"; fi
        exit 1
        """)
        _ = try executable("open", """
        #!/bin/sh
        [ "$#" = 1 ] || exit 1
        printf '%s' "$1" > "$PREVIEW_RECEIPT"
        """)
        let result = tmp.appendingPathComponent("browser-target")
        let configuration = NSWorkspace.OpenConfiguration()
        configuration.createsNewApplicationInstance = true
        configuration.activates = false
        configuration.environment = ["HOME": tmp.path, "PATH": tmp.path + ":/usr/bin:/bin",
                                     "GTMUX_BIN": adapter.path, "PREVIEW_CLI": cli.path, "PREVIEW_RECEIPT": result.path]
        let opened = expectation(description: "Apple event delivered to the candidate host")
        var application: NSRunningApplication?
        NSWorkspace.shared.open([try XCTUnwrap(URL(string: ref))], withApplicationAt: host, configuration: configuration) { app, error in
            application = app
            XCTAssertNil(error)
            opened.fulfill()
        }
        wait(for: [opened], timeout: 20)
        defer { application?.terminate() }
        XCTAssertEqual(application?.bundleURL?.standardizedFileURL, host.standardizedFileURL)
        let deadline = Date().addingTimeInterval(16)
        while !fm.fileExists(atPath: result.path), Date() < deadline {
            RunLoop.current.run(until: Date().addingTimeInterval(0.05))
        }
        XCTAssertEqual(try String(contentsOf: result, encoding: .utf8), target)
    }
}
