import AppKit
import Foundation

/// Opt-in, bounded diagnostics for the synthetic cache-tail test only.
/// No published state, timers, or file I/O on the observed reveal path.
@MainActor enum HQConversationTrace {
    #if DEBUG
    private static var destination: URL?
    private static var started: UInt64 = 0
    private static var events: [[String: Any]] = []
    private static var dropped = 0
    private static var attempt = 0
    private static var phase = "entry"
    private static var geometry: [String: Int] = [:]
    #endif

    static var enabled: Bool {
        #if DEBUG
        return destination != nil
        #else
        return false
        #endif
    }

    static func begin() {
        #if DEBUG
        guard let path = ProcessInfo.processInfo.environment["GTMUX_CACHE_TAIL_TRACE_DIR"], !path.isEmpty else { return }
        destination = URL(fileURLWithPath: path, isDirectory: true)
        started = DispatchTime.now().uptimeNanoseconds
        events = []; events.reserveCapacity(4096)
        dropped = 0; attempt = 0; phase = "entry"; geometry = [:]
        record("case-begin", ["selector": "GtmuxBarTests.HQWorkspaceTests/testCachedConversationOpensAtLatestAndReselectionReturnsFromHistory"])
        #endif
    }

    static func record(_ event: String, _ fields: @autoclosure () -> [String: String] = [:]) {
        #if DEBUG
        guard enabled else { return }
        guard events.count < 4096 else { dropped += 1; return }
        events.append(["sequence": events.count, "ns": DispatchTime.now().uptimeNanoseconds - started,
                       "phase": phase, "event": event, "fields": fields()])
        #endif
    }

    static func nextAttempt() -> Int {
        #if DEBUG
        guard enabled else { return 0 }
        attempt += 1
        return attempt
        #else
        return 0
        #endif
    }

    static func setPhase(_ value: String) {
        #if DEBUG
        guard enabled else { return }
        phase = value
        record("phase")
        #endif
    }

    static func observeGeometry(owner: AnyObject, fields: @autoclosure () -> [String: String]) {
        #if DEBUG
        guard enabled else { return }
        geometry[identity(owner)] = events.count
        record("geometry", fields())
        #endif
    }

    static func geometrySequence(_ owner: AnyObject) -> String {
        #if DEBUG
        return geometry[identity(owner)].map(String.init) ?? "none"
        #else
        return "none"
        #endif
    }

    static func identity(_ object: AnyObject?) -> String {
        object.map { String(describing: ObjectIdentifier($0)) } ?? "nil"
    }

    /// Called only after the original initial assertions have already evaluated.
    static func captureFailure(_ view: NSView) {
        #if DEBUG
        guard let destination else { return }
        do {
            try FileManager.default.createDirectory(at: destination, withIntermediateDirectories: true)
            guard let bitmap = view.bitmapImageRepForCachingDisplay(in: view.bounds) else {
                record("capture-failed", ["reason": "no-bitmap"]); return
            }
            view.cacheDisplay(in: view.bounds, to: bitmap)
            guard let data = bitmap.representation(using: .png, properties: [:]), data.count <= 16 * 1024 * 1024 else {
                record("capture-failed", ["reason": "encoding-or-size"]); return
            }
            try data.write(to: destination.appendingPathComponent("entry-failure.png"))
            record("capture-written", ["bytes": String(data.count)])
        } catch { record("capture-failed", ["reason": String(describing: error)]) }
        #endif
    }

    static func finish() throws {
        #if DEBUG
        guard let destination else { return }
        defer { self.destination = nil; events = []; geometry = [:] }
        record("case-end")
        try FileManager.default.createDirectory(at: destination, withIntermediateDirectories: true)
        var data = Data()
        for event in events {
            data.append(try JSONSerialization.data(withJSONObject: event, options: [.sortedKeys]))
            data.append(0x0a)
        }
        data.append(try JSONSerialization.data(withJSONObject: ["event": "trace-summary", "dropped": dropped,
            "events": events.count, "complete": dropped == 0], options: [.sortedKeys]))
        data.append(0x0a)
        guard data.count <= 8 * 1024 * 1024 else {
            throw NSError(domain: "HQConversationTrace", code: 1, userInfo: [NSLocalizedDescriptionKey: "Trace exceeds 8 MiB"])
        }
        try data.write(to: destination.appendingPathComponent("cache-tail.jsonl"))
        #endif
    }
}
