import * as Keychain from 'react-native-keychain';
import type {PairedMac} from '../pairing/qr';
import {RestoreError} from './restore';
export type PendingRestore = {id: string; revision: string};
// Bind the receipt to the pairing, not just an address that may be re-paired.
// Credentials remain in Keychain, exactly as the paired-server roster does.
export const restorePendingService = (mac: PairedMac) => `com.gtmux.app.restore.v1:${encodeURIComponent(mac.url)}`;
export async function loadPendingRestore(mac: PairedMac): Promise<PendingRestore | null> {
  const saved = await Keychain.getGenericPassword({service: restorePendingService(mac)});
  if (!saved) return null;
  let v: any;
  try {v = JSON.parse(saved.password);} catch {throw new RestoreError('local_record');}
  if (v.token !== mac.token) return null;
  if (!/^\d{10}-[A-Za-z0-9-]{16,64}$/.test(v.id) || !/^[a-f0-9]{64}$/.test(v.revision)) throw new RestoreError('local_record');
  return {id: v.id, revision: v.revision};
}
export async function savePendingRestore(mac: PairedMac, p: PendingRestore): Promise<void> {
  const saved = await Keychain.setGenericPassword('restore', JSON.stringify({...p, token: mac.token}), {service: restorePendingService(mac)});
  if (saved === false) throw new RestoreError('local_record');
}
export async function clearPendingRestore(mac: PairedMac): Promise<void> {
  const cleared = await Keychain.resetGenericPassword({service: restorePendingService(mac)});
  if (cleared === false) throw new RestoreError('local_record');
}
