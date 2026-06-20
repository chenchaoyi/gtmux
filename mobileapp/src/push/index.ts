// Push registration + tap deep-link (iOS APNs via the native token — no Firebase).
// On setup: request permission, register; the native 'register' event yields the
// hex APNs device token → POST /api/push/register. A tapped notification carries
// `pane` (a top-level custom key from the relay) → deep-link to that agent.

import {Platform} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {GtmuxClient} from '../api/client';

export type Teardown = () => void;

export async function setupPush(
  client: GtmuxClient,
  onTapPane: (pane: string) => void,
): Promise<Teardown> {
  if (Platform.OS !== 'ios') return () => {};

  const onRegister = (token: string) => {
    client.registerPush(token).catch(() => {});
  };

  const onNotification = (notification: any) => {
    const data = notification.getData?.() ?? {};
    const pane: string | undefined = data.pane;
    // Only deep-link when the user tapped the notification (not a foreground
    // delivery — those are surfaced as the in-app SSE banner instead).
    if (pane && data.userInteraction) {
      onTapPane(pane);
    }
    notification.finish?.(PushNotificationIOS.FetchResult.NoData);
  };

  PushNotificationIOS.addEventListener('register', onRegister);
  PushNotificationIOS.addEventListener('notification', onNotification);
  PushNotificationIOS.addEventListener('localNotification', onNotification);

  // Triggers the permission prompt + remote-notification registration.
  await PushNotificationIOS.requestPermissions();

  // Cold start: app launched by tapping a notification while it was killed.
  const initial = await PushNotificationIOS.getInitialNotification();
  if (initial) {
    const data: any = initial.getData?.() ?? {};
    if (data.pane) onTapPane(data.pane);
  }

  return () => {
    PushNotificationIOS.removeEventListener('register');
    PushNotificationIOS.removeEventListener('notification');
    PushNotificationIOS.removeEventListener('localNotification');
  };
}
