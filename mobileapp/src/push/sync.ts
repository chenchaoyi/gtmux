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

export type PushSyncState = 'syncing' | 'pending' | 'synced';

// Intent and successful route stay in memory, scoped to one pairing. Unrelated
// list edits must not replay a write or invalidate its only current result.
export class PushSyncPolicy {
  private intent = '';
  private phase: PushSyncState = 'pending';
  private failures = 0;
  private retryAt = 0;
  private route?: string;

  get status(): PushSyncState {return this.phase;}
  get preferredUrl(): string | undefined {return this.route;}
  get pendingRetryAt(): number | undefined {return this.phase === 'pending' ? this.retryAt : undefined;}

  claim(intent: string, force = false, now = Date.now(), foreground = true): boolean {
    if (intent !== this.intent) {
      this.invalidate();
      this.intent = intent;
    }
    if (!foreground || this.phase === 'syncing' || !force && now < this.retryAt) return false;
    this.phase = 'syncing';
    return true;
  }

  complete(intent: string, ok: boolean, route?: string, now = Date.now()): boolean {
    if (intent !== this.intent) return false;
    this.phase = ok ? 'synced' : 'pending';
    if (ok) {
      this.failures = 0;
      this.retryAt = now + 30 * 60 * 1000;
      this.route = route;
    } else {
      this.failures++;
      this.retryAt = now + Math.min(30000 * 2 ** Math.min(this.failures - 1, 4), 5 * 60 * 1000);
    }
    return true;
  }

  invalidate(): void {
    this.intent = ''; this.phase = 'pending'; this.failures = 0; this.retryAt = 0; this.route = undefined;
  }
}

export function reportedPushAddresses(server: PairedMac): string[] {
  return [...new Set([server.url, ...(server.alts ?? [])])].filter(url => /^https?:\/\//.test(url));
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
  options: {preferredUrl?: string; onSuccess?: (url: string) => void} = {},
): Promise<boolean> {
  if (server.scope === 'guest' || !token) return false;
  // A Mac may have moved between Direct routes. Try only addresses it reported
  // for this pairing, with the same owner credential and a short deadline.
  const reported = reportedPushAddresses(server);
  const addresses = options.preferredUrl && reported.includes(options.preferredUrl)
    ? [options.preferredUrl, ...reported.filter(url => url !== options.preferredUrl)] : reported;
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
    try {
      const client = makeClient(url, server.token);
      const ok = enabled
        ? await client.registerPush(token, kinds, env, controller.signal)
        : await client.unregisterPush(token, undefined, controller.signal);
      if (paused || !foreground()) return false;
      if (ok) {options.onSuccess?.(url); return true;}
    } catch {
      if (paused || !foreground()) return false;
      // Offline or old route: try the next reported address.
    } finally {
      clearTimeout(timer);
      subscription.remove();
    }
  }
  return false;
}
