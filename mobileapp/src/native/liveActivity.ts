// JS wrapper for the iOS Live Activity native module (LiveActivityModule). All
// calls are safe no-ops off iOS or when the module/widget isn't present. The app
// starts one activity and keeps it updated with the live agent tally; it ends it
// when nothing is running.

import {NativeEventEmitter, NativeModules, Platform} from 'react-native';
import {ActivityItem} from '../state/activityItems';

type Mod = {
  areEnabled(): Promise<boolean>;
  getPushToken(): Promise<string>;
  // items is a JSON string {items:[{title,status,time}], more} — passed as a string
  // since the RN bridge handles primitives cleanly; the native side decodes it.
  // server = the paired Mac's name (static per activity → WHICH server this tracks).
  start(waiting: number, working: number, idle: number, title: string, session: string, items: string, server: string): Promise<string>;
  update(waiting: number, working: number, idle: number, title: string, session: string, items: string): void;
  end(): void;
};

const M: Mod | undefined = NativeModules.LiveActivityModule;
const ok = Platform.OS === 'ios' && !!M;

// The APNs environment this BUILD targets — 'sandbox' for a dev-signed build,
// 'production' for App Store / TestFlight. Read from a native constant that mirrors
// the `aps-environment` entitlement (both are the $(APS_ENVIRONMENT) build setting),
// so the Mac routes each push token to the right APNs endpoint. Falls back to __DEV__
// if the native constant is missing (an older build).
export function apnsEnv(): 'sandbox' | 'production' {
  const e = (NativeModules.LiveActivityModule as {apnsEnv?: string} | undefined)?.apnsEnv;
  if (e === 'production') return 'production';
  // The native constant now returns the endpoint contract directly, but tolerate the
  // raw entitlement value too: Apple's aps-environment 'development' → APNs SANDBOX.
  // (Without this, a Release-configuration DEV build — __DEV__ is false — reporting
  // 'development' fell through to 'production' and its sandbox token was routed to the
  // wrong APNs host, so backgrounded pushes / Live Activity updates never landed.)
  if (e === 'sandbox' || e === 'development') return 'sandbox';
  return __DEV__ ? 'sandbox' : 'production';
}

let started = false;

export const LiveActivity = {
  areEnabled(): Promise<boolean> {
    return ok ? M!.areEnabled().catch(() => false) : Promise.resolve(false);
  },

  // sync drives the activity from the current tally: start it on the first
  // non-empty tally, update while anything runs, end when everything's gone.
  // waitingSession is the tmux session that needs you (the bold headline) and
  // waitingTitle is its prompt/task (the detail line).
  sync(
    waiting: number,
    working: number,
    idle: number,
    waitingTitle: string,
    waitingSession: string,
    items: ActivityItem[] = [],
    more = 0,
    server = '',
  ) {
    if (!ok) return;
    const any = waiting + working + idle > 0;
    if (!any) {
      if (started) {
        M!.end();
        started = false;
      }
      return;
    }
    const itemsJson = JSON.stringify({items, more});
    if (started) {
      M!.update(waiting, working, idle, waitingTitle, waitingSession, itemsJson);
    } else {
      started = true;
      M!.start(waiting, working, idle, waitingTitle, waitingSession, itemsJson, server).catch(() => {
        started = false;
      });
    }
  },

  stop() {
    if (ok && started) {
      M!.end();
      started = false;
    }
  },

  // currentPushToken returns the running activity's push token (null off iOS or when
  // no activity is live). removeServer sends it to the removed Mac so that Mac drops
  // the token and stops pushing lock-screen updates for a server you've deleted.
  currentPushToken(): Promise<string | null> {
    return ok ? M!.getPushToken().then(t => t || null).catch(() => null) : Promise.resolve(null);
  },

  // onPushToken subscribes to the Live Activity push token (emitted once iOS
  // issues it for the running activity, and again on rotation). Forward it to the
  // Mac so the relay can push-to-update the lock screen with the app closed.
  // Returns an unsubscribe fn. Also flushes any token already cached natively.
  onPushToken(cb: (token: string) => void): () => void {
    if (!ok) return () => {};
    const emitter = new NativeEventEmitter(NativeModules.LiveActivityModule);
    const sub = emitter.addListener('onActivityPushToken', (e: {token?: string}) => {
      if (e?.token) cb(e.token);
    });
    M!.getPushToken()
      .then(t => {
        if (t) cb(t);
      })
      .catch(() => {});
    return () => sub.remove();
  },
};
