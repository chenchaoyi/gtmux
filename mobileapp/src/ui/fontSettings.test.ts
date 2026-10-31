import { adjustSessionFont, restoreSessionFont } from './fontSettings';
test('one-point adjustments reach every size and clamp at the ends', () => {
  expect(adjustSessionFont(13, 1)).toBe(14);
  expect(adjustSessionFont(13, -1)).toBe(12);
  expect(adjustSessionFont(10, -1)).toBe(10);
  expect(adjustSessionFont(24, 1)).toBe(24);
  let size = 10;
  const sizes = [size];
  for (let i = 0; i < 14; i++) {
    size = adjustSessionFont(size, 1);
    sizes.push(size);
  }
  expect(sizes).toEqual(Array.from({ length: 15 }, (_, i) => i + 10));
});
test('saved sizes override old indices; migration retains actual old text size', () => {
  expect(restoreSessionFont('14', '1')).toBe(14);
  expect([0, 1, 2].map(n => restoreSessionFont(null, String(n)))).toEqual([
    11, 13, 15,
  ]);
  for (const value of [null, 'garbage', '1.5', '99'])
    expect(restoreSessionFont(value, null)).toBe(13);
});
