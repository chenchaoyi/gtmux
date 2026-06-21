// Keychain-backed storage of the paired Mac {url, token, name}. The token is a
// secret, so it lives in the Keychain (react-native-keychain), never AsyncStorage.

import * as Keychain from 'react-native-keychain';
import {PairedMac} from './qr';

const SERVICE = 'com.gtmux.app.paired-mac';

export async function savePairedMac(mac: PairedMac): Promise<void> {
  // username = url+name (non-secret), password = token (secret).
  await Keychain.setGenericPassword(
    JSON.stringify({url: mac.url, name: mac.name}),
    mac.token,
    {service: SERVICE},
  );
}

export async function loadPairedMac(): Promise<PairedMac | null> {
  try {
    const creds = await Keychain.getGenericPassword({service: SERVICE});
    if (!creds) return null;
    const meta = JSON.parse(creds.username);
    return {url: meta.url, name: meta.name, token: creds.password};
  } catch {
    return null;
  }
}

export async function clearPairedMac(): Promise<void> {
  try {
    await Keychain.resetGenericPassword({service: SERVICE});
  } catch {
    // ignore
  }
}
