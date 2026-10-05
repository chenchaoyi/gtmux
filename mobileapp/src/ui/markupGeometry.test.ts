import {captureSize, exportPixels, exportPoints, fitSize, MARKUP_FILE, MARKUP_SCALED_EDGE, toExport} from './markupGeometry';

// Where a mark sits, from the finger to the file (markupGeometry).
describe('markup geometry', () => {
  it('fits the picture to the canvas by its own shape, with no margin of its own', () => {
    // A portrait phone screenshot in a canvas wider than its shape: height-bound.
    const fit = fitSize({w: 1290, h: 2796}, {w: 378, h: 600});
    expect(fit.h).toBeCloseTo(600);
    expect(fit.w).toBeCloseTo((1290 / 2796) * 600);
    // A landscape photo in the same canvas: width-bound.
    const wide = fitSize({w: 4032, h: 3024}, {w: 378, h: 600});
    expect(wide.w).toBeCloseTo(378);
    expect(wide.h).toBeCloseTo(283.5);
  });

  it("exports at the picture's own pixels, however large; never a hidden cap", () => {
    expect(exportPixels({w: 1290, h: 2796})).toEqual({w: 1290, h: 2796});
    expect(exportPixels({w: 4032, h: 3024})).toEqual({w: 4032, h: 3024});
    expect(exportPixels({w: 6000, h: 4000})).toEqual({w: 6000, h: 4000}); // 24 MP
    expect(exportPixels({w: 8064, h: 6048})).toEqual({w: 8064, h: 6048}); // 48 MP
  });

  it('scales only to a size the reader chose, keeping the shape', () => {
    expect(exportPixels({w: 8064, h: 6048}, MARKUP_SCALED_EDGE)).toEqual({w: 4096, h: 3072});
    expect(exportPixels({w: 6048, h: 8064}, MARKUP_SCALED_EDGE)).toEqual({w: 3072, h: 4096});
    expect(exportPixels({w: 1290, h: 2796}, MARKUP_SCALED_EDGE)).toEqual({w: 1290, h: 2796}); // never up
  });

  it('lays the export out in points that capture to those pixels', () => {
    expect(exportPoints({w: 1290, h: 2796}, 3)).toEqual({w: 430, h: 932});
  });

  it('maps a mark by the same factor on both axes, corners to corners', () => {
    const fit = fitSize({w: 1290, h: 2796}, {w: 378, h: 600});
    const px = exportPixels({w: 1290, h: 2796});
    expect(toExport({x: 0, y: 0}, fit, px)).toEqual({x: 0, y: 0});
    const end = toExport({x: fit.w, y: fit.h}, fit, px);
    expect(end.x).toBeCloseTo(1290);
    expect(end.y).toBeCloseTo(2796);
    const mid = toExport({x: fit.w / 2, y: fit.h / 4}, fit, px);
    expect(mid.x).toBeCloseTo(645);
    expect(mid.y).toBeCloseTo(699);
  });

  it('names the file for what it is', () => {
    expect(MARKUP_FILE).toEqual({name: 'markup.jpg', type: 'image/jpeg'});
  });
});

describe('the capture size', () => {
  // The renderer's canvas is points × scale rounded UP. A size that comes back from the
  // layout a hair over px / scale (32-bit floats) gained a pixel; modelled here with
  // Math.fround, which is what a 32-bit float does to it.
  const canvas = (points: number, scale: number) => Math.ceil(Math.fround(points) * scale);
  const sizes = [1000, 1001, 1206, 1600, 2622, 2796, 3024, 4031, 4096, 6000, 8064];

  it('lands on exactly the picture\'s pixels at every size, multiple of the scale or not', () => {
    for (const scale of [3, 2]) {
      for (const n of sizes) {
        const {width} = captureSize({w: n, h: n}, scale);
        expect(canvas(width, scale)).toBe(n);
      }
    }
  });

  it('is the size whose canvas the plain px / scale overshot', () => {
    // 1000 px at @3x: the old path's canvas came out 1001 (the simulator's 1600 × 1001).
    expect(canvas(1000 / 3, 3)).toBe(1001);
    expect(canvas(captureSize({w: 1000, h: 1000}, 3).width, 3)).toBe(1000);
  });

  it('draws the frame at 99.999% or more of its size', () => {
    for (const n of sizes) expect(captureSize({w: n, h: n}, 3).width / (n / 3)).toBeGreaterThan(0.99999 - 0.00001);
  });
});
