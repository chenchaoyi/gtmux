// Isolated UIKit lifecycle checks. No React Native, network or real device data.
import UIKit

final class Animator: NSObject, UIEditMenuInteractionAnimating {
  var deferCompletion = false
  var completion: (() -> Void)?
  func addAnimations(_ animations: @escaping () -> Void) { animations() }
  func addCompletion(_ completion: @escaping () -> Void) {
    if deferCompletion { self.completion = completion } else { completion() }
  }
}

@main
final class TestApp: UIResponder, UIApplicationDelegate {
  var window: UIWindow?

  func application(_ application: UIApplication, didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
    let window = UIWindow(frame: UIScreen.main.bounds)
    let controller = UIViewController()
    controller.view.backgroundColor = .systemBackground
    window.rootViewController = controller
    self.window = window
    window.makeKeyAndVisible()
    DispatchQueue.main.async { self.runChecks(in: controller.view) }
    return true
  }

  private func runChecks(in parent: UIView) {
    let block = CopyTextView(frame: CGRect(x: 24, y: 180, width: 320, height: 100))
    parent.addSubview(block)
    let label = UILabel(frame: block.bounds)
    label.text = "A highlighted paragraph\n中文 bold link 😀"
    label.numberOfLines = 0
    block.addSubview(label)
    block.copyText = label.text!
    block.copyLabel = "Copy"
    let menu = block.interactions.compactMap { $0 as? UIEditMenuInteraction }.first!
    let originalLayers = block.layer.sublayers?.count ?? 0
    block.presentSelection(at: CGPoint(x: 40, y: 30))
    let first = block.activeSelection!
    let firstID = first.identifier as! UUID
    precondition((block.layer.sublayers?.count ?? 0) == originalLayers + 1, "missing highlight")
    block.presentSelection(at: .zero)
    precondition(block.activeSelection === first, "duplicate long press replaced the menu")
    let actions = block.editMenuInteraction(menu, menuFor: first, suggestedActions: [])!.children
    precondition((actions.first as? UIAction)?.title == "Copy")
    block.copySelection(firstID)
    precondition(UIPasteboard.general.string == label.text, "wrong copy scope")
    let animator = Animator()
    animator.deferCompletion = true
    block.editMenuInteraction(menu, willDismissMenuFor: first, animator: animator)
    UIPasteboard.general.string = "unchanged"
    let copyButton = UIButton(primaryAction: actions.first as! UIAction)
    copyButton.sendActions(for: .primaryActionTriggered)
    precondition(UIPasteboard.general.string == label.text, "action during dismissal did not copy")
    animator.completion?()
    precondition(block.activeSelection == nil && (block.layer.sublayers?.count ?? 0) == originalLayers, "dismiss left highlight")

    block.presentSelection(at: .zero)
    let second = block.activeSelection!
    block.editMenuInteraction(menu, willDismissMenuFor: first, animator: Animator())
    precondition(block.activeSelection === second, "late dismissal cleared a newer selection")
    UIPasteboard.general.string = "unchanged"
    block.copySelection(firstID)
    precondition(UIPasteboard.general.string == "unchanged", "stale menu copied text")
    block.copyText = "new content"
    precondition(block.activeSelection == nil && (block.layer.sublayers?.count ?? 0) == originalLayers, "live update left stale highlight")
    block.copySelection(second.identifier as! UUID)
    precondition(UIPasteboard.general.string == "unchanged", "live update let a stale menu copy new text")
    block.copyLabel = "复制"
    block.presentSelection(at: .zero)
    let third = block.activeSelection!
    precondition((block.editMenuInteraction(menu, menuFor: third, suggestedActions: [])!.children.first as? UIAction)?.title == "复制")
    block.removeFromSuperview()
    precondition(block.activeSelection == nil, "removed block kept a menu")
    parent.addSubview(block)
    block.copyText = ""
    block.presentSelection(at: .zero)
    precondition(block.activeSelection == nil, "empty block opened a menu")
    print("COPY_TEXT_CHECKS_PASSED: highlight, copy, dismiss, duplicate, stale callback, live update, localization, removal, empty")
    fflush(stdout)
    if ProcessInfo.processInfo.environment["GTMUX_COPY_PREVIEW"] == "1" {
      block.copyText = label.text!
      block.copyLabel = "Copy"
      DispatchQueue.main.asyncAfter(deadline: .now() + 1) { block.presentSelection(at: CGPoint(x: 40, y: 30)) }
    } else {
      exit(0)
    }
  }
}
