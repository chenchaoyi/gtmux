import AppKit
import ScreenCaptureKit

/// These short-lived references belong to this App instance. A window title or
/// a caller-supplied numeric window ID is never sufficient capture authority.
@MainActor final class ScreenshotAgentSources {
    struct Source {
        let reference: String
        let kind: String
        let id: UInt32
        let pid: pid_t
        let birth: Date?
        let bounds: CGRect
        let scale: CGFloat
        let title: String
        let expires: TimeInterval
    }
    private var sources: [String: Source] = [:]
    var uptime: () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }
    var recordingAllowed: () -> Bool = { CGPreflightScreenCaptureAccess() }

    func inventory(app: String?, displays: Bool) async throws -> [[String: Any]] {
        guard recordingAllowed() else { throw ScreenshotAgentFailure.refused("screen-recording-unavailable", "Screen Recording is not granted to the Gtmux App; no permission prompt was opened") }
        guard displays || !(app ?? "").isEmpty else { throw ScreenshotAgentFailure.refused("source-required", "Choose an app bundle identifier or explicitly list displays") }
        let content = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: true)
        sources = sources.filter { $0.value.expires > uptime() }
        guard sources.count+32 <= 128 else { throw ScreenshotAgentFailure.refused("source-budget", "Too many live source references; wait for older references to expire") }
        var result: [[String: Any]] = []
        if displays {
            for display in content.displays.prefix(32) {
                let bounds = CGDisplayBounds(display.displayID)
                let scale = NSScreen.screens.first { ($0.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID) == display.displayID }?.backingScaleFactor ?? 1
                result.append(insert(kind: "display", id: display.displayID, pid: 0, birth: nil, bounds: bounds, scale: scale, title: "Display \(display.displayID)"))
            }
        } else {
            for window in content.windows where window.owningApplication?.bundleIdentifier == app && window.windowLayer == 0 {
                guard result.count < 32, window.frame.width > 0, window.frame.height > 0,
                      let pid = window.owningApplication?.processID,
                      let process = NSRunningApplication(processIdentifier: pid), let birth = process.launchDate else { continue }
                let screen = NSScreen.screens.max { a, b in
                    let aRect = CGDisplayBounds(a.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID ?? 0).intersection(window.frame)
                    let bRect = CGDisplayBounds(b.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID ?? 0).intersection(window.frame)
                    return aRect.width*aRect.height < bRect.width*bRect.height
                }
                result.append(insert(kind: "window", id: window.windowID, pid: pid, birth: birth, bounds: window.frame, scale: screen?.backingScaleFactor ?? 1, title: window.title ?? ""))
            }
        }
        return result
    }

    func insert(kind: String, id: UInt32, pid: pid_t, birth: Date?, bounds: CGRect, scale: CGFloat, title: String) -> [String: Any] {
        let ref = UUID().uuidString.lowercased()
        sources[ref] = Source(reference: ref, kind: kind, id: id, pid: pid, birth: birth, bounds: bounds, scale: scale, title: title, expires: uptime()+30)
        return ["source_ref": ref, "source_kind": kind, "title": title, "window_or_display_id": id,
                "bounds_pt": [bounds.minX, bounds.minY, bounds.width, bounds.height], "scale": scale,
                "roi_origin": "source-top-left", "expires_in_seconds": 30]
    }

    func resolve(_ ref: String) throws -> Source {
        guard let source = sources[ref], source.expires > uptime() else { throw ScreenshotAgentFailure.refused("source-expired", "Observe the source again before capturing") }
        return source
    }

    func unchanged(_ source: Source) -> Bool {
        if source.kind == "display" { return CGDisplayIsActive(source.id) != 0 && CGDisplayBounds(source.id) == source.bounds }
        guard let process = NSRunningApplication(processIdentifier: source.pid), process.launchDate == source.birth,
              let windows = CGWindowListCopyWindowInfo(.optionIncludingWindow, source.id) as? [[String: Any]],
              let window = windows.first, let owner = window[kCGWindowOwnerPID as String] as? NSNumber,
              (window[kCGWindowIsOnscreen as String] as? NSNumber)?.boolValue == true,
              let bounds = window[kCGWindowBounds as String] as? [String: Any],
              let rect = CGRect(dictionaryRepresentation: bounds as CFDictionary) else { return false }
        return Self.matches(source, owner: owner.int32Value, birth: process.launchDate, bounds: rect)
    }

    static func matches(_ source: Source, owner: pid_t, birth: Date?, bounds: CGRect) -> Bool {
        return source.kind == "window" && source.pid == owner && source.birth != nil && source.birth == birth && source.bounds == bounds
    }

    func capture(reference: String, rect: [Double]?) async throws -> (CGImage, CGSize, CGRect, CGFloat, String) {
        guard recordingAllowed() else { throw ScreenshotAgentFailure.refused("screen-recording-unavailable", "Screen Recording is not granted; no permission prompt was opened") }
        let source = try resolve(reference)
        let roi = try Self.region(rect, in: source.bounds.size)
        let globalROI = roi.offsetBy(dx: source.bounds.minX, dy: source.bounds.minY)
        let scales = Set(NSScreen.screens.compactMap { screen -> CGFloat? in
            guard let id = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID,
                  CGDisplayBounds(id).intersects(globalROI) else { return nil }
            return screen.backingScaleFactor
        })
        guard scales.count <= 1 else { throw ScreenshotAgentFailure.refused("mixed-scale-region", "Capture separate regions for displays with different scales") }
        guard unchanged(source) else { throw ScreenshotAgentFailure.refused("source-changed", "The selected source moved, closed or changed owner; observe it again") }
        let config = ScreenshotCapture.configuration(pointSize: source.bounds.size, scale: source.scale, window: source.kind == "window")
        guard config.width > 0, config.height > 0, Int64(config.width)*Int64(config.height) <= ScreenshotAgentRendering.maximumPixels else {
            throw ScreenshotAgentFailure.refused("image-budget", "Selected source exceeds the image decode budget")
        }
        let content = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: true)
        let filter: SCContentFilter
        if source.kind == "display" {
            guard let display = content.displays.first(where: { $0.displayID == source.id }) else { throw ScreenshotAgentFailure.refused("source-unavailable", "The selected display is unavailable") }
            filter = SCContentFilter(display: display, excludingWindows: [])
        } else {
            guard let window = content.windows.first(where: { $0.windowID == source.id && $0.owningApplication?.processID == source.pid && $0.frame == source.bounds }) else {
                throw ScreenshotAgentFailure.refused("source-unavailable", "The selected window is unavailable")
            }
            filter = SCContentFilter(desktopIndependentWindow: window)
        }
        let image = try await SCScreenshotManager.captureImage(contentFilter: filter, configuration: config)
        guard unchanged(source) else { throw ScreenshotAgentFailure.refused("source-changed", "The source changed during capture; no result was published") }
        let scale = CGFloat(image.width)/source.bounds.width
        guard abs(CGFloat(image.height)/source.bounds.height-scale) < 0.02,
              let cropped = SelectionGeometry.crop(image, rect: roi, scale: scale) else { throw ScreenshotAgentFailure.refused("capture-geometry", "Captured geometry does not match the selected region") }
        let pixels = SelectionGeometry.pixelRect(roi, scale: scale, imageWidth: image.width, imageHeight: image.height)
        let actual = CGRect(x: pixels.minX/scale, y: pixels.minY/scale, width: pixels.width/scale, height: pixels.height/scale)
        return (cropped, actual.size, actual, scale, source.kind)
    }

    static func region(_ values: [Double]?, in size: CGSize) throws -> CGRect {
        guard let values else { return CGRect(origin: .zero, size: size) }
        guard values.count == 4, values.allSatisfy({ $0.isFinite }), values[0] >= 0, values[1] >= 0,
              values[2] > 0, values[3] > 0, values[0]+values[2] <= size.width, values[1]+values[3] <= size.height else {
            throw ScreenshotAgentFailure.refused("invalid-region", "Region must fit inside the explicitly selected source")
        }
        return CGRect(x: values[0], y: values[1], width: values[2], height: values[3])
    }
}
