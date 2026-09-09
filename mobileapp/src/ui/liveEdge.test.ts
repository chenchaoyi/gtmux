import {AT_TAIL, CHROME_ANIM_MS, ChromeState, chromeDecision, chromeShift, foldThreshold, makeFoldFollower} from './liveEdge';

const st = (hidden: boolean, settledAt = 0): ChromeState => ({hidden, settledAt});

// The chrome height measured on a real Detail page: header + neighbour strip + segmented
// control folded the viewport from 692.7 to 576.
const CHROME = 117;

describe('chromeDecision', () => {
  it('folds once you are clearly into history, and reveals at the tail', () => {
    expect(chromeDecision(st(false), 400, CHROME, 1000)).toEqual({
      change: true, hidden: true, settledAt: 1000 + CHROME_ANIM_MS,
    });
    expect(chromeDecision(st(true), 0, CHROME, 1000)).toEqual({
      change: true, hidden: false, settledAt: 1000 + CHROME_ANIM_MS,
    });
  });

  it('leaves the chrome alone in the band between the two thresholds', () => {
    // The whole point: a gap that is neither at the tail nor clearly away from it must
    // not move the chrome, in EITHER current state.
    const between = AT_TAIL + 20;
    expect(chromeDecision(st(false), between, CHROME, 1000).change).toBe(false);
    expect(chromeDecision(st(true), between, CHROME, 1000).change).toBe(false);
  });

  it('takes no decision from a measurement made mid-animation', () => {
    const mid = chromeDecision(st(false), 400, CHROME, 1000); // folds, settles at 1200
    expect(chromeDecision(mid, 0, CHROME, 1100).change).toBe(false); // 100ms in: ignored
    expect(chromeDecision(mid, 0, CHROME, 1250).change).toBe(true); // settled: answered
  });

  it('a repeat in the same direction is a no-op and does not re-arm the freeze', () => {
    const folded = st(true, 900);
    expect(chromeDecision(folded, 400, CHROME, 1000)).toEqual({
      change: false, hidden: true, settledAt: 900,
    });
  });

  // The property the whole module exists for. Simulate the loop the trace recorded: every
  // reveal shrinks the viewport (gap grows by chromeH), every fold grows it back — and
  // check the chrome reaches a state and stays there.
  it('cannot oscillate: revealing never re-triggers a fold, and vice versa', () => {
    for (const chromeH of [0, 40, 117, 260]) {
      // Start at the tail with the chrome folded; the reveal is the dangerous direction.
      let state = st(true);
      let gap = 0;
      let now = 1000;
      const seen: boolean[] = [];
      for (let i = 0; i < 20; i++) {
        const d = chromeDecision(state, gap, chromeH, now);
        if (d.change) {
          // Committing the change moves the viewport under us by the chrome's height.
          gap += d.hidden ? -chromeH : chromeH;
          seen.push(d.hidden);
          state = d;
        }
        now += CHROME_ANIM_MS + 1; // let every animation finish, so nothing is masked
      }
      // Exactly one transition: reveal. Never a reveal-fold-reveal sawtooth.
      expect(seen).toEqual([false]);
    }
  });

  it('cannot oscillate from the other side either', () => {
    for (const chromeH of [0, 117, 260]) {
      let state = st(false);
      let gap = foldThreshold(chromeH) + 5; // deep in history, chrome still shown
      let now = 1000;
      const seen: boolean[] = [];
      for (let i = 0; i < 20; i++) {
        const d = chromeDecision(state, gap, chromeH, now);
        if (d.change) {
          gap += d.hidden ? -chromeH : chromeH;
          seen.push(d.hidden);
          state = d;
        }
        now += CHROME_ANIM_MS + 1;
      }
      expect(seen).toEqual([true]);
    }
  });

  // The exact readings from the 2026-09-05 trace, which sawtoothed under the old rule.
  it('rides the real trace to a single reveal', () => {
    const trace = [253, 238, 195.3, 145, 100.7, 62.3, 28.3, 23.3, 21.3, 23.7, 27.7, 34, 42.7, 52, 44.7, 36.3, 25.3, 20.3, 17.7, 22];
    let state = st(true);
    let now = 1000;
    const flips: boolean[] = [];
    for (const gap of trace) {
      const d = chromeDecision(state, gap, CHROME, now);
      if (d.change) flips.push(d.hidden);
      state = d;
      now += 16; // one scroll frame
    }
    expect(flips).toEqual([false]); // it reveals at gap=28.3 and never argues again
  });
});

// Holding the content still while the chrome folds.
//
// The fold hands the scroll view `chromeH` of extra height at its TOP edge, and content
// keeps its offset — so it slides up by exactly that much unless the offset follows. 115pt
// over 200ms, against the finger, is what the operator felt as a bounce (2026-09-09).
describe('chromeShift', () => {
  it('cancels the fold exactly, in the direction that keeps content still', () => {
    // Folding (v 0→1) grows the viewport at the top, so the offset moves DOWN by as much.
    expect(chromeShift(0, 1, 115)).toBe(-115);
    // Half a frame of it is half the shift: the whole point is lockstep, not a jump at
    // the end — a single correction at the end is the same bounce, just later.
    expect(chromeShift(0, 0.5, 115)).toBe(-57.5);
    expect(chromeShift(0.5, 1, 115)).toBe(-57.5);
  });

  it('runs backwards for a reveal, so one function serves both', () => {
    expect(chromeShift(1, 0, 115)).toBe(115);
  });

  it('sums to the full height however the animation is sampled', () => {
    // Frames arrive irregularly; the total correction must not depend on that.
    let total = 0;
    let prev = 0;
    for (const v of [0.07, 0.31, 0.33, 0.78, 0.99, 1]) {
      total += chromeShift(prev, v, 115);
      prev = v;
    }
    expect(total).toBeCloseTo(-115, 6);
  });

  it('does nothing before the chrome has been measured', () => {
    // chromeH is 0 until layout; shifting by 0 is right, and NaN would be a dead scroll.
    expect(chromeShift(0, 1, 0)).toBe(0);
    expect(chromeShift(0, 1, -5)).toBe(0);
  });
});

describe('makeFoldFollower', () => {
  const run = (values: number[], chromeH = 115) => {
    const moves: number[] = [];
    const follow = makeFoldFollower(() => chromeH, () => (dy: number) => moves.push(dy));
    values.forEach(follow);
    return moves;
  };

  it('moves the content by the chrome’s whole height across a fold', () => {
    // The sum is what matters: at the end the viewport has grown by chromeH, so the
    // offset must have moved by the same amount or the content has slid.
    const moves = run([0.2, 0.5, 0.9, 1]);
    expect(moves.reduce((a, b) => a + b, 0)).toBeCloseTo(-115, 6);
  });

  it('spreads it over the frames instead of correcting once at the end', () => {
    // One correction at the end is the same bounce, just later. Every step must carry
    // its own share.
    const moves = run([0.25, 0.5, 0.75, 1]);
    expect(moves).toHaveLength(4);
    for (const m of moves) expect(Math.abs(m)).toBeCloseTo(28.75, 6);
  });

  it('says nothing when nothing moved', () => {
    // A listener can fire on the same value; a zero correction must not reach the layer,
    // where it would be a pointless scrollTo mid-gesture.
    expect(run([0, 0, 0])).toHaveLength(0);
  });

  it('is silent before the chrome has been measured', () => {
    expect(run([0.5, 1], 0)).toHaveLength(0);
  });

  it('does not throw when no layer is installed', () => {
    const follow = makeFoldFollower(() => 115, () => null);
    expect(() => follow(1)).not.toThrow();
  });
});
