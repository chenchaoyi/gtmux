import Foundation
import React
#if targetEnvironment(simulator)
import UIKit
#endif

// DebugSettings — a launch-time debug channel for UI automation, gated entirely by
// `GTMUX_DEBUG_*` flags from two places: Documents/gtmux-debug-flags.json, which the e2e
// harness writes before launch (launchWithFlags), and launch environment variables of the
// same names, which override the file (see constantsToExport). A normal launch has
// neither, so production behavior is unchanged.
//
// JS reads the flags as constants (NativeModules.DebugSettings.flags) and writes
// structured events with `record(...)`, which append to a JSONL file in the
// app's Documents dir — an e2e test reads it post-run via:
//   xcrun simctl get_app_container booted com.gtmux.app data → Documents/gtmux-debug.jsonl
@objc(DebugSettings)
class DebugSettings: NSObject {

  @objc static func requiresMainQueueSetup() -> Bool { false }

  // Export debug flags to JS as `flags`. Two sources, merged (env wins): a JSON
  // file the e2e harness writes before launch (deterministic — XCUITest's
  // launchEnvironment is cached per-session and unreliable on relaunch), then
  // GTMUX_DEBUG_* launch env. Keys are the full GTMUX_DEBUG_* names.
  @objc func constantsToExport() -> [AnyHashable: Any]! {
    var flags: [String: String] = [:]
    if let url = flagsURL, let data = try? Data(contentsOf: url),
       let obj = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] {
      for (k, v) in obj { flags[k] = "\(v)" }
    }
    for (k, v) in ProcessInfo.processInfo.environment where k.hasPrefix("GTMUX_DEBUG_") {
      flags[k] = v
    }
    return ["flags": flags]
  }

  private var docs: URL? {
    FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
  }
  private var logURL: URL? { docs?.appendingPathComponent("gtmux-debug.jsonl") }
  private var flagsURL: URL? { docs?.appendingPathComponent("gtmux-debug-flags.json") }

  // Append one line (a JSON object) to the debug log.
  @objc(record:)
  func record(_ line: NSString) {
    guard let url = logURL, let data = ((line as String) + "\n").data(using: .utf8) else { return }
    if let fh = try? FileHandle(forWritingTo: url) {
      defer { try? fh.close() }
      fh.seekToEndOfFile()
      fh.write(data)
    } else {
      try? data.write(to: url)
    }
  }

  // Truncate the debug log (call once at app start when logging is on).
  @objc func reset() {
    guard let url = logURL else { return }
    try? FileManager.default.removeItem(at: url)
  }
}

// Test-only lifecycle breadcrumbs: no notification contents, credentials or text.
private let notificationTraceLock = NSLock()
func traceNotification(_ event: String, action: String? = nil, hasText: Bool = false, geometry: [String: Any]? = nil) {
  guard let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first,
        let data = try? Data(contentsOf: docs.appendingPathComponent("gtmux-debug-flags.json")),
        let flags = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
        flags["GTMUX_DEBUG_LOG_NET"] as? String == "1" else { return }
  notificationTraceLock.lock()
  defer { notificationTraceLock.unlock() }
  var line: [String: Any] = ["event": event, "action": action ?? "", "hasText": hasText, "time": Date().timeIntervalSince1970]
  if let geometry { line["geometry"] = geometry }
  guard var bytes = try? JSONSerialization.data(withJSONObject: line) else { return }
  bytes.append(10)
  let url = docs.appendingPathComponent("gtmux-native-notification-trace.jsonl")
  if !FileManager.default.fileExists(atPath: url.path) { FileManager.default.createFile(atPath: url.path, contents: nil) }
  if let file = try? FileHandle(forWritingTo: url) {
    defer { try? file.close() }
    _ = try? file.seekToEnd()
    try? file.write(contentsOf: bytes)
  }
}

#if targetEnvironment(simulator)
// Weak keys bind the identifier to the actual UIView, not a scene or a cached
// "last host" number. Replacing a root must get a new number, even in one scene.
private let sceneHostIDs = NSMapTable<UIView, NSNumber>(keyOptions: [.weakMemory, .objectPointerPersonality], valueOptions: .strongMemory)
private var nextSceneHostID = 0

func traceSceneHost(_ event: String, root: UIView, window: UIWindow?) {
  guard Thread.isMainThread,
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first,
        let data = try? Data(contentsOf: docs.appendingPathComponent("gtmux-debug-flags.json")),
        let flags = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
        flags["GTMUX_DEBUG_LOG_NET"] as? String == "1" else { return }
  let host: NSNumber
  if let existing = sceneHostIDs.object(forKey: root) {
    host = existing
  } else {
    nextSceneHostID += 1
    host = NSNumber(value: nextSceneHostID)
    sceneHostIDs.setObject(host, forKey: root)
  }
  func rect(_ value: CGRect) -> [String: Double] {
    ["x": Double(value.origin.x), "y": Double(value.origin.y),
     "width": Double(value.width), "height": Double(value.height)]
  }
  var geometry: [String: Any] = [
    "pid": ProcessInfo.processInfo.processIdentifier, "host": host,
    "rootBounds": rect(root.bounds), "hasWindow": window != nil,
    "rootAttached": window != nil && root.window === window,
    "scenePresent": window?.windowScene != nil,
    "sceneState": window?.windowScene?.activationState.rawValue ?? -1,
  ]
  if let window {
    geometry["windowBounds"] = rect(window.bounds)
    geometry["rootInWindow"] = rect(root.convert(root.bounds, to: window))
    let insets = window.safeAreaInsets
    geometry["insets"] = ["top": Double(insets.top), "right": Double(insets.right),
                          "bottom": Double(insets.bottom), "left": Double(insets.left)]
  }
  traceNotification(event, geometry: geometry)
}
#endif
