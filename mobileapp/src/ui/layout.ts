// Layout breakpoints (MOBILE §5). One place to answer "can this canvas hold two
// columns?", because the answer was being spelled out as `width >= 768` in three files
// and they must never disagree about what device they are on.

export const splitMinWidth = 768;

/**
 * splitMinHeight is the half of the test that was missing.
 *
 * A modern iPhone in LANDSCAPE is 852-932pt wide — comfortably past the width
 * breakpoint — and 393-430pt tall. Width alone therefore handed a phone the iPad's
 * sidebar-plus-detail layout on a canvas barely taller than the keyboard, which is the
 * inverse of the rule §5 opens with: an iPad is not a big phone, and a phone in landscape
 * is not a small iPad. 600 clears every iPad orientation (the shortest is 768pt) and no
 * phone in either.
 */
export const splitMinHeight = 600;

/** isSplitCanvas reports whether the window can carry the two-column layout. */
export function isSplitCanvas(width: number, height: number): boolean {
  return width >= splitMinWidth && height >= splitMinHeight;
}
