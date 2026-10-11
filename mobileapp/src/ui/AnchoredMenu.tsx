// AnchoredMenu — a few actions that drop from the control that opened them, the way an
// iOS pull-down menu does. The Servers page's ••• opens one.
//
// It replaced a system alert (2026-10-07; the user marked it up as "optimise this"): the
// alert stacked five equal buttons in the middle of the screen, the destructive one among
// the neutral ones, and Cancel took a row of its own. A menu keeps the list in view, says
// what it belongs to in its header, and puts the destructive item last, in a group of its
// own. Tapping outside closes it, so there is no Cancel. The control that opened it stays
// lit above the dim (`lift`), so it is clear what the menu hangs from.
//
// Items are buttons to accessibility. "menuitem" would be the truer role, but on iOS's new
// architecture it maps to no trait at all, so VoiceOver read "Details" with no hint that it
// can be pressed (%6, review of #1532); the alert this replaced had real buttons.
//
// An item's action runs once the menu has gone (Modal onDismiss): iOS will not present an
// alert or another sheet while this modal is still dismissing, and Rename opens a prompt,
// Details a sheet, Remove an alert.

import React, {useEffect, useRef, useState} from 'react';
import {Modal, Platform, Pressable, ScrollView, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {IconName, SIcon} from './SettingsIcons';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {CHROME_MAX_SCALE} from './textScale';

export type MenuItem = {key: string; label: string; icon: IconName; danger?: boolean; selected?: boolean; disabled?: boolean; hint?: string; description?: string; onPress: () => void};
/** The opening control's frame in window coordinates (measureInWindow). */
export type MenuAnchor = {x: number; y: number; width: number; height: number};

// Danger is the same red SettingsRow uses for a destructive row.
const DANGER = '#EF4444';
const WIDTH = 250;
const EDGE = 8; // never closer to the window's edge than this
const GAP = 4; // between the control and the menu
// How long to wait for the control's measurement before showing the menu where it can.
const UNMEASURED_MS = 150;
const LIFT = 36; // the lit circle behind the control

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
  lift,
  sectionLabels,
  footer,
  preferredWidth = WIDTH,
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
  /** The opening control's glyph, drawn on a lit circle over it while the menu is open. */
  lift?: React.ReactNode;
  /** Optional settings group captions; regular action menus have none. */
  sectionLabels?: string[];
  /** Inline settings update without dismissing the menu. */
  footer?: React.ReactNode;
  preferredWidth?: number;
}) {
  const {width: winW, height: winH} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const settings = sectionLabels !== undefined;
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
    if (item.disabled) return;
    pending.current = item.onPress;
    onClose();
    if (Platform.OS !== 'ios') flush(); // onDismiss is iOS-only
  };

  // Right edges aligned with the control, kept inside the window; below it when it fits,
  // else above it, so a row near the bottom of the screen still shows every item.
  const width = Math.min(preferredWidth, winW - insets.left - insets.right - 2 * EDGE);
  const a = anchor ?? {x: winW - EDGE - 44, y: insets.top, width: 44, height: 44};
  const left = Math.min(Math.max(a.x + a.width - width, insets.left + EDGE), winW - insets.right - EDGE - width);
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
      {!!lift && anchor && ready && (
        <View
          testID={testID ? `${testID}-lift` : undefined}
          pointerEvents="none"
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={[styles.lift, {left: anchor.x + anchor.width / 2 - LIFT / 2, top: anchor.y + anchor.height / 2 - LIFT / 2, backgroundColor: pal.surface}]}>
          {lift}
        </View>
      )}
      <View
        testID={testID}
        accessibilityViewIsModal
        onAccessibilityEscape={onClose}
        onLayout={e => setMenuH(e.nativeEvent.layout.height)}
        // Hidden until both it and the control are measured, so it never flashes in one
        // place (or below a control it then has to move above) before settling.
        style={[styles.menu, {left, top, width, maxHeight: winH - insets.top - insets.bottom - 2 * EDGE, backgroundColor: pal.surface, borderColor: pal.divider}, !ready && styles.hidden]}>
        <View style={[styles.head, settings && styles.settingsHead]}>
          <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.title, settings && styles.settingsTitle, {color: settings ? pal.fg : pal.fg2}]} numberOfLines={1}>{title}</Text>
          {(subtitle ?? []).filter(Boolean).map(line => (
            <Text key={line} maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.subtitle, settings && styles.settingsSubtitle, {color: pal.fg2}]} numberOfLines={2} ellipsizeMode="middle">
              {line}
            </Text>
          ))}
        </View>
        <ScrollView style={styles.body} bounces={false}>
          {sections.map((items, index) => ({items, label: sectionLabels?.[index]})).filter(group => group.items.length > 0).map(({items, label}, si) => (
            <View key={items[0].key}>
              {/* The header ends in a hairline; between groups, a band of the page colour. */}
              <View style={si === 0 ? [styles.hair, {backgroundColor: pal.divider}] : [styles.band, {backgroundColor: pal.bg}]} />
              {!!label && <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.sectionLabel, {color: pal.fg2}]}>{label}</Text>}
              {items.map((item, ii) => (
                <View key={item.key}>
                  {ii > 0 && <View style={[styles.hair, styles.inset, {backgroundColor: pal.divider}]} />}
                  <TouchableOpacity
                    testID={testID ? `${testID}-${item.key}` : undefined}
                    accessibilityRole="button"
                    accessibilityLabel={item.label}
                    accessibilityHint={item.hint}
                    accessibilityState={{selected: item.selected, disabled: item.disabled}}
                    disabled={item.disabled}
                    activeOpacity={0.6}
                    onPress={() => choose(item)}
                    style={[styles.item, item.description !== undefined && styles.choice, item.selected && {backgroundColor: pal.rowSelected}, item.disabled && {opacity: 0.4}]}>
                    {item.selected !== undefined && <SIcon name={item.icon} size={20} color={pal.fg2} />}
                    <View style={styles.itemText}>
                      <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.label, item.description !== undefined && styles.choiceLabel, {color: item.danger ? DANGER : pal.fg}]} numberOfLines={1}>
                        {item.label}
                      </Text>
                      {item.description !== undefined && <Text accessible={false} maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.description, {color: pal.fg2}]}>{item.description}</Text>}
                    </View>
                    <View style={styles.mark}>
                      {item.selected === undefined
                        ? <SIcon name={item.icon} size={20} color={item.danger ? DANGER : pal.fg} />
                        : item.selected && <SIcon name="check" size={20} color={pal.fg} />}
                    </View>
                  </TouchableOpacity>
                </View>
              ))}
            </View>
          ))}
          {!!footer && <View style={[styles.footer, settings && styles.settingsFooter, {borderTopColor: pal.divider}]}>{footer}</View>}
        </ScrollView>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  dim: {backgroundColor: 'rgba(0,0,0,0.18)'},
  hidden: {opacity: 0},
  lift: {
    position: 'absolute',
    width: LIFT,
    height: LIFT,
    borderRadius: LIFT / 2,
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#000',
    shadowOpacity: 0.12,
    shadowRadius: 3,
    shadowOffset: {width: 0, height: 1},
  },
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
  settingsHead: {paddingHorizontal: 20},
  settingsTitle: {fontSize: 15, lineHeight: 20},
  settingsSubtitle: {fontSize: 12, lineHeight: 17},
  subtitle: {fontSize: 13, lineHeight: 17},
  hair: {height: StyleSheet.hairlineWidth},
  inset: {marginLeft: 16},
  band: {height: 8},
  item: {minHeight: 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, paddingHorizontal: 16},
  label: {fontSize: 17, flexShrink: 1},
  body: {flexGrow: 0, flexShrink: 1},
  itemText: {flex: 1, gap: 3},
  mark: {width: 20, alignItems: 'center'},
  choice: {minHeight: 64, paddingHorizontal: 12, paddingVertical: 10, marginHorizontal: 8, borderRadius: 8},
  choiceLabel: {fontSize: 15, lineHeight: 20, fontWeight: '400'},
  description: {fontSize: 12, lineHeight: 17},
  sectionLabel: {fontSize: 12, lineHeight: 17, fontWeight: '400', paddingHorizontal: 20, paddingTop: 12, paddingBottom: 4},
  settingsFooter: {paddingHorizontal: 20},
  footer: {borderTopWidth: StyleSheet.hairlineWidth, marginTop: 8, paddingHorizontal: 16, paddingTop: 12, paddingBottom: 14},
});
