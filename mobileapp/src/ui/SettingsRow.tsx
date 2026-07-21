// SettingsRow / SettingsGroup / PickerSheet — Moshi-style grouped preferences:
// labelled section cards of rows, each with a leading outline icon, a title, and a
// right-side affordance (current value + chevron for a drill-in, a toggle, or a
// plain action). Multi-option settings collapse to one value+chevron row that
// opens a PickerSheet, instead of spilling a long inline radio list.

import React, {useEffect, useRef, useState} from 'react';
import {Animated, Easing, Modal, Pressable, StyleSheet, Switch, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {SIcon, IconName} from './SettingsIcons';

const ACCENT = '#06B6D4';
const SELECTED_TINT = 'rgba(6,182,212,0.12)'; // current option's row highlight

export function SettingsGroup({title, pal, children}: {title?: string; pal: any; children: React.ReactNode}) {
  return (
    <View style={styles.group}>
      {!!title && <Text style={[styles.groupTitle, {color: pal.fg3}]}>{title.toUpperCase()}</Text>}
      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>{children}</View>
    </View>
  );
}

export function SettingsRow({
  icon,
  label,
  sub,
  value,
  pal,
  onPress,
  toggle,
  onToggle,
  toggleDisabled,
  chevron,
  danger,
  divider,
  right,
}: {
  icon?: IconName;
  label: string;
  sub?: string;
  value?: string;
  pal: any;
  onPress?: () => void;
  toggle?: boolean;
  onToggle?: (v: boolean) => void;
  toggleDisabled?: boolean;
  chevron?: boolean;
  danger?: boolean;
  divider?: boolean;
  right?: React.ReactNode;
}) {
  const labelColor = danger ? '#EF4444' : pal.fg;
  const inner = (
    <View style={[styles.row, divider && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth}]}>
      {icon && (
        <View style={styles.iconWrap}>
          <SIcon name={icon} size={21} color={danger ? '#EF4444' : pal.fg2} />
        </View>
      )}
      <View style={styles.textWrap}>
        <Text style={[styles.label, {color: labelColor}]} numberOfLines={2}>
          {label}
        </Text>
        {!!sub && (
          <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={2}>
            {sub}
          </Text>
        )}
      </View>
      {right ??
        (toggle !== undefined ? (
          <Switch value={toggle} onValueChange={onToggle} disabled={toggleDisabled} />
        ) : (
          <View style={styles.rightWrap}>
            {!!value && (
              <Text style={[styles.value, {color: pal.fg3}]} numberOfLines={1}>
                {value}
              </Text>
            )}
            {chevron && <Text style={[styles.chev, {color: pal.fg3}]}>›</Text>}
          </View>
        ))}
    </View>
  );
  return onPress ? (
    <TouchableOpacity activeOpacity={0.6} onPress={onPress}>
      {inner}
    </TouchableOpacity>
  ) : (
    inner
  );
}

// PickerSheet — a bottom sheet listing the options for a single setting; tap one
// to select + dismiss. Used by the value+chevron rows (theme/font/language/mode).
export function PickerSheet<T extends string>({
  visible,
  title,
  options,
  selected,
  pal,
  onSelect,
  onClose,
}: {
  visible: boolean;
  title: string;
  options: {key: T; label: string; sub?: string}[];
  selected: T;
  pal: any;
  onSelect: (key: T) => void;
  onClose: () => void;
}) {
  // Animate the backdrop and the sheet SEPARATELY: the dim fades in place while the
  // panel slides up from the bottom. RN's Modal animationType="slide" instead slid
  // the WHOLE modal (dim included) up together, so mid-animation you saw a gray
  // curtain sweeping up over the lower half with no panel — reading as a janky
  // half-screen overlay. `mounted` keeps the Modal alive through the exit animation.
  const [mounted, setMounted] = useState(visible);
  const [sheetH, setSheetH] = useState(0);
  const prog = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (visible) {
      setMounted(true);
      Animated.timing(prog, {toValue: 1, duration: 240, easing: Easing.out(Easing.cubic), useNativeDriver: true}).start();
    } else if (mounted) {
      Animated.timing(prog, {toValue: 0, duration: 180, easing: Easing.in(Easing.cubic), useNativeDriver: true}).start(({finished}) => {
        if (finished) setMounted(false);
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible]);

  if (!mounted) return null;

  const translateY = prog.interpolate({inputRange: [0, 1], outputRange: [sheetH || 800, 0]});

  return (
    <Modal visible={mounted} transparent animationType="none" onRequestClose={onClose}>
      <View style={styles.fill}>
        {/* dim: fades in place, never slides */}
        <Animated.View style={[StyleSheet.absoluteFill, styles.dim, {opacity: prog}]}>
          <Pressable style={styles.fill} onPress={onClose} />
        </Animated.View>
        {/* sheet: an ELEVATED surface (pal.surface, lighter than the page) that slides
            up from the bottom; measured once so the slide starts fully off-screen. */}
        <Animated.View style={[styles.sheetWrap, {transform: [{translateY}]}]}>
          <Pressable
            onLayout={e => setSheetH(e.nativeEvent.layout.height)}
            style={[styles.sheet, {backgroundColor: pal.surface, borderTopColor: pal.divider}]}
            onPress={() => {}}>
            <SafeAreaView edges={['bottom']}>
              <View style={styles.sheetHandle}>
                <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
              </View>
              <Text style={[styles.sheetTitle, {color: pal.fg2}]}>{title}</Text>
              <View style={[styles.sheetSep, {backgroundColor: pal.divider}]} />
              {options.map((o, i) => {
                const sel = selected === o.key;
                return (
                  <TouchableOpacity
                    key={o.key}
                    activeOpacity={0.6}
                    onPress={() => {
                      onSelect(o.key);
                      onClose();
                    }}
                    style={[
                      styles.pickRow,
                      sel && {backgroundColor: SELECTED_TINT},
                      i < options.length - 1 && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth},
                    ]}>
                    <View style={styles.textWrap}>
                      <Text style={[styles.pickLabel, {color: sel ? ACCENT : pal.fg}]}>{o.label}</Text>
                      {!!o.sub && <Text style={[styles.sub, {color: pal.fg3}]}>{o.sub}</Text>}
                    </View>
                    {sel && <Text style={styles.check}>✓</Text>}
                  </TouchableOpacity>
                );
              })}
            </SafeAreaView>
          </Pressable>
        </Animated.View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  group: {marginBottom: 22},
  groupTitle: {fontSize: 11.5, fontWeight: '700', letterSpacing: 0.6, marginBottom: 8, marginLeft: 16},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  row: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 13, minHeight: 52},
  iconWrap: {width: 30, alignItems: 'center', marginRight: 8},
  textWrap: {flex: 1, minWidth: 0},
  label: {fontSize: 16},
  sub: {fontSize: 12.5, marginTop: 2, lineHeight: 17},
  rightWrap: {flexDirection: 'row', alignItems: 'center', marginLeft: 8, flexShrink: 1},
  value: {fontSize: 15, flexShrink: 1},
  chev: {fontSize: 20, fontWeight: '300', marginLeft: 6},
  check: {color: ACCENT, fontSize: 18, fontWeight: '700', marginLeft: 10},
  fill: {flex: 1},
  // a noticeably stronger dim so the modal clearly takes focus (0.4 black over a
  // dark page was nearly invisible). Its opacity is animated 0→1 in place.
  dim: {backgroundColor: 'rgba(0,0,0,0.5)'},
  // pins the sliding sheet to the bottom, centered (not edge-to-edge) on iPad/wide.
  sheetWrap: {position: 'absolute', left: 0, right: 0, bottom: 0, alignItems: 'center'},
  sheet: {
    width: '100%',
    maxWidth: 520, // centered, not edge-to-edge, on iPad/wide
    alignSelf: 'center',
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingBottom: 6,
    shadowColor: '#000',
    shadowOpacity: 0.3,
    shadowRadius: 18,
    shadowOffset: {width: 0, height: -4},
    elevation: 16,
  },
  sheetHandle: {alignItems: 'center', paddingTop: 8, paddingBottom: 4},
  grabber: {width: 38, height: 5, borderRadius: 3},
  sheetTitle: {fontSize: 13, fontWeight: '600', textAlign: 'center', paddingVertical: 8},
  sheetSep: {height: StyleSheet.hairlineWidth},
  pickRow: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 18, paddingVertical: 15, minHeight: 56},
  pickLabel: {fontSize: 16.5},
});
