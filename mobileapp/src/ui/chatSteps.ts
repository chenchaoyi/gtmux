// Tool activity is secondary to the conversation, including during a live turn.
// Reading a running turn still shows its commentary and elapsed-time indicator.
// Only the reader opens its details, and that choice survives later updates.
export function stepsOpen(chosen: boolean | undefined): boolean {
  return chosen ?? false;
}

/**
 * segmentKey identifies a step group by the TURN it belongs to, not by where that turn
 * currently sits.
 *
 * The reader's choice used to be keyed `${turnIndex}-${segIndex}`. Indexes are stable
 * while turns are only appended — but the server drops the oldest turns to bound the
 * payload, and each drop shifts every remaining turn down by one. Measured: open the
 * steps on the newest turn, let one old turn fall out, and the expansion is now on the
 * turn AFTER the one that was tapped — open where nobody asked, closed where they did.
 *
 * The turn's own timestamp plus the head of its prompt is what survives that. Two turns
 * would have to share a second AND an opening to collide. A turn with no timestamp keeps
 * the positional key: there is nothing better to key it by, and saying so is better than
 * inventing an identity.
 */
export function segmentKey(
  turn: {time?: string; prompt?: string},
  segIndex: number,
  turnIndex: number,
): string {
  if (!turn.time) return `${turnIndex}-${segIndex}`;
  return `${turn.time}|${(turn.prompt ?? '').slice(0, 40)}|${segIndex}`;
}
