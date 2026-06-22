// ScanScreen — camera QR scanner for pairing (real device only; the simulator has
// no camera). Parses the menu-bar app's pairing QR, validates reachability+token,
// and pairs. Rendered as a full-screen overlay by PairingScreen (which lives
// outside the navigator), so success → useApp().pair() flips the app to Radar.
//
// Uses react-native-camera-kit (purpose-built QR/barcode; new-arch TurboModule).
// VisionCamera was dropped — its v5 removed the built-in code scanner.

import React, {useCallback, useRef, useState} from 'react';
import {ActivityIndicator, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Camera, CameraType} from 'react-native-camera-kit';
import {GtmuxClient} from '../api/client';
import {parsePairingQR} from '../pairing/qr';
import {useApp} from '../state/AppContext';

export function ScanScreen({onCancel}: {onCancel: () => void}) {
  const {t, pair} = useApp();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const handled = useRef(false);

  const onScanned = useCallback(
    async (value: string) => {
      if (handled.current || !value) return;
      handled.current = true;
      setBusy(true);
      setError('');
      try {
        const mac = parsePairingQR(value);
        const client = new GtmuxClient(mac.url, mac.token);
        if (!(await client.health())) throw new Error(t('cantReach'));
        await client.agents(); // validates the token
        await pair(mac); // flips the app to Radar
      } catch (e: any) {
        setError(e?.message ? String(e.message) : t('cantReach'));
        setTimeout(() => {
          handled.current = false;
          setBusy(false);
        }, 1500);
      }
    },
    [pair, t],
  );

  return (
    <View style={styles.fill}>
      <Camera
        style={StyleSheet.absoluteFill}
        cameraType={CameraType.Back}
        scanBarcode={!busy}
        onReadCode={(e: any) => onScanned(e?.nativeEvent?.codeStringValue)}
        showFrame={false}
      />
      <SafeAreaView style={styles.overlay} pointerEvents="box-none">
        <View style={styles.topBar} pointerEvents="box-none">
          <TouchableOpacity onPress={onCancel} hitSlop={hit} style={styles.cancel}>
            <Text style={styles.cancelText}>✕</Text>
          </TouchableOpacity>
          <Text style={styles.hint}>{t('scanHint')}</Text>
          <View style={styles.cancel} />
        </View>
        <View style={styles.reticleWrap} pointerEvents="none">
          <View style={styles.reticle} />
          {busy && <ActivityIndicator color="#fff" style={styles.spin} />}
          {!!error && <Text style={styles.error}>{error}</Text>}
        </View>
        <TouchableOpacity style={styles.manual} onPress={onCancel}>
          <Text style={styles.manualText}>{t('manualEntry')}</Text>
        </TouchableOpacity>
      </SafeAreaView>
    </View>
  );
}

const hit = {top: 12, bottom: 12, left: 12, right: 12};

const styles = StyleSheet.create({
  fill: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: '#000'},
  overlay: {flex: 1, justifyContent: 'space-between'},
  topBar: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 16},
  cancel: {width: 40, height: 40, alignItems: 'center', justifyContent: 'center'},
  cancelText: {color: '#fff', fontSize: 22, fontWeight: '300'},
  hint: {color: '#fff', fontSize: 14, fontWeight: '600', flex: 1, textAlign: 'center'},
  reticleWrap: {alignItems: 'center', justifyContent: 'center'},
  reticle: {width: 240, height: 240, borderWidth: 3, borderColor: 'rgba(255,255,255,0.9)', borderRadius: 24},
  spin: {position: 'absolute'},
  error: {
    color: '#fff',
    backgroundColor: '#EF4444',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 8,
    marginTop: 20,
    overflow: 'hidden',
  },
  manual: {alignItems: 'center', padding: 24},
  manualText: {color: '#fff', fontSize: 15, fontWeight: '600', textDecorationLine: 'underline'},
});
