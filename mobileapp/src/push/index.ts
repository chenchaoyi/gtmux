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

import {AppState, NativeModules, Platform} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import * as Keychain from 'react-native-keychain';
import {Debug} from '../debug';

export type Teardown = () => void;

// Notification action id → the digit typed into the waiting pane. The buttons are the
// digits 1–4 of the options the Mac parsed (one category per option count below), not a
// fixed Yes / Always / No; those three ids remain only for an older Mac. Sent WITHOUT Enter: the
// agent's numbered menu commits on the digit (see ApprovalCard); a trailing Enter
// leaks onto the next prompt on consecutive selections.
// Action id → the digit typed into the pane. The id IS the digit now: nothing about a
// choice's meaning is assumed on this side.
// The notification that launched this process, handled once. iOS hands the same one back
// from getInitialNotification for as long as the app runs, and setupPush runs again on
// every server switch (the bridge remounts with the new Mac): replaying it switched the
// phone back to the Mac that sent it and opened its pane, so tapping another Mac in the
// server list landed on the first Mac's session instead (2026-10-05).
let launchNotificationHandled = false;
// A scene response can also arrive through the notification delegate. Bound the
// short-lived receipt set so one tap or action cannot be dispatched twice.
const responseReceipts = new Map<string, number>();
const pendingReplies = new Set<string>();
const RESPONSE_RECEIPT_TTL_MS = 30_000;
const RESPONSE_RECEIPT_LIMIT = 128;

function firstResponse(id: unknown, action: string | undefined): boolean {
  if (typeof id !== 'string' || !id) return true; // older native bridge
  const now = Date.now();
  for (const [key, at] of responseReceipts) {
    if (now - at >= RESPONSE_RECEIPT_TTL_MS) responseReceipts.delete(key);
  }
  const key = JSON.stringify([id, action ?? '']);
  if (pendingReplies.has(key) || responseReceipts.has(key)) return false;
  if (responseReceipts.size >= RESPONSE_RECEIPT_LIMIT) {
    const oldest = responseReceipts.keys().next().value;
    if (oldest !== undefined) responseReceipts.delete(oldest);
  }
  responseReceipts.set(key, now);
  return true;
}

/** For tests: forget that the launch notification was handled. */
export function resetLaunchNotificationForTesting() {
  launchNotificationHandled = false;
  responseReceipts.clear();
  pendingReplies.clear();
}

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

// The last APNs token iOS handed us, also persisted for cold-start reconciliation.
let lastToken: string | null = null;
const TOKEN_SERVICE = 'com.gtmux.app.apns-token';

export async function loadPushToken(): Promise<string | null> {
  try {
    const saved = await Keychain.getGenericPassword({service: TOKEN_SERVICE});
    lastToken = saved ? saved.password : null;
  } catch {
    lastToken = null;
  }
  return lastToken;
}

// getPushToken returns the cached APNs device token (null before it has arrived).
// removeServer uses it to tell the removed Mac to drop this device, so that Mac
// stops pushing to a phone that has unpaired it. Device-wide → same value across
// servers, and each Mac only drops its own copy.
export function getPushToken(): string | null {
  return lastToken;
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
  // server = the sending Mac's name (carried top-level), so the tap can route to
  // the RIGHT paired server before opening the pane.
  onTapPane: (pane: string, server?: string) => void,
  onToken: (token: string) => void,
  onQuickReply: (pane: string, server: string | undefined, text: string) => void | Promise<void>,
  askPermission = true,
): Promise<Teardown> {
  if (Platform.OS !== 'ios') return () => {};
  if (!PushNotificationIOS || typeof PushNotificationIOS.addEventListener !== 'function') {
    throw new Error('PushNotificationIOS native module unavailable');
  }

  const onRegister = (token: string) => {
    lastToken = token;
    Keychain.setGenericPassword('apns', token, {service: TOKEN_SERVICE}).catch(() => {});
    onToken(token);
  };

  const onNotification = (notification: any, initial = false) => {
    const data = notification.getData?.() ?? {};
    const pane: string | undefined = data.pane;
    const rawAction: string | undefined = notification.getActionIdentifier?.();
    // Older native bridges forwarded UIKit's default identifier verbatim.
    const action = rawAction === 'com.apple.UNNotificationDefaultActionIdentifier' ? undefined : rawAction;
    const isInteraction = initial || !!data.userInteraction;
    if (Debug.logNet) Debug.record({event: 'push-response', at: Date.now(), initial, isInteraction, hasPane: !!pane, action: action ?? '', hasText: typeof data.userText === 'string'});
    const acknowledge = () => {
      if (typeof data.gtmuxResponseId === 'string' && data.gtmuxResponseId) {
        NativeModules?.GtmuxNotificationResponses?.acknowledge?.(data.gtmuxResponseId, action ?? '');
      }
    };
    if (pane && isInteraction && !firstResponse(data.gtmuxResponseId, action)) {
      if (Debug.logNet) Debug.record({event: 'push-duplicate', at: Date.now()});
      notification.finish?.(PushNotificationIOS.FetchResult.NoData);
      return;
    }

    // A quick-reply action button was tapped: answer the waiting pane in the
    // background (no deep-link, no app foreground).
    if (pane && isInteraction && action && Object.prototype.hasOwnProperty.call(QUICK_REPLY, action)) {
      const replyKey = typeof data.gtmuxResponseId === 'string' && data.gtmuxResponseId
        ? JSON.stringify([data.gtmuxResponseId, action]) : null;
      if (replyKey) pendingReplies.add(replyKey);
      const complete = () => {
        if (replyKey) {
          pendingReplies.delete(replyKey);
          responseReceipts.set(replyKey, Date.now());
        }
        acknowledge();
        notification.finish?.(PushNotificationIOS.FetchResult.NoData);
      };
      try {
        const reply = onQuickReply(pane, data.server, QUICK_REPLY[action]);
        if (reply && typeof reply.then === 'function') void reply.then(complete, complete);
        else complete();
      } catch { complete(); }
      return;
    }

    // Plain tap on the body → deep-link to the agent. Skip a foreground delivery
    // (those surface as the in-app SSE banner instead).
    if (pane && isInteraction && !action) {
      onTapPane(pane, data.server);
    }
    acknowledge();
    notification.finish?.(PushNotificationIOS.FetchResult.NoData);
  };

  PushNotificationIOS.addEventListener('register', onRegister);
  PushNotificationIOS.addEventListener('notification', notification => onNotification(notification));
  PushNotificationIOS.addEventListener('localNotification', notification => onNotification(notification));

  // Register the quick-reply actions iOS attaches to a `waiting` notification.
  PushNotificationIOS.setNotificationCategories?.(WAITING_CATEGORIES);

  let closed = false;
  const nativeResponses = NativeModules?.GtmuxNotificationResponses;
  const drainNative = async () => {
    if (!nativeResponses?.drain || closed) return;
    try {
      if (Debug.logNet) Debug.record({event: 'push-drain-start', at: Date.now()});
      const pending: Record<string, unknown>[] = await nativeResponses.drain();
      if (Debug.logNet) Debug.record({event: 'push-drain-resolved', at: Date.now(), count: pending.length});
      if (!closed) for (const data of pending) {
        onNotification({getData: () => data, getActionIdentifier: () => data.actionIdentifier}, true);
      }
    } catch (error) {
      if (Debug.logNet) Debug.record({event: 'push-drain-error', at: Date.now(), name: error instanceof Error ? error.name : typeof error});
    }
  };
  // A bridge response may stall while launched in the background. Retained
  // native receipts are retried when the app becomes active, without blocking
  // the existing initial-notification fallback or processing after teardown.
  const stateSubscription = AppState.addEventListener('change', state => {
    if (state === 'active') void drainNative();
  });
  const teardown = () => {
    closed = true;
    stateSubscription.remove();
    PushNotificationIOS.removeEventListener('register');
    PushNotificationIOS.removeEventListener('notification');
    PushNotificationIOS.removeEventListener('localNotification');
  };

  try {
    // UIKit can deliver a response before React has a listener. Drain native
    // receipts after listeners are attached; the shared identity also suppresses
    // overlap with the bridge event or initial launch notification.
    void drainNative();
    // The launch action must not wait for the permission request to finish.
    if (!launchNotificationHandled) {
      launchNotificationHandled = true;
      const initial = await PushNotificationIOS.getInitialNotification();
      if (initial) {
        onNotification(initial, true);
      }
    }
    // When the master switch is off, obtain a previously authorized token to
    // remove old registrations without showing a first-time permission prompt.
    if (askPermission) {
      const permissions = await PushNotificationIOS.requestPermissions();
      if (Debug.logNet) Debug.record({event: 'push-permissions', at: Date.now(), alert: !!permissions?.alert});
    } else {
      const permissions: any = await new Promise(resolve => PushNotificationIOS.checkPermissions(resolve));
      if (permissions?.authorizationStatus >= 2) await PushNotificationIOS.requestPermissions();
    }

    return teardown;
  } catch (error) {
    teardown();
    throw error;
  }
}
