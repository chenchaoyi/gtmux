// collapseDecision — the pure anti-flip-flop rule behind the collapsing header on the
// HQ console (and the terminal/Detail header, which collapse the same way). Extracted
// from HQScreen because it is exactly the logic that regressed repeatedly ("两段/卡涩"
// jank, the header bouncing mid-animation) and had no test.
//
// The feedback loop it breaks: hiding the header GROWS the scroll viewport, which
// changes the viewport's own atBottom/atTop test, which can immediately request the
// REVERSE change — so the header oscillates. The rule: a change that REVERSES the last
// one within `lockMs` is ignored (it's the loop echoing); a settled change, or one in
// the same direction, lands. A no-op (already in the requested state) never counts as a
// flip, so it neither animates nor arms the lock.

export interface CollapseState {
  hidden: boolean; // is the header currently hidden?
  lastFlip: number; // timestamp (ms) of the last committed change; 0 = never
}

export interface CollapseResult {
  change: boolean; // did the state actually change (→ run the animation)?
  hidden: boolean; // the resulting hidden value (unchanged when change=false)
  lastFlip: number; // the resulting lastFlip (advanced only on a real change)
}

// Given the current state, a requested hidden value, and `now` (ms), decide whether to
// commit. `lockMs` is the reversal-suppression window (default 250ms, matching the
// ~200ms collapse animation plus slack).
export function collapseDecision(
  state: CollapseState,
  requestedHide: boolean,
  now: number,
  lockMs = 250,
): CollapseResult {
  // Already in the requested state → nothing to do (and don't arm the lock).
  if (requestedHide === state.hidden) {
    return {change: false, hidden: state.hidden, lastFlip: state.lastFlip};
  }
  // A reversal within the lock window is the viewport-resize echo — ignore it.
  if (now - state.lastFlip < lockMs) {
    return {change: false, hidden: state.hidden, lastFlip: state.lastFlip};
  }
  // A settled, genuine change: commit and arm the lock at `now`.
  return {change: true, hidden: requestedHide, lastFlip: now};
}
