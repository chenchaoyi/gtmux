// chatSteps — whether an agent's tool steps start expanded (hq-page-shows-its-work).
//
// The supervisor can take minutes over one turn. What the phone showed while that
// happened was an elapsed timer, and what it showed afterwards was an 11.5pt toggle in
// the dimmest colour on the page reading "6 steps". So the process was presented exactly
// when it could not be watched, and hidden while it could — which is the whole of
// 「对话过程不可见」.
//
// Watching and archaeology are different needs. The turn in flight opens itself; history
// stays closed. A tap always wins over both: once the reader has said what they want for
// a segment, nothing decides it for them again.

/** Which segment this is, for the default decision. */
export interface StepContext {
  /** This segment belongs to the newest turn in the transcript. */
  isLatestTurn: boolean;
  /** The agent is mid-turn right now. */
  working: boolean;
}

/**
 * stepsOpenByDefault decides a segment's initial state.
 *
 * Only the turn IN FLIGHT opens. A finished turn — even the newest one — stays closed,
 * so a conversation scrolled back through reads as a conversation rather than as a log
 * of tool calls.
 */
export function stepsOpenByDefault(ctx: StepContext): boolean {
  return ctx.isLatestTurn && ctx.working;
}

/**
 * stepsOpen resolves the actual state: the reader's choice when they have made one, the
 * default otherwise.
 *
 * The distinction matters at the moment a turn ends. The default flips to closed then,
 * which is right for a reader who never touched it — and wrong for one who deliberately
 * opened the steps to read them, whose choice must survive the turn finishing.
 */
export function stepsOpen(chosen: boolean | undefined, ctx: StepContext): boolean {
  return chosen ?? stepsOpenByDefault(ctx);
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
