import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ConversationReload} from './ConversationReload';
import {InfoIcon} from './Icons';
import {readingColors, useReadingPalette} from './ReadingPalette';

/** Presentation only. The parent retains request identity and retry ownership. */
export function ReadFeedback({label, zh, onRetry, busy = false, retryFailed = false, secondary, compact = false,
  testID = 'read-feedback', retryTestID = 'chat-read-retry', retryAccessibilityLabel}: {
  label: string; zh: boolean; onRetry?: () => void; busy?: boolean; retryFailed?: boolean; compact?: boolean;
  secondary?: {label: string; onPress: () => void; testID?: string}; testID?: string; retryTestID?: string; retryAccessibilityLabel?: string;
}) {
  const pal = useReadingPalette();
  return <View testID={testID} style={[styles.card, compact && styles.compact, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
    <View style={styles.message}>
      <View accessible={false}><InfoIcon color={pal.fg3} /></View>
      <Text accessibilityLiveRegion={retryFailed ? 'polite' : undefined} style={[styles.label, {color: pal.fg}]}>
        {retryFailed && !busy ? (zh ? '仍无法加载，请稍后重试。' : 'Still unable to load. Try again shortly.') : label}
      </Text>
    </View>
    {(onRetry || secondary) && <View style={styles.actions}>
      {onRetry && <ConversationReload onRetry={onRetry} busy={busy} zh={zh} testID={retryTestID} accessibilityLabel={retryAccessibilityLabel} />}
      {secondary && <TouchableOpacity testID={secondary.testID} accessibilityRole="button" onPress={secondary.onPress} style={styles.secondary}>
        <Text style={[styles.secondaryText, {color: readingColors(pal).link}]}>{secondary.label}</Text>
      </TouchableOpacity>}
    </View>}
  </View>;
}
const styles = StyleSheet.create({
  card: {alignSelf: 'center', width: '92%', maxWidth: 520, marginVertical: 16, padding: 16, borderWidth: StyleSheet.hairlineWidth, borderRadius: 14},
  compact: {width: '100%', marginVertical: 8, padding: 12},
  message: {flexDirection: 'row', alignItems: 'flex-start', gap: 10},
  label: {flex: 1, minWidth: 0, fontSize: 14, lineHeight: 21},
  actions: {marginTop: 12, flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 8},
  secondary: {minHeight: 44, minWidth: 44, paddingHorizontal: 8, justifyContent: 'center'},
  secondaryText: {fontSize: 13, fontWeight: '500'},
});
