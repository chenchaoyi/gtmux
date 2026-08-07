// Bridges the Swift TermSelectionViewManager (ios/TermSelection/) to React
// Native as the `TermSelectionView` component and declares the props the JS
// side (NativeTerm.tsx) sets. Geometry props MUST come from term.ts's helpers
// (rowHeightFor / PAD) — the native layer never re-derives them (the Stage 1
// handoff rule: recomputing row height from UIFont metrics reintroduces drift).
#import <React/RCTViewManager.h>

@interface RCT_EXTERN_MODULE (TermSelectionViewManager, RCTViewManager)

RCT_EXPORT_VIEW_PROPERTY(text, NSString)
RCT_EXPORT_VIEW_PROPERTY(rowHeight, CGFloat)
RCT_EXPORT_VIEW_PROPERTY(fontSize, CGFloat)
RCT_EXPORT_VIEW_PROPERTY(fontName, NSString)
RCT_EXPORT_VIEW_PROPERTY(padTop, CGFloat)
RCT_EXPORT_VIEW_PROPERTY(padLeft, CGFloat)
// A hex STRING, not a color: the interop layer decodes processColor's ARGB
// int as RGBA (blue came out red) — the Swift side parses "#RRGGBB" itself.
RCT_EXPORT_VIEW_PROPERTY(selectionTint, NSString)
RCT_EXPORT_VIEW_PROPERTY(menuLang, NSString)
RCT_EXPORT_VIEW_PROPERTY(onSelectionActive, RCTDirectEventBlock)

@end
