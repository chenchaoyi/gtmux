import Foundation
import React

// Registers the Stage 2 selection overlay as the legacy view-manager component
// `TermSelectionView` (manager name minus the "Manager" suffix). The app runs
// the new architecture; RN's interop layer hosts legacy view managers
// automatically, and NativeTerm.tsx mounts it via
// requireNativeComponent('TermSelectionView') — iOS only.
@objc(TermSelectionViewManager)
class TermSelectionViewManager: RCTViewManager {
  override func view() -> UIView! { TermSelectionView() }
  override static func requiresMainQueueSetup() -> Bool { true }
}
