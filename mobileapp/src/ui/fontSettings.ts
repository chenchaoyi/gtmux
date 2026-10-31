export const MIN_SESSION_FONT = 10;
export const MAX_SESSION_FONT = 24;
export const DEFAULT_SESSION_FONT = 13;
export const SESSION_FONT_KEY = 'detail.fontSize';
export const LEGACY_FONT_KEY = 'detail.fontIdx';
export function adjustSessionFont(size: number, delta: number) {
  return Math.max(MIN_SESSION_FONT, Math.min(MAX_SESSION_FONT, size + delta));
}
export function restoreSessionFont(
  saved: string | null,
  legacy: string | null,
) {
  const size = saved === null ? NaN : Number(saved);
  if (
    Number.isInteger(size) &&
    size >= MIN_SESSION_FONT &&
    size <= MAX_SESSION_FONT
  )
    return size;
  const index = legacy === null ? NaN : Number(legacy);
  return Number.isInteger(index) && index >= 0 && index <= 2
    ? [11, 13, 15][index]
    : DEFAULT_SESSION_FONT;
}
