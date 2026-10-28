import XCTest
@testable import GtmuxBar

final class LocalWorkspaceTests: XCTestCase {
    func testBrowserEntryAlwaysUsesAuthenticatedLoopback() throws {
        let url = try XCTUnwrap(LocalWorkspace.browserURL(code: "TEST-1234"))
        XCTAssertEqual(url.scheme, "http")
        XCTAssertEqual(url.host, "127.0.0.1")
        XCTAssertEqual(url.port, 8765)
        XCTAssertEqual(url.fragment, "c=TEST-1234")
        XCTAssertNil(url.query)
        XCTAssertNil(LocalWorkspace.browserURL(code: ""))
    }

    func testLocalStartupCannotEnableRemoteService() {
        XCTAssertEqual(LocalWorkspace.serveArguments, ["serve", "--bind", "127.0.0.1", "--port", "8765"])
    }

    @MainActor func testRemoteHandoffWaitsForOwnedChildToExit() async throws {
        let child = Process()
        child.executableURL = URL(fileURLWithPath: "/bin/sleep")
        child.arguments = ["30"]
        try child.run()
        defer { if child.isRunning { child.terminate() } }
        let workspace = LocalWorkspace(process: child)
        let released = expectation(description: "owned local listener released")
        workspace.relinquish { success in
            XCTAssertTrue(success)
            XCTAssertFalse(child.isRunning)
            released.fulfill()
        }
        await fulfillment(of: [released], timeout: 6)
    }
}
