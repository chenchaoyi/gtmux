import type {TranscriptTurn} from '../api/client';
import {promptIdentity} from './promptIdentity';

export type PendingChatEcho = {id: string; text: string; observed: Map<string, number>};
function occurrences(text: string, turns: TranscriptTurn[]) {
  const counts = new Map<string, number>();
  for (const turn of turns) {
    if (promptIdentity(turn.prompt) !== promptIdentity(text)) continue;
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
export function chatEchoRecord(echo: PendingChatEcho, turns: TranscriptTurn[]): number {
  const counts = new Map<string, number>();
  const identity = promptIdentity(echo.text);
  return turns.findIndex(turn => {
    if (promptIdentity(turn.prompt) !== identity) return false;
    const key = turn.time || '';
    const count = (counts.get(key) || 0) + 1; counts.set(key, count);
    return count > (echo.observed.get(key) || 0);
  });
}
export function chatEchoRecorded(echo: PendingChatEcho, turns: TranscriptTurn[]): boolean {
  return chatEchoRecord(echo, turns) >= 0;
}
