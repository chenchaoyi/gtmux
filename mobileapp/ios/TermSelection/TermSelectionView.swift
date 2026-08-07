import UIKit
import CoreText
import React

// TermSelectionView — the mobile-native-term-selection Stage 2 layer: a
// transparent view mounted (by NativeTerm.tsx, iOS only) absoluteFill over the
// Stage 1 uniform row grid, implementing the READ-ONLY subset of UITextInput so
// the SYSTEM selection UI (UITextSelectionDisplayInteraction's band +
// both-direction lollipop handles, UITextLoupeSession's loupe,
// UIEditMenuInteraction's Copy menu) draws directly over gtmux's own colored
// terminal rendering, driven by our own gestures.
//
// Why this exists (proposal §Why): every text-view-based attempt misaligned,
// because UITextView owns its own TextKit layout, which drifts from RN Text's
// on wrapped CJK lines. Here the GEOMETRY IS OURS: the JS side hands us the
// grid's plain text (visual rows joined by '\n'), the uniform rowHeight, and
// the font — so a row is pure arithmetic (row = floor(y / rowHeight), y = row ×
// rowHeight; Stage 1 guarantees a row never native-wraps) and the in-row x↔char
// mapping comes from measured Core Text advances on the SAME font RN Text
// renders with (same shaping engine — CJK fallback advances are measured, never
// assumed 2×cell). Zero alignment risk by construction.
//
// Touch model: the view is INVISIBLE to touches until a selection is active
// (point(inside:) returns false → links/scroll on the layers below are
// untouched; the JS side additionally mounts it pointerEvents="box-none" so the
// new-arch interop wrapper can't swallow touches either). Activation comes from
// a long-press recognizer attached to the ENCLOSING scroll view (an ancestor of
// the touched Text blocks, so it sees touches this view passes through): press
// → select the word under the finger; keep dragging → extend; release → edit
// menu. From then on the view IS hit-testable: the handle pan drives the
// drags (loupe riding along), a tap outside the band clears (and the view
// goes pass-through again). While active, JS freezes the snapshot
// (onSelectionActive) so the text under the band can't change mid-selection.
//
// Offsets are UTF-16 code-unit offsets into rows.joined("\n") — the same units
// CTLine string indices use, so geometry and document positions never convert.

// MARK: - Position / Range / SelectionRect primitives

final class TermTextPosition: UITextPosition {
  let offset: Int
  init(_ offset: Int) {
    self.offset = offset
    super.init()
  }
  override func isEqual(_ object: Any?) -> Bool {
    guard let o = object as? TermTextPosition else { return false }
    return o.offset == offset
  }
  override var hash: Int { offset }
}

final class TermTextRange: UITextRange {
  let lo: Int
  let hi: Int
  init(_ a: Int, _ b: Int) {
    lo = min(a, b)
    hi = max(a, b)
    super.init()
  }
  override var start: UITextPosition { TermTextPosition(lo) }
  override var end: UITextPosition { TermTextPosition(hi) }
  override var isEmpty: Bool { lo == hi }
}

final class TermSelectionRect: UITextSelectionRect {
  private let r: CGRect
  private let s: Bool
  private let e: Bool
  init(_ rect: CGRect, containsStart: Bool = false, containsEnd: Bool = false) {
    r = rect
    s = containsStart
    e = containsEnd
    super.init()
  }
  override var rect: CGRect { r }
  override var writingDirection: NSWritingDirection { .leftToRight }
  override var containsStart: Bool { s }
  override var containsEnd: Bool { e }
  override var isVertical: Bool { false }
}

// MARK: - The view

final class TermSelectionView: UIView {

  // ── Props (set via RCT_EXPORT_VIEW_PROPERTY / KVC from TermSelection.m) ──
  @objc var text: String = "" {
    didSet { if text != oldValue { rebuild() } }
  }
  @objc var rowHeight: CGFloat = 19 {
    didSet { if rowHeight != oldValue { clearSelection() } }
  }
  @objc var fontSize: CGFloat = 12 {
    didSet { if fontSize != oldValue { fontChanged() } }
  }
  @objc var fontName: String = "Menlo" {
    didSet { if fontName != oldValue { fontChanged() } }
  }
  @objc var padTop: CGFloat = 0
  @objc var padLeft: CGFloat = 0
  @objc var menuLang: String = "en"
  // Hex string ("#RRGGBB"), parsed here — NOT a processColor number: the
  // new-arch interop layer decodes those as RGBA while processColor packs
  // ARGB, which turned the blue tint red (sim-verified).
  @objc var selectionTint: String = "" {
    didSet { if let c = Self.color(fromHex: selectionTint) { tintColor = c } }
  }
  @objc var onSelectionActive: RCTDirectEventBlock?

  private static func color(fromHex hex: String) -> UIColor? {
    var s = hex.trimmingCharacters(in: .whitespaces)
    guard s.hasPrefix("#") else { return nil }
    s.removeFirst()
    guard s.count == 6, let v = UInt32(s, radix: 16) else { return nil }
    return UIColor(
      red: CGFloat((v >> 16) & 0xFF) / 255.0,
      green: CGFloat((v >> 8) & 0xFF) / 255.0,
      blue: CGFloat(v & 0xFF) / 255.0,
      alpha: 1)
  }

  // ── Document model: visual rows (never re-wrapped here — Stage 1 wrapped) ──
  private var doc: NSString = ""
  private var rows: [String] = [""]
  private var rowStarts: [Int] = [0] // UTF-16 offset of each row's first char
  private var rowLens: [Int] = [0] // UTF-16 length of each row
  private var docLen = 0

  // Lazily measured Core Text lines, one per touched row (x↔char mapping).
  private var ctCache: [Int: CTLine] = [:]
  private var fontCache: UIFont?

  // ── Selection state ──
  private var sel: (lo: Int, hi: Int)? // always normalized lo ≤ hi
  private var reportedActive = false
  private var pressAnchor: (lo: Int, hi: Int)? // the initially selected word
  private var menuTimer: Timer?

  weak var inputDelegate: UITextInputDelegate?
  lazy var tokenizer: UITextInputTokenizer = UITextInputStringTokenizer(textInput: self)

  private var editMenu: NSObject? // UIEditMenuInteraction (iOS 16+)
  private var selDisplay: NSObject? // UITextSelectionDisplayInteraction (iOS 17+)
  private var loupe: NSObject? // UITextLoupeSession (iOS 17+), live during a drag
  private weak var pressHost: UIView? // the enclosing UIScrollView the long-press rides on
  private var press: UILongPressGestureRecognizer?
  private var handlePan: UIPanGestureRecognizer?
  private var panAnchor: Int? // the fixed end while a handle drag moves the other
  // Suppresses the keyboard if the system ever makes us first responder for the
  // selection session: a zero-frame input view renders nothing.
  private let blankInput = UIView(frame: .zero)

  override init(frame: CGRect) {
    super.init(frame: frame)
    backgroundColor = .clear
    isUserInteractionEnabled = true
    // NOTE deliberately NO UITextInteraction: attached alongside our own
    // machinery it FIGHTS it (sim-verified on iOS 26 — during a handle drag its
    // internal gesture spammed empty selectedTextRange sets every frame, which
    // cleared the selection mid-drag). Everything it would provide is
    // implemented explicitly here instead: activation long-press + drag-extend
    // (the scroll-view recognizer), handle drags (handlePan), band+handles
    // (UITextSelectionDisplayInteraction), loupe (UITextLoupeSession), menu
    // (UIEditMenuInteraction), tap-away (tap recognizer).
    if #available(iOS 16.0, *) {
      let em = UIEditMenuInteraction(delegate: self)
      addInteraction(em)
      editMenu = em
    }
    // The system SELECTION VISUALS (band + the two lollipop handles). A custom
    // UITextInput does NOT get them from UITextInteraction alone on modern iOS
    // (sim-verified on iOS 26: only a transient underline during the gesture) —
    // UITextSelectionDisplayInteraction is the public piece that draws them,
    // fed by our selectionRects/caretRect and poked via setNeedsSelectionUpdate.
    if #available(iOS 17.0, *) {
      let di = UITextSelectionDisplayInteraction(textInput: self, delegate: self)
      addInteraction(di)
      selDisplay = di
    }
    // Tap-away clears (tap inside the band re-summons the edit menu).
    let tap = UITapGestureRecognizer(target: self, action: #selector(onTap(_:)))
    tap.delegate = self
    addGestureRecognizer(tap)
    // Handle drags, ours deterministically: a pan that only RECEIVES touches
    // beginning on a selection edge (shouldReceive below), so plain pans
    // inside the band's rows still scroll the terminal.
    let pan = UIPanGestureRecognizer(target: self, action: #selector(onHandlePan(_:)))
    pan.delegate = self
    addGestureRecognizer(pan)
    handlePan = pan
  }

  required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

  // MARK: pass-through until active

  // The whole trick: while no selection is active this view does not exist to
  // hit-testing — link taps land on the Text blocks below, scrolls on the
  // scroll view. Activation comes from the scroll-view long-press instead.
  override func point(inside point: CGPoint, with event: UIEvent?) -> Bool {
    isActive && super.point(inside: point, with: event)
  }

  private var isActive: Bool {
    if let s = sel { return s.lo != s.hi }
    return false
  }

  // MARK: activation long-press (on the enclosing scroll view)

  override func didMoveToWindow() {
    super.didMoveToWindow()
    if window == nil {
      detachPress()
      clearSelection()
    } else {
      attachPress()
    }
  }

  private func attachPress() {
    guard press == nil else { return }
    var v: UIView? = superview
    while let cur = v, !(cur is UIScrollView) { v = cur.superview }
    guard let scroll = v else { return }
    let lp = UILongPressGestureRecognizer(target: self, action: #selector(onPress(_:)))
    lp.minimumPressDuration = 0.45
    lp.delegate = self
    scroll.addGestureRecognizer(lp)
    // A handle grab must beat the scroll: when handlePan receives the touch
    // (only ever on a selection edge), the scroll's own pan waits for its
    // outcome; for every other touch handlePan isn't participating, so this
    // adds no scroll latency.
    if let pan = handlePan { (scroll as? UIScrollView)?.panGestureRecognizer.require(toFail: pan) }
    press = lp
    pressHost = scroll
  }

  private func detachPress() {
    if let lp = press { pressHost?.removeGestureRecognizer(lp) }
    press = nil
    pressHost = nil
  }

  @objc private func onPress(_ g: UILongPressGestureRecognizer) {
    let pt = g.location(in: self)
    switch g.state {
    case .began:
      beginSelection(at: pt)
      if isActive { loupeBegin(at: pt) }
    case .changed:
      guard let anchor = pressAnchor else { return }
      let o = clampedOffset(at: pt)
      if o < anchor.lo {
        setSelection((o, anchor.hi))
      } else if o > anchor.hi {
        setSelection((anchor.lo, o))
      } else {
        setSelection(anchor)
      }
      loupeMove(to: pt)
    case .ended, .cancelled, .failed:
      loupeEnd()
      guard isActive else { return }
      scheduleMenu(after: 0.05)
    default:
      break
    }
  }

  private func beginSelection(at pt: CGPoint) {
    guard docLen > 0 else { return }
    let (row, _) = rowCol(of: clampedOffset(at: pt))
    guard rowLens[row] > 0 else { return } // an empty row has nothing to select
    let w = wordRange(at: clampedOffset(at: pt))
    guard w.lo != w.hi else { return }
    pressAnchor = w
    // First responder BEFORE the selection so the system input stack sees the
    // change; blankInput keeps any keyboard from rising.
    _ = becomeFirstResponder()
    if #available(iOS 17.0, *) { (selDisplay as? UITextSelectionDisplayInteraction)?.isActivated = true }
    setSelection(w)
  }

  // Which selection END a touch at `pt` grabs, if any: the lo/hi caret rect
  // inflated to a comfortable finger zone (the system handle knobs sit just
  // above the start / below the end caret, covered by the vertical slop).
  private func grabbedEnd(at pt: CGPoint) -> (end: Int, anchor: Int)? {
    guard let s = sel, s.lo != s.hi else { return nil }
    let a = caretRect(for: TermTextPosition(s.lo)).insetBy(dx: -22, dy: -24)
    let b = caretRect(for: TermTextPosition(s.hi)).insetBy(dx: -22, dy: -24)
    let inA = a.contains(pt)
    let inB = b.contains(pt)
    if inA && inB { // ends close together — pick the nearer
      let da = abs(pt.x - a.midX) + abs(pt.y - a.midY)
      let db = abs(pt.x - b.midX) + abs(pt.y - b.midY)
      return da <= db ? (s.lo, s.hi) : (s.hi, s.lo)
    }
    if inA { return (s.lo, s.hi) }
    if inB { return (s.hi, s.lo) }
    return nil
  }

  // Handle drag: move the grabbed end, the other end anchored; crossing the
  // anchor swaps naturally (min/max). The loupe rides along on iOS 17+.
  @objc private func onHandlePan(_ g: UIPanGestureRecognizer) {
    let pt = g.location(in: self)
    switch g.state {
    case .began:
      guard let grab = grabbedEnd(at: pt) else {
        g.state = .cancelled
        return
      }
      panAnchor = grab.anchor
      dismissMenu()
      menuTimer?.invalidate()
      loupeBegin(at: pt)
    case .changed:
      guard let anchor = panAnchor else { return }
      let o = clampedOffset(at: pt)
      if o != anchor { setSelection((min(anchor, o), max(anchor, o))) }
      loupeMove(to: pt)
    case .ended, .cancelled, .failed:
      loupeEnd()
      if panAnchor != nil, isActive { scheduleMenu(after: 0.15) }
      panAnchor = nil
    default:
      break
    }
  }

  // ── Loupe (iOS 17+) ──

  private func loupeBegin(at pt: CGPoint) {
    guard #available(iOS 17.0, *), loupe == nil else { return }
    loupe = UITextLoupeSession.begin(at: pt, fromSelectionWidgetView: nil, in: self)
  }

  private func loupeMove(to pt: CGPoint) {
    guard #available(iOS 17.0, *), let l = loupe as? UITextLoupeSession else { return }
    l.move(to: pt, withCaretRect: caretRect(for: TermTextPosition(clampedOffset(at: pt))), trackingCaret: false)
  }

  private func loupeEnd() {
    guard #available(iOS 17.0, *), let l = loupe as? UITextLoupeSession else { return }
    l.invalidate()
    loupe = nil
  }

  // Tap outside the band clears; tap INSIDE it re-summons the edit menu (the
  // menu's outside-touch dismissal consumes one touch, so this is how a user
  // gets Copy back after dismissing it — same pattern as native text views).
  @objc private func onTap(_ g: UITapGestureRecognizer) {
    guard isActive else { return }
    let pt = g.location(in: self)
    let inBand = selectionRects(for: TermTextRange(sel!.lo, sel!.hi))
      .contains { $0.rect.insetBy(dx: -8, dy: -4).contains(pt) }
    if inBand {
      scheduleMenu(after: 0.05)
    } else {
      clearSelection()
    }
  }

  // MARK: selection plumbing

  // Programmatic mutation: bracket with inputDelegate notifications so the
  // system input stack sees the change (selectionMutated then pokes the
  // display interaction to redraw band + handles).
  private func setSelection(_ r: (lo: Int, hi: Int)?) {
    inputDelegate?.selectionWillChange(self)
    sel = r
    inputDelegate?.selectionDidChange(self)
    selectionMutated()
  }

  func clearSelection() {
    menuTimer?.invalidate()
    guard sel != nil || reportedActive else { return }
    dismissMenu()
    setSelection(nil)
    pressAnchor = nil
    if isFirstResponder { resignFirstResponder() }
  }

  private func selectionMutated() {
    if #available(iOS 17.0, *), let di = selDisplay as? UITextSelectionDisplayInteraction {
      di.setNeedsSelectionUpdate()
      // The system handle views carry their OWN internal gesture recognizers;
      // left interactive, a grab landing on a knob is swallowed by them (and
      // goes nowhere without UITextInteraction) instead of reaching handlePan.
      // The visuals don't need touches — OUR pan owns the drags.
      for v in di.handleViews { v.isUserInteractionEnabled = false }
      di.cursorView.isUserInteractionEnabled = false
      di.highlightView.isUserInteractionEnabled = false
    }
    let active = isActive
    if active != reportedActive {
      reportedActive = active
      onSelectionActive?(["active": active])
      if !active {
        dismissMenu()
        menuTimer?.invalidate()
        if #available(iOS 17.0, *) { (selDisplay as? UITextSelectionDisplayInteraction)?.isActivated = false }
        if isFirstResponder { resignFirstResponder() }
      }
    }
  }

  private func rebuild() {
    doc = text as NSString
    rows = text.components(separatedBy: "\n")
    var starts: [Int] = []
    var lens: [Int] = []
    starts.reserveCapacity(rows.count)
    lens.reserveCapacity(rows.count)
    var o = 0
    for r in rows {
      starts.append(o)
      let l = (r as NSString).length
      lens.append(l)
      o += l + 1 // the joining "\n"
    }
    rowStarts = starts
    rowLens = lens
    docLen = doc.length
    ctCache.removeAll()
    clearSelection()
  }

  private func fontChanged() {
    fontCache = nil
    ctCache.removeAll()
    clearSelection()
  }

  // MARK: geometry (rows by arithmetic, in-row x by Core Text)

  private func font() -> UIFont {
    if let f = fontCache { return f }
    let f = UIFont(name: fontName, size: fontSize)
      ?? UIFont.monospacedSystemFont(ofSize: fontSize, weight: .regular)
    fontCache = f
    return f
  }

  private func ctLine(_ row: Int) -> CTLine {
    if let l = ctCache[row] { return l }
    let attr = NSAttributedString(string: rows[row], attributes: [.font: font()])
    let l = CTLineCreateWithAttributedString(attr)
    ctCache[row] = l
    return l
  }

  private func rowY(_ row: Int) -> CGFloat { padTop + CGFloat(row) * rowHeight }

  private func xFor(_ row: Int, _ col: Int) -> CGFloat {
    padLeft + CGFloat(CTLineGetOffsetForStringIndex(ctLine(row), col, nil))
  }

  private func rowTextWidth(_ row: Int) -> CGFloat { xFor(row, rowLens[row]) }

  private func colFor(_ row: Int, x: CGFloat) -> Int {
    let idx = CTLineGetStringIndexForPosition(ctLine(row), CGPoint(x: x - padLeft, y: 0))
    if idx == kCFNotFound { return 0 }
    return max(0, min(idx, rowLens[row]))
  }

  // offset → (row, col). Binary search; an offset sitting ON a joining "\n"
  // resolves to the end of its row.
  private func rowCol(of offset: Int) -> (row: Int, col: Int) {
    var lo = 0
    var hi = rowStarts.count - 1
    while lo < hi {
      let mid = (lo + hi + 1) / 2
      if rowStarts[mid] <= offset { lo = mid } else { hi = mid - 1 }
    }
    return (lo, min(max(0, offset - rowStarts[lo]), rowLens[lo]))
  }

  private func clampedOffset(at pt: CGPoint) -> Int {
    guard !rows.isEmpty else { return 0 }
    let row = max(0, min(rows.count - 1, Int(floor((pt.y - padTop) / max(rowHeight, 1)))))
    return rowStarts[row] + colFor(row, x: pt.x)
  }

  private func wordRange(at offset: Int) -> (lo: Int, hi: Int) {
    let p = TermTextPosition(offset)
    for dir in [UITextStorageDirection.backward, .forward] {
      if let r = tokenizer.rangeEnclosingPosition(p, with: .word, inDirection: UITextDirection.storage(dir)) as? TermTextRange,
         !r.isEmpty {
        return (r.lo, r.hi)
      }
    }
    // Whitespace / symbols: fall back to the composed character under the touch.
    guard docLen > 0 else { return (0, 0) }
    let idx = min(offset, docLen - 1)
    let r = doc.rangeOfComposedCharacterSequence(at: idx)
    return (r.location, r.location + r.length)
  }

  // MARK: edit menu (Copy)

  private func scheduleMenu(after t: TimeInterval) {
    menuTimer?.invalidate()
    menuTimer = Timer.scheduledTimer(withTimeInterval: t, repeats: false) { [weak self] _ in
      self?.presentMenu()
    }
  }

  private func presentMenu() {
    guard isActive else { return }
    guard #available(iOS 16.0, *), let em = editMenu as? UIEditMenuInteraction else { return }
    let rects = selectionRects(for: TermTextRange(sel!.lo, sel!.hi)).map { $0.rect }
    guard let u = rects.dropFirst().reduce(rects.first, { $0?.union($1) }) else { return }
    em.presentEditMenu(with: UIEditMenuConfiguration(identifier: nil, sourcePoint: CGPoint(x: u.midX, y: u.minY)))
  }

  private func dismissMenu() {
    if #available(iOS 16.0, *) { (editMenu as? UIEditMenuInteraction)?.dismissMenu() }
  }

  private func copySelection() {
    guard let s = sel, s.lo != s.hi else { return }
    UIPasteboard.general.string = doc.substring(with: NSRange(location: s.lo, length: s.hi - s.lo))
    dismissMenu()
  }

  // MARK: responder plumbing

  override var canBecomeFirstResponder: Bool { true }
  override var inputView: UIView? { blankInput }

  override func canPerformAction(_ action: Selector, withSender sender: Any?) -> Bool {
    action == #selector(copy(_:)) && isActive
  }

  override func copy(_ sender: Any?) { copySelection() }

  // UITextInputTraits (via UITextInput) — keep the system input machinery inert.
  @objc var autocorrectionType: UITextAutocorrectionType = .no
  @objc var spellCheckingType: UITextSpellCheckingType = .no
  @objc var smartQuotesType: UITextSmartQuotesType = .no
  @objc var smartDashesType: UITextSmartDashesType = .no
  @objc var smartInsertDeleteType: UITextSmartInsertDeleteType = .no
}

// MARK: - UITextInput (read-only subset)

extension TermSelectionView: UITextInput {

  // UIKeyInput — read-only: nothing ever mutates the document.
  var hasText: Bool { docLen > 0 }
  func insertText(_ text: String) {}
  func deleteBackward() {}
  func replace(_ range: UITextRange, withText text: String) {}

  func text(in range: UITextRange) -> String? {
    guard let r = range as? TermTextRange else { return nil }
    let lo = max(0, min(r.lo, docLen))
    let hi = max(lo, min(r.hi, docLen))
    return doc.substring(with: NSRange(location: lo, length: hi - lo))
  }

  var selectedTextRange: UITextRange? {
    get { sel.map { TermTextRange($0.lo, $0.hi) } }
    set {
      // System-initiated (e.g. the tokenizer machinery): store WITHOUT
      // re-notifying the inputDelegate (the system already knows), then
      // re-arm the edit menu once the change settles.
      let next = (newValue as? TermTextRange).map { (lo: $0.lo, hi: $0.hi) }
      let changed = next?.lo != sel?.lo || next?.hi != sel?.hi
      sel = next
      selectionMutated()
      if changed && isActive {
        dismissMenu()
        scheduleMenu(after: 0.5)
      }
    }
  }

  var markedTextRange: UITextRange? { nil }
  var markedTextStyle: [NSAttributedString.Key: Any]? {
    get { nil }
    set {}
  }
  func setMarkedText(_ markedText: String?, selectedRange: NSRange) {}
  func unmarkText() {}

  var beginningOfDocument: UITextPosition { TermTextPosition(0) }
  var endOfDocument: UITextPosition { TermTextPosition(docLen) }

  func textRange(from fromPosition: UITextPosition, to toPosition: UITextPosition) -> UITextRange? {
    guard let a = fromPosition as? TermTextPosition, let b = toPosition as? TermTextPosition else { return nil }
    return TermTextRange(a.offset, b.offset)
  }

  func position(from position: UITextPosition, offset: Int) -> UITextPosition? {
    guard let p = position as? TermTextPosition else { return nil }
    let o = p.offset + offset
    guard o >= 0 && o <= docLen else { return nil }
    return TermTextPosition(o)
  }

  func position(from position: UITextPosition, in direction: UITextLayoutDirection, offset: Int) -> UITextPosition? {
    guard let p = position as? TermTextPosition else { return nil }
    switch direction {
    case .right:
      return self.position(from: p, offset: offset)
    case .left:
      return self.position(from: p, offset: -offset)
    case .up, .down:
      let (row, col) = rowCol(of: p.offset)
      let nr = max(0, min(rows.count - 1, row + (direction == .down ? offset : -offset)))
      return TermTextPosition(rowStarts[nr] + min(col, rowLens[nr]))
    @unknown default:
      return nil
    }
  }

  func compare(_ position: UITextPosition, to other: UITextPosition) -> ComparisonResult {
    guard let a = position as? TermTextPosition, let b = other as? TermTextPosition else { return .orderedSame }
    if a.offset < b.offset { return .orderedAscending }
    if a.offset > b.offset { return .orderedDescending }
    return .orderedSame
  }

  func offset(from: UITextPosition, to toPosition: UITextPosition) -> Int {
    guard let a = from as? TermTextPosition, let b = toPosition as? TermTextPosition else { return 0 }
    return b.offset - a.offset
  }

  func position(within range: UITextRange, farthestIn direction: UITextLayoutDirection) -> UITextPosition? {
    guard let r = range as? TermTextRange else { return nil }
    switch direction {
    case .left, .up: return TermTextPosition(r.lo)
    case .right, .down: return TermTextPosition(r.hi)
    @unknown default: return nil
    }
  }

  func characterRange(byExtending position: UITextPosition, in direction: UITextLayoutDirection) -> UITextRange? {
    guard let p = position as? TermTextPosition else { return nil }
    let (row, _) = rowCol(of: p.offset)
    switch direction {
    case .left, .up: return TermTextRange(rowStarts[row], p.offset)
    case .right, .down: return TermTextRange(p.offset, rowStarts[row] + rowLens[row])
    @unknown default: return nil
    }
  }

  func baseWritingDirection(for position: UITextPosition, in direction: UITextStorageDirection) -> NSWritingDirection {
    .leftToRight
  }
  func setBaseWritingDirection(_ writingDirection: NSWritingDirection, for range: UITextRange) {}

  // ── Geometry the selection UI consumes ──

  func firstRect(for range: UITextRange) -> CGRect {
    guard let r = range as? TermTextRange, r.lo != r.hi else {
      return caretRect(for: (range.start as? TermTextPosition) ?? TermTextPosition(0))
    }
    let (ra, ca) = rowCol(of: r.lo)
    let (rb, cb) = rowCol(of: r.hi)
    let xa = xFor(ra, ca)
    let xb = ra == rb ? xFor(rb, cb) : max(rowTextWidth(ra), xa + 2)
    return CGRect(x: xa, y: rowY(ra), width: max(xb - xa, 2), height: rowHeight)
  }

  func caretRect(for position: UITextPosition) -> CGRect {
    guard let p = position as? TermTextPosition else { return .zero }
    let (row, col) = rowCol(of: p.offset)
    return CGRect(x: xFor(row, col), y: rowY(row), width: 2, height: rowHeight)
  }

  func selectionRects(for range: UITextRange) -> [UITextSelectionRect] {
    guard let r = range as? TermTextRange, r.lo != r.hi else { return [] }
    let (ra, ca) = rowCol(of: r.lo)
    let (rb, cb) = rowCol(of: r.hi)
    let xa = xFor(ra, ca)
    if ra == rb {
      let xb = xFor(rb, cb)
      return [TermSelectionRect(
        CGRect(x: xa, y: rowY(ra), width: max(xb - xa, 2), height: rowHeight),
        containsStart: true, containsEnd: true)]
    }
    var out: [UITextSelectionRect] = []
    // First row: from the start x to that row's text end.
    let wa = max(rowTextWidth(ra), xa + 2)
    out.append(TermSelectionRect(
      CGRect(x: xa, y: rowY(ra), width: wa - xa, height: rowHeight), containsStart: true))
    // Middle rows: one merged full-width band (uniform rows make this exact).
    if rb - ra > 1 {
      out.append(TermSelectionRect(CGRect(
        x: padLeft, y: rowY(ra + 1),
        width: max(bounds.width - padLeft, 2), height: CGFloat(rb - ra - 1) * rowHeight)))
    }
    // Last row: from the left edge to the end x.
    let xb = xFor(rb, cb)
    out.append(TermSelectionRect(
      CGRect(x: padLeft, y: rowY(rb), width: max(xb - padLeft, 2), height: rowHeight),
      containsEnd: true))
    return out
  }

  func closestPosition(to point: CGPoint) -> UITextPosition? {
    TermTextPosition(clampedOffset(at: point))
  }

  func closestPosition(to point: CGPoint, within range: UITextRange) -> UITextPosition? {
    guard let r = range as? TermTextRange else { return nil }
    let o = clampedOffset(at: point)
    return TermTextPosition(max(r.lo, min(o, r.hi)))
  }

  func characterRange(at point: CGPoint) -> UITextRange? {
    guard docLen > 0 else { return nil }
    let o = min(clampedOffset(at: point), docLen - 1)
    let r = doc.rangeOfComposedCharacterSequence(at: o)
    return TermTextRange(r.location, r.location + r.length)
  }

  var textInputView: UIView { self }
}

// MARK: - Gesture coexistence

extension TermSelectionView: UIGestureRecognizerDelegate {
  func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer,
                         shouldRecognizeSimultaneouslyWith otherGestureRecognizer: UIGestureRecognizer) -> Bool {
    // The tap-away observer may ride alongside anything. The activation
    // long-press and the handle pan stay EXCLUSIVE: recognizing them must
    // block the scroll view's pan (a drag that extends a selection must not
    // scroll), which is the default.
    gestureRecognizer is UITapGestureRecognizer
  }

  // Touch-reception routing decides handle-vs-press up front (sim-verified
  // necessity: a SLOW handle drag stays under the long-press's 10pt movement
  // allowance for 0.45s, so the long-press recognized first and blocked the pan
  // by default exclusivity — the grab went dead):
  //   • handlePan only ever receives a touch starting on a selection edge, so
  //     plain drags over the terminal scroll exactly as before;
  //   • the activation long-press receives every touch EXCEPT those — a hold
  //     on a handle belongs to the pan, never re-anchors the selection.
  func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer, shouldReceive touch: UITouch) -> Bool {
    let onHandle = isActive && grabbedEnd(at: touch.location(in: self)) != nil
    if gestureRecognizer === handlePan { return onHandle }
    if gestureRecognizer === press { return !onHandle }
    return true
  }
}

// MARK: - Selection display (iOS 17+)

@available(iOS 17.0, *)
extension TermSelectionView: UITextSelectionDisplayInteractionDelegate {}

// MARK: - Edit menu (iOS 16+)

@available(iOS 16.0, *)
extension TermSelectionView: UIEditMenuInteractionDelegate {
  func editMenuInteraction(_ interaction: UIEditMenuInteraction,
                           menuFor configuration: UIEditMenuConfiguration,
                           suggestedActions: [UIMenuElement]) -> UIMenu? {
    guard isActive else { return nil }
    let title = menuLang == "zh" ? "拷贝" : "Copy"
    return UIMenu(children: [UIAction(title: title) { [weak self] _ in self?.copySelection() }])
  }
}
