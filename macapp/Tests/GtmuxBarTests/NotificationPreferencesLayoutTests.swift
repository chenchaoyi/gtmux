import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class NotificationPreferencesLayoutTests: XCTestCase {
    @MainActor private func render(_ state: NotificationManager.TestState,
                                   permission: DesktopNotificationAuthorization.Permission = .allowed,
                                   width: CGFloat = 460, scheme: ColorScheme = .light) throws -> Data {
        let content = Form {
            NotificationPreferences(l10n: L10n.shared, enabled: .constant(true),
                authorization: DesktopNotificationAuthorization(permission: permission, banners: true),
                testState: state, tabAlert: .constant(true), tabAlertBusy: false,
                sendTest: { XCTFail("Rendering must not submit notifications") },
                openSystemSettings: { XCTFail("Rendering must not open settings") })
        }
        .formStyle(.grouped).frame(width: width, height: 500)
        .environment(\.colorScheme, scheme)
        // Grouped Form uses AppKit-backed views, which ImageRenderer omits.
        let host = NSHostingView(rootView: content)
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: width, height: 500),
                              styleMask: [.borderless], backing: .buffered, defer: false)
        window.contentView = host
        host.frame = window.contentLayoutRect
        host.layoutSubtreeIfNeeded()
        RunLoop.current.run(until: Date().addingTimeInterval(0.05))
        host.layoutSubtreeIfNeeded()
        let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
        host.cacheDisplay(in: host.bounds, to: bitmap)
        return try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
    }

    @MainActor func testTestFeedbackAndPermissionStatesRemainVisible() throws {
        let mode = L10n.shared.mode
        defer { L10n.shared.mode = mode }
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            let idle = try render(.idle)
            XCTAssertNotEqual(idle, try render(.accepted), "Submission feedback must be drawn next to its action")
            XCTAssertNotEqual(idle, try render(.blocked, permission: .denied), "Denied permission must remain visible")
            for scheme in [ColorScheme.light, .dark] {
                for width: CGFloat in [360, 460] {
                    for (name, state, permission) in [
                        ("idle", NotificationManager.TestState.idle, DesktopNotificationAuthorization.Permission.allowed),
                        ("accepted", .accepted, .allowed), ("denied", .blocked, .denied), ("failed", .failed, .allowed),
                    ] {
                        let data = try render(state, permission: permission, width: width, scheme: scheme)
                        if let dir = ProcessInfo.processInfo.environment["GTMUX_PREFERENCES_RENDER_DIR"] {
                            let folder = URL(fileURLWithPath: dir, isDirectory: true)
                            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
                            try data.write(to: folder.appendingPathComponent("\(language.rawValue)-\(scheme)-\(Int(width))-\(name).png"))
                        }
                    }
                }
            }
        }
    }
}
