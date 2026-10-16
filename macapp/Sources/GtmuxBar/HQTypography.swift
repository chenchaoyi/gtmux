import AppKit
import SwiftUI

/// Reading-window roles are deliberately separate from the compact menu popover.
enum HQTypography {
    enum Role: CaseIterable, Equatable {
        case title, section, rowTitle, body, navigation, sidebarGroup, sidebarItem, caption, code
        var size: CGFloat {
            switch self {
            case .title: return 20
            case .section: return 16
            case .rowTitle, .body: return 15
            case .navigation, .sidebarGroup, .sidebarItem: return 14
            case .caption, .code: return 13
            }
        }
        var weight: NSFont.Weight {
            switch self {
            case .title, .section, .sidebarGroup: return .semibold
            case .rowTitle, .navigation: return .medium
            default: return .regular
            }
        }
        var nativeFont: NSFont {
            self == .code ? .monospacedSystemFont(ofSize: size, weight: weight) : .systemFont(ofSize: size, weight: weight)
        }
        var font: Font { Font(nativeFont) }
    }
    static let title = Role.title.font
    static let section = Role.section.font
    static let rowTitle = Role.rowTitle.font
    static let body = Role.body.font
    static let navigation = Role.navigation.font
    static let sidebarGroup = Role.sidebarGroup.font
    static let sidebarItem = Role.sidebarItem.font
    static let caption = Role.caption.font
    static let code = Role.code.font
}

/// A document's host chooses its reading scale; other Markdown readers keep theirs.
struct MarkdownTypography: Equatable {
    var body: CGFloat = 12
    var caption: CGFloat = 11.5
    var code: CGFloat = 11.5
    var heading1: CGFloat = 17
    var heading2: CGFloat = 14.5
    var heading3: CGFloat = 12.5
    var lineSpacing: CGFloat = 2
    static let standard = MarkdownTypography()
    static let workspace = MarkdownTypography(
        body: HQTypography.Role.body.size, caption: HQTypography.Role.caption.size,
        code: HQTypography.Role.code.size, heading1: 21, heading2: 18, heading3: 16, lineSpacing: 4)
}
private struct MarkdownTypographyKey: EnvironmentKey {
    static let defaultValue = MarkdownTypography.standard
}
extension EnvironmentValues {
    var markdownTypography: MarkdownTypography {
        get { self[MarkdownTypographyKey.self] }
        set { self[MarkdownTypographyKey.self] = newValue }
    }
}
