import {exportPixels, exportPoints, fitSize, MARKUP_FILE, toExport} from './markupGeometry';

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

  it("exports at the picture's own pixels; only a very large one is scaled down", () => {
    expect(exportPixels({w: 1290, h: 2796})).toEqual({w: 1290, h: 2796});
    expect(exportPixels({w: 4032, h: 3024})).toEqual({w: 4032, h: 3024});
    // 48 MP: long edge to 4096, shape kept.
    expect(exportPixels({w: 8064, h: 6048})).toEqual({w: 4096, h: 3072});
    expect(exportPixels({w: 6048, h: 8064})).toEqual({w: 3072, h: 4096});
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
