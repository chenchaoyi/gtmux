// Push registration + tap deep-link (iOS APNs via the native token — no Firebase).
// On setup: request permission, register; the native 'register' event yields the
// hex APNs device token → POST /api/push/register. A tapped notification carries
// `pane` (a top-level custom key from the relay) → deep-link to that agent.
//
// Quick-reply: a `waiting` push arrives with a category whose actions send a digit
// straight into the pane via /api/send — in the BACKGROUND, no app open.
//
// The buttons say ONLY THE NUMBER, and there is one category per option count.
//
// They used to say "1 · Yes / 2 · Always / 3 · No", which was not a simplification but a
// LIE an iOS category cannot avoid telling: its actions are frozen at registration, while
// the choices differ per prompt. On Claude's common two-option menu — "1. Yes / 2. No,
// and tell Claude what to do" — the button labelled ALWAYS sent the answer that means NO,
// and a third button offered a digit that was not a choice at all. A label that claims a
// meaning it cannot know turns a convenience into a misfire, on a surface whose whole
// point is answering without looking closely.
//
// So: the label says what the button DOES (sends this number), the notification body says
// what it MEANS (one option per line — see optionsBody in serve.go), and the Mac picks
// the category from the count so no phantom button is ever offered.

import {Platform} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {GtmuxClient} from '../api/client';
import {apnsEnv} from '../native/liveActivity';

export type Teardown = () => void;

// Notification action id → the digit typed into the waiting pane. Mirrors the
// in-app waiting context keys (1·Yes / 2·Always / 3·No). Sent WITHOUT Enter: the
// agent's numbered menu commits on the digit (see ApprovalCard); a trailing Enter
// leaks onto the next prompt on consecutive selections.
// Action id → the digit typed into the pane. The id IS the digit now: nothing about a
// choice's meaning is assumed on this side.
const QUICK_REPLY: Record<string, string> = {
  '1': '1', '2': '2', '3': '3', '4': '4',
  // Legacy ids, kept so a notification delivered by an older Mac still answers instead
  // of doing nothing when tapped.
  yes: '1', always: '2', no: '3',
};

const numAction = (n: number) => ({id: String(n), title: String(n), options: {foreground: false}});

// One category per option count (2..4), plus the legacy id an older Mac still sends.
// iOS needs every category registered up front; the PUSH chooses which one applies.
const WAITING_CATEGORIES = [
  {id: 'AGENT_WAITING_2', actions: [numAction(1), numAction(2)]},
  {id: 'AGENT_WAITING_3', actions: [numAction(1), numAction(2), numAction(3)]},
  {id: 'AGENT_WAITING_4', actions: [numAction(1), numAction(2), numAction(3), numAction(4)]},
  // An older Mac sends AGENT_WAITING for every waiting push, whatever the pane offers.
  // Three neutral numbers is the safest thing that category can mean.
  {id: 'AGENT_WAITING', actions: [numAction(1), numAction(2), numAction(3)]},
];

// The last APNs token iOS handed us, kept so a later kinds-toggle can re-register
// the same device without re-running setup (which would re-add native listeners).
let lastToken: string | null = null;

// getPushToken returns the cached APNs device token (null before it has arrived).
// removeServer uses it to tell the removed Mac to drop this device, so that Mac
// stops pushing to a phone that has unpaired it. Device-wide → same value across
// servers, and each Mac only drops its own copy.
export function getPushToken(): string | null {
  return lastToken;
}

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
  PushNotificationIOS.setNotificationCategories?.(WAITING_CATEGORIES);

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
