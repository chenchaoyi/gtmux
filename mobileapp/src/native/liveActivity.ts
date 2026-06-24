// JS wrapper for the iOS Live Activity native module (LiveActivityModule). All
// calls are safe no-ops off iOS or when the module/widget isn't present. The app
// starts one activity and keeps it updated with the live agent tally; it ends it
// when nothing is running.

import {NativeEventEmitter, NativeModules, Platform} from 'react-native';

type Mod = {
  areEnabled(): Promise<boolean>;
  getPushToken(): Promise<string>;
  start(waiting: number, working: number, idle: number, title: string): Promise<string>;
  update(waiting: number, working: number, idle: number, title: string): void;
  end(): void;
};

const M: Mod | undefined = NativeModules.LiveActivityModule;
const ok = Platform.OS === 'ios' && !!M;

let started = false;

export const LiveActivity = {
  areEnabled(): Promise<boolean> {
    return ok ? M!.areEnabled().catch(() => false) : Promise.resolve(false);
  },

  // sync drives the activity from the current tally: start it on the first
  // non-empty tally, update while anything runs, end when everything's gone.
  // waitingTitle is the name of the agent that needs you (shown as the headline).
  sync(waiting: number, working: number, idle: number, waitingTitle: string) {
    if (!ok) return;
    const any = waiting + working + idle > 0;
    if (!any) {
      if (started) {
        M!.end();
        started = false;
      }
      return;
    }
    if (started) {
      M!.update(waiting, working, idle, waitingTitle);
    } else {
      started = true;
      M!.start(waiting, working, idle, waitingTitle).catch(() => {
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
