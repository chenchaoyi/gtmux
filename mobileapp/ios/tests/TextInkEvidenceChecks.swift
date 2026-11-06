import CoreGraphics
import Foundation

@main enum TextInkEvidenceChecks {
  static func main() {
    func paint(_ rect: CGRect, surface: CGRect, scale: CGFloat = 2, alpha: CGFloat = 1) -> TextInkEvidence.Sample {
      TextInkEvidence.capture(surface: surface, scale: scale) { context in
        context.setShouldAntialias(false)
        context.setFillColor(CGColor(red: 1, green: 0, blue: 0, alpha: alpha))
        context.fill(rect)
      }
    }
    let surface = CGRect(x: -4, y: -5, width: 31, height: 40)
    let target = CGRect(x: -2, y: 3, width: 5, height: 7)
    for scale: CGFloat in [2, 3] {
      let sample = paint(target, surface: surface, scale: scale)
      precondition(sample.complete && sample.bounds == target, "orientation/origin/scale: \(sample.json)")
      precondition(sample.pixels == Int(35 * scale * scale) && !sample.touchesEdge)
    }
    let faint = paint(target, surface: surface, alpha: 0.01)
    precondition(faint.complete && faint.bounds == target, "nonzero alpha must not be filtered")
    let overflow = CGRect(x: 1, y: 20, width: 8, height: 3)
    let outside = paint(overflow, surface: CGRect(x: 0, y: 0, width: 10, height: 23))
    precondition(outside.complete && outside.bounds == overflow)
    precondition(!CGRect(x: 0, y: 0, width: 10, height: 20).contains(outside.bounds!))
    let clipped = paint(CGRect(x: -100, y: -100, width: 200, height: 200), surface: surface)
    precondition(!clipped.complete && clipped.touchesEdge)
    let blank = TextInkEvidence.capture(surface: surface, scale: 2) { _ in }
    precondition(!blank.complete && blank.bounds == nil)
    var drew = false
    let huge = TextInkEvidence.capture(surface: CGRect(x: 0, y: 0, width: 100000, height: 100000), scale: 3) { _ in drew = true }
    precondition(!huge.complete && !drew)
    let invalid = TextInkEvidence.capture(surface: .zero, scale: .nan) { _ in drew = true }
    precondition(!invalid.complete && !drew)
    print("TextInkEvidence checks passed: orientation, origins, scales, alpha, overflow, edge, blank and memory bounds")
  }
}
