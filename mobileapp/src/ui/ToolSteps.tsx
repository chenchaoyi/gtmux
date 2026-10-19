import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {TranscriptStep} from '../api/client';

/** A call is evidence of an action, not evidence that it succeeded. */
export function ToolSteps({steps, open, onToggle, lang, fontSize}: {
  steps: TranscriptStep[];
  open: boolean;
  onToggle: () => void;
  lang: 'en' | 'zh';
  fontSize: number;
}) {
  const counts = new Map<string, number>();
  steps.forEach(step => counts.set(step.title || 'tool', (counts.get(step.title || 'tool') || 0) + 1));
  const tools = [...counts];
  const summary = tools.slice(0, 3).map(([name, count]) => count > 1 ? `${name} ×${count}` : name).join(' · ')
    + (tools.length > 3 ? ` · +${tools.length - 3}` : '');
  const heading = lang === 'zh' ? '执行过程' : 'Activity';
  const count = lang === 'zh' ? `${steps.length} 次调用` : `${steps.length} call${steps.length === 1 ? '' : 's'}`;
  const size = Math.max(12, fontSize - 1);
  return (
    <View style={styles.card}>
      <TouchableOpacity
        testID="tool-steps-toggle"
        accessibilityRole="button"
        accessibilityLabel={`${heading}, ${count}`}
        accessibilityHint={lang === 'zh' ? (open ? '收起工具调用' : '展开工具调用') : (open ? 'Hide tool calls' : 'Show tool calls')}
        accessibilityState={{expanded: open}}
        activeOpacity={0.7}
        onPress={onToggle}
        style={styles.header}>
        <View style={styles.headingRow}>
          <Text style={[styles.heading, {fontSize: size}]}>{heading}</Text>
          <Text style={[styles.count, {fontSize: size}]}>{count}</Text>
          <Text style={styles.chevron}>{open ? '⌃' : '⌄'}</Text>
        </View>
        {!open && <Text style={[styles.summary, {fontSize: size}]} numberOfLines={1}>{summary}</Text>}
      </TouchableOpacity>
      {open && <View testID="tool-steps-list" style={styles.list}>
        {steps.map((step, i) => <View key={i} testID="tool-step-row" style={styles.row}>
          <View style={styles.rail}>
            <Text style={[styles.ordinal, {fontSize: size - 1}]}>{i + 1}</Text>
            {i < steps.length - 1 && <View style={styles.line} />}
          </View>
          <View style={styles.body}>
            <Text selectable style={[styles.tool, {fontSize: size - 1}]}>{step.title || 'tool'}</Text>
            {!!step.detail && <Text selectable style={[styles.detail, {fontSize: size}]}>{step.detail}</Text>}
          </View>
        </View>)}
      </View>}
    </View>
  );
}

const styles = StyleSheet.create({
  // Chat has its own dark reading canvas in either device appearance.
  card: {marginLeft: 35, borderWidth: StyleSheet.hairlineWidth, borderColor: '#36363B', borderRadius: 12, backgroundColor: '#151518', overflow: 'hidden'},
  header: {minHeight: 44, paddingHorizontal: 12, paddingVertical: 10, gap: 5},
  headingRow: {flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap'},
  heading: {color: '#C7C7CE', fontWeight: '600'},
  count: {color: '#A4A4AD', flexShrink: 1},
  chevron: {color: '#A4A4AD', marginLeft: 'auto', fontSize: 16},
  summary: {color: '#A4A4AD', fontFamily: 'Menlo'},
  list: {paddingHorizontal: 12, paddingTop: 4, borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: '#36363B'},
  row: {flexDirection: 'row', gap: 10, minWidth: 0},
  rail: {width: 20, alignItems: 'center'},
  ordinal: {color: '#A4A4AD', fontVariant: ['tabular-nums'], paddingTop: 7},
  line: {width: StyleSheet.hairlineWidth, flex: 1, backgroundColor: '#36363B', marginTop: 5},
  body: {flex: 1, minWidth: 0, paddingTop: 6, paddingBottom: 12, gap: 4},
  tool: {color: '#A4A4AD', fontFamily: 'Menlo', flexShrink: 1},
  detail: {color: '#E2E2E7', fontFamily: 'Menlo'},
});
