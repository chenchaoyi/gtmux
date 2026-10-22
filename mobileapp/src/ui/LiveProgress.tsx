import React from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {AnsiLine} from './ansi';
import {BrandLoader} from './BrandLoader';
import {Chevron} from './Icons';
import {TestIds} from '../constants/testIds';
import {Lang} from '../i18n';

/** The conversation owns disclosure state; this row can disappear between observations. */
export function LiveProgress({avatar, label, lines, fontSize, fontFamily, lang, open, onToggle, inspectAvailable = true}: {
  avatar: React.ReactNode;
  label: string;
  lines: AnsiLine[];
  fontSize: number;
  fontFamily: string | undefined;
  lang: Lang;
  open: boolean;
  onToggle: () => void;
  inspectAvailable?: boolean;
}) {
  const action = lang === 'zh' ? (open ? '收起实时画面' : '展开实时画面') : (open ? 'Hide live view' : 'Show live view');
  const marker = <>
    <BrandLoader size={14} neutral="rgba(255,255,255,0.30)" />
    <Text style={styles.label} numberOfLines={1}>{label}</Text>
  </>;
  return <View testID={TestIds.detail.chatThinking} style={[styles.root, open && inspectAvailable && styles.expanded]}>
    <View style={styles.row}>
      {avatar}
      {inspectAvailable ? <TouchableOpacity testID={TestIds.detail.chatLiveToggle}
        accessibilityRole="button" accessibilityLabel={`${label}, ${action}`}
        accessibilityState={{expanded: open}}
        onPress={onToggle} activeOpacity={0.7} style={[styles.disclosure, open && styles.openDisclosure]}>
        {marker}
        <View style={styles.action}>
          <Text style={styles.actionText}>{action}</Text>
          <Chevron open={open} size={12} color="#27C7E6" />
        </View>
      </TouchableOpacity> : <View style={styles.disclosure}>{marker}</View>}
    </View>
    {inspectAvailable && open && <View testID={TestIds.detail.chatLivePreview} style={styles.preview}>
      {lines.length ? <ScrollView style={styles.viewport} nestedScrollEnabled>
        <ScrollView horizontal nestedScrollEnabled contentContainerStyle={styles.screen}>
          <Text selectable style={[styles.mono, {fontFamily, fontSize, lineHeight: Math.round(fontSize * 1.4)}]}>
            {lines.map((spans, i) => <Text key={i}>
              {i > 0 ? '\n' : ''}
              {spans.length === 0 ? ' ' : spans.map((span, j) =>
                <Text key={j} style={{color: span.color, fontWeight: span.bold ? '700' : '400'}}>{span.text}</Text>)}
            </Text>)}
          </Text>
        </ScrollView>
      </ScrollView> : <Text style={styles.empty}>{lang === 'zh' ? '尚无实时输出' : 'No terminal output yet'}</Text>}
    </View>}
  </View>;
}

const styles = StyleSheet.create({
  root: {marginTop: 6},
  expanded: {backgroundColor: 'rgba(6,182,212,0.06)', borderWidth: StyleSheet.hairlineWidth, borderColor: 'rgba(6,182,212,0.25)', borderRadius: 10, padding: 10},
  row: {flexDirection: 'row', alignItems: 'center'},
  disclosure: {flex: 1, flexDirection: 'row', alignItems: 'center', gap: 7, minHeight: 44, paddingHorizontal: 10, paddingVertical: 8, borderRadius: 12, backgroundColor: '#1C1C1F'},
  openDisclosure: {backgroundColor: 'transparent', paddingHorizontal: 0},
  label: {flex: 1, fontSize: 12, color: 'rgba(235,235,245,0.65)', fontVariant: ['tabular-nums']},
  action: {flexDirection: 'row', alignItems: 'center', gap: 5, flexShrink: 1},
  actionText: {fontSize: 12, color: '#27C7E6', flexShrink: 1},
  preview: {marginTop: 5, overflow: 'hidden'},
  viewport: {maxHeight: 220},
  screen: {paddingVertical: 4},
  mono: {color: '#D6D6DA'},
  empty: {padding: 12, color: 'rgba(235,235,245,0.6)', fontSize: 12},
});
