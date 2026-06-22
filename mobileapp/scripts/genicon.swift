import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

// gtmux app icon (MOBILE §1): a 2×2 pane grid, top-right cell lit cyan, bottom
// cell spans both columns. Full-bleed square (iOS applies the squircle mask).
// Renders the Default / Dark / Tinted iOS 18 appearance variants at 1024².

let S = 1024
let cyan = (r: 6.0 / 255, g: 182.0 / 255, b: 212.0 / 255)

func roundedRect(_ ctx: CGContext, _ x: Double, _ y: Double, _ w: Double, _ h: Double, _ r: Double,
                 _ col: (r: Double, g: Double, b: Double), _ a: Double, shadow: Bool = false) {
  ctx.saveGState()
  if shadow {
    ctx.setShadow(offset: .init(width: 0, height: -10), blur: 34,
                  color: CGColor(red: cyan.r, green: cyan.g, blue: cyan.b, alpha: 0.55))
  }
  let path = CGPath(roundedRect: CGRect(x: x, y: y, width: w, height: h), cornerWidth: r, cornerHeight: r, transform: nil)
  ctx.addPath(path)
  ctx.setFillColor(CGColor(red: col.r, green: col.g, blue: col.b, alpha: a))
  ctx.fillPath()
  ctx.restoreGState()
}

func render(_ path: String, _ variant: String) {
  let cs = CGColorSpaceCreateDeviceRGB()
  guard let ctx = CGContext(data: nil, width: S, height: S, bitsPerComponent: 8, bytesPerRow: 0,
                            space: cs, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { return }
  let sd = Double(S)

  // --- background ---
  switch variant {
  case "dark":
    ctx.setFillColor(CGColor(red: 0, green: 0, blue: 0, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: sd, height: sd))
  case "tinted":
    ctx.setFillColor(CGColor(red: 26.0 / 255, green: 26.0 / 255, blue: 29.0 / 255, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: sd, height: sd))
  default: // gradient #262B36 → #0E1016 (top-left to bottom-right)
    let grad = CGGradient(colorsSpace: cs, colors: [
      CGColor(red: 38.0 / 255, green: 43.0 / 255, blue: 54.0 / 255, alpha: 1),
      CGColor(red: 14.0 / 255, green: 16.0 / 255, blue: 22.0 / 255, alpha: 1),
    ] as CFArray, locations: [0, 1])!
    ctx.drawLinearGradient(grad, start: CGPoint(x: 0, y: sd), end: CGPoint(x: sd, y: 0), options: [])
  }

  // --- grid geometry ---
  let grid = 580.0, origin = (sd - grid) / 2, gutter = 26.0
  let cell = (grid - gutter) / 2, r = 52.0
  let topY = origin + cell + gutter

  // neutral + lit colors per variant
  let neutral: (r: Double, g: Double, b: Double)
  let neutralA: Double
  let lit: (r: Double, g: Double, b: Double)
  let litA: Double
  let glow: Bool
  switch variant {
  case "dark":
    neutral = (1, 1, 1); neutralA = 0.16; lit = cyan; litA = 1; glow = true
  case "tinted": // monochrome — system re-colors
    neutral = (1, 1, 1); neutralA = 0.30; lit = (1, 1, 1); litA = 0.85; glow = false
  default:
    neutral = (1, 1, 1); neutralA = 0.22; lit = cyan; litA = 1; glow = true
  }

  roundedRect(ctx, origin, topY, cell, cell, r, neutral, neutralA)            // top-left
  roundedRect(ctx, origin + cell + gutter, topY, cell, cell, r, lit, litA, shadow: glow) // top-right (lit)
  roundedRect(ctx, origin, origin, grid, cell, r, neutral, neutralA)          // bottom (wide)

  guard let img = ctx.makeImage() else { return }
  let url = URL(fileURLWithPath: path)
  guard let dest = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil) else { return }
  CGImageDestinationAddImage(dest, img, nil)
  CGImageDestinationFinalize(dest)
  FileHandle.standardError.write("wrote \(path)\n".data(using: .utf8)!)
}

let dir = CommandLine.arguments[1]
render("\(dir)/icon-1024.png", "default")
render("\(dir)/icon-1024-dark.png", "dark")
render("\(dir)/icon-1024-tinted.png", "tinted")
