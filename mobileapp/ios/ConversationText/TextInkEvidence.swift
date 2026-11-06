#if targetEnvironment(simulator) || GTMUX_TEXT_INK_CHECKS
import CoreGraphics
import Foundation

/// Bounded diagnostic raster of an existing layout, never a replacement layout.
/// The caller draws in UIKit's top-left coordinate system. No image is persisted.
enum TextInkEvidence {
  struct Sample {
    let complete: Bool
    let bounds: CGRect?
    let pixels: Int
    let touchesEdge: Bool
    let reason: String?
    var json: [String: Any] {
      var value: [String: Any] = ["complete": complete, "pixels": pixels, "touchesEdge": touchesEdge]
      if let bounds { value["bounds"] = ["x": bounds.minX, "y": bounds.minY, "width": bounds.width, "height": bounds.height] }
      if let reason { value["reason"] = reason }
      return value
    }
  }

  static func capture(surface: CGRect, scale: CGFloat, draw: (CGContext) -> Void) -> Sample {
    func failure(_ reason: String) -> Sample { Sample(complete: false, bounds: nil, pixels: 0, touchesEdge: false, reason: reason) }
    guard [surface.minX, surface.minY, surface.width, surface.height, scale].allSatisfy({ $0.isFinite }),
          surface.width > 0, surface.height > 0, scale >= 1, scale <= 4 else { return failure("invalid surface or scale") }
    let region = CGRect(x: floor(surface.minX * scale) / scale - 2,
                        y: floor(surface.minY * scale) / scale - 2,
                        width: ceil(surface.maxX * scale) / scale - floor(surface.minX * scale) / scale + 4,
                        height: ceil(surface.maxY * scale) / scale - floor(surface.minY * scale) / scale + 4)
    let w = ceil(region.width * scale), h = ceil(region.height * scale)
    guard w.isFinite, h.isFinite, w > 0, h > 0, w <= 4_000_000, h <= 4_000_000, w * h <= 4_000_000 else { return failure("raster exceeds 4 million pixels") }
    let width = Int(w), height = Int(h), stride = width * 4
    var bytes = [UInt8](repeating: 0, count: stride * height)
    return bytes.withUnsafeMutableBytes { buffer in
      guard let context = CGContext(data: buffer.baseAddress, width: width, height: height, bitsPerComponent: 8,
          bytesPerRow: stride, space: CGColorSpaceCreateDeviceRGB(),
          bitmapInfo: CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue) else { return failure("context unavailable") }
      context.translateBy(x: -region.minX * scale, y: CGFloat(height) + region.minY * scale)
      context.scaleBy(x: scale, y: -scale)
      draw(context)
      context.flush()
      let data = buffer.bindMemory(to: UInt8.self)
      var minX = width, minY = height, maxX = -1, maxY = -1, count = 0
      for row in 0..<height {
        for x in 0..<width where data[row * stride + x * 4 + 3] > 0 {
          // Memory rows follow the top-left image origin after the CTM flip.
          let y = row
          minX = min(minX, x); minY = min(minY, y)
          maxX = max(maxX, x); maxY = max(maxY, y); count += 1
        }
      }
      guard count > 0 else { return failure("empty raster") }
      let edge = minX == 0 || minY == 0 || maxX == width - 1 || maxY == height - 1
      let ink = CGRect(x: region.minX + CGFloat(minX) / scale, y: region.minY + CGFloat(minY) / scale,
                       width: CGFloat(maxX - minX + 1) / scale, height: CGFloat(maxY - minY + 1) / scale)
      return Sample(complete: !edge, bounds: ink, pixels: count, touchesEdge: edge,
                    reason: edge ? "ink touches raster edge" : nil)
    }
  }
}
#endif
