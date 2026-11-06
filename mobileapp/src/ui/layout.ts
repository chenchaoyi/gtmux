import {useWindowDimensions} from './windowMetrics';

// Central layout rules for the available workspace (MOBILE §5).

export const splitMinWidth = 768;

/** A two-column workspace also needs enough vertical room for its controls and reader.
 * Decide from the current window, not from device family or physical orientation. */
export const splitMinHeight = 600;

/** isSplitCanvas reports whether the window can carry the two-column layout. */
export function isSplitCanvas(width: number, height: number): boolean {
  return width >= splitMinWidth && height >= splitMinHeight;
}

/**
 * The workspace modes (MOBILE §5): `regular` shows a sidebar beside the reader;
 * `compact` shows the list or reader. The choice follows available space on any device.
 * `shellDrift.test.ts` guards against screens introducing divergent shell decisions.
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

export const HQ_INSPECTOR_WIDTH = 360;
const HQ_CONSOLE_MIN_WIDTH = 440;

/** A regular shell can still have a narrow main pane, especially in portrait. */
export function hqInspectorFits(windowWidth: number, sidebarHidden = false): boolean {
  const mainWidth = windowWidth - (sidebarHidden ? 0 : sidebarWidth(windowWidth));
  return mainWidth >= HQ_CONSOLE_MIN_WIDTH + HQ_INSPECTOR_WIDTH;
}

/**
 * The reading width for chat, the HQ console and settings on the regular shell: text
 * wider than this is harder to read, not easier. The terminal never caps (more columns
 * is the point of the big screen).
 */
export const READING_WIDTH = 760;

/** Trace uses its measured tool canvas, including split windows and large text. */
export function traceSplitFits(contentWidth: number, fontScale: number): boolean {
  return contentWidth >= 820 && fontScale < 1.6;
}
export function useTraceSplit(contentWidth: number): boolean {
  return traceSplitFits(contentWidth, useWindowDimensions().fontScale);
}
