// Reconcile one paired Mac's APNs subscription. The registration lives on the
// Mac, independently of which Mac the phone is currently viewing.
import {GtmuxClient} from '../api/client';
import {PairedMac} from '../pairing/qr';

export interface PushTarget {
  registerPush(token: string, kinds: string[], env: string, signal?: AbortSignal): Promise<boolean>;
  unregisterPush(token: string, activityToken?: string, signal?: AbortSignal): Promise<boolean>;
}

// Run network writes in order; a newer preference supersedes queued older work.
// An in-flight request completes before the newer request starts.
export class PushSyncQueue {
  private tail: Promise<void> = Promise.resolve();
  private generation = 0;

  invalidate(): void { this.generation++; }

  schedule(work: (current: () => boolean) => Promise<void>): Promise<void> {
    const generation = ++this.generation;
    this.tail = this.tail.catch(() => {}).then(() => work(() => generation === this.generation));
    return this.tail;
  }
}

// Whether a Mac may reach this phone's lock screen at all: alerts, silent badges and the
// Live Activity alike. The device-wide switch, at least one alert kind and this Mac's own
// bell must all be on. A guest link has no bell, so only the device-wide part applies.
export function mayNotify(
  pushEnabled: boolean,
  kinds: {waiting: boolean; done: boolean},
  server: Pick<PairedMac, 'pushEnabled'>,
): boolean {
  return pushEnabled && (kinds.waiting || kinds.done) && server.pushEnabled !== false;
}

export async function syncServerPush(
  server: PairedMac,
  token: string,
  enabled: boolean,
  kinds: string[],
  env: string,
  makeClient: (url: string, auth: string) => PushTarget = (url, auth) => new GtmuxClient(url, auth),
): Promise<boolean> {
  if (server.scope === 'guest' || !token) return false;
  // A Mac may have moved between Direct routes. Try only addresses it reported
  // for this pairing, with the same owner credential and a short deadline.
  const addresses = [...new Set([server.url, ...(server.alts ?? [])])].filter(u => /^https?:\/\//.test(u));
  for (const url of addresses) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 4000);
    try {
      const client = makeClient(url, server.token);
      const ok = enabled
        ? await client.registerPush(token, kinds, env, controller.signal)
        : await client.unregisterPush(token, undefined, controller.signal);
      if (ok) return true;
    } catch {
      // Offline or old route: try the next reported address.
    } finally {
      clearTimeout(timer);
    }
  }
  return false;
}
