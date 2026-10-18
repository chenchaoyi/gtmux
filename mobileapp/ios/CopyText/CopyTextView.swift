import UIKit

/// Whole-block Copy feedback without changing React Native's text layout or links.
@available(iOS 16.0, *)
final class CopyTextView: UIView, UIEditMenuInteractionDelegate {
  @objc var copyText: String = "" {
    didSet { if copyText != oldValue { dismissSelection() } }
  }
  @objc var copyLabel: String = "Copy"
  private lazy var editMenu = UIEditMenuInteraction(delegate: self)
  private let highlight = CALayer()
  private(set) var activeSelection: UIEditMenuConfiguration?
  private var selectionID: UUID? { activeSelection?.identifier as? UUID }

  override init(frame: CGRect) {
    super.init(frame: frame)
    addInteraction(editMenu)
    let press = UILongPressGestureRecognizer(target: self, action: #selector(longPress(_:)))
    addGestureRecognizer(press)
    highlight.backgroundColor = UIColor.systemBlue.withAlphaComponent(0.28).cgColor
    highlight.cornerRadius = 3
  }

  required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }

  override func layoutSubviews() {
    super.layoutSubviews()
    CATransaction.begin()
    CATransaction.setDisableActions(true)
    highlight.frame = bounds
    CATransaction.commit()
  }

  override func didMoveToWindow() {
    super.didMoveToWindow()
    if window == nil { dismissSelection() }
  }

  @objc private func longPress(_ recognizer: UILongPressGestureRecognizer) {
    if recognizer.state == .began { presentSelection(at: recognizer.location(in: self)) }
    else if recognizer.state == .cancelled { dismissSelection() }
  }

  func presentSelection(at point: CGPoint) {
    guard window != nil, !copyText.isEmpty, selectionID == nil else { return }
    let id = UUID()
    activeSelection = UIEditMenuConfiguration(identifier: id as NSUUID, sourcePoint: point)
    highlight.frame = bounds
    layer.insertSublayer(highlight, at: 0)
    editMenu.presentEditMenu(with: activeSelection!)
  }

  func dismissSelection() {
    activeSelection = nil
    highlight.removeFromSuperlayer()
    editMenu.dismissMenu()
  }

  func editMenuInteraction(_ interaction: UIEditMenuInteraction, menuFor configuration: UIEditMenuConfiguration,
                           suggestedActions: [UIMenuElement]) -> UIMenu? {
    guard let id = configuration.identifier as? UUID, id == selectionID else { return nil }
    return UIMenu(children: [UIAction(title: copyLabel, image: UIImage(systemName: "doc.on.doc")) { [weak self] _ in
      self?.copySelection(id)
    }])
  }

  func copySelection(_ id: UUID) {
    guard selectionID == id else { return }
    UIPasteboard.general.string = copyText
  }

  func editMenuInteraction(_ interaction: UIEditMenuInteraction, willDismissMenuFor configuration: UIEditMenuConfiguration,
                           animator: any UIEditMenuInteractionAnimating) {
    // A late dismissal must not clear a newer block selection.
    guard let id = configuration.identifier as? UUID, id == selectionID else { return }
    highlight.removeFromSuperlayer()
    // UIKit may dispatch the chosen action while the menu is dismissing. Keep
    // that snapshot valid through the animation, then invalidate it. A content
    // change or explicit dismissal still invalidates immediately.
    animator.addCompletion { [weak self] in
      if self?.selectionID == id { self?.activeSelection = nil }
    }
  }
}
