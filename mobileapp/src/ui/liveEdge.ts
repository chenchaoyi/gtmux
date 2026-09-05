// liveEdge — when the collapsing top chrome folds and unfolds, as one testable rule.
//
// The chrome (Detail's header + neighbour strip + segmented control · the HQ page's
// verdict card) folds away while you read back through history and returns at the live
// tail. That is the design. The implementation had a feedback loop in it:
//
//   folding the chrome GROWS the scroll viewport
//     → the viewport is what the "am I at the tail" test measures
//       → so the answer changes
//         → which asks the chrome to do the opposite
//
// Measured on a real pane (2026-09-05, `e2e/__tests__/edge-stability.test.ts`, flicking
// back down to the tail and then holding still):
//
//   gap=28.3 view=692.7   ← under the 40pt threshold: reveal
//   gap=42.7 view=635.3   ← revealing pushed the gap back OVER it: fold
//   gap=25.3 view=628.7   ← folding pulled it back under: reveal
//   gap=22.0 view=576.0   ← …settling only after three reversals and 117pt of travel
//
// Nothing was moving but the app arguing with itself, and that argument is what reads as
// "回到底部的时候会跳来跳去". The old defence was a 250ms lock that ignored a reversal
// arriving too soon after the last one — a timing guess, and the trace above happened
// with it in place, because the reversals were not echoes. They were the honest answer to
// a question asked with the wrong geometry: a 40pt threshold cannot be stable when the
// thing being toggled is 117pt tall.
//
// So the thresholds are now derived from the chrome, and the loop is impossible rather
// than merely discouraged:
//
//   reveal when   gap ≤ AT_TAIL
//   fold when     gap ≥ AT_TAIL + chromeH + FOLD_SLACK
//   otherwise     stay as you are
//
// Revealing costs at most `chromeH` of gap, so a reveal lands at gap ≤ AT_TAIL + chromeH,
// which is below the fold line by FOLD_SLACK. Folding gives back at most `chromeH`, so a
// fold lands at gap ≥ AT_TAIL + FOLD_SLACK, which is above the reveal line. Neither
// transition can trigger the other. That is a property of the arithmetic, and
// `liveEdge.test.ts` checks it by simulating the loop rather than by trusting the prose.
//
// The second rule covers the frames DURING the animation, which the trace above is mostly
// made of: while the height is animating, every measurement describes a layout that is on
// its way somewhere else, so no decision is taken from one. The host re-asks with the
// latest gap when the animation finishes, which is what makes the freeze safe: a genuine
// change that arrived mid-flight is answered one animation later, not dropped.

/** Within this many points of the tail, you are at the live edge. */
export const AT_TAIL = 40;

/**
 * How far past "the tail plus the chrome you would be revealing" you must be before the
 * chrome folds again. It is the separation between the two thresholds, and the only
 * reason it is not zero: at zero, a reveal lands exactly ON the fold line and rounding
 * decides what happens next.
 */
export const FOLD_SLACK = 32;

/** The collapse animation, in ms. Decisions are frozen for this long after a change. */
export const CHROME_ANIM_MS = 200;

export interface ChromeState {
  /** Is the chrome currently folded away? */
  hidden: boolean;
  /** ms timestamp at which the last committed change finishes animating; 0 = settled. */
  settledAt: number;
}

export interface ChromeDecision {
  /** Did the state actually change (→ run the animation)? */
  change: boolean;
  hidden: boolean;
  settledAt: number;
}

/** The gap at or beyond which the chrome folds, given how tall the chrome is. */
export function foldThreshold(chromeH: number): number {
  return AT_TAIL + Math.max(0, chromeH) + FOLD_SLACK;
}

/**
 * chromeDecision answers "fold, unfold, or leave it" from one distance reading.
 *
 * `gap` is how far the content's tail is below the viewport's bottom edge, in points; 0
 * means pinned to the live edge. A top-anchored list (the HQ page's calls/activity zones,
 * where folding the chrome does not move the content) passes its distance from the TOP
 * and a `chromeH` of 0 — it has no feedback to defend against, and still gets the
 * hysteresis, which is what keeps a scroll resting near the threshold from flickering.
 *
 * `chromeH` is the height the chrome occupies when shown. The caller measures it; a 0
 * (not yet laid out) simply narrows the band to AT_TAIL…AT_TAIL+FOLD_SLACK.
 */
export function chromeDecision(
  state: ChromeState,
  gap: number,
  chromeH: number,
  now: number,
  animMs = CHROME_ANIM_MS,
): ChromeDecision {
  const keep: ChromeDecision = {change: false, hidden: state.hidden, settledAt: state.settledAt};
  // Mid-animation: this reading describes a layout that is still moving.
  if (now < state.settledAt) return keep;

  const wantHidden =
    gap >= foldThreshold(chromeH) ? true : gap <= AT_TAIL ? false : state.hidden;
  if (wantHidden === state.hidden) return keep;
  return {change: true, hidden: wantHidden, settledAt: now + animMs};
}
