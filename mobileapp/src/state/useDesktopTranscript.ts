import {useEffect, useRef, useState} from 'react';
import {DesktopTranscriptError, GtmuxClient, TranscriptTurn} from '../api/client';
import {createReadTask, startReadPolling} from './readTask';

export function useDesktopTranscript(client: GtmuxClient, id: string, enabled: boolean, homeID?: string) {
  const reload = useRef<() => void>(() => {});
  const [state, setState] = useState<{client: GtmuxClient; id: string; homeID?: string; turns: TranscriptTurn[]; dropped: number; loading: boolean; retrying?: boolean; attempted?: boolean; error?: number}>({client, id, homeID, turns: [], dropped: 0, loading: true});
  useEffect(() => {setState({client, id, homeID, turns: [], dropped: 0, loading: true});}, [client, id, homeID]);
  useEffect(() => {
    if (!enabled || !id) return;
    let etag: string | undefined;
    const task = createReadTask(signal => client.desktopTranscript(id, etag, signal, homeID), result => {
      etag = result.etag;
      setState(previous => result.unchanged ? {...previous, loading: false, error: undefined} : {...previous, client, id, homeID, turns: result.turns, dropped: result.dropped, loading: false, error: undefined});
    }, {onError: error => setState(previous => {
      const status = error instanceof DesktopTranscriptError ? error.status : 0;
      return {...previous, ...(status === 401 || status === 403 ? {turns: [], dropped: 0} : {}), loading: false, error: status};
    }), onSettled: () => setState(previous => previous.retrying ? {...previous, retrying: false} : previous)});
    reload.current = () => {
      setState(previous => ({...previous, retrying: true, attempted: true}));
      task.invalidate(); etag = undefined; task.run();
    };
    const stop = startReadPolling(task, 2000);
    return () => {reload.current = () => {}; stop();};
  }, [client, id, homeID, enabled]);
  const current = state.client === client && state.id === id && state.homeID === homeID ? state : {client, id, homeID, turns: [], dropped: 0, loading: true, error: undefined, retrying: false, attempted: false};
  return {...current, retryFailed: !!current.attempted && !current.retrying && current.error !== undefined, retry: () => reload.current()};
}
