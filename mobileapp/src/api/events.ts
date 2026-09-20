// SSE subscription for GET /api/events. The contract: `agents` ⇒ refetch
// /api/agents (the ONLY data source), `alert` ⇒ in-app banner, `ping` ⇒ ignore.
// /api/agents stays the single authoritative payload; SSE only signals *that*
// something changed.

import EventSource from 'react-native-sse';
import {Alert} from './types';
import {clientTag} from './client';
import {Diag} from '../diag';

export type Unsubscribe = () => void;

/** Per-Mac stream state, so a retry loop does not re-record the same outage. */
const streamState = new Map<string, {up: boolean | null; downAt: number}>();

export function subscribe(
  base: string,
  token: string,
  handlers: {
    onAgents: () => void;
    onAlert: (a: Alert) => void;
    onOpen?: () => void;
    onError?: () => void;
  },
): Unsubscribe {
  const es = new EventSource(`${base}/api/events`, {
    headers: {Authorization: `Bearer ${token}`, 'X-Gtmux-Client': clientTag()},
    // react-native-sse reconnects on drop by default.
  });

  // The live stream's drops and returns go to the diagnostics buffer, once per change:
  // react-native-sse retries every few seconds, and a dead Mac must not fill the buffer.
  // Whether the stream to this Mac is up, and when it went down — kept ACROSS
  // subscriptions, not inside one. The app rebuilds the subscription on every failed
  // attempt (AgentsContext), so a per-call flag would write one "dropped" line per retry:
  // a Mac left off for an hour would push the entries that explain it out of a 500-entry
  // buffer with its own noise.
  const st = streamState.get(base) ?? {up: null, downAt: 0};
  streamState.set(base, st);
  es.addEventListener('open', () => {
    if (st.up === false) {
      Diag.info('sse.connected', 'the live stream from the Mac is back',
        st.downAt ? {downSec: Math.round((Date.now() - st.downAt) / 1000)} : undefined);
    }
    st.up = true;
    st.downAt = 0;
    handlers.onOpen?.();
  });
  es.addEventListener('error', (e: any) => {
    if (st.up !== false) {
      st.downAt = Date.now();
      Diag.warn('sse.disconnected', 'the live stream from the Mac dropped',
        {error: String(e?.message ?? e?.type ?? ''), status: typeof e?.xhrStatus === 'number' ? e.xhrStatus : undefined});
    }
    st.up = false;
    handlers.onError?.();
  });
  // Custom SSE event names from the server.
  (es as any).addEventListener('agents', () => handlers.onAgents());
  (es as any).addEventListener('alert', (e: any) => {
    try {
      handlers.onAlert(JSON.parse(e.data) as Alert);
    } catch {
      // ignore malformed alert payloads
    }
  });
  // 'ping' is intentionally ignored — it only keeps the stream alive.

  return () => es.close();
}
