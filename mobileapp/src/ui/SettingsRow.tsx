// SettingsRow / SettingsGroup / PickerSheet — Moshi-style grouped preferences:
// labelled section cards of rows, each with a leading outline icon, a title, and a
// right-side affordance (current value + chevron for a drill-in, a toggle, or a
// plain action). Multi-option settings collapse to one value+chevron row that
// opens a PickerSheet, instead of spilling a long inline radio list.

import React from 'react';
import {Modal, Pressable, StyleSheet, Switch, Text, TouchableOpacity, View} from 'react-native';
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
        <Text style={[styles.label, {color: labelColor}]} numberOfLines={1}>
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
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose}>
        {/* the sheet is an ELEVATED surface (pal.surface, lighter than the page) with
            a top border + shadow, so on a dark screen it reads as a distinct panel
            instead of blending into the dimmed background. */}
        <Pressable style={[styles.sheet, {backgroundColor: pal.surface, borderTopColor: pal.divider}]} onPress={() => {}}>
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
      </Pressable>
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
  // a noticeably stronger dim so the modal clearly takes focus (0.4 black over a
  // dark page was nearly invisible).
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.5)', justifyContent: 'flex-end'},
  sheet: {
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
