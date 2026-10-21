import React, {useEffect, useState} from 'react';
import {Keyboard, ScrollView, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import type {PaneInteraction} from '../api/types';
import type {Lang} from '../i18n';
import {ApprovalCard} from './ApprovalCard';
import {Chevron} from './Icons';
import {TerminalKeyFeedback, TerminalKeyPanel} from './TerminalKeyPanel';
import type {TerminalKeyController} from './terminalKeys';
import {type Palette, StatusColor} from './theme';

/** A live screen observation stays separate from the recorded conversation. */
export function SessionInteractionCard({scope, waiting, snapshot, loading, failed, enabled, control, pal, lang, onRetry, onLegacyKey, onReply, onCompose, onClose}: {
  scope: string; waiting: boolean; snapshot: PaneInteraction | null; loading: boolean; failed: boolean; enabled: boolean;
  control: TerminalKeyController; pal: Palette; lang: Lang; onRetry: () => void; onLegacyKey: (key: string) => void;
  onReply: (n: number) => void; onCompose: () => void; onClose: () => void;
}) {
  const zh = lang === 'zh';
  const {height, fontScale} = useWindowDimensions();
  const [collapsed, setCollapsed] = useState(false);
  const [peek, setPeek] = useState(false);
  const [keysOpen, setKeysOpen] = useState(false);
  useEffect(() => { setCollapsed(false); setPeek(false); setKeysOpen(false); }, [scope, waiting]);
  const options = failed || loading ? [] : snapshot?.options ?? [];
  const canInput = enabled && control.cap?.can_input !== false && !control.busy && !control.uncertain;
  const color = waiting ? StatusColor.waiting : pal.fg2;
  const press = (key: string) => {
    if (!canInput || control.isBlocked()) return;
    if (!control.legacy(key)) onLegacyKey(key);
  };
  return <View testID="session-interaction" key={fontScale} style={[styles.card, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
    <View style={styles.header}>
      <TouchableOpacity testID="interaction-toggle" accessibilityRole="button" accessibilityState={{expanded: !collapsed}} style={styles.heading} onPress={() => setCollapsed(v => !v)}>
        <Text style={{color, fontSize: 13, fontWeight: '600'}}>{waiting ? (zh ? '需要你回应' : 'Needs your reply') : (zh ? '终端交互' : 'Terminal interaction')}</Text>
        <Chevron open={!collapsed} color={color} />
      </TouchableOpacity>
      {!waiting && <TouchableOpacity testID="interaction-close" accessibilityRole="button" style={styles.button} onPress={onClose}><Text style={{color: pal.fg2}}>{zh ? '关闭' : 'Close'}</Text></TouchableOpacity>}
    </View>
    {!collapsed && <>
      {(control.cap?.actions ?? []).map(action => <TouchableOpacity key={action.id} testID="interaction-view-question" accessibilityRole="button" disabled={!canInput} style={styles.previewToggle} onPress={() => control.fire(action.event, action)}><Text style={{color: pal.fg2}}>{zh ? `${action.count ?? 1} 个问题 · 查看` : `${action.count ?? 1} question${action.count === 1 ? '' : 's'} · View`}</Text></TouchableOpacity>)}
      <ScrollView testID="interaction-content" style={{maxHeight: Math.min(280, height * 0.32)}} keyboardShouldPersistTaps="handled">
        {loading && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '正在读取当前提示…' : 'Reading current prompt…'}</Text>}
        {failed && <TouchableOpacity testID="interaction-retry" accessibilityRole="button" onPress={onRetry}><Text style={[styles.note, {color: pal.fg2}]}>{zh ? '暂时无法读取提示，点此重试。按键仍可使用。' : 'Prompt unavailable. Tap to retry; keys remain available.'}</Text></TouchableOpacity>}
        {!!snapshot && !failed && !loading && <>
          {options.length > 0 && <ApprovalCard options={options} embedded enabled={canInput} pal={pal} lang={lang} onSend={onReply} />}
          {options.length > 0 && <TouchableOpacity testID="interaction-preview-toggle" accessibilityRole="button" accessibilityState={{expanded: peek}} style={styles.previewToggle} onPress={() => setPeek(v => !v)}><Text style={{color: pal.fg2, fontSize: 12}}>{peek ? (zh ? '收起当前提示' : 'Hide current prompt') : (zh ? '查看当前提示' : 'View current prompt')}</Text></TouchableOpacity>}
          {(peek || !options.length) && <View style={styles.preview}>
            <Text style={[styles.caption, {color: pal.fg3}]}>{zh ? '当前终端画面' : 'Current terminal screen'}</Text>
            {snapshot.unavailable ? <Text style={{color: pal.fg2, fontSize: 12}}>{zh ? '这台 Mac 暂未提供当前画面，按键仍可使用。' : 'This Mac does not provide the current screen. Keys remain available.'}</Text>
              : snapshot.text ? <ScrollView horizontal showsHorizontalScrollIndicator><Text testID="interaction-screen" selectable style={{fontFamily: 'Menlo', fontSize: 12, lineHeight: 19, color: pal.fg}}>{snapshot.text}</Text></ScrollView>
                : <Text style={{color: pal.fg2, fontSize: 12}}>{zh ? '当前画面没有文字。可用按键继续操作。' : 'The current screen has no text. Use keys to continue.'}</Text>}
            {snapshot.truncated && <Text style={[styles.caption, {color: pal.fg3}]}>{zh ? '画面较长，仅显示末尾部分。' : 'Long screen; only its final portion is shown.'}</Text>}
          </View>}
        </>}
      </ScrollView>
      {!enabled && <Text style={[styles.note, {color: pal.fg3}]}>{zh ? '此会话为只读。' : 'This conversation is read-only.'}</Text>}
      <ScrollView horizontal keyboardShouldPersistTaps="always" contentContainerStyle={styles.keys}>
        {([['Up', '↑'], ['Down', '↓'], ['Enter', zh ? '回车' : 'Return'], ['Space', zh ? '空格' : 'Space']] as const).map(([key, label]) => <TouchableOpacity key={key} testID={`interaction-key-${key}`} accessibilityRole="button" accessibilityLabel={key === 'Up' ? (zh ? '向上' : 'Up arrow') : key === 'Down' ? (zh ? '向下' : 'Down arrow') : label} disabled={!canInput} style={[styles.button, !canInput && styles.disabled]} onPress={() => press(key)}><Text style={{color: pal.fg, fontSize: 14}}>{label}</Text></TouchableOpacity>)}
        <TouchableOpacity testID="interaction-more-keys" accessibilityRole="button" disabled={!canInput} style={[styles.button, !canInput && styles.disabled]} onPress={() => { Keyboard.dismiss(); control.refresh(); setKeysOpen(true); }}><Text style={{color: pal.fg2}}>{zh ? '更多按键' : 'More keys'}</Text></TouchableOpacity>
        <TouchableOpacity testID="interaction-compose" accessibilityRole="button" disabled={!canInput} style={[styles.button, !canInput && styles.disabled]} onPress={onCompose}><Text style={{color: pal.fg2}}>{zh ? '输入文字' : 'Type reply'}</Text></TouchableOpacity>
      </ScrollView>
      {control.uncertain && <TerminalKeyFeedback control={control} pal={pal} lang={lang} />}
    </>}
    <TerminalKeyPanel visible={keysOpen} onClose={() => setKeysOpen(false)} control={control} enabled={enabled} pane={scope} pal={pal} lang={lang} />
  </View>;
}

const styles = StyleSheet.create({
  card: {marginHorizontal: 10, marginBottom: 4, borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, overflow: 'hidden', flexShrink: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12},
  heading: {flex: 1, minHeight: 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 10},
  preview: {paddingHorizontal: 12, paddingBottom: 10}, previewToggle: {minHeight: 44, justifyContent: 'center', paddingHorizontal: 12},
  caption: {fontSize: 11, marginBottom: 5, marginTop: 4}, note: {fontSize: 12, lineHeight: 19, paddingHorizontal: 12, paddingBottom: 10},
  keys: {paddingHorizontal: 5, gap: 2}, button: {minWidth: 44, minHeight: 44, paddingHorizontal: 10, alignItems: 'center', justifyContent: 'center'}, disabled: {opacity: 0.4},
});
