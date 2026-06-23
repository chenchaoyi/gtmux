// ImageMarkup — a minimal annotate-then-send editor (MOBILE §4). When the
// clipboard holds an image and you tap Paste, this opens full-screen: the image
// fills the canvas and a finger drag draws a red stroke over it (Moshi-style
// markup). Done flattens the image + strokes into a PNG (react-native-view-shot)
// and hands the file back to the Composer, which uploads it to the Mac.

import React, {useRef, useState} from 'react';
import {
  ActivityIndicator,
  Image,
  Modal,
  PanResponder,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import Svg, {Path} from 'react-native-svg';
import {captureRef} from 'react-native-view-shot';
import {Lang} from '../i18n';

const STROKE = '#FF3B30';

export function ImageMarkup({
  visible,
  uri,
  lang,
  onCancel,
  onDone,
}: {
  visible: boolean;
  uri: string | null;
  lang: Lang;
  onCancel: () => void;
  onDone: (fileUri: string) => void;
}) {
  const [paths, setPaths] = useState<string[]>([]);
  const curRef = useRef('');
  const [, setTick] = useState(0);
  const shotRef = useRef<View>(null);
  const [busy, setBusy] = useState(false);

  const responder = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: () => true,
      onPanResponderGrant: e => {
        const {locationX, locationY} = e.nativeEvent;
        curRef.current = `M${locationX.toFixed(1)},${locationY.toFixed(1)}`;
        setTick(t => t + 1);
      },
      onPanResponderMove: e => {
        const {locationX, locationY} = e.nativeEvent;
        curRef.current += ` L${locationX.toFixed(1)},${locationY.toFixed(1)}`;
        setTick(t => t + 1);
      },
      onPanResponderRelease: () => {
        if (curRef.current) {
          const done = curRef.current;
          setPaths(p => [...p, done]);
          curRef.current = '';
          setTick(t => t + 1);
        }
      },
    }),
  ).current;

  const reset = () => {
    setPaths([]);
    curRef.current = '';
    setBusy(false);
  };
  const cancel = () => {
    reset();
    onCancel();
  };
  const undo = () => setPaths(p => p.slice(0, -1));
  const clear = () => {
    setPaths([]);
    curRef.current = '';
    setTick(t => t + 1);
  };
  const done = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const out = await captureRef(shotRef, {format: 'png', quality: 1, result: 'tmpfile'});
      reset();
      onDone(out);
    } catch {
      setBusy(false);
      onCancel();
    }
  };

  const live = curRef.current;

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={cancel}>
      <StatusBar hidden />
      <View style={styles.root}>
        <View style={styles.bar}>
          <TouchableOpacity onPress={cancel} hitSlop={hit}>
            <Text style={styles.barText}>{lang === 'zh' ? '取消' : 'Cancel'}</Text>
          </TouchableOpacity>
          <View style={styles.barMid}>
            <TouchableOpacity onPress={undo} disabled={!paths.length} hitSlop={hit}>
              <Text style={[styles.barText, !paths.length && styles.dim]}>{lang === 'zh' ? '撤销' : 'Undo'}</Text>
            </TouchableOpacity>
            <TouchableOpacity onPress={clear} disabled={!paths.length} hitSlop={hit} style={styles.clearBtn}>
              <Text style={[styles.barText, !paths.length && styles.dim]}>{lang === 'zh' ? '清除' : 'Clear'}</Text>
            </TouchableOpacity>
          </View>
          <TouchableOpacity onPress={done} disabled={busy} hitSlop={hit}>
            {busy ? (
              <ActivityIndicator color={STROKE} />
            ) : (
              <Text style={[styles.barText, styles.done]}>{lang === 'zh' ? '完成' : 'Done'}</Text>
            )}
          </TouchableOpacity>
        </View>

        <View style={styles.canvasWrap}>
          <View ref={shotRef} collapsable={false} style={styles.canvas}>
            {uri && <Image source={{uri}} style={StyleSheet.absoluteFill} resizeMode="contain" />}
            <View style={StyleSheet.absoluteFill} {...responder.panHandlers}>
              <Svg style={StyleSheet.absoluteFill}>
                {paths.map((d, i) => (
                  <Path key={i} d={d} stroke={STROKE} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" fill="none" />
                ))}
                {!!live && (
                  <Path d={live} stroke={STROKE} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" fill="none" />
                )}
              </Svg>
            </View>
          </View>
        </View>

        <Text style={styles.tip}>
          {lang === 'zh' ? '手指滑动以标注 · 完成后发送给 agent' : 'Drag to annotate · Done sends it to the agent'}
        </Text>
      </View>
    </Modal>
  );
}

const hit = {top: 12, bottom: 12, left: 12, right: 12};

const styles = StyleSheet.create({
  root: {flex: 1, backgroundColor: '#000'},
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 18,
    paddingTop: 56,
    paddingBottom: 12,
  },
  barMid: {flexDirection: 'row', alignItems: 'center'},
  clearBtn: {marginLeft: 18},
  barText: {color: '#fff', fontSize: 16, fontWeight: '600'},
  dim: {color: 'rgba(255,255,255,0.3)'},
  done: {color: STROKE, fontWeight: '700'},
  canvasWrap: {flex: 1, margin: 12, borderRadius: 12, overflow: 'hidden', backgroundColor: '#0A0A0C'},
  canvas: {flex: 1},
  tip: {color: 'rgba(255,255,255,0.5)', fontSize: 12.5, textAlign: 'center', paddingBottom: 28, paddingTop: 4},
});
