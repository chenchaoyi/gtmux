// Reconcile one paired Mac's APNs subscription. The registration lives on the
// Mac, independently of which Mac the phone is currently viewing.
import {AppState} from 'react-native';
import {GtmuxClient} from '../api/client';
import {abortAtDeadline} from '../api/requestAbort';
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
  const foreground = () => AppState.currentState !== 'background' && AppState.currentState !== 'inactive';
  for (const url of addresses) {
    if (!foreground()) return false;
    const controller = new AbortController();
    let paused = false;
    const subscription = AppState.addEventListener('change', state => {
      if (state !== 'active') {paused=true;controller.abort();}
    });
    const timer = setTimeout(() => {
      if (foreground()) abortAtDeadline(controller, 4000);
      else {paused=true;controller.abort();}
    }, 4000);
    let onAbort: (() => void) | undefined;
    const cancelled = new Promise<boolean>(resolve => {
      onAbort = () => resolve(false);
      controller.signal.addEventListener('abort', onAbort, {once:true});
    });
    try {
      const client = makeClient(url, server.token);
      const request = enabled
        ? client.registerPush(token, kinds, env, controller.signal)
        : client.unregisterPush(token, undefined, controller.signal);
      const ok = await Promise.race([request, cancelled]);
      if (paused || !foreground()) return false;
      if (ok) return true;
    } catch {
      if (paused || !foreground()) return false;
      // Offline or old route: try the next reported address.
    } finally {
      clearTimeout(timer);
      subscription.remove();
      if (onAbort) controller.signal.removeEventListener('abort', onAbort);
    }
  }
  return false;
}
