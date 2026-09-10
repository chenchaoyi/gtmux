import {AT_TAIL, CHROME_ANIM_MS, ChromeState, chromeDecision, FOLD_AT} from './liveEdge';

const st = (hidden: boolean, settledAt = 0): ChromeState => ({hidden, settledAt});

// The chrome no longer resizes anything, so its height is not an input to the decision —
// it floats above the scroll view. What is left is the hysteresis band: reveal at or
// under AT_TAIL, fold at or beyond FOLD_AT.

describe('chromeDecision', () => {
  it('folds once you are clearly into history, and reveals at the tail', () => {
    expect(chromeDecision(st(false), {gap: 400, now: 1000})).toEqual({
      change: true, hidden: true, settledAt: 1000 + CHROME_ANIM_MS,
    });
    expect(chromeDecision(st(true), {gap: 0, now: 1000})).toEqual({
      change: true, hidden: false, settledAt: 1000 + CHROME_ANIM_MS,
    });
  });

  it('leaves the chrome alone in the band between the two thresholds', () => {
    // The whole point: a gap that is neither at the tail nor clearly away from it must
    // not move the chrome, in EITHER current state.
    const between = AT_TAIL + 20;
    expect(chromeDecision(st(false), {gap: between, now: 1000}).change).toBe(false);
    expect(chromeDecision(st(true), {gap: between, now: 1000}).change).toBe(false);
  });

  it('takes no decision from a measurement made mid-animation', () => {
    const mid = chromeDecision(st(false), {gap: 400, now: 1000}); // folds, settles at 1200
    expect(chromeDecision(mid, {gap: 0, now: 1100}).change).toBe(false); // 100ms in: ignored
    expect(chromeDecision(mid, {gap: 0, now: 1250}).change).toBe(true); // settled: answered
  });

  it('a repeat in the same direction is a no-op and does not re-arm the freeze', () => {
    const folded = st(true, 900);
    expect(chromeDecision(folded, {gap: 400, now: 1000})).toEqual({
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
        const d = chromeDecision(state, {gap: gap, now: chromeH, animMs: now});
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
      let gap = FOLD_AT + 5; // deep in history, chrome still shown
      let now = 1000;
      const seen: boolean[] = [];
      for (let i = 0; i < 20; i++) {
        const d = chromeDecision(state, {gap: gap, now: chromeH, animMs: now});
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
      const d = chromeDecision(state, {gap: gap, now: now});
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

// The bug this shape exists to prevent, kept as a test because the type alone is a
// promise about the FUTURE and this is what went wrong in the past.
//
// A parameter was removed from the middle of a positional list on 2026-09-10. The HQ page
// kept passing four numbers, so its chrome height landed on `now` and its clock landed on
// `animMs`; every number went to a number, nothing failed to compile, and the header
// folded once and never came back — `settledAt` had become a timestamp plus a header
// height, which no later reading could get past.
describe('a reading whose clock is wrong', () => {
  it('would freeze the decision forever — which is why the clock is named', () => {
    // Reconstruct the old mistake by hand: a "now" that is really a header height.
    const folded = chromeDecision(st(false), {gap: 400, now: 180, animMs: Date.now()});
    expect(folded.change).toBe(true);
    // Every later reading, with the same wrong clock, is refused as mid-animation.
    expect(chromeDecision(folded, {gap: 0, now: 181, animMs: Date.now()}).change).toBe(false);
    expect(chromeDecision(folded, {gap: 0, now: 9_999, animMs: Date.now()}).change).toBe(false);

    // With a real clock the same sequence comes back, which is all the page ever needed.
    const ok = chromeDecision(st(false), {gap: 400, now: 1_000});
    expect(chromeDecision(ok, {gap: 0, now: 1_000 + CHROME_ANIM_MS + 1}).change).toBe(true);
  });
});
