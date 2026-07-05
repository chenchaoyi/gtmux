// HistoryModal — pick a previously-sent message (MOBILE §10 toolbar "↑ 历史").
// A light bottom sheet listing recent inputs (newest first); tap to load it back
// into the composer, or SWIPE a row left to reveal a Delete button (iOS-style).
// The parent owns the list + persistence. Swipe is done with RN's own
// PanResponder/Animated — no react-native-gesture-handler (bare RN, no new deps).

import React, {useRef} from 'react';
import {
  Alert,
  Animated,
  Modal,
  PanResponder,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';

const hit = {top: 10, bottom: 10, left: 10, right: 10};
const DELETE_W = 78; // revealed delete-button width

// Clearing ALL history is destructive → confirm first.
function confirmClear(lang: Lang, onClear: () => void) {
  Alert.alert(
    lang === 'zh' ? '清空全部历史？' : 'Clear all history?',
    lang === 'zh' ? '此操作不可撤销。' : 'This cannot be undone.',
    [
      {text: lang === 'zh' ? '取消' : 'Cancel', style: 'cancel'},
      {text: lang === 'zh' ? '清空' : 'Clear', style: 'destructive', onPress: onClear},
    ],
  );
}

// SwipeRow: horizontal drag reveals a Delete action; a tap still passes through to
// the row (onMoveShouldSet only claims clearly-horizontal gestures, so vertical
// ScrollView scrolls and taps are untouched).
function SwipeRow({
  pal,
  lang,
  onDelete,
  children,
}: {
  pal: Palette;
  lang: Lang;
  onDelete: () => void;
  children: React.ReactNode;
}) {
  const tx = useRef(new Animated.Value(0)).current;
  const open = useRef(0); // last settled offset (0 or -DELETE_W)
  const spring = (to: number) => {
    open.current = to;
    Animated.spring(tx, {toValue: to, useNativeDriver: true, bounciness: 0, speed: 20}).start();
  };
  const settle = (dx: number) => spring(open.current + dx < -DELETE_W / 2 ? -DELETE_W : 0);
  const pan = useRef(
    PanResponder.create({
      // Claim as soon as the drag is clearly horizontal…
      onMoveShouldSetPanResponder: (_, g) =>
        Math.abs(g.dx) > 6 && Math.abs(g.dx) > Math.abs(g.dy),
      onMoveShouldSetPanResponderCapture: (_, g) =>
        Math.abs(g.dx) > 6 && Math.abs(g.dx) > Math.abs(g.dy),
      // …and DON'T let the enclosing ScrollView steal it mid-swipe (that was the
      // "slides half-out then snaps back" jitter).
      onPanResponderTerminationRequest: () => false,
      onPanResponderMove: (_, g) => {
        const next = Math.min(0, Math.max(-DELETE_W, open.current + g.dx));
        tx.setValue(next);
      },
      onPanResponderRelease: (_, g) => settle(g.dx),
      onPanResponderTerminate: (_, g) => settle(g.dx),
    }),
  ).current;
  return (
    <View>
      <View style={styles.deleteBg}>
        <TouchableOpacity onPress={onDelete} hitSlop={hit} style={styles.deleteBtn}>
          <Text style={styles.deleteText}>{lang === 'zh' ? '删除' : 'Delete'}</Text>
        </TouchableOpacity>
      </View>
      <Animated.View
        style={{transform: [{translateX: tx}], backgroundColor: pal.bg}}
        {...pan.panHandlers}>
        {children}
      </Animated.View>
    </View>
  );
}

export function HistoryModal({
  visible,
  history,
  pal,
  lang,
  onPick,
  onDelete,
  onClear,
  onClose,
}: {
  visible: boolean;
  history: string[];
  pal: Palette;
  lang: Lang;
  onPick: (text: string) => void;
  onDelete: (index: number) => void;
  onClear: () => void;
  onClose: () => void;
}) {
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity activeOpacity={1} style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={styles.head}>
            <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '历史' : 'History'}</Text>
            {history.length > 0 && (
              <TouchableOpacity onPress={() => confirmClear(lang, onClear)} hitSlop={hit}>
                <Text style={[styles.clear, {color: pal.fg3}]}>{lang === 'zh' ? '清空' : 'Clear'}</Text>
              </TouchableOpacity>
            )}
          </View>
          {history.length === 0 ? (
            <Text style={[styles.empty, {color: pal.fg3}]}>
              {lang === 'zh' ? '还没有发送过消息' : 'No messages sent yet'}
            </Text>
          ) : (
            <ScrollView style={styles.list} keyboardShouldPersistTaps="always">
              {history.map((h, i) => (
                <SwipeRow key={`${i}-${h}`} pal={pal} lang={lang} onDelete={() => onDelete(i)}>
                  <TouchableOpacity
                    onPress={() => onPick(h)}
                    style={[styles.row, {borderBottomColor: pal.divider}]}>
                    <Text style={[styles.rowText, {color: pal.fg}]} numberOfLines={2}>
                      {h}
                    </Text>
                  </TouchableOpacity>
                </SwipeRow>
              ))}
            </ScrollView>
          )}
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 16, borderTopRightRadius: 16, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 14, paddingBottom: 28, maxHeight: '70%'},
  head: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8},
  title: {fontSize: 16, fontWeight: '700'},
  clear: {fontSize: 13, fontWeight: '600'},
  empty: {fontSize: 13, textAlign: 'center', paddingVertical: 28},
  list: {marginTop: 2},
  row: {paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  rowText: {fontSize: 14, lineHeight: 19},
  // delete action sits BEHIND the row, revealed as it slides left.
  deleteBg: {position: 'absolute', top: 0, bottom: 0, right: 0, width: DELETE_W, alignItems: 'center', justifyContent: 'center', backgroundColor: '#EF4444'},
  deleteBtn: {flex: 1, width: DELETE_W, alignItems: 'center', justifyContent: 'center'},
  deleteText: {color: '#fff', fontSize: 13, fontWeight: '700'},
});
