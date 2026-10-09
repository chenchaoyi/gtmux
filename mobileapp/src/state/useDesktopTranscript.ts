import {useEffect, useState} from 'react';
import {DesktopTranscriptError, GtmuxClient, TranscriptTurn} from '../api/client';

export function useDesktopTranscript(client: GtmuxClient, id: string, enabled: boolean) {
  const [retry, setRetry] = useState(0);
  const [state, setState] = useState<{client: GtmuxClient; id: string; turns: TranscriptTurn[]; dropped: number; etag?: string; loading: boolean; error?: number}>({client, id, turns: [], dropped: 0, loading: true});
  useEffect(() => {
    setState({client, id, turns: [], dropped: 0, loading: true});
  }, [client, id]);
  useEffect(() => {
    if (!enabled || !id) return;
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let timeout: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;
    let etag: string | undefined;
    const pull = async () => {
      controller = new AbortController();
      timeout = setTimeout(() => controller?.abort(), 15_000);
      try {
        const result = await client.desktopTranscript(id, etag, controller.signal);
        if (!alive) return;
        etag = result.etag;
        setState(previous => result.unchanged && previous.client === client && previous.id === id
          ? {...previous, loading: false, error: undefined}
          : {client, id, turns: result.turns, dropped: result.dropped, etag, loading: false});
      } catch (error) {
        if (alive) setState(previous => ({...previous, loading: false, error: error instanceof DesktopTranscriptError ? error.status : 0}));
      } finally {
        clearTimeout(timeout);
        if (alive) timer = setTimeout(pull, 2000);
      }
    };
    pull();
    return () => {alive = false; clearTimeout(timer); clearTimeout(timeout); controller?.abort();};
  }, [client, id, enabled, retry]);
  const current = state.client === client && state.id === id ? state : {client, id, turns: [], dropped: 0, loading: true};
  return {...current, retry: () => setRetry(value => value + 1)};
}
