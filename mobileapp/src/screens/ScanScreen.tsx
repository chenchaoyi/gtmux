// ScanScreen — the camera QR scanner for pairing (MOBILE §6). Reads a gtmux
// pairing QR ({v,url,token,name}) and hands the raw string back to the pairing
// flow, which validates + connects. Rendered as a modal over PairingScreen.

import React, {useRef} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Camera, CameraType} from 'react-native-camera-kit';
import {useApp} from '../state/AppContext';

export function ScanScreen({
  onClose,
  onScanned,
}: {
  onClose: () => void;
  onScanned: (raw: string) => void;
}) {
  const {lang} = useApp();
  const done = useRef(false);

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <Camera
        style={StyleSheet.absoluteFill}
        cameraType={CameraType.Back}
        scanBarcode
        onReadCode={(e: any) => {
          if (done.current) return;
          const v = e?.nativeEvent?.codeStringValue;
          if (v) {
            done.current = true;
            onScanned(String(v));
          }
        }}
      />
      <View style={styles.overlay} pointerEvents="box-none">
        <View style={styles.frame} />
        <Text style={styles.hint}>
          {lang === 'zh' ? '对准 Mac 上的配对二维码' : 'Point at the pairing QR on your Mac'}
        </Text>
      </View>
      <TouchableOpacity style={styles.close} onPress={onClose} hitSlop={hit}>
        <Text style={styles.closeText}>{lang === 'zh' ? '✕ 取消' : '✕ Cancel'}</Text>
      </TouchableOpacity>
    </SafeAreaView>
  );
}

const hit = {top: 12, bottom: 12, left: 12, right: 12};

const styles = StyleSheet.create({
  safe: {flex: 1, backgroundColor: '#000'},
  overlay: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, alignItems: 'center', justifyContent: 'center'},
  frame: {
    width: 240,
    height: 240,
    borderWidth: 3,
    borderColor: '#06B6D4',
    borderRadius: 18,
    backgroundColor: 'transparent',
  },
  hint: {color: '#fff', fontSize: 14, fontWeight: '600', marginTop: 24, opacity: 0.9},
  close: {position: 'absolute', top: 56, right: 18, paddingHorizontal: 12, paddingVertical: 8},
  closeText: {color: '#fff', fontSize: 16, fontWeight: '700'},
});
