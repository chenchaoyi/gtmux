import React, {useEffect, useRef, useState} from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import type {PaneInteraction} from '../api/types';
import type {Lang} from '../i18n';
import {ApprovalCard} from './ApprovalCard';
import {Chevron} from './Icons';
import type {TerminalKeyController} from './terminalKeys';
import {type Palette, StatusColor} from './theme';

/** A live screen observation stays separate from the recorded conversation. */
export function SessionInteractionCard({scope, snapshot, loading, failed, enabled, control, pal, lang, onRetry, onReply}: {
  scope: string; snapshot: PaneInteraction | null; loading: boolean; failed: boolean; enabled: boolean;
  control: TerminalKeyController; pal: Palette; lang: Lang; onRetry: () => void;
  onReply: (n: number) => void;
}) {
  const zh = lang === 'zh';
  const {height, fontScale} = useWindowDimensions();
  const [collapsed, setCollapsed] = useState(false);
  const [peek, setPeek] = useState(false);
  useEffect(() => { setCollapsed(false); setPeek(false); }, [scope]);
  const options = failed || loading ? [] : snapshot?.options ?? [];
  const question = options.length ? JSON.stringify(options) : '';
  const previousQuestion = useRef('');
  useEffect(() => {
    if (question && question !== previousQuestion.current) {
      previousQuestion.current = question;
      setCollapsed(false);
    }
  }, [question]);
  const canInput = enabled && control.cap?.can_input !== false && !control.busy && !control.uncertain;
  const color = StatusColor.waiting;
  return <View testID="session-interaction" key={fontScale} style={[styles.card, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
    <View style={styles.header}>
      <TouchableOpacity testID="interaction-toggle" accessibilityRole="button" accessibilityState={{expanded: !collapsed}} style={styles.heading} onPress={() => setCollapsed(v => !v)}>
        <Text style={{color, fontSize: 13, fontWeight: '600'}}>{zh ? '需要你回应' : 'Needs your reply'}</Text>
        <Chevron open={!collapsed} color={color} />
      </TouchableOpacity>
    </View>
    {!collapsed && <>
      <ScrollView testID="interaction-content" style={{maxHeight: Math.min(280, height * 0.32)}} keyboardShouldPersistTaps="handled">
        {loading && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '正在读取当前提示…' : 'Reading current prompt…'}</Text>}
        {failed && <TouchableOpacity testID="interaction-retry" accessibilityRole="button" onPress={onRetry}><Text style={[styles.note, {color: pal.fg2}]}>{zh ? '暂时无法读取提示，点此重试。按键仍可使用。' : 'Prompt unavailable. Tap to retry; keys remain available.'}</Text></TouchableOpacity>}
        {!!snapshot && !failed && !loading && <>
          {options.length > 0 && <ApprovalCard options={options} embedded enabled={canInput} pal={pal} lang={lang} onSend={n => {if (!canInput || control.isBlocked()) return; setCollapsed(true); onReply(n);}} />}
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
    </>}
  </View>;
}

const styles = StyleSheet.create({
  card: {marginHorizontal: 10, marginBottom: 4, borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, overflow: 'hidden', flexShrink: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12},
  heading: {flex: 1, minHeight: 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 10},
  preview: {paddingHorizontal: 12, paddingBottom: 10}, previewToggle: {minHeight: 44, justifyContent: 'center', paddingHorizontal: 12},
  caption: {fontSize: 11, marginBottom: 5, marginTop: 4}, note: {fontSize: 12, lineHeight: 19, paddingHorizontal: 12, paddingBottom: 10},
});
