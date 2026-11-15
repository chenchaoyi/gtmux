import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {BRAND, type Palette} from './theme';
import {classifySendFailure, failureCopy} from './sendFailure';
import type {SendFailureCode} from '../api/sendFailureCode';

/** Recovery for one attempted send, beside the composer that retains its content. */
export function SendFailedBar({text, reason, status, code, retrying = false, canEdit = false, canRetry = true, pal, lang,
  onRetry, onSendAnyway, onBackToRadar, onPairAgain, onDismiss}: {
  text: string; reason?: string; status?: number; code?: SendFailureCode; retrying?: boolean; canEdit?: boolean; canRetry?: boolean;
  pal: Palette; lang: string; onRetry: () => void; onSendAnyway?: () => void;
  onBackToRadar?: () => void; onPairAgain?: () => void; onDismiss: () => void;
}) {
  const zh = lang === 'zh';
  const copy = failureCopy(classifySendFailure(reason ?? '', status, code), zh);
  if (!copy.show) return null;
  const act = copy.action === 'send-anyway' ? onSendAnyway
    : copy.action === 'back-to-radar' ? onBackToRadar
    : copy.action === 'pair-again' ? onPairAgain
    : copy.action === 'retry' && canRetry ? onRetry : undefined;
  return (
    <View testID="send-failed-bar" style={[styles.bar, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
      <View style={styles.body}>
        <Text accessibilityLiveRegion="polite" style={[styles.title, {color: pal.fg}]} numberOfLines={2}>
          {retrying ? (zh ? '正在重试…' : 'Retrying…') : copy.title}
        </Text>
        <Text style={[styles.preview, {color: pal.fg2}]} numberOfLines={1}>{text}</Text>
      </View>
      <View style={styles.actions}>
        {act && <TouchableOpacity testID={`send-failed-${copy.action}`} accessibilityRole="button"
          accessibilityState={{busy: retrying, disabled: retrying}} disabled={retrying}
          onPress={() => {if (!retrying) act();}} style={styles.action}>
          <Text style={[styles.actionText, {color: retrying ? pal.fg3 : BRAND}]}>
            {retrying ? (zh ? '重试中' : 'Retrying') : copy.actionLabel}
          </Text>
        </TouchableOpacity>}
        <TouchableOpacity testID="send-failed-edit" accessibilityRole="button"
          accessibilityLabel={canEdit ? (zh ? '编辑未发送内容' : 'Edit unsent message') : (zh ? '关闭发送提示' : 'Dismiss send notice')}
          accessibilityState={{disabled: retrying}} disabled={retrying}
          onPress={() => {if (!retrying) onDismiss();}} style={styles.action}>
          <Text style={[styles.edit, {color: pal.fg2}]}>{canEdit ? (zh ? '编辑' : 'Edit') : (zh ? '关闭' : 'Close')}</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}
const styles = StyleSheet.create({
  bar: {flexDirection: 'row', alignItems: 'center', gap: 10, marginHorizontal: 10, marginTop: 8, marginBottom: 6,
    paddingLeft: 12, paddingRight: 4, paddingVertical: 12, borderRadius: 12, borderWidth: StyleSheet.hairlineWidth},
  body: {flex: 1, minWidth: 0},
  title: {fontSize: 13, lineHeight: 18, fontWeight: '500'},
  preview: {fontSize: 12, lineHeight: 17, marginTop: 3},
  actions: {flexDirection: 'row', alignItems: 'center', flexShrink: 0},
  action: {minWidth: 44, minHeight: 44, paddingHorizontal: 8, justifyContent: 'center', alignItems: 'center'},
  actionText: {fontSize: 13, fontWeight: '600'},
  edit: {fontSize: 13},
});
