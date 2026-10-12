import XCTest
@testable import GtmuxBar

final class EnvironmentHealthTests: XCTestCase {
    private func defaults() -> UserDefaults {
        UserDefaults(suiteName: "gtmux-health-tests-" + UUID().uuidString)!
    }
    private let report = #"{"schema":1,"version":"9.0.1","checked_at":100,"sections":[{"title":"Tools","rows":[{"status":"blocking","label":"hook","value":"missing","note":"repair"}]}],"counts":{"ok":0,"recommended":0,"blocking":1,"info":0}}"#
    @MainActor func testBlockingResultPersistsAcrossRelaunchWithoutAutomaticRepair() async {
        let prefs = defaults(), called = expectation(description: "check")
        let json = report
        let model = EnvironmentHealth(defaults: prefs, version: "9.0.1", run: { args in
            XCTAssertEqual(args, ["doctor", "--json", "--no-prompt"])
            called.fulfill(); return (1, json, "")
        })
        model.autoCheck()
        await fulfillment(of: [called], timeout: 2)
        for _ in 0..<100 { if !model.checking { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertFalse(model.failed); XCTAssertEqual(model.report?.findings, 1)
        let relaunch = EnvironmentHealth(defaults: prefs, version: "9.0.1", run: { _ in XCTFail("repeated check"); return (-1,"","") })
        relaunch.autoCheck()
        XCTAssertFalse(relaunch.checking); XCTAssertEqual(relaunch.report?.findings, 1)
    }
    @MainActor func testFailedNewVersionKeepsPreviousReportAndAllowsRetry() async {
        let prefs = defaults()
        prefs.set(report.data(using: .utf8), forKey: "environmentHealth.v1.report")
        let called = expectation(description: "new version")
        called.expectedFulfillmentCount = 2
        let model = EnvironmentHealth(defaults: prefs, version: "9.0.2", run: { _ in called.fulfill(); return (-2,"", "timeout") })
        model.autoCheck()
        for _ in 0..<100 { if !model.checking { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        model.check()
        await fulfillment(of: [called], timeout: 2)
        for _ in 0..<100 { if !model.checking { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertTrue(model.failed); XCTAssertEqual(model.report?.version, "9.0.1")
        XCTAssertEqual(EnvironmentHealth.repairArguments, ["doctor", "--fix", "--terminal"])
    }
    @MainActor func testWrongCLIVersionDoesNotReplaceReportAndRepairFailureIsVisible() async {
        let prefs = defaults(), checked = expectation(description: "checked"), repaired = expectation(description: "repair")
        let json = report
        let model = EnvironmentHealth(defaults: prefs, version: "9.0.2", run: { args in
            if args == EnvironmentHealth.repairArguments { repaired.fulfill(); return (1, "", "cannot open") }
            checked.fulfill(); return (1, json, "")
        })
        model.check()
        await fulfillment(of: [checked], timeout: 2)
        for _ in 0..<100 { if !model.checking { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertTrue(model.failed); XCTAssertNil(model.report)
        model.repair()
        await fulfillment(of: [repaired], timeout: 2)
        for _ in 0..<100 { if !model.repairOpening { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertTrue(model.repairFailed); XCTAssertFalse(model.repairOpened)
    }
    func testReportCountsAndSchemaMustAgree() throws {
        let decoder = JSONDecoder()
        XCTAssertTrue(try decoder.decode(EnvironmentReport.self, from: Data(report.utf8)).valid)
        XCTAssertFalse(try decoder.decode(EnvironmentReport.self, from: Data(report.replacingOccurrences(of: "\"blocking\":1", with: "\"blocking\":0").utf8)).valid)
        XCTAssertFalse(try decoder.decode(EnvironmentReport.self, from: Data(report.replacingOccurrences(of: "\"schema\":1", with: "\"schema\":2").utf8)).valid)
    }
    func testProcessTimeoutAndBothStreams() {
        let output = Process(); output.executableURL = URL(fileURLWithPath: "/bin/sh")
        output.arguments = ["-c", "printf report; printf progress >&2; exit 1"]
        let result = GtmuxCLI.runBounded(output, timeout: 2)
        XCTAssertEqual(result.status, 1); XCTAssertEqual(result.stdout, "report"); XCTAssertEqual(result.stderr, "progress")
        let sleeper = Process(); sleeper.executableURL = URL(fileURLWithPath: "/bin/sleep"); sleeper.arguments = ["10"]
        let start = Date(); let timeout = GtmuxCLI.runBounded(sleeper, timeout: 0.03)
        XCTAssertEqual(timeout.status, -2); XCTAssertLessThan(Date().timeIntervalSince(start), 3)
    }
}
