import SwiftUI

// Theme is the single source of design tokens — colors, sizes, fonts — mirroring
// docs/design/DESIGN.md §9. The status colors here are the authoritative values;
// keep them in sync with DESIGN (a conformance test asserts they match the hex).
enum Theme {

    // MARK: status colors (DESIGN §1/§9 — authoritative hex)
    enum Status {
        static let waiting = Color(hex: 0xEF4444) // red
        static let working = Color(hex: 0x06B6D4) // cyan
        static let idle    = Color(hex: 0x22C55E) // green
        static let none    = Color(hex: 0x8E8E93) // gray (none / running)

        // NSColor variants for the (AppKit) status-bar glyph rendering.
        static let waitingNS = NSColor(srgbRed: 0xEF/255, green: 0x44/255, blue: 0x44/255, alpha: 1)
        static let workingNS = NSColor(srgbRed: 0x06/255, green: 0xB6/255, blue: 0xD4/255, alpha: 1)
        static let idleNS    = NSColor(srgbRed: 0x22/255, green: 0xC5/255, blue: 0x5E/255, alpha: 1)
        static let noneNS    = NSColor(srgbRed: 0x8E/255, green: 0x8E/255, blue: 0x93/255, alpha: 1)
    }

    // MARK: layout (DESIGN §3)
    enum Size {
        static let popoverWidth: CGFloat = 320
        static let radiusPopover: CGFloat = 13
        static let radiusRow: CGFloat = 8
        static let radiusChip: CGFloat = 5
        static let rowHeight: CGFloat = 46
        static let rowHeightCompact: CGFloat = 28
        static let avatar: CGFloat = 30
        static let badge: CGFloat = 15
        static let listMaxHeight: CGFloat = 520
        static let pad: CGFloat = 7
        static let gap: CGFloat = 11
    }

    // MARK: fonts (DESIGN §9)
    enum Font {
        static let session = SwiftUI.Font.system(size: 13, weight: .semibold)
        static let window  = SwiftUI.Font.system(size: 11.5)
        static let task    = SwiftUI.Font.system(size: 12)
        static let section = SwiftUI.Font.system(size: 11, weight: .bold)
        static let summary = SwiftUI.Font.system(size: 11, weight: .semibold)
        static let action  = SwiftUI.Font.system(size: 12)
        static let mono    = SwiftUI.Font.system(size: 11, design: .monospaced)
        static let title   = SwiftUI.Font.system(size: 18, weight: .bold)
        static let footer  = SwiftUI.Font.system(size: 10)
    }

    // MARK: popover palette (DESIGN §9) — resolved per color scheme
    struct Palette {
        let bg: Color // tint over the vibrancy blur (DESIGN §9)
        let fg: Color
        let fg2: Color
        let fg3: Color
        let divider: Color
        let rowSelected: Color
        let waitingRowTint: Color // rgba(239,68,68,0.08), both schemes

        static func of(_ scheme: ColorScheme) -> Palette {
            if scheme == .dark {
                return Palette(
                    bg: Color(hex: 0x1C1C1F, opacity: 0.60),
                    fg: Color(white: 1, opacity: 0.95),
                    fg2: Color(red: 235/255, green: 235/255, blue: 245/255, opacity: 0.62),
                    fg3: Color(red: 235/255, green: 235/255, blue: 245/255, opacity: 0.34),
                    divider: Color(white: 1, opacity: 0.09),
                    rowSelected: Color(white: 1, opacity: 0.12),
                    waitingRowTint: Color(hex: 0xEF4444, opacity: 0.08))
            }
            return Palette(
                bg: Color(hex: 0xFCFCFD, opacity: 0.72),
                fg: Color(hex: 0x1D1D1F),
                fg2: Color(red: 60/255, green: 60/255, blue: 67/255, opacity: 0.62),
                fg3: Color(red: 60/255, green: 60/255, blue: 67/255, opacity: 0.34),
                divider: Color(black: 0, opacity: 0.08),
                rowSelected: Color(black: 0, opacity: 0.07),
                waitingRowTint: Color(hex: 0xEF4444, opacity: 0.08))
        }
    }
}

extension Color {
    init(hex: UInt32, opacity: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: opacity)
    }
    init(black: Double, opacity: Double) { self.init(.sRGB, white: black, opacity: opacity) }
}
