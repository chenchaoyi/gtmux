import UIKit

/// One TextKit layout owns both the rich text and its native selection geometry.
final class ConversationTextView: UIView, UITextViewDelegate, UIGestureRecognizerDelegate {
  @objc var document: NSDictionary = [:] { didSet { acceptDocument() } }
  @objc var onSize: (([String: Any]) -> Void)?
  @objc var onSelection: (([String: Any]) -> Void)?
  @objc var onLink: (([String: Any]) -> Void)?
  let textView = ConversationSelectionTextView()
  private var displayed: NSDictionary = [:]
  private var queued: NSDictionary?
  private var applying = false
  private var selectionActive = false
  private var lastSize: CGSize = .zero
  private var lastRevision = ""
  private var naturalWidth: CGFloat = 1
  private var outsideTap: UITapGestureRecognizer?

  override init(frame: CGRect) {
    super.init(frame: frame)
    textView.isEditable = false
    textView.isSelectable = true
    textView.isScrollEnabled = false
    textView.backgroundColor = .clear
    textView.textContainerInset = .zero
    textView.textContainer.lineFragmentPadding = 0
    textView.contentInset = .zero
    textView.dataDetectorTypes = []
    textView.linkTextAttributes = [.underlineStyle: NSUnderlineStyle.single.rawValue]
    textView.delegate = self
    textView.onResign = { [weak self] in self?.updateSelection(false) }
    addSubview(textView)
  }

  required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

  private var revision: String { displayed["revision"] as? String ?? "" }

  private func acceptDocument() {
    if selectionActive { queued = document; return }
    guard !document.isEqual(displayed) else { return }
    displayed = document
    applying = true
    let base = displayed["style"] as? [String: Any] ?? [:]
    let scale = (displayed["scale"] as? NSNumber)?.doubleValue ?? 1
    let attributed = NSMutableAttributedString(string: "")
    for run in displayed["runs"] as? [[String: Any]] ?? [] {
      let style = base.merging(run["style"] as? [String: Any] ?? [:]) { _, new in new }
      var attributes = Self.attributes(style, scale: scale)
      if let index = run["link"] as? Int, let url = URL(string: "gtmux-text-link:\(index)") { attributes[.link] = url }
      attributed.append(NSAttributedString(string: run["text"] as? String ?? "", attributes: attributes))
    }
    naturalWidth = max(1, ceil(attributed.boundingRect(with: CGSize(width: 200000, height: CGFloat.greatestFiniteMagnitude), options: [.usesLineFragmentOrigin, .usesFontLeading], context: nil).width))
    textView.attributedText = attributed
    textView.selectedRange = NSRange(location: 0, length: 0)
    applying = false
    lastSize = .zero
    setNeedsLayout()
  }

  override func layoutSubviews() {
    super.layoutSubviews()
    textView.frame = bounds
    let unbounded = displayed["unbounded"] as? Bool == true
    // A temporary unresolved frame is not a measurement of wrapped prose.
    // Bounded RN text starts at its available width; only horizontal code uses
    // its natural width before the bridge has committed a measured frame.
    guard unbounded || bounds.width > 1 else { return }
    let width = unbounded ? naturalWidth : bounds.width
    let size = textView.sizeThatFits(CGSize(width: width, height: .greatestFiniteMagnitude))
    let height = max(1, ceil(size.height))
    if lastSize.width != bounds.width || lastSize.height != height || lastRevision != revision {
      lastSize = CGSize(width: bounds.width, height: height)
      lastRevision = revision
      onSize?(["height": height, "width": naturalWidth, "revision": revision])
    }
  }

  override func didMoveToWindow() {
    super.didMoveToWindow()
    if window == nil { clearSelection() }
  }

  func clearSelection() {
    textView.selectedRange = NSRange(location: 0, length: 0)
    _ = textView.resignFirstResponder()
    updateSelection(false)
  }

  func textViewDidChangeSelection(_ textView: UITextView) {
    guard !applying else { return }
    updateSelection(textView.selectedRange.length > 0)
  }

  func textViewDidEndEditing(_ textView: UITextView) { updateSelection(false) }

  private func updateSelection(_ active: Bool) {
    guard active != selectionActive else { return }
    selectionActive = active
    if active, outsideTap == nil, let window {
      let tap = UITapGestureRecognizer(target: self, action: #selector(tappedOutside))
      tap.cancelsTouchesInView = false
      tap.delegate = self
      window.addGestureRecognizer(tap)
      outsideTap = tap
    } else if !active, let tap = outsideTap {
      tap.view?.removeGestureRecognizer(tap)
      outsideTap = nil
    }
    onSelection?(["active": active, "revision": revision])
    if !active, let next = queued {
      queued = nil
      document = next
    }
  }

  @objc private func tappedOutside() { clearSelection() }

  func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer, shouldReceive touch: UITouch) -> Bool {
    !bounds.contains(touch.location(in: self))
  }

  func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer,
                         shouldRecognizeSimultaneouslyWith otherGestureRecognizer: UIGestureRecognizer) -> Bool { true }

  func textView(_ textView: UITextView, shouldInteractWith URL: URL, in characterRange: NSRange,
                interaction: UITextItemInteraction) -> Bool {
    guard interaction == .invokeDefaultAction, URL.scheme == "gtmux-text-link",
          let index = Int(URL.absoluteString.dropFirst("gtmux-text-link:".count)) else { return false }
    onLink?(["index": index, "revision": revision])
    return false
  }

  static func attributes(_ style: [String: Any], scale: Double) -> [NSAttributedString.Key: Any] {
    let fs = max(8, (style["fontSize"] as? NSNumber)?.doubleValue ?? 14) * max(0.5, min(scale, 4))
    let family = style["fontFamily"] as? String ?? ""
    let base = UIFont(name: family, size: fs) ?? (family.isEmpty ? UIFont.systemFont(ofSize: fs) : UIFont.monospacedSystemFont(ofSize: fs, weight: .regular))
    var traits = base.fontDescriptor.symbolicTraits
    let weight = String(describing: style["fontWeight"] ?? "400")
    if weight == "bold" || (Int(weight) ?? 0) >= 600 { traits.insert(.traitBold) }
    if style["fontStyle"] as? String == "italic" { traits.insert(.traitItalic) }
    let font = base.fontDescriptor.withSymbolicTraits(traits).map { UIFont(descriptor: $0, size: fs) } ?? base
    let paragraph = NSMutableParagraphStyle()
    if let line = (style["lineHeight"] as? NSNumber)?.doubleValue, line > 0 {
      paragraph.minimumLineHeight = line * scale
      paragraph.maximumLineHeight = line * scale
    }
    if style["textAlign"] as? String == "center" { paragraph.alignment = .center }
    if style["textAlign"] as? String == "right" { paragraph.alignment = .right }
    var result: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: color(style["color"] as? String) ?? UIColor.label, .paragraphStyle: paragraph]
    if let background = color(style["backgroundColor"] as? String) { result[.backgroundColor] = background }
    if let spacing = style["letterSpacing"] as? NSNumber { result[.kern] = spacing.doubleValue * scale }
    let decoration = style["textDecorationLine"] as? String ?? ""
    if decoration.contains("underline") { result[.underlineStyle] = NSUnderlineStyle.single.rawValue }
    if decoration.contains("line-through") { result[.strikethroughStyle] = NSUnderlineStyle.single.rawValue }
    return result
  }

  static func color(_ value: String?) -> UIColor? {
    guard let value else { return nil }
    if value.hasPrefix("#") {
      let hex = String(value.dropFirst())
      guard (hex.count == 6 || hex.count == 8), let bits = UInt64(hex, radix: 16) else { return nil }
      let shift = hex.count == 8 ? 8 : 0
      return UIColor(red: CGFloat((bits >> (16 + shift)) & 255) / 255,
                     green: CGFloat((bits >> (8 + shift)) & 255) / 255,
                     blue: CGFloat((bits >> shift) & 255) / 255,
                     alpha: hex.count == 8 ? CGFloat(bits & 255) / 255 : 1)
    }
    if value.hasPrefix("rgba("), value.hasSuffix(")") {
      let numbers = value.dropFirst(5).dropLast().split(separator: ",").compactMap { Double($0.trimmingCharacters(in: .whitespaces)) }
      if numbers.count == 4 { return UIColor(red: numbers[0]/255, green: numbers[1]/255, blue: numbers[2]/255, alpha: numbers[3]) }
    }
    return nil
  }
}

final class ConversationSelectionTextView: UITextView {
  var onResign: (() -> Void)?
  override func resignFirstResponder() -> Bool {
    let resigned = super.resignFirstResponder()
    if resigned { onResign?() }
    return resigned
  }
}
