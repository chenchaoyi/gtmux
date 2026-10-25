// ScanScreen — the camera QR scanner for pairing (MOBILE §6). A dimmed overlay
// with a clear scan window (corner brackets + a moving scan line) over the live
// camera; reads a gtmux pairing QR and hands the raw string to the pairing flow.

import React, {useEffect, useRef} from 'react';
import {Animated, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {SafeAreaView, useSafeAreaInsets} from 'react-native-safe-area-context';
import {Camera, CameraType} from 'react-native-camera-kit';
import {useApp} from '../state/AppContext';

const CYAN = '#06B6D4';

export function ScanScreen({
  onClose,
  onScanned,
}: {
  onClose: () => void;
  onScanned: (raw: string) => void;
}) {
  const {lang} = useApp();
  const {width, height} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const compact = height < 500;
  // Leave the title, cancel action and safe areas visible on a landscape phone.
  const win = Math.max(100, Math.min(256, width - insets.left - insets.right - 48,
    height - insets.top - insets.bottom - 136));
  const done = useRef(false);
  const scan = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const anim = Animated.loop(
      Animated.sequence([
        Animated.timing(scan, {toValue: 1, duration: 1700, useNativeDriver: true}),
        Animated.timing(scan, {toValue: 0, duration: 1700, useNativeDriver: true}),
      ]),
    );
    anim.start();
    return () => { done.current = true; anim.stop(); };
  }, [scan]);

  const translateY = scan.interpolate({inputRange: [0, 1], outputRange: [10, win - 10]});

  return (
    <View style={styles.root}>
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

      {/* dim top + title */}
      <View style={styles.dimTop}>
        <SafeAreaView edges={['top']}>
          <Text style={styles.title}>{lang === 'zh' ? '扫描配对码' : 'Scan pairing code'}</Text>
          {!compact && <Text style={styles.subtitle}>
            {lang === 'zh'
              ? '对准服务器上「配对设备」或 gtmux pair 的二维码'
              : 'Point at the QR from “Pair a device” or gtmux pair'}
          </Text>}
        </SafeAreaView>
      </View>

      {/* middle: dim sides + clear window */}
      <View style={[styles.middle, {height: win}]}>
        <View style={styles.dimSide} />
        <View style={[styles.window, {width: win, height: win}]}>
          <View style={[styles.corner, styles.tl]} />
          <View style={[styles.corner, styles.tr]} />
          <View style={[styles.corner, styles.bl]} />
          <View style={[styles.corner, styles.br]} />
          <Animated.View style={[styles.scanLine, {transform: [{translateY}]}]} />
        </View>
        <View style={styles.dimSide} />
      </View>

      {/* dim bottom + cancel */}
      <View style={[styles.dimBottom, {paddingTop: compact ? 12 : 28, paddingBottom: insets.bottom}]}>
        <TouchableOpacity style={styles.cancel} onPress={() => { done.current = true; onClose(); }}>
          <Text style={styles.cancelText}>{lang === 'zh' ? '取消' : 'Cancel'}</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}

const DIM = 'rgba(0,0,0,0.6)';

const styles = StyleSheet.create({
  root: {flex: 1, backgroundColor: '#000'},
  dimTop: {flex: 1, backgroundColor: DIM, paddingHorizontal: 28},
  title: {color: '#fff', fontSize: 20, fontWeight: '700', marginTop: 8, textAlign: 'center'},
  subtitle: {color: 'rgba(255,255,255,0.75)', fontSize: 13, marginTop: 8, textAlign: 'center', lineHeight: 18},
  middle: {flexDirection: 'row'},
  dimSide: {flex: 1, backgroundColor: DIM},
  window: {},
  corner: {position: 'absolute', width: 30, height: 30, borderColor: CYAN},
  tl: {top: -2, left: -2, borderTopWidth: 3, borderLeftWidth: 3, borderTopLeftRadius: 10},
  tr: {top: -2, right: -2, borderTopWidth: 3, borderRightWidth: 3, borderTopRightRadius: 10},
  bl: {bottom: -2, left: -2, borderBottomWidth: 3, borderLeftWidth: 3, borderBottomLeftRadius: 10},
  br: {bottom: -2, right: -2, borderBottomWidth: 3, borderRightWidth: 3, borderBottomRightRadius: 10},
  scanLine: {
    position: 'absolute',
    left: 10,
    right: 10,
    height: 2,
    backgroundColor: CYAN,
    borderRadius: 2,
    shadowColor: CYAN,
    shadowOpacity: 0.9,
    shadowRadius: 6,
    shadowOffset: {width: 0, height: 0},
  },
  dimBottom: {flex: 1, backgroundColor: DIM, alignItems: 'center', justifyContent: 'flex-start', paddingTop: 28},
  cancel: {paddingHorizontal: 24, paddingVertical: 11, borderRadius: 22, borderWidth: StyleSheet.hairlineWidth, borderColor: 'rgba(255,255,255,0.4)'},
  cancelText: {color: '#fff', fontSize: 15, fontWeight: '600'},
});
