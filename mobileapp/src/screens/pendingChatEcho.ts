import type {TranscriptTurn} from '../api/client';

export type PendingChatEcho = {id: string; text: string; observed: Map<string, number>};
const textOf = (text: string) => text.replace(/\r\n/g, '\n').trim();
function occurrences(text: string, turns: TranscriptTurn[]) {
  const counts = new Map<string, number>();
  for (const turn of turns) {
    if (textOf(turn.prompt) !== textOf(text)) continue;
    const key = turn.time || '';
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  return counts;
}
export function beginChatEcho(id: string, text: string, turns: TranscriptTurn[]): PendingChatEcho {
  return {id, text, observed: occurrences(text, turns)};
}
// Match any newly recorded occurrence, not only the tail: polling may skip turns.
// A pre-existing identical prompt cannot confirm a new send. Missing timestamps
// use occurrence counts; a bounded transcript can leave such a send unconfirmed.
export function chatEchoRecorded(echo: PendingChatEcho, turns: TranscriptTurn[]): boolean {
  return [...occurrences(echo.text, turns)].some(([key, count]) => count > (echo.observed.get(key) || 0));
}
