import {collapseDecision, CollapseState} from './collapse';

const st = (hidden: boolean, lastFlip: number): CollapseState => ({hidden, lastFlip});

describe('collapseDecision — anti-flip-flop header collapse', () => {
  it('commits a genuine first change and arms the lock at now', () => {
    const r = collapseDecision(st(false, 0), true, 1000);
    expect(r).toEqual({change: true, hidden: true, lastFlip: 1000});
  });

  it('is a no-op when already in the requested state (no animate, lock untouched)', () => {
    // The key property: an idle "still hidden" request must NOT arm the lock, or a
    // later genuine reveal would be wrongly suppressed.
    const r = collapseDecision(st(true, 500), true, 10_000);
    expect(r).toEqual({change: false, hidden: true, lastFlip: 500});
  });

  it('IGNORES a reversal within the lock window (the viewport-resize echo → the jank)', () => {
    // Just hid at t=1000; the grown viewport immediately asks to show again at t=1100.
    const r = collapseDecision(st(true, 1000), false, 1100);
    expect(r.change).toBe(false);
    expect(r.hidden).toBe(true); // stays hidden — no bounce
    expect(r.lastFlip).toBe(1000); // lock not re-armed
  });

  it('ALLOWS a reversal once the lock window has passed (settled scroll back to top)', () => {
    const r = collapseDecision(st(true, 1000), false, 1300); // 300ms > 250ms lock
    expect(r).toEqual({change: true, hidden: false, lastFlip: 1300});
  });

  it('respects a custom lock window', () => {
    expect(collapseDecision(st(true, 1000), false, 1400, 500).change).toBe(false); // 400 < 500
    expect(collapseDecision(st(true, 1000), false, 1600, 500).change).toBe(true); //  600 > 500
  });

  it('a same-direction repeat within the window is a no-op, not a suppressed change', () => {
    // Already hidden, asked to hide again shortly after — no flip, so the later reveal
    // isn't blocked by a spuriously re-armed lock.
    const again = collapseDecision(st(true, 1000), true, 1100);
    expect(again).toEqual({change: false, hidden: true, lastFlip: 1000});
    const reveal = collapseDecision(again, false, 1300); // 300ms after the ORIGINAL flip
    expect(reveal.change).toBe(true);
  });
});
