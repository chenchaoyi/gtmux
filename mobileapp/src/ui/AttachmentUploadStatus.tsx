import React, {useEffect, useRef, useState} from 'react';
import {AccessibilityInfo, Animated, Easing, StyleSheet, Text, View} from 'react-native';
import type {UploadState} from '../api/client';
import type {Lang} from '../i18n';
import type {Palette} from './theme';

function MeasuredBar({fraction, color, track}: {fraction?: number; color: string; track: string}) {
  const value = useRef(new Animated.Value(0)).current;
  const [reduce, setReduce] = useState<boolean | null>(null);
  useEffect(() => {
    let alive = true;
    let changed = false;
    const sub = AccessibilityInfo.addEventListener('reduceMotionChanged', next => {
      changed = true;
      if (alive) setReduce(next);
    });
    AccessibilityInfo.isReduceMotionEnabled().then(next => {
      if (alive && !changed) setReduce(next);
    }).catch(() => {});
    return () => {alive = false; sub?.remove?.();};
  }, []);
  useEffect(() => {
    const target = fraction ?? 0;
    value.stopAnimation();
    if (reduce !== false) {value.setValue(target); return;}
    const motion = Animated.timing(value, {toValue: target, duration: 180, easing: Easing.out(Easing.quad), useNativeDriver: false});
    motion.start();
    return () => motion.stop();
  }, [fraction, reduce, value]);
  return <View style={[styles.track, {backgroundColor: track}]} accessible={false}>
    <Animated.View style={{height: 3, backgroundColor: color, width: value.interpolate({inputRange: [0, 1], outputRange: ['0%', '100%']})}} />
  </View>;
}

export function AttachmentUploadStatus({attachments, states, pal, lang}: {
  attachments: {id: string; name: string}[];
  states: Record<string, UploadState>;
  pal: Palette;
  lang: Lang;
}) {
  if (!attachments.length) return null;
  const active = attachments.findIndex(att => states[att.id]?.phase !== 'complete');
  const index = active < 0 ? attachments.length - 1 : active;
  const attachment = attachments[index];
  const state = states[attachment.id] ?? {phase: 'preparing'};
  const fraction = state.fraction != null && Number.isFinite(state.fraction) ? Math.max(0, Math.min(1, state.fraction)) : undefined;
  const zh = lang === 'zh';
  const phase = state.phase === 'preparing' ? (zh ? '准备中' : 'Preparing')
    : state.phase === 'uploading' ? (zh ? '上传中' : 'Uploading')
    : state.phase === 'processing' ? (zh ? '保存中' : 'Saving') : (zh ? '已上传' : 'Uploaded');
  const percentage = state.phase === 'uploading' && fraction != null ? ` · ${Math.floor(fraction * 100)}%` : '';
  const count = `${index + 1}/${attachments.length}`;
  return <View testID="attachment-upload-status" style={styles.wrap} accessible accessibilityRole="progressbar"
    accessibilityLabel={`${attachment.name}, ${count}, ${phase}`}
    accessibilityValue={fraction == null ? {text: phase} : {min: 0, max: 100, now: Math.floor(fraction * 100), text: phase}}>
    <View style={styles.line}>
      <Text numberOfLines={1} style={[styles.name, {color: pal.fg2}]}>{attachment.name}</Text>
      <Text style={[styles.count, {color: pal.fg2}]}>{count}</Text>
    </View>
    <Text style={[styles.phase, {color: pal.fg}]}>{phase}{percentage}</Text>
    <MeasuredBar key={attachment.id} fraction={fraction} color={pal.fg2} track={pal.divider} />
  </View>;
}

const styles = StyleSheet.create({
  wrap: {paddingHorizontal: 8, paddingBottom: 8, gap: 4},
  line: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  name: {flex: 1, fontSize: 13, lineHeight: 18},
  count: {fontSize: 13, lineHeight: 18, fontVariant: ['tabular-nums']},
  phase: {fontSize: 13, lineHeight: 18},
  track: {height: 3, overflow: 'hidden', borderRadius: 2},
});
