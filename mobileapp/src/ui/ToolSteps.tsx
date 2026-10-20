import React, {useState} from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {TranscriptStep} from '../api/client';
import {ActivityIcon} from './ActivityIcon';
import {activityLabel, activityOperation, activityPreview, activitySummary} from './activity';
import {Chevron} from './Icons';

/** A call is evidence of an action, not evidence that it succeeded. */
export function ToolSteps({steps, open, onToggle, lang, fontSize}: {
  steps: TranscriptStep[];
  open: boolean;
  onToggle: () => void;
  lang: 'en' | 'zh';
  fontSize: number;
}) {
  const [selected, setSelected] = useState<number | null>(null);
  const heading = activitySummary(steps, lang);
  const firstOperation = steps.length ? activityOperation(steps[0]) : 'tool';
  const groupOperation = steps.every(step => activityOperation(step) === firstOperation) ? firstOperation : 'tool';
  const count = lang === 'zh' ? `${steps.length} 次调用` : `${steps.length} call${steps.length === 1 ? '' : 's'}`;
  const size = Math.max(12, fontSize - 1);
  return (
    <View style={styles.card}>
      <TouchableOpacity
        testID="tool-steps-toggle"
        accessibilityRole="button"
        accessibilityLabel={`${heading}, ${count}`}
        accessibilityHint={lang === 'zh' ? (open ? '收起全部工具调用' : '展开工具调用') : (open ? 'Hide all tool calls' : 'Show tool calls')}
        accessibilityState={{expanded: open}}
        activeOpacity={0.7}
        onPress={onToggle}
        style={styles.header}>
        <ActivityIcon operation={groupOperation} />
        <Text style={[styles.heading, {fontSize: size}]} numberOfLines={1}>{heading}</Text>
        <Text style={[styles.count, {fontSize: size - 1}]}>{steps.length}</Text>
        <Chevron open={open} size={15} color="#A4A4AD" />
      </TouchableOpacity>
      {open && <View testID="tool-steps-list" style={styles.list}>
        {steps.map((step, i) => {
          const expanded = selected === i;
          const operation = activityOperation(step);
          const label = activityLabel(operation, lang);
          return <View key={i} testID="tool-step-row" style={styles.row}>
            <TouchableOpacity
              testID={`tool-step-toggle-${i}`}
              accessibilityRole="button"
              accessibilityLabel={`${i + 1}. ${label}: ${activityPreview(step)}`}
              accessibilityState={{expanded}}
              accessibilityHint={lang === 'zh' ? (expanded ? '收起详情' : '查看命令或参数详情') : (expanded ? 'Hide details' : 'Inspect command or input')}
              onPress={() => setSelected(expanded ? null : i)}
              activeOpacity={0.7}
              style={styles.rowButton}>
              <ActivityIcon operation={operation} />
              <View style={styles.rowBody}>
                <Text style={[styles.label, {fontSize: size - 1}]}>{operation === 'tool' ? step.title || label : label}</Text>
                <Text style={[styles.preview, {fontSize: size}]} numberOfLines={1}>{activityPreview(step)}</Text>
              </View>
              <Chevron open={expanded} size={13} color="#898994" />
            </TouchableOpacity>
            {expanded && <ToolInput step={step} lang={lang} size={size} />}
          </View>;
        })}
      </View>}
    </View>
  );
}

function ToolInput({step, lang, size}: {step: TranscriptStep; lang: 'en' | 'zh'; size: number}) {
  const input = step.input || step.detail || '';
  const [viewport, setViewport] = useState(0);
  const [content, setContent] = useState(0);
  const [copied, setCopied] = useState(false);
  return <View testID="tool-step-details" style={styles.inspector}>
    <View style={styles.inspectorHeader}>
      <Text selectable style={[styles.tool, {fontSize: size - 2}]}>{step.title || 'tool'}{step.input_kind === 'arguments' ? ' · argv (JSON)' : ''}</Text>
      {!!input && <TouchableOpacity testID="tool-input-copy" accessibilityRole="button"
        accessibilityLabel={lang === 'zh' ? '复制显示内容' : 'Copy displayed input'}
        onPress={() => {Clipboard.setString(input); setCopied(true);}} style={styles.copy}>
        <Text style={[styles.label, {fontSize: size - 2}]}>{lang === 'zh' ? (copied ? '已复制' : '复制') : (copied ? 'Copied' : 'Copy')}</Text>
      </TouchableOpacity>}
    </View>
    {!!step.workdir && <Text selectable style={[styles.workdir, {fontSize: size - 2}]}>{lang === 'zh' ? '目录' : 'Directory'} · {step.workdir}</Text>}
    {!!input && <ScrollView testID="tool-input-vertical" nestedScrollEnabled style={styles.codeHeight}>
      <ScrollView testID="tool-input-horizontal" horizontal nestedScrollEnabled
        onLayout={e => setViewport(e.nativeEvent.layout.width)}
        onContentSizeChange={width => setContent(width)}
        showsHorizontalScrollIndicator contentContainerStyle={styles.codeContent}>
        <Text testID="tool-input-text" selectable style={[styles.code, {fontSize: size, lineHeight: Math.ceil(size * 1.55)}]}>{input}</Text>
      </ScrollView>
    </ScrollView>}
    {!input && <Text style={styles.note}>{lang === 'zh' ? '日志未提供参数详情' : 'No input details recorded'}</Text>}
    {!!input && !step.input && !step.details_truncated && <Text style={styles.note}>{lang === 'zh' ? '仅参数摘要' : 'Argument summary only'}</Text>}
    {step.details_truncated && <Text style={styles.note}>{lang === 'zh' ? '部分详情超过显示限制' : 'Some details exceed the display limit'}</Text>}
    {viewport > 0 && content > viewport + 1 && <Text testID="tool-input-scroll-hint" style={styles.note}>{lang === 'zh' ? '左右滑动查看完整行' : 'Scroll sideways to inspect the line'}</Text>}
  </View>;
}

const styles = StyleSheet.create({
  // Chat has its own dark reading canvas in either device appearance.
  card: {marginLeft: 35, borderWidth: StyleSheet.hairlineWidth, borderColor: '#343439', borderRadius: 12, backgroundColor: '#151518', overflow: 'hidden'},
  header: {minHeight: 46, paddingHorizontal: 12, paddingVertical: 10, gap: 9, flexDirection: 'row', alignItems: 'center'},
  heading: {color: '#C7C7CE', fontWeight: '500', flex: 1, minWidth: 0},
  count: {color: '#A4A4AD', fontVariant: ['tabular-nums']},
  list: {paddingHorizontal: 12, borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: '#343439'},
  row: {borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: '#28282E'},
  rowButton: {flexDirection: 'row', gap: 10, alignItems: 'center', paddingVertical: 11, minHeight: 54},
  rowBody: {flex: 1, minWidth: 0, gap: 4},
  label: {color: '#A4A4AD'},
  preview: {color: '#E2E2E7', fontFamily: 'Menlo'},
  inspector: {marginLeft: 28, marginBottom: 12, borderRadius: 8, backgroundColor: '#0C0C0F', overflow: 'hidden'},
  inspectorHeader: {flexDirection: 'row', alignItems: 'center', paddingLeft: 10, gap: 8},
  tool: {color: '#A4A4AD', fontFamily: 'Menlo', flex: 1, paddingVertical: 10},
  copy: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 10},
  workdir: {color: '#A4A4AD', fontFamily: 'Menlo', paddingHorizontal: 10, paddingBottom: 8},
  codeHeight: {maxHeight: 220},
  codeContent: {paddingHorizontal: 10, paddingBottom: 10},
  code: {color: '#E2E2E7', fontFamily: 'Menlo', flexShrink: 0},
  note: {fontSize: 12, color: '#A4A4AD', paddingHorizontal: 10, paddingBottom: 10, paddingTop: 4},
});
