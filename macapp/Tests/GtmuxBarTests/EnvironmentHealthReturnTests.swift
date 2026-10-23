import AppKit
import XCTest
@testable import GtmuxBar

final class EnvironmentHealthReturnTests: XCTestCase {
    private let oldReport = #"{"schema":1,"version":"9.0.1","checked_at":100,"sections":[{"title":"Tools","rows":[{"status":"blocking","label":"hook","value":"missing","note":"repair"}]}],"counts":{"ok":0,"recommended":0,"blocking":1,"info":0}}"#
    private let newReport = #"{"schema":1,"version":"9.0.1","checked_at":200,"sections":[{"title":"Tools","rows":[{"status":"ok","label":"hook","value":"installed","note":""}]}],"counts":{"ok":1,"recommended":0,"blocking":0,"info":0}}"#

    private func recentDefaults() -> UserDefaults {
        let prefs = UserDefaults(suiteName: "gtmux-health-return-tests-" + UUID().uuidString)!
        prefs.set(oldReport.data(using: .utf8), forKey: "environmentHealth.v1.report")
        prefs.set("9.0.1", forKey: "environmentHealth.v1.attempt")
        prefs.set(Date().timeIntervalSince1970, forKey: "environmentHealth.v1.attemptAt")
        return prefs
    }

    @MainActor private func settle(_ model: EnvironmentHealth) async {
        for _ in 0..<200 {
            if !model.checking && !model.repairOpening { return }
            try? await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTFail("Environment operation did not finish")
    }

    @MainActor func testReportFocusRefreshesRecentResultsOnEveryReturnAndCoalescesReads() async {
        let prefs = recentDefaults(), checked = expectation(description: "both returns checked")
        checked.expectedFulfillmentCount = 2
        let json = newReport
        let model = EnvironmentHealth(defaults: prefs, version: "9.0.1", run: { args in
            XCTAssertEqual(args, ["doctor", "--json", "--no-prompt"])
            checked.fulfill()
            return (0, json, "")
        })
        let controller = EnvironmentHealthController(model: model)
        let key = Notification(name: NSWindow.didBecomeKeyNotification)
        controller.windowDidBecomeKey(key)
        controller.windowDidBecomeKey(key)
        model.refreshForPresentation()
        XCTAssertTrue(model.checking)
        await settle(model)
        XCTAssertEqual(model.report?.findings, 0)
        XCTAssertEqual(model.report?.checkedAt, 200)

        // The user may have returned once before finishing the terminal prompts.
        controller.windowDidBecomeKey(key)
        await fulfillment(of: [checked], timeout: 2)
        await settle(model)
        XCTAssertFalse(model.failed)
        XCTAssertEqual(EnvironmentHealth(defaults: prefs, version: "9.0.1").report, model.report)
    }

    @MainActor func testFirstMenuReturnAfterRepairBypassesFreshnessWithoutApplyingRepairsAgain() async {
        let checked = expectation(description: "post-repair check"), opened = expectation(description: "opened")
        let json = newReport
        let model = EnvironmentHealth(defaults: recentDefaults(), version: "9.0.1", run: { args in
            if args == EnvironmentHealth.repairArguments { opened.fulfill(); return (0, "", "") }
            XCTAssertEqual(args, ["doctor", "--json", "--no-prompt"])
            checked.fulfill()
            return (0, json, "")
        })
        model.repair()
        await fulfillment(of: [opened], timeout: 2)
        await settle(model)
        XCTAssertTrue(model.repairOpened)
        XCTAssertEqual(model.report?.findings, 1, "Opening the terminal is not a repair receipt")
        model.refreshIfStale()
        model.refreshIfStale()
        await fulfillment(of: [checked], timeout: 2)
        await settle(model)
        XCTAssertEqual(model.report?.findings, 0)
        model.refreshIfStale()
        XCTAssertFalse(model.checking, "Ordinary menu opens retain the freshness interval")
    }

    @MainActor func testFailedReturnCheckKeepsOldFindingsAndDoesNotRetryByItself() async {
        let checked = expectation(description: "failed read")
        let model = EnvironmentHealth(defaults: recentDefaults(), version: "9.0.1", run: { args in
            XCTAssertEqual(args, ["doctor", "--json", "--no-prompt"])
            checked.fulfill()
            return (-2, "", "timeout")
        })
        EnvironmentHealthController(model: model).windowDidBecomeKey(Notification(name: NSWindow.didBecomeKeyNotification))
        await fulfillment(of: [checked], timeout: 2)
        await settle(model)
        XCTAssertTrue(model.failed)
        XCTAssertEqual(model.report?.findings, 1)
        XCTAssertEqual(model.report?.checkedAt, 100)
        model.refreshIfStale()
        XCTAssertFalse(model.checking)
    }

    @MainActor func testFailedTerminalOpenDoesNotForceAnUnrelatedMenuCheck() async {
        let opened = expectation(description: "failed open")
        let model = EnvironmentHealth(defaults: recentDefaults(), version: "9.0.1", run: { args in
            XCTAssertEqual(args, EnvironmentHealth.repairArguments)
            opened.fulfill()
            return (1, "", "cannot open terminal")
        })
        model.repair()
        await fulfillment(of: [opened], timeout: 2)
        await settle(model)
        XCTAssertTrue(model.repairFailed)
        model.refreshIfStale()
        XCTAssertFalse(model.checking)
        XCTAssertEqual(model.report?.findings, 1)
    }
}
