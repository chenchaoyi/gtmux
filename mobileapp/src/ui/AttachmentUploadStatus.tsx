import React, {useEffect, useRef, useState} from 'react';
import {AccessibilityInfo, Animated, Easing, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Svg, {Circle, Path} from 'react-native-svg';
import type {UploadState} from '../api/client';
import type {Lang} from '../i18n';
import {LoadingMark} from './LoadingMark';
import {BRAND_CYAN} from './BrandMark';

const ProgressCircle = React.forwardRef<Circle, React.ComponentProps<typeof Circle>>((props, ref) => <Circle {...props} ref={ref} />);
ProgressCircle.displayName = 'ProgressCircle';
const AnimatedCircle = Animated.createAnimatedComponent(ProgressCircle);
const circumference = 2 * Math.PI * 21;
function MeasuredRing({fraction}: {fraction: number}) {
  const value = useRef(new Animated.Value(0)).current;
  const [reduce, setReduce] = useState<boolean | null>(null);
  useEffect(() => {
    let alive = true, changed = false;
    const sub = AccessibilityInfo.addEventListener('reduceMotionChanged', next => {changed = true; if (alive) setReduce(next);});
    AccessibilityInfo.isReduceMotionEnabled().then(next => {if (alive && !changed) setReduce(next);}).catch(() => {});
    return () => {alive = false; sub?.remove?.();};
  }, []);
  useEffect(() => {
    value.stopAnimation();
    if (reduce !== false) {value.setValue(fraction); return;}
    const motion = Animated.timing(value, {toValue: fraction, duration: 180, easing: Easing.out(Easing.quad), useNativeDriver: false});
    motion.start(); return () => motion.stop();
  }, [fraction, reduce, value]);
  return <View style={styles.ring} accessible={false}>
    <Svg width={48} height={48} viewBox="0 0 48 48">
      <Circle cx={24} cy={24} r={21} fill="none" stroke="#fff" strokeOpacity={0.25} strokeWidth={2.5} />
      <AnimatedCircle cx={24} cy={24} r={21} fill="none" stroke={BRAND_CYAN} strokeWidth={2.5} strokeLinecap="round"
        strokeDasharray={`${circumference} ${circumference}`} strokeDashoffset={value.interpolate({inputRange: [0, 1], outputRange: [circumference, 0]})} rotation={-90} origin="24,24" />
    </Svg>
    <Text style={styles.percent}>{Math.floor(fraction * 100)}%</Text>
  </View>;
}

// One thumbnail owns its feedback. No standalone bar or invented progress.
export function AttachmentUploadStatus({attachment, state, failed = false, lang, onRetry}: {
  attachment: {id: string; name: string}; state?: UploadState; failed?: boolean; lang: Lang; onRetry?: () => void;
}) {
  const zh = lang === 'zh';
  const fraction = state?.fraction != null && Number.isFinite(state.fraction) ? Math.max(0, Math.min(1, state.fraction)) : undefined;
  const phase = state?.phase === 'uploading' && fraction === 1 ? 'processing' : state?.phase ?? 'queued';
  const label = failed ? (zh ? '上传失败，重试发送' : 'Upload failed. Retry sending message')
    : phase === 'queued' ? (zh ? '待上传' : 'Queued')
    : phase === 'preparing' ? (zh ? '准备中' : 'Preparing')
    : phase === 'uploading' ? (zh ? '上传中' : 'Uploading')
    : phase === 'processing' ? (zh ? '保存中' : 'Saving') : (zh ? '已上传' : 'Uploaded');
  const testID = `attachment-upload-${attachment.id}`;
  if (failed) return <TouchableOpacity testID={testID} style={styles.overlay} accessibilityRole="button"
    accessibilityLabel={`${attachment.name}, ${label}`} accessibilityState={{disabled: !onRetry}} disabled={!onRetry} onPress={onRetry}>
    <Svg width={22} height={22} viewBox="0 0 24 24" fill="none"><Path d="M12 3 2 21h20L12 3Zm0 6v5m0 3h.01" stroke="#fff" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" /></Svg>
    <Text style={styles.phase}>{zh ? '重试发送' : 'Retry'}</Text>
  </TouchableOpacity>;
  const value = phase === 'uploading' && fraction != null ? {min: 0, max: 100, now: Math.floor(fraction * 100), text: label}
    : phase === 'complete' ? {min: 0, max: 100, now: 100, text: label} : {text: label};
  return <View testID={testID} style={styles.overlay} pointerEvents="none" accessible accessibilityRole="progressbar"
    accessibilityLabel={`${attachment.name}, ${label}`} accessibilityValue={value}>
    {phase === 'complete' ? <Svg width={24} height={24} viewBox="0 0 24 24" fill="none"><Path d="m5 12 4 4 10-10" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" /></Svg>
      : phase === 'queued' ? <Svg width={22} height={22} viewBox="0 0 24 24" fill="none"><Circle cx={12} cy={12} r={9} stroke="#fff" strokeWidth={1.5} /><Path d="M12 7v5l3 2" stroke="#fff" strokeWidth={1.5} strokeLinecap="round" /></Svg>
      : phase === 'uploading' && fraction != null ? <MeasuredRing fraction={fraction} />
      : <><LoadingMark size={20} color="#fff" accessible={false} /><Text style={styles.phase}>{label}</Text></>}
  </View>;
}
const styles = StyleSheet.create({
  overlay: {position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, backgroundColor: 'rgba(0,0,0,0.56)', alignItems: 'center', justifyContent: 'center', padding: 2},
  ring: {width: 48, height: 48, alignItems: 'center', justifyContent: 'center'},
  percent: {position: 'absolute', color: '#fff', fontSize: 12, lineHeight: 16, fontWeight: '600', fontVariant: ['tabular-nums']},
  phase: {color: '#fff', fontSize: 10, lineHeight: 12, textAlign: 'center', marginTop: 3},
});
