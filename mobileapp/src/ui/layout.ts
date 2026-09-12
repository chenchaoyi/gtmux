// Layout breakpoints (MOBILE §5). One place to answer "can this canvas hold two
import {useWindowDimensions} from 'react-native';
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

/**
 * The two shells (MOBILE §5, change ipad-universal-app). `regular` is a sidebar beside a
 * main pane; `compact` is the phone's stack. This is the ONLY place a layout decision is
 * made from the window: screens receive the class (or a prop derived from it) and never
 * read the window size to pick a layout themselves — `shellDrift.test.ts` enforces it.
 */
export type SizeClass = 'regular' | 'compact';

export function sizeClassFor(width: number, height: number): SizeClass {
  return isSplitCanvas(width, height) ? 'regular' : 'compact';
}

export function useSizeClass(): SizeClass {
  const {width, height} = useWindowDimensions();
  return sizeClassFor(width, height);
}

/** The sidebar's width on the regular shell: 300 on a wide window, 280 under 1000. */
export const sidebarWidth = (windowWidth: number): number => (windowWidth >= 1000 ? 300 : 280);

/**
 * The reading width for chat, the HQ console and settings on the regular shell: text
 * wider than this is harder to read, not easier. The terminal never caps (more columns
 * is the point of the big screen).
 */
export const READING_WIDTH = 760;
