// liveEdge — when the collapsing top chrome folds and unfolds, as one testable rule.
//
// The chrome (Detail's header + neighbour strip + segmented control · the HQ page's
// verdict card) folds away while you read back through history and returns at the live
// tail. That is the design, and it took three user reports to get the MECHANISM right.
//
// The first two implementations folded by animating the chrome's HEIGHT, which resizes
// the scroll viewport underneath it. Everything that went wrong followed from that one
// decision:
//
//   · the viewport is what the "am I at the tail" test measures, so folding changed the
//     answer, which asked the chrome to do the opposite — 117pt of travel and three
//     reversals before it settled ("回到底部的时候会跳来跳去", 2026-09-05)
//   · growing the viewport at the TOP slides the content up by the same amount: 115pt in
//     200ms, nobody asked for it ("屏幕会向上弹跳一小段", 2026-09-09)
//   · holding the content still through that means writing `contentOffset` every frame,
//     which the pan recogniser overwrites from its own translation on the next frame, so
//     the scroll STICKS at the fold point ("到了折叠的地方就会停住", 2026-09-10) — and
//     waiting for the gesture to end instead just moved the cost to the fold arriving
//     late ("遮掩、收回延迟感比较大", same day). Those same writes also cancel an
//     in-flight animated `scrollToEnd`, which is why the jump-to-bottom arrow flashed the
//     chrome and went nowhere.
//
// None of those are fixable in the decision rule, because none of them are decisions. So
// the chrome no longer participates in layout: it FLOATS above the scroll view and slides
// out on `translateY`, and the scroll view keeps one frame for its whole life. The
// content carries a constant top padding the height of the chrome, so the oldest line can
// still be scrolled clear of it.
//
// With the geometry constant there is nothing to compensate, nothing to fight, and no
// feedback: the gap the decision reads means the same thing before and after a fold. That
// removed the offset-driving code, the mid-gesture wait, and the chrome-derived threshold
// that existed only to out-run the loop. Folding at 72pt from the tail instead of ~250
// is the visible half of that.
//
// What remains is plain hysteresis, which is still worth having: it keeps a scroll resting
// near the threshold from flickering.

/** Within this many points of the tail, you are at the live edge. */
export const AT_TAIL = 40;

/**
 * The separation between the two thresholds. Its only job is to keep a reveal from
 * landing exactly ON the fold line, where rounding would decide what happens next.
 */
export const FOLD_SLACK = 32;

/** The collapse animation, in ms. Decisions are frozen for this long after a change. */
export const CHROME_ANIM_MS = 200;

/** The gap at or beyond which the chrome folds. */
export const FOLD_AT = AT_TAIL + FOLD_SLACK;

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

/**
 * chromeDecision answers "fold, unfold, or leave it" from one distance reading.
 *
 * `gap` is how far the content's tail is below the viewport's bottom edge, in points; 0
 * means pinned to the live edge. A top-anchored list (the HQ page's zones, where the
 * chrome sits above content that does not scroll under it) passes its distance from the
 * TOP and reads the same way.
 *
 * Readings that arrive mid-animation are ignored: they describe a layout on its way
 * somewhere else. The host re-asks with the latest gap when the animation finishes, so a
 * genuine change that arrived mid-flight is answered one animation later, not dropped.
 */
export function chromeDecision(
  state: ChromeState,
  gap: number,
  now: number,
  animMs = CHROME_ANIM_MS,
): ChromeDecision {
  const keep: ChromeDecision = {change: false, hidden: state.hidden, settledAt: state.settledAt};
  if (now < state.settledAt) return keep;
  const wantHidden = gap >= FOLD_AT ? true : gap <= AT_TAIL ? false : state.hidden;
  if (wantHidden === state.hidden) return keep;
  return {change: true, hidden: wantHidden, settledAt: now + animMs};
}
