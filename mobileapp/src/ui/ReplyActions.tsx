// Actions apply to the complete textual reply, including collapsed segments.
import React, {useEffect, useState} from 'react';
import {AccessibilityInfo, Alert, Share, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {CopyIcon, ShareIcon} from './Icons';
import type {Lang} from '../i18n';

export function ReplyActions({text, lang}: {text: string; lang: Lang}) {
  const [copied, setCopied] = useState<string | null>(null);
  const zh = lang === 'zh';
  const done = copied === text;
  useEffect(() => {
    if (copied === null) return;
    const timer = setTimeout(() => setCopied(null), 1800);
    return () => clearTimeout(timer);
  }, [copied]);
  if (!text) return null;
  const copyLabel = done ? zh ? '已复制' : 'Copied' : zh ? '复制全文' : 'Copy reply';
  const shareLabel = zh ? '分享' : 'Share';
  return (
    <View style={styles.row}>
      <TouchableOpacity accessibilityRole="button" accessibilityLabel={copyLabel}
        style={styles.button} activeOpacity={0.6} onPress={() => {
          Clipboard.setString(text);
          setCopied(text);
          AccessibilityInfo.announceForAccessibility(zh ? '已复制回复全文' : 'Reply copied');
        }}>
        <CopyIcon size={17} color={done ? '#E6F7FB' : FG} />
        <Text style={styles.label}>{copyLabel}</Text>
      </TouchableOpacity>
      <TouchableOpacity accessibilityRole="button" accessibilityLabel={zh ? '分享回复' : 'Share reply'}
        style={styles.button} activeOpacity={0.6} onPress={() => {
          Share.share({message: text}).catch(() => Alert.alert(zh ? '无法打开分享' : 'Could not open sharing',
            zh ? '请复制全文后分享。' : 'Copy the reply to share it.'));
        }}>
        <ShareIcon size={17} color={FG} />
        <Text style={styles.label}>{shareLabel}</Text>
      </TouchableOpacity>
    </View>
  );
}
const FG = 'rgba(235,235,245,0.6)';
const styles = StyleSheet.create({
  row: {flexDirection: 'row', flexWrap: 'wrap', marginLeft: 34, gap: 8},
  button: {minHeight: 44, minWidth: 44, flexDirection: 'row', alignItems: 'center', gap: 5, paddingHorizontal: 6},
  label: {color: FG, fontSize: 12.5, flexShrink: 1},
});
