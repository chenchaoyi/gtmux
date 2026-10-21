import {useEffect, useState} from 'react';
import type {GtmuxClient} from '../api/client';
import type {ParticipantProfile, ProfileTarget} from './participantProfile';

type Request = Parameters<GtmuxClient['participantProfile']>[0];
type Status = 'loading' | 'ready' | 'failed' | 'unrecorded';
export interface ProfileRead {status: Status; profile?: ParticipantProfile; retry: () => void}

export function participantRequest(target: ProfileTarget): Request | undefined {
  if (target.kind !== 'agent' || target.turn && (!target.turn.session_id || !target.turn.time)) return;
  return {id: target.agent.source === 'native' ? undefined : target.agent.pane_id || undefined,
    session_id: target.turn?.session_id ?? target.agent.session_id, home_id: target.agent.home_id, at: target.turn?.time};
}

/** An old message may inspect current observations only for the same source session. */
export function currentChainRequest(target: ProfileTarget): Request | undefined {
  if (target.kind !== 'agent' || !target.turn?.session_id) return;
  if (target.agent.session_id && target.turn.session_id !== target.agent.session_id) return;
  // Tmux rows identify a pane, not its session. The profile endpoint verifies
  // this message's session against the pane's current binding before returning.
  if (target.agent.source === 'native' ? !target.agent.session_id : !target.agent.pane_id) return;
  return {id: target.agent.source === 'native' ? undefined : target.agent.pane_id || undefined,
    session_id: target.turn.session_id, home_id: target.agent.home_id};
}

export function useParticipantProfile(client: GtmuxClient | undefined, request: Request | undefined, enabled = true): ProfileRead {
  const key = request ? JSON.stringify(request) : undefined;
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{key: string; client: GtmuxClient; attempt: number; status: Status; profile?: ParticipantProfile}>();
  useEffect(() => {
    if (!enabled || !client || !key) return;
    const abort = new AbortController();
    const identity = {key, client, attempt};
    setResult({...identity, status: 'loading'});
    client.participantProfile(JSON.parse(key) as Request, abort.signal).then(profile => {
      if (!abort.signal.aborted) setResult({...identity, status: 'ready', profile});
    }).catch(() => {
      if (!abort.signal.aborted) setResult({...identity, status: 'failed'});
    });
    return () => abort.abort();
  }, [client, key, enabled, attempt]);
  // Derive the pending state before effects run, including when the target changes.
  // A previous target's facts and the "unrecorded" notice must never flash here.
  const matches = result?.key === key && result?.client === client && result?.attempt === attempt;
  const status = !enabled || !client || !key ? 'unrecorded' : matches ? result!.status : 'loading';
  return {status, profile: status === 'ready' ? result?.profile : undefined, retry: () => setAttempt(v => v + 1)};
}
