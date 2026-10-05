// markupGeometry — where a mark sits, from the finger to the file.
//
// The editor used to capture its whole canvas: a full-screen view with the picture
// letterboxed inside, at the screen's size. The upload was therefore the canvas, margins
// included (a phone screenshot filled 41% of it), at roughly the size of the phone's
// screen rather than the picture's (simulator, 2026-10-05). Now the marks are drawn on a
// frame laid out to the picture's own shape, and the export is that frame alone, at the
// picture's pixels.
//
// Size: the picture's own pixels, always; only a full-size export the device cannot make
// leads to a smaller one, and only when the reader picks it (MARKUP_SCALED_EDGE).
//
// Coordinates: a mark is recorded in the fitted frame's points (origin at the picture's top
// left, as displayed, so an EXIF-rotated photo is in its upright orientation, which is
// also how Image.getSize reports it). The export draws the same marks through an SVG
// viewBox of the fitted size onto a canvas of the export size, so every point is scaled by
// exactly exportW / fitW and exportH / fitH, the same factor on both axes.

export interface Size {
  w: number;
  h: number;
}

/**
 * The export keeps the picture's own pixels, whatever their size: a hidden cap would call
 * a scaled picture the original. This is only the size OFFERED when a full-size export
 * fails on the device (memory, a renderer limit): the reader chooses it, it is never
 * applied unasked.
 */
export const MARKUP_SCALED_EDGE = 4096;

/** The file the editor produces: JPEG is always 8-bit and opaque, which is what an agent reads. */
export const MARKUP_FILE = {name: 'markup.jpg', type: 'image/jpeg'} as const;
export const MARKUP_JPEG_QUALITY = 0.92;

/** fitSize is the largest size with `natural`'s shape inside `box` (resizeMode "contain"). */
export function fitSize(natural: Size, box: Size): Size {
  if (natural.w <= 0 || natural.h <= 0) return {w: Math.max(box.w, 0), h: Math.max(box.h, 0)};
  const k = Math.min(box.w / natural.w, box.h / natural.h);
  return {w: natural.w * k, h: natural.h * k};
}

/**
 * exportPixels is the picture's own pixel size; with `maxEdge` (only when the reader chose
 * to scale down after a full-size export failed) its long edge is brought to that.
 */
export function exportPixels(natural: Size, maxEdge = Infinity): Size {
  const long = Math.max(natural.w, natural.h);
  const k = long > maxEdge ? maxEdge / long : 1;
  return {w: Math.round(natural.w * k), h: Math.round(natural.h * k)};
}

/**
 * exportPoints is the view size whose capture comes out at `px`: react-native-view-shot
 * renders a view's bounds at the screen's scale, so the view is laid out in points.
 */
export function exportPoints(px: Size, scale: number): Size {
  return {w: px.w / scale, h: px.h / scale};
}

/**
 * captureSize is the size to hand react-native-view-shot for an export of `px` pixels, in
 * points, a hundredth of a pixel short of px / scale.
 *
 * The renderer's canvas is the size in points times the screen's scale, rounded UP to whole
 * pixels. When px is not a multiple of the scale, the view's size in points is a fraction
 * that comes back from the layout (32-bit floats) a hair over px / scale, and the canvas
 * gained a pixel: a 1600 × 1000 picture came out 1600 × 1001 with a black last row, a
 * 1000 × 1600 one 1001 × 1601 (simulator, %6, 2026-10-05). Short by 0.01 px, the canvas
 * rounds up to exactly px, and the frame is drawn into it at 99.999% of its size, a
 * difference no pixel shows.
 */
export function captureSize(px: Size, scale: number): {width: number; height: number} {
  return {width: (px.w - 0.01) / scale, height: (px.h - 0.01) / scale};
}

/** toExport maps a point on the fitted frame to the exported picture's pixels. */
export function toExport(p: {x: number; y: number}, fit: Size, px: Size): {x: number; y: number} {
  return {x: (p.x * px.w) / fit.w, y: (p.y * px.h) / fit.h};
}
