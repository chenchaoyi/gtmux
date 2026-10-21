import {useCallback, useEffect, useRef, useState, type RefObject} from 'react';
import type {GtmuxClient} from '../api/client';
import type {PaneInteraction} from '../api/types';
import {createReadTask, startReadPolling} from './readTask';

export function usePaneInteraction(client: GtmuxClient, pane: string, active: boolean, answeredAt: RefObject<number>) {
  const [record, setRecord] = useState<{client: GtmuxClient; pane: string; value: PaneInteraction} | null>(null);
  const [loading, setLoading] = useState(active);
  const [failed, setFailed] = useState(false);
  const lane = useRef<ReturnType<typeof createReadTask> | null>(null);
  useEffect(() => {
    setRecord(null); setLoading(active); setFailed(false);
    if (!active) return;
    const task = createReadTask(signal => client.interaction(pane, signal), next => {
      if (Date.now() - answeredAt.current < 1500) next = {...next, options: []};
      setRecord(prev => prev?.client === client && prev.pane === pane && JSON.stringify(prev.value) === JSON.stringify(next) ? prev : {client, pane, value: next});
      setFailed(false);
    }, {onError: () => {setRecord(null); setFailed(true);}, onSettled: () => setLoading(false)});
    lane.current = task;
    const stop = startReadPolling(task, 1500);
    return () => { stop(); if (lane.current === task) lane.current = null; };
  }, [client, pane, active, answeredAt]);
  const refresh = useCallback(() => {lane.current?.invalidate(); lane.current?.run();}, []);
  const answered = useCallback(() => {
    answeredAt.current = Date.now();
    setRecord(prev => prev ? {...prev, value: {...prev.value, options: []}} : prev);
    lane.current?.invalidate();
  }, [answeredAt]);
  const matches = active && record?.client === client && record.pane === pane;
  return {snapshot: matches ? record.value : null, loading: active && (loading || !record && !failed), failed: active && failed, refresh, answered};
}
