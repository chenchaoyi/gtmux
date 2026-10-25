// Actions apply to the complete textual reply, including collapsed segments.
import React, {useEffect, useState} from 'react';
import {AccessibilityInfo, Alert, Share, StyleSheet, TouchableOpacity, View} from 'react-native';
import {ChromeText as Text} from './ChromeText';
import Clipboard from '@react-native-clipboard/clipboard';
import {CheckIcon, CopyIcon, ShareIcon, Chevron} from './Icons';
import type {Lang} from '../i18n';

export function ReplyActions({text, lang, open = true, onToggle}: {text: string; lang: Lang; open?: boolean; onToggle?: () => void}) {
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
  return (
    <View style={styles.row}>
      <TouchableOpacity accessibilityRole="button" accessibilityLabel={copyLabel}
        style={styles.button} activeOpacity={0.6} onPress={() => {
          Clipboard.setString(text);
          setCopied(text);
          AccessibilityInfo.announceForAccessibility(zh ? '已复制回复全文' : 'Reply copied');
        }}>
        {done ? <CheckIcon size={17} color="#E6F7FB" /> : <CopyIcon size={17} color={FG} />}
      </TouchableOpacity>
      <TouchableOpacity accessibilityRole="button" accessibilityLabel={zh ? '分享回复' : 'Share reply'}
        style={styles.button} activeOpacity={0.6} onPress={() => {
          Share.share({message: text}).catch(() => Alert.alert(zh ? '无法打开分享' : 'Could not open sharing',
            zh ? '请复制全文后分享。' : 'Copy the reply to share it.'));
        }}>
        <ShareIcon size={17} color={FG} />
      </TouchableOpacity>
      {onToggle && <TouchableOpacity testID="reply-fold" accessibilityRole="button" accessibilityState={{expanded: open}}
        accessibilityLabel={open ? (zh ? '收起本轮' : 'Collapse this reply') : (zh ? '展开本轮' : 'Expand this reply')}
        style={styles.fold} activeOpacity={0.6} onPress={onToggle}>
        <View style={{transform: [{rotate: open ? '-90deg' : '90deg'}]}}><Chevron size={16} color={FG} /></View>
        <Text style={styles.foldLabel}>{open ? (zh ? '收起本轮' : 'Collapse reply') : (zh ? '展开本轮' : 'Expand reply')}</Text>
      </TouchableOpacity>}
    </View>
  );
}
const FG = 'rgba(235,235,245,0.6)';
const styles = StyleSheet.create({
  row: {flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', marginLeft: 52, marginTop: -2, gap: 2},
  fold: {marginLeft: 'auto', minHeight: 44, paddingHorizontal: 10, flexDirection: 'row', gap: 6, alignItems: 'center'},
  foldLabel: {fontSize: 12, color: FG},
  button: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center'},
});
