import Foundation
import React
import UIKit

// Haptics — the tap you feel when a long press is RECOGNISED.
//
// The app had none: a long press on a radar row dimmed the row to 0.6 and then, 350ms
// later, a sheet appeared. Nothing in between said "this is happening", and nothing at
// the moment of recognition said "it happened" (user report, 2026-09-10). On iOS the
// impact at recognition IS the language of a long-press menu — every system context menu
// fires one — and its absence is what reads as "the long press has no feedback".
//
// Written here rather than pulled in: the repo already writes its own small native
// modules (the Live Activity, the terminal's selection overlay, the debug channel), and
// this is thirty lines against a third-party dependency and its build surface.
//
// Generators are kept and prepared, not built per call: `prepare()` warms the Taptic
// Engine so the impact lands with the gesture instead of a beat after it.
@objc(Haptics)
class Haptics: NSObject {

  @objc static func requiresMainQueueSetup() -> Bool { true }

  private lazy var light = UIImpactFeedbackGenerator(style: .light)
  private lazy var medium = UIImpactFeedbackGenerator(style: .medium)
  private lazy var selection = UISelectionFeedbackGenerator()

  /// Warm the engine ahead of an impact we expect within the next moment — called when a
  /// finger goes down, so the long press that may follow fires with no warm-up latency.
  @objc func prepare() {
    DispatchQueue.main.async { [self] in
      medium.prepare()
    }
  }

  /// One impact. `kind` is "light" | "medium" | "selection"; anything else is medium, so a
  /// JS typo is a haptic rather than silence.
  @objc func impact(_ kind: NSString) {
    DispatchQueue.main.async { [self] in
      switch kind as String {
      case "light": light.impactOccurred()
      case "selection": selection.selectionChanged()
      default: medium.impactOccurred()
      }
    }
  }
}
