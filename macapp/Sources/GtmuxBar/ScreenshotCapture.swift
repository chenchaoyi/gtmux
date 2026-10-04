import AppKit
import CoreGraphics

/// Screen Recording permission, as the capture sees it. The two calls are injectable so
/// the flow can be tested without TCC.
struct ScreenCaptureAccess {
    var preflight: () -> Bool = { CGPreflightScreenCaptureAccess() }
    /// Asks macOS once; the first call shows the system prompt and registers the app in
    /// the Screen Recording list. Returns whether access is granted now (usually not until
    /// the user flips the switch, and often only after the app is reopened).
    var request: () -> Bool = { CGRequestScreenCaptureAccess() }
    /// The System Settings pane where the switch lives.
    static let settingsURL = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture")!
}

enum CaptureOutcome: Equatable {
    case image(URL)
    case cancelled
    case failed(String)
}

/// The region capture, done by the system's own interactive tool. Apple's selection UI
/// already handles several displays, Retina and mixed scales, the crosshair, Space for a
/// window and Esc to cancel, which is the part of a screenshot tool that is hard to get
/// right and that users already know. A child of this app is attributed to it by TCC, so
/// it needs Gtmux's Screen Recording permission like any in-process capture would.
enum ScreenshotCapture {
    static let tool = "/usr/sbin/screencapture"

    /// -i interactive, -x no sound, -o no window shadow, PNG. The DPI metadata stays in
    /// (no -r): the editor reads it to show the image at its real point size.
    static func arguments(output: URL) -> [String] {
        ["-i", "-x", "-o", "-t", "png", output.path]
    }

    /// Esc leaves no file (or an empty one); anything else is the image.
    static func outcome(fileAt url: URL, exitStatus: Int32) -> CaptureOutcome {
        let size = (try? FileManager.default.attributesOfItem(atPath: url.path)[.size] as? NSNumber)?.intValue ?? 0
        if size > 0 { return .image(url) }
        try? FileManager.default.removeItem(at: url)
        // screencapture exits non-zero on a cancelled selection too; without a file there
        // is nothing to show either way, and an error dialog for Esc would be wrong.
        return exitStatus == 0 || exitStatus == 1 ? .cancelled : .failed("screencapture exited \(exitStatus)")
    }

    /// A fresh path for one capture, in the app's own temporary directory.
    static func temporaryURL(now: Date = Date()) -> URL {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyyMMdd-HHmmss"
        return FileManager.default.temporaryDirectory
            .appendingPathComponent("gtmux-shot-\(f.string(from: now))-\(UUID().uuidString.prefix(6)).png")
    }

    /// Runs the interactive capture and reports on the main queue.
    static func run(output: URL, completion: @escaping (CaptureOutcome) -> Void) {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: tool)
        p.arguments = arguments(output: output)
        p.terminationHandler = { proc in
            let result = outcome(fileAt: output, exitStatus: proc.terminationStatus)
            DispatchQueue.main.async { completion(result) }
        }
        do {
            try p.run()
        } catch {
            DispatchQueue.main.async { completion(.failed(error.localizedDescription)) }
        }
    }
}
