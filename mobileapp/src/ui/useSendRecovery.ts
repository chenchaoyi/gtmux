import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import type {SendPayload} from '../api/client';
import type {SendFailureCode} from '../api/sendFailureCode';

export type HeldSendFailure = {
  scope: object;
  payload: SendPayload;
  reason: string;
  status?: number;
  code?: SendFailureCode;
  retrying: boolean;
};

/** One recoverable attempt, bound to the actual client/conversation identity. */
export function useSendRecovery(scope: object) {
  const activeScope = useRef(scope); activeScope.current = scope;
  const mounted = useRef(true);
  const held = useRef<HeldSendFailure | null>(null);
  const [snapshot, setSnapshot] = useState<HeldSendFailure | null>(null);
  const [restoration, setRestoration] = useState<{scope: object; text: string; at: number} | null>(null);
  const restoreSequence = useRef(0);
  useEffect(() => {mounted.current = true; return () => {mounted.current = false;};}, []);
  const isCurrent = useCallback(() => mounted.current && activeScope.current === scope, [scope]);
  const put = useCallback((next: HeldSendFailure | null) => {held.current = next; setSnapshot(next);}, []);
  useEffect(() => {
    if (held.current?.scope !== scope) put(null);
    setRestoration(previous => previous?.scope === scope ? previous : null);
  }, [scope, put]);

  const begin = useCallback((payload: SendPayload) => {
    if (!isCurrent()) return false;
    const previous = held.current;
    if (previous?.scope === scope && previous.payload.send_id === payload.send_id) {
      if (previous.retrying) return false;
      // Ref first: two captured presses in the same React frame start one retry.
      put({...previous, retrying: true});
    } else put(null);
    return true;
  }, [isCurrent, scope, put]);
  const record = useCallback((payload: SendPayload, reason: string, status?: number, code?: SendFailureCode) => {
    if (!isCurrent()) return false;
    put({scope, payload, reason, status, code, retrying: false});
    return true;
  }, [isCurrent, scope, put]);
  const accept = useCallback((payload: SendPayload) => {
    if (!isCurrent()) return;
    if (held.current?.scope === scope && held.current.payload.send_id === payload.send_id) put(null);
  }, [isCurrent, scope, put]);
  const retry = useCallback((expected: HeldSendFailure, dispatch: (payload: SendPayload) => void) => {
    if (!isCurrent() || held.current !== expected || expected.scope !== scope || expected.retrying) return;
    dispatch(expected.payload);
  }, [isCurrent, scope]);
  const edit = useCallback((expected: HeldSendFailure) => {
    if (!isCurrent() || held.current !== expected || expected.scope !== scope || expected.retrying) return;
    if (expected.payload.text) setRestoration({scope, text: expected.payload.text, at: ++restoreSequence.current});
    put(null);
  }, [isCurrent, scope, put]);
  const failure = snapshot?.scope === scope ? snapshot : null;
  const restore = restoration?.scope === scope ? restoration : null;
  return useMemo(() => ({failure, restore, begin, record, accept, edit, retry, isCurrent}),
    [failure, restore, begin, record, accept, edit, retry, isCurrent]);
}
