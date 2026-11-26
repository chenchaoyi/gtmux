import {useLayoutEffect, useMemo, useRef} from 'react';
import type {GtmuxClient, TranscriptTurn} from '../api/client';
import type {Agent} from '../api/types';

type Source = {client: GtmuxClient; pane: string; agent: string; session?: string; home?: string};
type Transcript = {loaded: boolean; failed?: boolean; readOnlyHistory?: boolean; turns: TranscriptTurn[]; reset?: unknown};

// A normal tmux roster drops the observation on binding and may not name the
// bound SID. Keep the reader mounted but concealed until this scope's read settles.
export function useLivePaneSource(client: GtmuxClient, row: Agent, transcript?: Transcript) {
  const previous = useRef<{source: Source; observed: boolean} | null>(null);
  const old = previous.current;
  const samePane = old?.source.client === client && old.source.pane === row.pane_id &&
    old.source.agent === row.agent && (!row.home_id || old.source.home === row.home_id);
  const observation = row.observed_conversation;
  let session = observation?.session_id ?? row.session_id;
  let home = observation?.home_id ?? row.home_id;
  const prior = samePane ? old : null;
  let pending = false;
  if (!session) {
    const verified = transcript?.loaded && !transcript.failed && !transcript.readOnlyHistory;
    const latest = verified ? transcript.turns[transcript.turns.length - 1]?.session_id : undefined;
    pending = !!prior?.observed && !transcript?.loaded;
    if (latest) {
      session = latest;
    } else if (pending || (prior && !prior.observed && !transcript?.reset)) {
      session = prior?.source.session;
    }
  }
  if (session && session === prior?.source.session) home ??= prior.source.home;
  const source = useMemo(() => ({client, pane: row.pane_id, agent: row.agent, home, session}),
    [client, row.pane_id, row.agent, home, session]);
  useLayoutEffect(() => {previous.current = {source, observed: !!observation || pending};}, [source, observation, pending]);
  return {source, pending};
}
