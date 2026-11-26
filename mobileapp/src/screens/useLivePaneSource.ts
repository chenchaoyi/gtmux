import {useLayoutEffect, useMemo, useRef} from 'react';
import type {GtmuxClient, TranscriptTurn} from '../api/client';
import type {Agent} from '../api/types';
import type {SessionReset} from '../ui/chatWindow';

type Source = {client: GtmuxClient; pane: string; agent: string; generation: number};
type Reader = {source: Source; observed: boolean; session?: string; home?: string; reset?: string};
type Transcript = {
  loaded: boolean; failed?: boolean; retryBinding?: boolean; readOnlyHistory?: boolean;
  turns: TranscriptTurn[]; reset?: SessionReset; includesEarlier?: boolean;
};

export function useLivePaneSource(client: GtmuxClient, row: Agent, transcript?: Transcript) {
  const previous = useRef<Reader | null>(null);
  const old = previous.current;
  const observation = row.observed_conversation;
  const home = observation?.home_id ?? row.home_id;
  const prior = old?.source.client === client && old.source.pane === row.pane_id &&
    old.source.agent === row.agent && !(home && old.home && home !== old.home) ? old : null;
  const verified = transcript?.loaded && !transcript.failed && !transcript.readOnlyHistory;
  const marker = verified ? transcript.reset : undefined;
  const reset = marker ? JSON.stringify([marker.kind, marker.id ?? marker.at]) : undefined;
  // With earlier turns loaded, the last row may still belong to the predecessor
  // of a just-cleared session. The reset header describes the current pane.
  const latest = verified && !(transcript.includesEarlier && marker)
    ? transcript.turns[transcript.turns.length - 1]?.session_id : undefined;
  let session = observation?.session_id ?? row.session_id ?? latest;
  // The ordinary roster can omit a binding's SID. Keep the observed reader
  // concealed across a temporary outage until that binding can be checked.
  const pending = !session && !!prior?.observed &&
    (!transcript?.loaded || (transcript.failed && !!transcript.retryBinding));
  if (!session && (pending || (!prior?.observed && (!reset || reset === prior?.reset)))) session = prior?.session;

  // The first verified turn names an existing pane reader. Only a replacement
  // needs a new native scroll view and a fresh pane capture.
  const replaced = !prior || (!!prior.session && session !== prior.session) || (!!reset && reset !== prior.reset);
  const generation = (old?.source.generation ?? 0) + (replaced ? 1 : 0);
  const source = useMemo(() => ({client, pane: row.pane_id, agent: row.agent, generation}),
    [client, row.pane_id, row.agent, generation]);
  useLayoutEffect(() => {
    previous.current = {source, session, home: home ?? (replaced ? undefined : prior?.home),
      observed: !!observation || !!pending, reset: reset ?? (replaced ? undefined : prior?.reset)};
  });
  return {source, pending: !!pending};
}
