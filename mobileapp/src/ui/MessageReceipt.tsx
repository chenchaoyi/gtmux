import React, {useEffect, useState} from 'react';
import {ActivityIndicator, StyleSheet, View} from 'react-native';
import {ChromeText} from './ChromeText';
import {CheckIcon} from './Icons';
import {sendReceiptLabel, type SendDelivery} from './sendReceipt';
import {useReadingPalette} from './ReadingPalette';

/** Visual ingress feedback only. Expiry never confirms, drops or resends a message. */
export function MessageReceipt({delivery, delivered, expiresAt, zh}: {delivery?: SendDelivery; delivered: boolean; expiresAt?: number; zh: boolean}) {
  const pal = useReadingPalette();
  const [expired, setExpired] = useState<number | null>(null);
  const receipt = delivered && delivery !== 'queued';
  useEffect(() => {
    if (!receipt || expiresAt === undefined || !Number.isFinite(expiresAt)) return;
    const remaining = expiresAt - Date.now();
    if (remaining <= 0) return;
    const timer = setTimeout(() => setExpired(expiresAt), remaining);
    return () => clearTimeout(timer);
  }, [receipt, expiresAt]);
  // Unknown lifetime stays quiet; mounting a view must never create a new deadline.
  if (receipt && (expiresAt === undefined || !Number.isFinite(expiresAt) || Date.now() >= expiresAt || expired === expiresAt)) return null;
  return <View testID="chat-pending-echo" pointerEvents="none" accessible
    accessibilityLabel={sendReceiptLabel(delivery, delivered, zh)} accessibilityLiveRegion="polite" style={styles.slot}>
    {!delivered ? <ActivityIndicator size={10} color={pal.fg2} />
      : delivery === 'queued' ? <ChromeText numberOfLines={1} style={[styles.queued, {color: pal.fg2}]}>{zh ? '排队中' : 'Queued'}</ChromeText>
      : <CheckIcon size={11} color={pal.fg2} />}
  </View>;
}
const styles = StyleSheet.create({
  slot: {position: 'absolute', bottom: 0, left: 0, right: 0, height: 14, alignItems: 'center', justifyContent: 'center'},
  queued: {fontSize: 10, lineHeight: 12},
});
