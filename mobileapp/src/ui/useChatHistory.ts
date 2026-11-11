import {useEffect, useMemo, useRef, useState} from 'react';
import {isAuthError} from '../api/client';
import type {GtmuxClient, TranscriptTurn} from '../api/client';
import type {Agent} from '../api/types';

// Source-position IDs identify records across windows; timestamps/text cannot
// distinguish repeated prompts. Never keep an old prefix for identity-less data.
export function mergeChatHistory(older: TranscriptTurn[], recent: TranscriptTurn[]): TranscriptTurn[] {
  if (!older.length || !recent.length || [...older, ...recent].some(t => !t.record_id || !t.turn_ordinal)) return recent;
  const source = (t: TranscriptTurn) => t.session_id ?? t.agent ?? '';
  const sessions = [...new Set(older.map(source))];
  const incoming = [...new Set(recent.map(source))];
  for (let i = 0; i < incoming.length; i++) {
    if (sessions.includes(incoming[i])) continue;
    const next = incoming.slice(i + 1).find(s => sessions.includes(s));
    sessions.splice(next === undefined ? sessions.length : sessions.indexOf(next), 0, incoming[i]);
  }
  // A contradictory source chain cannot safely extend the saved prefix.
  if (incoming.some((s, i) => i > 0 && sessions.indexOf(s) < sessions.indexOf(incoming[i - 1]))) return recent;
  const key = (t: TranscriptTurn) => `${source(t)}\0${t.record_id}`;
  const records = new Map(older.map(t => [key(t), t]));
  for (const t of recent) records.set(key(t), t);
  return [...records.values()].sort((a, b) => sessions.indexOf(source(a)) - sessions.indexOf(source(b)) || a.turn_ordinal! - b.turn_ordinal!);
}

export function useChatHistory(client: GtmuxClient | undefined, agent: Agent, recent: TranscriptTurn[], beforePrepend: () => void) {
  const base = client?.base;
  const identity = useMemo(() => ({client, base, pane: agent.pane_id, session: agent.session_id, home: agent.home_id, agent: agent.agent}), [client, base, agent.pane_id, agent.session_id, agent.home_id, agent.agent]);
  const current = useRef(identity); current.current = identity;
  const [record, setRecord] = useState<{identity: typeof identity; turns: TranscriptTurn[]; loading: boolean; failed: boolean; deniedFor?: TranscriptTurn[]}>({identity, turns: [], loading: false, failed: false});
  const request = useRef<AbortController | undefined>(undefined);
  useEffect(() => {setRecord({identity, turns: [], loading: false, failed: false}); return () => {request.current?.abort(); request.current = undefined;};}, [identity]);
  const turns = useMemo(() => record.identity === identity && record.deniedFor === recent ? [] : record.identity === identity ? mergeChatHistory(record.turns, recent) : recent, [record, identity, recent]);
  const cursor = turns[0]?.history_before;
  const load = async () => {
    if (!client || !cursor || request.current) return 0;
    const controller = new AbortController(); request.current = controller;
    let timer: ReturnType<typeof setTimeout>;
    let abortDeadline: () => void;
    const deadline = new Promise<never>((_, reject) => {
      abortDeadline = () => reject(new Error('History read cancelled'));
      controller.signal.addEventListener('abort', abortDeadline, {once: true});
      timer = setTimeout(() => controller.abort(), 15000);
    });
    setRecord(previous => ({identity, turns: previous.identity === identity ? previous.turns : [], loading: true, failed: false}));
    try {
      const page = await Promise.race([client.traceHistory(agent, cursor, controller.signal), deadline]);
      if (controller.signal.aborted || current.current !== identity || request.current !== controller) return 0;
      if (!page.length || page.some(t => !t.record_id) || page[0].record_id === turns[0]?.record_id) throw new Error('History did not progress');
      beforePrepend();
      setRecord({identity, turns: mergeChatHistory(page, turns), loading: false, failed: false});
      return page.length;
    } catch (error) {
      if (current.current === identity && request.current === controller) setRecord(previous => ({identity, turns: isAuthError(error) ? [] : previous.identity === identity ? previous.turns : [], loading: false, failed: true, ...(isAuthError(error) ? {deniedFor: recent} : {})}));
      return 0;
    } finally {
      clearTimeout(timer!);
      controller.signal.removeEventListener('abort', abortDeadline!);
      if (request.current === controller) request.current = undefined;
    }
  };
  return {turns, load, denied: record.identity === identity && record.deniedFor === recent, available: !!client && !!cursor, loading: record.identity === identity && record.loading, failed: record.identity === identity && record.failed && (!record.deniedFor || record.deniedFor === recent)};
}
