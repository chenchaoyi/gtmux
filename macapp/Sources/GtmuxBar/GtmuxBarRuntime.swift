import AppKit

/// Shared entry point for the SwiftPM launcher and the Xcode app bundle.
public enum GtmuxBarRuntime {
    public static func run() {
        // Apply the CLI's private-file policy before the app writes anything.
        umask(0o077)
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.accessory)
        withExtendedLifetime(delegate) { app.run() }
    }
}
