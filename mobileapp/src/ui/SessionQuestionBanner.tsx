import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View, useWindowDimensions} from 'react-native';
import type {Lang} from '../i18n';
import type {KeyAction, TerminalKeyController} from './terminalKeys';
import {BRAND, type Palette} from './theme';
import {Chevron, QuestionBubbleIcon} from './Icons';

// Attention belongs near the conversation, separate from the keyboard shortcuts.
// Keep the advertised action/revision intact: opening a question is a typed key
// operation, never an invented text reply or an automatic approval.
export function SessionQuestionBanner({control, enabled, pal, lang, onOpen}: {
  control: TerminalKeyController; enabled: boolean; pal: Palette; lang: Lang; onOpen?: (action: KeyAction) => void;
}) {
  const compact = useWindowDimensions().height < 500;
  const actions = control.cap?.actions ?? [];
  if (!actions.length) return null;
  const disabled = !enabled || !control.cap?.can_input || control.busy || control.uncertain;
  const zh = lang === 'zh';
  return <View style={[styles.wrap, compact && styles.compactWrap, {backgroundColor: pal.bg}]}>
    {actions.map(action => {
      const count = Number.isInteger(action.count) && (action.count ?? 0) > 0 ? action.count! : 1;
      const question = action.id === 'codex.view-question';
      const title = question
        ? (zh ? (count === 1 ? '有个问题等你回答' : `${count} 个问题等你回答`) : (count === 1 ? 'A question needs your reply' : `${count} questions need your reply`))
        : (zh ? action.label_zh || action.label : action.label);
      const hint = !enabled || !control.cap?.can_input
        ? (zh ? '此会话未开放输入' : 'Input is not available for this session')
        : control.uncertain ? (zh ? '请先确认上次操作的结果' : 'Check the previous action before continuing')
          : (question ? (zh ? '查看问题并回复' : 'Open the question to reply') : (zh ? '打开会话中的操作' : 'Open this session action'));
      return <TouchableOpacity key={action.id} testID="session-question-banner"
        accessibilityRole="button" accessibilityLabel={title} accessibilityHint={hint}
        accessibilityState={{disabled, busy: control.busy}}
        activeOpacity={0.75} disabled={disabled}
        onPress={() => {
          if (!disabled && !control.isBlocked() && control.fire(action.event, action)) onOpen?.(action);
        }}
        style={[styles.card, compact && styles.compactCard, {backgroundColor: pal.raised, borderColor: BRAND}, disabled && styles.disabled]}>
        <QuestionBubbleIcon size={compact ? 20 : 24} color={BRAND} />
        <View style={styles.copy}>
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={2}>{title}</Text>
          {!compact && <Text style={[styles.hint, {color: pal.fg2}]} numberOfLines={2}>{hint}</Text>}
        </View>
        <View style={styles.open}>
          <Text style={[styles.openText, {color: pal.fg}]}>{zh ? '查看' : 'View'}</Text>
          <Chevron size={16} color={BRAND} />
        </View>
      </TouchableOpacity>;
    })}
  </View>;
}
const styles = StyleSheet.create({
  wrap: {paddingHorizontal: 12, paddingVertical: 8, gap: 8},
  compactWrap: {paddingVertical: 4},
  compactCard: {minHeight: 44, paddingVertical: 8},
  card: {minHeight: 68, borderWidth: 1, borderRadius: 14, paddingHorizontal: 14, paddingVertical: 11, flexDirection: 'row', alignItems: 'center', gap: 12},
  copy: {flex: 1, minWidth: 0, gap: 3},
  title: {fontSize: 15, lineHeight: 21, fontWeight: '600'},
  hint: {fontSize: 12, lineHeight: 17},
  open: {flexDirection: 'row', alignItems: 'center', gap: 3},
  openText: {fontSize: 13, fontWeight: '600'},
  disabled: {opacity: 0.55},
});
