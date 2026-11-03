// Android is a typed FCM adapter. An unconfigured build never submits an APNs token.
import {NativeEventEmitter, NativeModules} from 'react-native';

export type Teardown = () => void;
interface NativePush {
  token(askPermission: boolean): Promise<string | null>;
  initial(): Promise<{pane?: string; server?: string; digit?: string} | null>;
  badge(n: number): void;
}
const native = () => NativeModules.PushNotifications as NativePush | undefined;
let lastToken: string | null = null;
let launchHandled = false;
export function resetLaunchNotificationForTesting() { launchHandled = false; }
export async function loadPushToken(): Promise<string | null> {
  try { lastToken = await native()?.token(false) ?? null; } catch { lastToken = null; }
  return lastToken;
}
export function getPushToken() { return lastToken; }
export function setBadge(n: number) { native()?.badge(Math.max(0, n)); }
export async function setupPush(
  onTap: (pane: string, server?: string) => void,
  onToken: (token: string) => void,
  _onReply: (pane: string, server: string | undefined, text: string) => void,
  askPermission = true,
): Promise<Teardown> {
  const module = native();
  if (!module) return () => {};
  let live = true;
  const open = (event: {pane?: string; server?: string; digit?: string}) => {
    if (!live || !event.pane || !/^%\d+$/.test(event.pane) || !event.server) return;
    // Native actions are one-use, bounded-lived user taps, never received-data execution.
    onTap(event.pane, event.server);
  };
  const emitter = new NativeEventEmitter(NativeModules.PushNotifications);
  const tokenSub = emitter.addListener('onPushToken', (event: {token?: string}) => {
    if (live && event.token) { lastToken = event.token; onToken(event.token); }
  });
  const tapSub = emitter.addListener('onPushTap', open);
  try {
    const token = await module.token(askPermission);
    if (live && token) {lastToken = token; onToken(token);}
    if (live && !launchHandled) { launchHandled = true; const event = await module.initial(); if (event) open(event); }
  } catch { /* Missing services, denied permission or no Firebase config remains unavailable, never a fake registration. */ }
  return () => {live = false; tokenSub.remove(); tapSub.remove();};
}
