import React, {useState} from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {TranscriptStep} from '../api/client';
import {ActivityIcon} from './ActivityIcon';
import {activityLabel, activityOperation, activityPreview, activitySummary} from './activity';
import {CheckIcon, Chevron, CopyIcon} from './Icons';

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
  const count = lang === 'zh' ? `${steps.length} 次调用` : `${steps.length} call${steps.length === 1 ? '' : 's'}`;
  const size = Math.max(12, Math.min(14, fontSize - 1));
  return (
    <View style={styles.group}>
      <TouchableOpacity
        testID="tool-steps-toggle"
        accessibilityRole="button"
        accessibilityLabel={`${heading}, ${count}`}
        accessibilityHint={lang === 'zh' ? (open ? '收起全部工具调用' : '展开工具调用') : (open ? 'Hide all tool calls' : 'Show tool calls')}
        accessibilityState={{expanded: open}}
        activeOpacity={0.7}
        onPress={onToggle}
        style={styles.header}>
        <Chevron open={open} size={12} color="#A4A4AD" />
        <Text style={[styles.heading, {fontSize: size}]} numberOfLines={1}>{open ? (lang === 'zh' ? '工具调用' : 'Tool calls') : heading} · {count}</Text>
      </TouchableOpacity>
      {open && <View testID="tool-steps-list" style={styles.list}>
        {steps.map((step, i) => {
          const expanded = selected === i;
          const operation = activityOperation(step);
          const label = activityLabel(operation, lang);
          const preview = activityPreview(step);
          const showPreview = preview.toLowerCase() !== (step.title || '').toLowerCase() && preview.toLowerCase() !== label.toLowerCase();
          const hasDetails = !!(step.input || step.workdir || step.details_truncated || step.detail && step.detail.trim().toLowerCase() !== (step.title || '').trim().toLowerCase());
          const rowContent = <>
            <ActivityIcon operation={operation} size={16} />
            <Text style={[styles.rowText, {fontSize: size}]} numberOfLines={2}>
              {operation === 'tool' ? step.title || label : label}
              {showPreview && <Text style={styles.preview}> · {preview}</Text>}
            </Text>
            {hasDetails && <Chevron open={expanded} size={12} color="#898994" />}
          </>;
          return <View key={i} testID="tool-step-row" style={styles.row}>
            {hasDetails ? <TouchableOpacity
              testID={`tool-step-toggle-${i}`}
              accessibilityRole="button"
              accessibilityLabel={`${i + 1}. ${label}: ${activityPreview(step)}`}
              accessibilityState={{expanded}}
              accessibilityHint={lang === 'zh' ? (expanded ? '收起详情' : '查看命令或参数详情') : (expanded ? 'Hide details' : 'Inspect command or input')}
              onPress={() => setSelected(expanded ? null : i)}
              activeOpacity={0.7}
              style={styles.rowButton}>
              {rowContent}
            </TouchableOpacity> : <View testID={`tool-step-static-${i}`} style={styles.staticRow} accessible accessibilityLabel={operation === 'tool' ? step.title || label : label}>{rowContent}</View>}
            {hasDetails && expanded && <ToolInput step={step} lang={lang} size={size} />}
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
  const [copied, setCopied] = useState<string | null>(null);
  const done = copied === input;
  return <View testID="tool-step-details" style={styles.inspector}>
    <View style={styles.inspectorHeader}>
      <Text selectable style={[styles.tool, {fontSize: size - 2}]}>{step.title || 'tool'}{step.input_kind === 'arguments' ? ' · argv (JSON)' : step.input_kind === 'parameters' ? (lang === 'zh' ? ' · 参数' : ' · Parameters') : ''}</Text>
      {!!input && <TouchableOpacity testID="tool-input-copy" accessibilityRole="button"
        accessibilityLabel={done ? (lang === 'zh' ? '已复制' : 'Copied') : (lang === 'zh' ? '复制显示内容' : 'Copy displayed input')}
        onPress={() => {Clipboard.setString(input); setCopied(input);}} style={styles.copy}>
        {done ? <CheckIcon size={15} color="#A4A4AD" /> : <CopyIcon size={15} color="#A4A4AD" />}
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
    {!!input && !step.input && !step.details_truncated && <Text style={styles.note}>{lang === 'zh' ? '仅参数摘要' : 'Argument summary only'}</Text>}
    {step.details_truncated && <Text style={styles.note}>{lang === 'zh' ? '部分详情超过显示限制' : 'Some details exceed the display limit'}</Text>}
    {viewport > 0 && content > viewport + 1 && <Text testID="tool-input-scroll-hint" style={styles.note}>{lang === 'zh' ? '左右滑动查看完整行' : 'Scroll sideways to inspect the line'}</Text>}
  </View>;
}

const styles = StyleSheet.create({
  // Chat has its own dark reading canvas in either device appearance.
  group: {marginLeft: 52},
  header: {minHeight: 44, paddingVertical: 6, gap: 8, flexDirection: 'row', alignItems: 'center'},
  heading: {color: '#A4A4AD', flexShrink: 1},
  list: {paddingLeft: 6, marginBottom: 4, borderLeftWidth: StyleSheet.hairlineWidth, borderLeftColor: '#343439'},
  row: {paddingLeft: 8},
  rowButton: {flexDirection: 'row', gap: 8, alignItems: 'center', paddingVertical: 6, minHeight: 44},
  staticRow: {flexDirection: 'row', gap: 8, alignItems: 'center', paddingVertical: 6, minHeight: 32},
  rowText: {color: '#A4A4AD', flex: 1, minWidth: 0},
  preview: {color: '#A4A4AD', fontFamily: 'Menlo'},
  inspector: {marginLeft: 24, marginBottom: 8, overflow: 'hidden'},
  inspectorHeader: {flexDirection: 'row', alignItems: 'center', gap: 8},
  tool: {color: '#A4A4AD', fontFamily: 'Menlo', flex: 1, paddingVertical: 6},
  copy: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 10},
  workdir: {color: '#A4A4AD', fontFamily: 'Menlo', paddingBottom: 6},
  codeHeight: {maxHeight: 220},
  codeContent: {paddingBottom: 6},
  code: {color: '#B8B8C2', fontFamily: 'Menlo', flexShrink: 0},
  note: {fontSize: 12, color: '#A4A4AD', paddingBottom: 6, paddingTop: 4},
});
