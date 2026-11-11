import AppKit
import ScreenCaptureKit

/// These short-lived references belong to this App instance. A window title or
/// a caller-supplied numeric window ID is never sufficient capture authority.
@MainActor final class ScreenshotAgentSources {
    struct DisplayMapping: Equatable {
        let id: CGDirectDisplayID
        let bounds: CGRect
        let scale: CGFloat
    }
    struct WindowObservation {
        let owner: pid_t
        let birth: Date?
        let bounds: CGRect
    }
    struct Source {
        let reference: String
        let kind: String
        let id: UInt32
        let pid: pid_t
        let birth: Date?
        let bounds: CGRect
        let scale: CGFloat
        let displays: [DisplayMapping]
        let title: String
        let expires: TimeInterval
    }
    private var sources: [String: Source] = [:]
    var uptime: () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }
    var displaySnapshot: () -> [DisplayMapping] = {
        NSScreen.screens.compactMap { screen in
            guard let id = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID,
                  CGDisplayIsActive(id) != 0 else { return nil }
            return DisplayMapping(id: id, bounds: CGDisplayBounds(id), scale: screen.backingScaleFactor)
        }
    }
    var displayActive: (CGDirectDisplayID) -> Bool = { CGDisplayIsActive($0) != 0 }
    var windowObservation: (Source) -> WindowObservation? = { source in
        guard let process = NSRunningApplication(processIdentifier: source.pid),
              let windows = CGWindowListCopyWindowInfo(.optionIncludingWindow, source.id) as? [[String: Any]],
              let window = windows.first, let owner = window[kCGWindowOwnerPID as String] as? NSNumber,
              (window[kCGWindowIsOnscreen as String] as? NSNumber)?.boolValue == true,
              let bounds = window[kCGWindowBounds as String] as? [String: Any],
              let rect = CGRect(dictionaryRepresentation: bounds as CFDictionary) else { return nil }
        return WindowObservation(owner: owner.int32Value, birth: process.launchDate, bounds: rect)
    }
    var recordingAllowed: () -> Bool = { CGPreflightScreenCaptureAccess() }

    var contentProvider: (@escaping (Result<SCShareableContent, Error>) -> Void) -> Void = { done in
        SCShareableContent.getExcludingDesktopWindows(true, onScreenWindowsOnly: true) { content, error in
            if let content { done(.success(content)) }
            else { done(.failure(error ?? ScreenshotAgentFailure.refused("source-unavailable", "Screen content is unavailable"))) }
        }
    }
    var imageProvider: (SCContentFilter, SCStreamConfiguration, @escaping (Result<CGImage, Error>) -> Void) -> Void = { filter, configuration, done in
        SCScreenshotManager.captureImage(contentFilter: filter, configuration: configuration) { image, error in
            if let image { done(.success(image)) }
            else { done(.failure(error ?? ScreenshotAgentFailure.refused("capture-failed", "Screen image is unavailable"))) }
        }
    }

    private func content(_ context: ScreenshotAgentCancellation) async throws -> SCShareableContent {
        try await ScreenshotAgentWork.callback(context: context, admission: ScreenshotAgentWork.sdkAdmission, begin: contentProvider)
    }

    func inventory(app: String?, displays: Bool, context suppliedContext: ScreenshotAgentCancellation? = nil) async throws -> [[String: Any]] {
        let context = suppliedContext ?? ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+10)
        defer { if suppliedContext == nil { context.finish() } }
        try context.check()
        guard recordingAllowed() else { throw ScreenshotAgentFailure.refused("screen-recording-unavailable", "Screen Recording is not granted to the Gtmux App; no permission prompt was opened") }
        guard displays || !(app ?? "").isEmpty else { throw ScreenshotAgentFailure.refused("source-required", "Choose an app bundle identifier or explicitly list displays") }
        let content = try await content(context)
        try context.check()
        sources = sources.filter { $0.value.expires > uptime() }
        guard sources.count+32 <= 128 else { throw ScreenshotAgentFailure.refused("source-budget", "Too many live source references; wait for older references to expire") }
        var result: [[String: Any]] = []
        if displays {
            for display in content.displays.prefix(32) {
                let bounds = CGDisplayBounds(display.displayID)
                guard let scale = displaySnapshot().first(where: { $0.id == display.displayID && $0.bounds == bounds })?.scale,
                      scale.isFinite, scale > 0 else { continue }
                result.append(insert(kind: "display", id: display.displayID, pid: 0, birth: nil, bounds: bounds, scale: scale, title: "Display \(display.displayID)"))
            }
        } else {
            for window in content.windows where window.owningApplication?.bundleIdentifier == app && window.windowLayer == 0 {
                guard result.count < 32, window.frame.width > 0, window.frame.height > 0,
                      let pid = window.owningApplication?.processID,
                      let process = NSRunningApplication(processIdentifier: pid), let birth = process.launchDate else { continue }
                guard let screen = dominantDisplay(in: window.frame), screen.scale.isFinite, screen.scale > 0 else { continue }
                result.append(insert(kind: "window", id: window.windowID, pid: pid, birth: birth, bounds: window.frame, scale: screen.scale, title: window.title ?? ""))
            }
        }
        return result
    }

    func insert(kind: String, id: UInt32, pid: pid_t, birth: Date?, bounds: CGRect, scale: CGFloat, title: String) -> [String: Any] {
        let ref = UUID().uuidString.lowercased()
        sources[ref] = Source(reference: ref, kind: kind, id: id, pid: pid, birth: birth, bounds: bounds, scale: scale, displays: mapping(in: bounds), title: title, expires: uptime()+30)
        return ["source_ref": ref, "source_kind": kind, "title": title, "window_or_display_id": id,
                "bounds_pt": [bounds.minX, bounds.minY, bounds.width, bounds.height], "scale": scale,
                "roi_origin": "source-top-left", "expires_in_seconds": 30]
    }

    func resolve(_ ref: String) throws -> Source {
        guard let source = sources[ref], source.expires > uptime() else { throw ScreenshotAgentFailure.refused("source-expired", "Observe the source again before capturing") }
        return source
    }

    func mapping(in bounds: CGRect) -> [DisplayMapping] {
        displaySnapshot().filter { $0.bounds.intersects(bounds) }.sorted { $0.id < $1.id }
    }

    private func dominantDisplay(in bounds: CGRect) -> DisplayMapping? {
        mapping(in: bounds).max { a, b in
            let x = a.bounds.intersection(bounds), y = b.bounds.intersection(bounds)
            return x.width*x.height < y.width*y.height
        }
    }

    func unchanged(_ source: Source) -> Bool {
        let current = mapping(in: source.bounds)
        guard !current.isEmpty, current == source.displays,
              Set(current.map(\.id)).count == current.count,
              current.allSatisfy({ $0.scale.isFinite && $0.scale > 0 }),
              let expected = source.kind == "display" ? current.first(where: { $0.id == source.id }) : current.max(by: { a, b in
                  let x = a.bounds.intersection(source.bounds), y = b.bounds.intersection(source.bounds)
                  return x.width*x.height < y.width*y.height
              }),
              expected.scale == source.scale else { return false }
        if source.kind == "display" { return displayActive(source.id) && displaySnapshot().contains { $0.id == source.id && $0.bounds == source.bounds } }
        guard let observation = windowObservation(source) else { return false }
        return Self.matches(source, owner: observation.owner, birth: observation.birth, bounds: observation.bounds)
    }

    static func matches(_ source: Source, owner: pid_t, birth: Date?, bounds: CGRect) -> Bool {
        return source.kind == "window" && source.pid == owner && source.birth != nil && source.birth == birth && source.bounds == bounds
    }

    func capture(reference: String, rect: [Double]?, context: ScreenshotAgentCancellation) async throws -> (CGImage, CGSize, CGRect, CGFloat, String) {
        try context.check()
        guard recordingAllowed() else { throw ScreenshotAgentFailure.refused("screen-recording-unavailable", "Screen Recording is not granted; no permission prompt was opened") }
        let source = try resolve(reference)
        let roi = try Self.region(rect, in: source.bounds.size)
        let globalROI = roi.offsetBy(dx: source.bounds.minX, dy: source.bounds.minY)
        let scales = Set(mapping(in: globalROI).map(\.scale))
        guard scales.count <= 1 else { throw ScreenshotAgentFailure.refused("mixed-scale-region", "Capture separate regions for displays with different scales") }
        guard unchanged(source) else { throw ScreenshotAgentFailure.refused("source-changed", "The selected source moved, closed or changed owner; observe it again") }
        let config = ScreenshotCapture.configuration(pointSize: source.bounds.size, scale: source.scale, window: source.kind == "window")
        guard config.width > 0, config.height > 0, Int64(config.width)*Int64(config.height) <= ScreenshotAgentRendering.maximumPixels else {
            throw ScreenshotAgentFailure.refused("image-budget", "Selected source exceeds the image decode budget")
        }
        let content = try await content(context)
        try context.check()
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
        guard unchanged(source) else { throw ScreenshotAgentFailure.refused("source-changed", "The source changed while preparing capture; observe it again") }
        try context.check()
        let image: CGImage = try await ScreenshotAgentWork.callback(context: context, admission: ScreenshotAgentWork.sdkAdmission) { done in
            imageProvider(filter, config, done)
        }
        try context.check()
        return try finishCapture(image, source: source, roi: roi)
    }

    func finishCapture(_ image: CGImage, source: Source, roi: CGRect) throws -> (CGImage, CGSize, CGRect, CGFloat, String) {
        guard unchanged(source) else { throw ScreenshotAgentFailure.refused("source-changed", "The source changed during capture; no result was published") }
        let scale = CGFloat(image.width)/source.bounds.width
        guard abs(scale-source.scale) < 0.02,
              abs(CGFloat(image.height)/source.bounds.height-scale) < 0.02,
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
