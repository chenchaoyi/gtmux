// SSE subscription for GET /api/events. The contract: `agents` ⇒ refetch
// /api/agents (the ONLY data source), `alert` ⇒ in-app banner, `ping` ⇒ ignore.
// /api/agents stays the single authoritative payload; SSE only signals *that*
// something changed.

import EventSource from 'react-native-sse';
import {Alert} from './types';

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
    headers: {Authorization: `Bearer ${token}`},
    // react-native-sse reconnects on drop by default.
  });

  es.addEventListener('open', () => handlers.onOpen?.());
  es.addEventListener('error', () => handlers.onError?.());
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
