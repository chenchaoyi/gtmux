import {dirFromDelta} from './MoveKey';

describe('dirFromDelta', () => {
  it('returns null inside the dead zone', () => {
    expect(dirFromDelta(0, 0)).toBeNull();
    expect(dirFromDelta(10, -10)).toBeNull();
    expect(dirFromDelta(15, 15)).toBeNull();
  });

  it('maps the dominant horizontal axis past the threshold', () => {
    expect(dirFromDelta(40, 5)).toBe('Right');
    expect(dirFromDelta(-40, -5)).toBe('Left');
  });

  it('maps the dominant vertical axis past the threshold', () => {
    expect(dirFromDelta(5, 40)).toBe('Down');
    expect(dirFromDelta(-5, -40)).toBe('Up');
  });

  it('picks the larger axis on a diagonal', () => {
    expect(dirFromDelta(50, 30)).toBe('Right'); // |dx| > |dy|
    expect(dirFromDelta(20, 60)).toBe('Down'); // |dy| > |dx|
  });

  it('honors a custom threshold', () => {
    expect(dirFromDelta(10, 0, 8)).toBe('Right'); // clears the smaller threshold
    expect(dirFromDelta(10, 0, 20)).toBeNull(); // still inside a larger dead zone
  });

  it('registers once either axis clears the threshold (one axis still small)', () => {
    expect(dirFromDelta(20, 3)).toBe('Right');
    expect(dirFromDelta(3, -20)).toBe('Up');
  });
});
