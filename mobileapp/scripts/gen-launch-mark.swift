import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

// The same generated mark is consumed by LaunchScreen and React Native. It keeps
// BrandMark's pane geometry on an ink plate, with visible neutral panes in either
// appearance. No app-icon or third-party agent artwork is changed.
let directory = CommandLine.arguments[1]
let srgb = CGColorSpace(name: CGColorSpace.sRGB)!
func rgb(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat) -> CGColor {
  CGColor(colorSpace: srgb, components: [r / 255, g / 255, b / 255, 1])!
}
for scale in 1...3 {
  let size = 112 * scale
  let context = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8,
                          bytesPerRow: 0, space: srgb,
                          bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
  context.scaleBy(x: CGFloat(scale), y: CGFloat(scale))
  func fill(_ rect: CGRect, radius: CGFloat, color: CGColor) {
    context.addPath(CGPath(roundedRect: rect, cornerWidth: radius, cornerHeight: radius, transform: nil))
    context.setFillColor(color)
    context.fillPath()
  }
  let ink = rgb(28, 28, 31)
  let neutral = rgb(163, 166, 173)
  let cyan = rgb(6, 182, 212)
  fill(CGRect(x: 0, y: 0, width: 112, height: 112), radius: 28, color: ink)
  // BrandMark's 100-unit canvas at 88pt, inset 12pt. CoreGraphics is bottom-up.
  context.translateBy(x: 12, y: 12)
  context.scaleBy(x: 0.88, y: 0.88)
  fill(CGRect(x: 8, y: 52, width: 40, height: 40), radius: 9, color: neutral)
  fill(CGRect(x: 52, y: 52, width: 40, height: 40), radius: 9, color: cyan)
  fill(CGRect(x: 8, y: 8, width: 84, height: 40), radius: 9, color: neutral)
  let suffix = scale == 1 ? "" : "@\(scale)x"
  let path = URL(fileURLWithPath: "\(directory)/launch-mark\(suffix).png")
  let destination = CGImageDestinationCreateWithURL(path as CFURL, UTType.png.identifier as CFString, 1, nil)!
  CGImageDestinationAddImage(destination, context.makeImage()!, nil)
  precondition(CGImageDestinationFinalize(destination), "Could not write launch mark")
}
