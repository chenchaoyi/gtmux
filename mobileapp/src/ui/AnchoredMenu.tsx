// AnchoredMenu — a few actions that drop from the control that opened them, the way an
// iOS pull-down menu does. The Servers page's ••• opens one.
//
// It replaced a system alert (2026-10-07; the user marked it up as "optimise this"): the
// alert stacked five equal buttons in the middle of the screen, the destructive one among
// the neutral ones, and Cancel took a row of its own. A menu keeps the list in view, says
// what it belongs to in its header, and puts the destructive item last, in a group of its
// own. Tapping outside closes it, so there is no Cancel.
//
// An item's action runs once the menu has gone (Modal onDismiss): iOS will not present an
// alert or another sheet while this modal is still dismissing, and Rename opens a prompt,
// Details a sheet, Remove an alert.

import React, {useEffect, useRef, useState} from 'react';
import {Modal, Platform, Pressable, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {IconName, SIcon} from './SettingsIcons';
import {MODAL_ORIENTATIONS} from './modalOrientations';

export type MenuItem = {key: string; label: string; icon: IconName; danger?: boolean; onPress: () => void};
/** The opening control's frame in window coordinates (measureInWindow). */
export type MenuAnchor = {x: number; y: number; width: number; height: number};

// Danger is the same red SettingsRow uses for a destructive row.
const DANGER = '#EF4444';
const WIDTH = 250;
const EDGE = 8; // never closer to the window's edge than this
const GAP = 4; // between the control and the menu
// How long to wait for the control's measurement before showing the menu where it can.
const UNMEASURED_MS = 150;

export function AnchoredMenu({
  visible,
  anchor,
  title,
  subtitle,
  sections,
  pal,
  closeLabel,
  onClose,
  testID,
}: {
  visible: boolean;
  /** null until the control is measured; if that never comes, the window's top right. */
  anchor: MenuAnchor | null;
  title: string;
  subtitle?: string[];
  sections: MenuItem[][];
  pal: any;
  closeLabel: string;
  onClose: () => void;
  testID?: string;
}) {
  const {width: winW, height: winH} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const [menuH, setMenuH] = useState(0);
  // The control is measured asynchronously; the menu waits for it rather than appearing in
  // one place and jumping to another, but not forever.
  const [gaveUp, setGaveUp] = useState(false);
  useEffect(() => {
    if (!visible) {
      setGaveUp(false);
      return;
    }
    const id = setTimeout(() => setGaveUp(true), UNMEASURED_MS);
    return () => clearTimeout(id);
  }, [visible]);
  const ready = menuH > 0 && (anchor !== null || gaveUp);
  const pending = useRef<(() => void) | null>(null);

  const flush = () => {
    const run = pending.current;
    pending.current = null;
    run?.();
  };
  const choose = (item: MenuItem) => {
    pending.current = item.onPress;
    onClose();
    if (Platform.OS !== 'ios') flush(); // onDismiss is iOS-only
  };

  // Right edges aligned with the control, kept inside the window; below it when it fits,
  // else above it, so a row near the bottom of the screen still shows every item.
  const width = Math.min(WIDTH, winW - 2 * EDGE);
  const a = anchor ?? {x: winW - EDGE - 44, y: insets.top, width: 44, height: 44};
  const left = Math.min(Math.max(a.x + a.width - width, EDGE), winW - EDGE - width);
  const below = a.y + a.height + GAP;
  const fitsBelow = below + menuH <= winH - insets.bottom - EDGE;
  const top = fitsBelow ? below : Math.max(insets.top + EDGE, a.y - GAP - menuH);

  return (
    <Modal
      supportedOrientations={MODAL_ORIENTATIONS}
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onClose}
      onDismiss={flush}>
      <Pressable
        style={[StyleSheet.absoluteFill, styles.dim]}
        onPress={onClose}
        accessibilityRole="button"
        accessibilityLabel={closeLabel}
      />
      <View
        testID={testID}
        accessibilityViewIsModal
        onAccessibilityEscape={onClose}
        onLayout={e => setMenuH(e.nativeEvent.layout.height)}
        // Hidden until both it and the control are measured, so it never flashes in one
        // place (or below a control it then has to move above) before settling.
        style={[styles.menu, {left, top, width, backgroundColor: pal.surface, borderColor: pal.divider}, !ready && styles.hidden]}>
        <View style={styles.head}>
          <Text style={[styles.title, {color: pal.fg2}]} numberOfLines={1}>{title}</Text>
          {(subtitle ?? []).filter(Boolean).map(line => (
            <Text key={line} style={[styles.subtitle, {color: pal.fg2}]} numberOfLines={1} ellipsizeMode="middle">
              {line}
            </Text>
          ))}
        </View>
        {sections.filter(s => s.length > 0).map((items, si) => (
          <View key={items[0].key}>
            {/* The header ends in a hairline; between groups, a band of the page colour. */}
            <View style={si === 0 ? [styles.hair, {backgroundColor: pal.divider}] : [styles.band, {backgroundColor: pal.bg}]} />
            {items.map((item, ii) => (
              <View key={item.key}>
                {ii > 0 && <View style={[styles.hair, styles.inset, {backgroundColor: pal.divider}]} />}
                <TouchableOpacity
                  testID={testID ? `${testID}-${item.key}` : undefined}
                  accessibilityRole="menuitem"
                  accessibilityLabel={item.label}
                  activeOpacity={0.6}
                  onPress={() => choose(item)}
                  style={styles.item}>
                  <Text style={[styles.label, {color: item.danger ? DANGER : pal.fg}]} numberOfLines={1}>
                    {item.label}
                  </Text>
                  <SIcon name={item.icon} size={20} color={item.danger ? DANGER : pal.fg} />
                </TouchableOpacity>
              </View>
            ))}
          </View>
        ))}
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  dim: {backgroundColor: 'rgba(0,0,0,0.18)'},
  hidden: {opacity: 0},
  menu: {
    position: 'absolute',
    borderRadius: 14,
    borderWidth: StyleSheet.hairlineWidth,
    overflow: 'hidden',
    shadowColor: '#000',
    shadowOpacity: 0.18,
    shadowRadius: 16,
    shadowOffset: {width: 0, height: 8},
    elevation: 12,
  },
  head: {paddingHorizontal: 16, paddingTop: 11, paddingBottom: 10, gap: 2},
  title: {fontSize: 13, lineHeight: 17, fontWeight: '600'},
  subtitle: {fontSize: 13, lineHeight: 17},
  hair: {height: StyleSheet.hairlineWidth},
  inset: {marginLeft: 16},
  band: {height: 8},
  item: {minHeight: 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, paddingHorizontal: 16},
  label: {fontSize: 17, flexShrink: 1},
});
