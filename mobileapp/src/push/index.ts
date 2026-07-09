// Push registration + tap deep-link (iOS APNs via the native token — no Firebase).
// On setup: request permission, register; the native 'register' event yields the
// hex APNs device token → POST /api/push/register. A tapped notification carries
// `pane` (a top-level custom key from the relay) → deep-link to that agent.
//
// Quick-reply: a `waiting` push arrives with the AGENT_WAITING category, which we
// register with three actions (1 Yes / 2 Always / 3 No). Tapping one sends that
// answer straight into the pane via /api/send — in the BACKGROUND, no app open.

import {Platform} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {GtmuxClient} from '../api/client';
import {apnsEnv} from '../native/liveActivity';

export type Teardown = () => void;

// Notification action id → the digit typed into the waiting pane. Mirrors the
// in-app waiting context keys (1·Yes / 2·Always / 3·No). Sent WITHOUT Enter: the
// agent's numbered menu commits on the digit (see ApprovalCard); a trailing Enter
// leaks onto the next prompt on consecutive selections.
const QUICK_REPLY: Record<string, string> = {yes: '1', always: '2', no: '3'};

const WAITING_CATEGORY = {
  id: 'AGENT_WAITING',
  actions: [
    {id: 'yes', title: '1 · Yes', options: {foreground: false}},
    {id: 'always', title: '2 · Always', options: {foreground: false}},
    {id: 'no', title: '3 · No', options: {foreground: false, destructive: true}},
  ],
};

// The last APNs token iOS handed us, kept so a later kinds-toggle can re-register
// the same device without re-running setup (which would re-add native listeners).
let lastToken: string | null = null;

// reregisterKinds updates the device's per-kind push filter on the server, using
// the cached APNs token. No-op until the token has arrived.
export function reregisterKinds(client: GtmuxClient, kinds: string[]): void {
  if (lastToken) client.registerPush(lastToken, kinds, apnsEnv()).catch(() => {});
}

// setBadge sets the app-icon badge to the live waiting count. The server's silent
// push keeps it right while backgrounded/killed; this keeps it right (and reconciled)
// while the app is running — the two target the same absolute count. Best-effort;
// no-op off iOS or without notification permission.
export function setBadge(n: number): void {
  if (Platform.OS !== 'ios' || !PushNotificationIOS?.setApplicationIconBadgeNumber) return;
  try {
    PushNotificationIOS.setApplicationIconBadgeNumber(Math.max(0, n));
  } catch {
    // best-effort
  }
}

export async function setupPush(
  client: GtmuxClient,
  // server = the sending Mac's name (carried top-level), so the tap can route to
  // the RIGHT paired server before opening the pane.
  onTapPane: (pane: string, server?: string) => void,
  getKinds: () => string[] = () => [],
): Promise<Teardown> {
  if (Platform.OS !== 'ios') return () => {};
  if (!PushNotificationIOS || typeof PushNotificationIOS.addEventListener !== 'function') {
    throw new Error('PushNotificationIOS native module unavailable');
  }

  const onRegister = (token: string) => {
    lastToken = token;
    client.registerPush(token, getKinds(), apnsEnv()).catch(() => {});
  };

  const onNotification = (notification: any) => {
    const data = notification.getData?.() ?? {};
    const pane: string | undefined = data.pane;
    const action: string | undefined = notification.getActionIdentifier?.();

    // A quick-reply action button was tapped: answer the waiting pane in the
    // background (no deep-link, no app foreground).
    if (pane && action && QUICK_REPLY[action] !== undefined) {
      client.send(pane, {text: QUICK_REPLY[action]}).catch(() => {});
      notification.finish?.(PushNotificationIOS.FetchResult.NoData);
      return;
    }

    // Plain tap on the body → deep-link to the agent. Skip a foreground delivery
    // (those surface as the in-app SSE banner instead).
    if (pane && data.userInteraction) {
      onTapPane(pane, data.server);
    }
    notification.finish?.(PushNotificationIOS.FetchResult.NoData);
  };

  PushNotificationIOS.addEventListener('register', onRegister);
  PushNotificationIOS.addEventListener('notification', onNotification);
  PushNotificationIOS.addEventListener('localNotification', onNotification);

  // Register the quick-reply actions iOS attaches to a `waiting` notification.
  PushNotificationIOS.setNotificationCategories?.([WAITING_CATEGORY]);

  // Triggers the permission prompt + remote-notification registration.
  await PushNotificationIOS.requestPermissions();

  // Cold start: app launched by tapping a notification while it was killed.
  const initial = await PushNotificationIOS.getInitialNotification();
  if (initial) {
    const data: any = initial.getData?.() ?? {};
    if (data.pane) onTapPane(data.pane, data.server);
  }

  return () => {
    PushNotificationIOS.removeEventListener('register');
    PushNotificationIOS.removeEventListener('notification');
    PushNotificationIOS.removeEventListener('localNotification');
  };
}
