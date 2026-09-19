// SSE subscription for GET /api/events. The contract: `agents` ⇒ refetch
// /api/agents (the ONLY data source), `alert` ⇒ in-app banner, `ping` ⇒ ignore.
// /api/agents stays the single authoritative payload; SSE only signals *that*
// something changed.

import EventSource from 'react-native-sse';
import {Alert} from './types';
import {clientTag} from './client';
import {Diag} from '../diag';

export type Unsubscribe = () => void;

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
  let up: boolean | null = null;
  es.addEventListener('open', () => {
    if (up === false) Diag.info('sse.connected', 'the live stream from the Mac is back');
    up = true;
    handlers.onOpen?.();
  });
  es.addEventListener('error', (e: any) => {
    if (up !== false) {
      Diag.warn('sse.disconnected', 'the live stream from the Mac dropped',
        {error: String(e?.message ?? e?.type ?? ''), status: typeof e?.xhrStatus === 'number' ? e.xhrStatus : undefined});
    }
    up = false;
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
