// clock — the server's notion of "now", estimated from responses we already make.
//
// Durations shown next to a session (the "Thinking… 42s" label) are computed against
// timestamps the MAC produced (`since`, `activity_at`). Subtracting them from the phone's
// own clock is only right while the two agree, and they routinely do not: a phone whose
// clock runs a few minutes fast turns a twelve-second turn into "Thinking… 3m12s", and
// that number is the whole point of the label — it is what tells a hung turn from a
// thinking one, and only one of those is worth interrupting.
//
// The backwards case was already guarded (a negative elapsed falls back to the bare
// state). This closes the forward one, and it needs nothing new on the wire: every HTTP
// response carries a `Date` header, which is the server's clock.
//
// Accuracy is about a second — Date has one-second resolution and the reading includes
// however long the response took to arrive. That is far inside what a duration in seconds
// needs, and immeasurably better than an arbitrary offset.

let skewSec = 0;
let known = false;

/**
 * noteServerDate records the skew from one response's `Date` header.
 *
 * The LATEST reading wins rather than an average: a phone's clock can be corrected in one
 * step (an NTP sync, a timezone-less manual change), and smoothing would drag the estimate
 * across that jump for as long as the window lasts.
 */
export function noteServerDate(header: string | null | undefined): void {
  if (!header) return;
  const t = Date.parse(header);
  if (Number.isNaN(t)) return;
  skewSec = Math.round((t - Date.now()) / 1000);
  known = true;
}

/** serverNowSec is the current time in the SERVER's frame, in unix seconds. */
export function serverNowSec(): number {
  return Math.floor(Date.now() / 1000) + skewSec;
}

/** How far the phone's clock is from the Mac's, in seconds. 0 until a response is seen. */
export function clockSkewSec(): number {
  return skewSec;
}

/** Whether any response has been read yet — false means serverNowSec is the local clock. */
export function clockKnown(): boolean {
  return known;
}

/** Test seam. */
export function resetClock(): void {
  skewSec = 0;
  known = false;
}
