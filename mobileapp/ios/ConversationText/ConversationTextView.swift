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
#if targetEnvironment(simulator)
  private var lastDiagnostic: Data?
  private var renderingCacheKey = ""
  private var renderingCache: [String: Any] = [:]
#endif

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
#if targetEnvironment(simulator)
    if TextLayoutEvidence.enabled { textView.accessibilityIdentifier = "duo-text-" + UUID().uuidString }
#endif
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
#if targetEnvironment(simulator)
    recordTextLayout(measuredHeight: height)
#endif
  }

#if targetEnvironment(simulator)
  // Opt-in fixture evidence only. UITextInput geometry keeps the active TextKit
  // engine and selection untouched; no conversation content enters this file.
  private func recordTextLayout(measuredHeight: CGFloat) {
    guard TextLayoutEvidence.enabled, let identifier = textView.accessibilityIdentifier, let window, !isHidden,
          let range = textView.textRange(from: textView.beginningOfDocument, to: textView.endOfDocument),
          let attributed = textView.attributedText, attributed.length > 0 else { return }
    func rect(_ r: CGRect) -> [String: CGFloat] {
      ["x": r.origin.x, "y": r.origin.y, "width": r.width, "height": r.height]
    }
    let rectangles = textView.selectionRects(for: range).map { textView.convert($0.rect, from: textView.textInputView) }
    let union = rectangles.reduce(CGRect.null) { $0.union($1) }
    let last = (attributed.string as NSString).rangeOfComposedCharacterSequence(at: attributed.length - 1)
    guard let start = textView.position(from: textView.beginningOfDocument, offset: last.location),
          let end = textView.position(from: start, offset: last.length),
          let tail = textView.textRange(from: start, to: end) else { return }
    let lastRect = textView.convert(textView.firstRect(for: tail), from: textView.textInputView)
    var points = Set<CGFloat>()
    attributed.enumerateAttribute(.font, in: NSRange(location: 0, length: attributed.length)) { font, _, _ in
      if let font = font as? UIFont { points.insert(font.pointSize) }
    }
    let record: [String: Any] = [
      "identifier": identifier, "revision": revision, "scale": displayed["scale"] ?? 1,
      "selectionActive": selectionActive, "queued": queued != nil,
      "fontPoints": points.sorted(), "utf16Length": attributed.length,
      "frame": rect(textView.convert(textView.bounds, to: window)),
      "bounds": rect(textView.bounds), "measuredHeight": measuredHeight,
      "selectionRectCount": rectangles.count, "selectionBounds": rect(union),
      "lastCharacterRect": rect(lastRect),
      "rendering": renderingEvidence(length: attributed.length),
    ]
    guard JSONSerialization.isValidJSONObject(record),
          let bytes = try? JSONSerialization.data(withJSONObject: record, options: [.sortedKeys]), bytes != lastDiagnostic else { return }
    lastDiagnostic = bytes
    TextLayoutEvidence.record(identifier, record)
  }

  // Read the active engine: accessing layoutManager first would permanently
  // switch TextKit 2 to compatibility mode and invalidate this experiment.
  // These conservative rendering surfaces are not selection rectangles or a
  // claim that every pixel inside the surface contains a visible glyph.
  private func renderingEvidence(length: Int) -> [String: Any] {
    let key = "\(revision):\(textView.bounds.size):\(window?.screen.scale ?? 1)"
    if key == renderingCacheKey { return renderingCache }
    func rect(_ r: CGRect) -> [String: CGFloat] {
      ["x": r.origin.x, "y": r.origin.y, "width": r.width, "height": r.height]
    }
    let inset = textView.textContainerInset
    if #available(iOS 16.0, *), let manager = textView.textLayoutManager {
      guard let content = manager.textContentManager else {
        return ["engine": "TextKit2", "complete": false, "utf16Coverage": 0]
      }
      var fragments: [[String: Any]] = []
      var surface = CGRect.null
      var covered = 0
      var complete = true
      var activeFragments: [NSTextLayoutFragment] = []
      manager.enumerateTextLayoutFragments(from: nil, options: [.ensuresLayout]) { fragment in
        let start = content.offset(from: content.documentRange.location, to: fragment.rangeInElement.location)
        let end = content.offset(from: content.documentRange.location, to: fragment.rangeInElement.endLocation)
        complete = complete && start == covered && end >= start && fragment.state == .layoutAvailable
        covered = end
        let frame = fragment.layoutFragmentFrame
        let render = fragment.renderingSurfaceBounds.offsetBy(dx: frame.minX + inset.left, dy: frame.minY + inset.top)
        surface = surface.union(render)
        fragments.append(["start": start, "end": end, "state": fragment.state.rawValue,
                          "layout": rect(frame), "rendering": rect(render)])
        activeFragments.append(fragment)
        return true
      }
      var result: [String: Any] = ["engine": "TextKit2", "complete": complete && covered == length && !fragments.isEmpty,
                                   "utf16Coverage": covered, "fragments": fragments]
      if !surface.isNull { result["bounds"] = rect(surface) }
      // Draw the existing fragments into a padded reference surface, without
      // the UITextView clip. Conservative surface bounds alone are not ink.
      let ink = TextInkEvidence.capture(surface: surface, scale: window?.screen.scale ?? 1) { context in
        for fragment in activeFragments {
          let frame = fragment.layoutFragmentFrame
          fragment.draw(at: CGPoint(x: frame.minX + inset.left, y: frame.minY + inset.top), in: context)
        }
      }
      result["ink"] = ink.json
      if complete && covered == length && ink.complete {
        renderingCacheKey = key
        renderingCache = result
      }
      return result
    }
    // Legacy access is safe only when no TextKit 2 manager is present.
    let manager = textView.layoutManager
    let glyphs = manager.glyphRange(for: textView.textContainer)
    let characters = manager.characterRange(forGlyphRange: glyphs, actualGlyphRange: nil)
    let surface = manager.boundingRect(forGlyphRange: glyphs, in: textView.textContainer)
      .offsetBy(dx: inset.left, dy: inset.top)
    return ["engine": "TextKit1", "complete": characters.location == 0 && characters.length == length,
            "utf16Coverage": characters.length, "glyphCount": glyphs.length, "bounds": rect(surface)]
  }
#endif

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

#if targetEnvironment(simulator)
private enum TextLayoutEvidence {
  private static let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
  static let enabled: Bool = {
    guard let url = docs?.appendingPathComponent("gtmux-debug-flags.json"),
          let data = try? Data(contentsOf: url),
          let flags = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { return false }
    return flags["GTMUX_DEBUG_TEXT_LAYOUT"] as? String == "1"
  }()
  private static var records: [String: [String: Any]] = [:]
  static func record(_ id: String, _ value: [String: Any]) {
    if records[id] == nil && records.count >= 128 { return }
    records[id] = value
    guard let url = docs?.appendingPathComponent("gtmux-text-layout.json"),
          let data = try? JSONSerialization.data(withJSONObject: records, options: [.sortedKeys]) else { return }
    try? data.write(to: url, options: .atomic)
  }
}
#endif
