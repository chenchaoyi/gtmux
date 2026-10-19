import React, {useEffect, useState} from 'react';
import {
  AppState,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import type {Palette} from './theme';
import type {Lang} from '../i18n';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {
  keyEventLabel,
  sortedModifiers,
  supportsKey,
  type KeyModifier,
  type TerminalKeyController,
} from './terminalKeys';

export function TerminalKeyFeedback({
  control,
  pal,
  lang,
}: {
  control: TerminalKeyController;
  pal: Palette;
  lang: Lang;
}) {
  if (!control.message && !control.busy) return null;
  const zh = lang === 'zh';
  const words: Record<string, string> = zh
    ? {
        sent: '已发送到终端',
        uncertain: '结果未确认，请先查看终端',
        stale: '目标或提示已变化，请核对后重新选择',
        forbidden: '此窗格不允许输入',
        failed: '按键未能发送',
      }
    : {
        sent: 'Sent to terminal',
        uncertain: 'Result unconfirmed. Inspect the terminal first',
        stale: 'Target or prompt changed. Inspect it and select again',
        forbidden: 'Input is not allowed in this pane',
        failed: 'The key could not be sent',
      };
  return (
    <View style={styles.feedback}>
      <Text
        accessibilityLiveRegion="polite"
        style={{color: pal.fg2, fontSize: 13}}
      >
        {control.lastKey}
        {control.lastKey ? ' · ' : ''}
        {control.busy ? (zh ? '发送中…' : 'Sending…') : words[control.message]}
      </Text>
      {control.uncertain && (
        <View style={styles.row}>
          {control.canRetry && (
            <TouchableOpacity
              style={styles.action}
              onPress={control.retry}
              disabled={control.busy}
              accessibilityRole="button"
            >
              <Text style={{color: pal.fg}}>
                {zh ? '重试同一按键' : 'Retry same key'}
              </Text>
            </TouchableOpacity>
          )}
          <TouchableOpacity
            style={styles.action}
            onPress={control.resume}
            disabled={control.busy}
            accessibilityRole="button"
          >
            <Text style={{color: pal.fg}}>
              {zh ? '不重试，继续操作' : 'Continue without retry'}
            </Text>
          </TouchableOpacity>
        </View>
      )}
    </View>
  );
}

export function TerminalKeyPanel({
  visible,
  onClose,
  onDismiss,
  control,
  enabled,
  pane,
  pal,
  lang,
}: {
  visible: boolean;
  onClose: () => void;
  onDismiss?: () => void;
  control: TerminalKeyController;
  enabled: boolean;
  pane?: string;
  pal: Palette;
  lang: Lang;
}) {
  const [group, setGroup] = useState('navigation');
  const [mods, setMods] = useState<KeyModifier[]>([]);
  const {width, height, fontScale} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const zh = lang === 'zh';
  useEffect(() => {
    setMods([]);
  }, [visible, pane, enabled, control.cap?.can_input]);
  useEffect(() => {
    const sub = AppState.addEventListener('change', s => {
      if (s !== 'active') setMods([]);
    });
    return () => sub.remove();
  }, []);
  const panelWidth = Math.min(width - 24, 480);
  const columns = Math.max(
    2,
    Math.min(
      group === 'characters' ? 6 : 4,
      Math.floor((panelWidth - 32) / (fontScale > 1.3 ? 90 : 52)),
    ),
  );
  const keyWidth = Math.floor((panelWidth - 34 - (columns - 1) * 6) / columns);
  const keys = control.cap?.keys?.filter(k => k.group === group) ?? [];
  const maySend =
    enabled && !!control.cap?.can_input && !control.busy && !control.uncertain;
  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={onClose}
      onDismiss={onDismiss}
    >
      <TouchableOpacity
        accessible={false}
        activeOpacity={1}
        style={[
          styles.backdrop,
          {
            alignItems: width >= 768 ? 'flex-end' : 'center',
            paddingBottom: Math.max(insets.bottom, 12),
          },
        ]}
        onPress={onClose}
      >
        <View
          accessibilityViewIsModal
          onAccessibilityEscape={onClose}
          onStartShouldSetResponder={() => true}
          style={[
            styles.panel,
            {
              width: panelWidth,
              maxHeight: Math.max(
                180,
                Math.min(height - insets.top - insets.bottom - 24, 540),
              ),
              backgroundColor: pal.surface,
              borderColor: pal.divLoud,
            },
          ]}
        >
          <View style={styles.row}>
            <Text
              accessibilityRole="header"
              style={[styles.title, {color: pal.fg}]}
            >
              {zh ? '终端按键' : 'Terminal keys'} {pane}
            </Text>
            <TouchableOpacity
              testID="terminal-keys-close"
              accessibilityRole="button"
              accessibilityLabel={zh ? '关闭按键面板' : 'Close terminal keys'}
              style={styles.action}
              onPress={onClose}
            >
              <Text style={{color: pal.fg}}>{zh ? '关闭' : 'Close'}</Text>
            </TouchableOpacity>
          </View>
          <Text style={{color: pal.fg2, fontSize: 12}}>
            {mods.length
              ? `${sortedModifiers(mods)
                  .map(m =>
                    m === 'control' ? 'Ctrl' : m === 'alt' ? 'Alt' : 'Shift',
                  )
                  .join(' + ')} + …`
              : zh
              ? '点按发送一个键，不附加回车'
              : 'Tap to send one key, without an extra Return'}
          </Text>
          <View style={[styles.row, styles.section]}>
            {(['control', 'alt', 'shift'] as KeyModifier[]).map(mod => (
              <TouchableOpacity
                key={mod}
                testID={`terminal-modifier-${mod}`}
                accessibilityRole="button"
                accessibilityState={{
                  selected: mods.includes(mod),
                  disabled: !maySend,
                }}
                disabled={!maySend}
                onPress={() =>
                  setMods(m =>
                    m.includes(mod) ? m.filter(v => v !== mod) : [...m, mod],
                  )
                }
                style={[
                  styles.modifier,
                  {
                    backgroundColor: mods.includes(mod)
                      ? pal.raised
                      : pal.surface,
                    borderColor: mods.includes(mod) ? pal.fg2 : pal.divider,
                  },
                ]}
              >
                <Text style={{color: pal.fg, fontSize: 14}}>
                  {mods.includes(mod) ? '✓ ' : ''}
                  {mod === 'control' ? 'Ctrl' : mod === 'alt' ? 'Alt' : 'Shift'}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
          <View style={[styles.row, styles.section]}>
            {[
              ['navigation', zh ? '导航' : 'Navigation'],
              ['function', zh ? '功能' : 'Function'],
              ['characters', zh ? '字符' : 'Characters'],
            ].map(([id, label]) => (
              <TouchableOpacity
                key={id}
                testID={`terminal-category-${id}`}
                accessibilityRole="tab"
                accessibilityState={{selected: group === id}}
                style={[
                  styles.tab,
                  {borderBottomColor: group === id ? pal.fg : 'transparent'},
                ]}
                onPress={() => setGroup(id)}
              >
                <Text
                  style={{
                    color: group === id ? pal.fg : pal.fg2,
                    fontSize: 13,
                  }}
                >
                  {label}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
          {group === 'characters' && (
            <Text style={{color: pal.fg2, fontSize: 12}}>
              {zh
                ? '终端别名：Ctrl+I = Tab，Ctrl+M = 回车，Ctrl+[ = Esc'
                : 'Terminal aliases: Ctrl+I = Tab, Ctrl+M = Return, Ctrl+[ = Esc'}
            </Text>
          )}
          {!control.cap && (
            <Text style={[styles.feedback, {color: pal.fg2}]}>
              {zh
                ? '暂时无法读取通用按键能力，原有快捷键仍可使用'
                : 'General keys are unavailable. Existing shortcuts remain available'}
            </Text>
          )}
          {control.cap && !control.cap.can_input && (
            <Text style={{color: pal.fg2}}>
              {zh ? '此窗格未开放输入' : 'Input is not shared for this pane'}
            </Text>
          )}
          <ScrollView
            style={styles.scroll}
            keyboardShouldPersistTaps="always"
            contentContainerStyle={styles.grid}
          >
            {keys.map(k => {
              const supported = supportsKey(k, mods);
              const label = keyEventLabel({key: k.key, modifiers: mods});
              return (
                <TouchableOpacity
                  key={k.key}
                  testID={`terminal-key-${k.key}`}
                  accessibilityRole="button"
                  accessibilityLabel={`${label}${
                    supported
                      ? ''
                      : zh
                      ? '，当前终端不支持此组合'
                      : ', unsupported combination'
                  }`}
                  accessibilityState={{disabled: !maySend || !supported}}
                  disabled={!maySend || !supported}
                  style={[
                    styles.key,
                    {
                      width: keyWidth,
                      backgroundColor: pal.raised,
                      borderColor: pal.divider,
                      opacity: maySend && supported ? 1 : 0.45,
                    },
                  ]}
                  onPress={() => {
                    const event = {
                      key: k.key,
                      modifiers: sortedModifiers(mods),
                    };
                    setMods([]);
                    control.fire(event);
                  }}
                >
                  <Text
                    style={{color: pal.fg, fontSize: 14, textAlign: 'center'}}
                  >
                    {zh ? k.label_zh : k.label}
                  </Text>
                </TouchableOpacity>
              );
            })}
          </ScrollView>
          <TerminalKeyFeedback control={control} pal={pal} lang={lang} />
        </View>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    justifyContent: 'flex-end',
    paddingHorizontal: 12,
    backgroundColor: 'rgba(0,0,0,0.55)',
  },
  panel: {
    borderRadius: 16,
    borderWidth: StyleSheet.hairlineWidth,
    padding: 16,
    flexShrink: 1,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 6,
  },
  title: {flex: 1, fontSize: 17, fontWeight: '700'},
  action: {
    minHeight: 44,
    minWidth: 44,
    justifyContent: 'center',
    paddingHorizontal: 8,
  },
  section: {marginTop: 8},
  modifier: {
    flex: 1,
    minHeight: 44,
    borderRadius: 8,
    borderWidth: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  tab: {
    flex: 1,
    minHeight: 44,
    borderBottomWidth: 2,
    justifyContent: 'center',
    alignItems: 'center',
  },
  scroll: {flexShrink: 1},
  grid: {flexDirection: 'row', flexWrap: 'wrap', gap: 6, paddingVertical: 10},
  key: {
    minHeight: 48,
    borderRadius: 8,
    borderWidth: StyleSheet.hairlineWidth,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 5,
  },
  feedback: {paddingVertical: 6},
});
