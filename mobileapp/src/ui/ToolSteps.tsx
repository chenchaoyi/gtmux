import React, {useRef, useState} from 'react';
import {Alert, ScrollView, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {TranscriptStep} from '../api/client';
import {ActivityIcon} from './ActivityIcon';
import {activityLabel, activityOperation, activityPreview, activitySummary} from './activity';
import {CheckIcon, Chevron, CopyIcon, InfoIcon} from './Icons';
import {HorizontalScrollHandle} from './HorizontalScrollHandle';

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
        <Text style={[styles.heading, {fontSize: size}]} numberOfLines={1}>{open ? (lang === 'zh' ? '工具调用' : 'Tool calls') : heading} · {steps.length}</Text>
      </TouchableOpacity>
      {open && <View testID="tool-steps-list" style={styles.list}>
        {steps.map((step, i) => {
          const expanded = selected === i;
          const operation = activityOperation(step);
          const label = activityLabel(operation, lang);
          const preview = activityPreview(step, lang);
          const showPreview = preview.toLowerCase() !== (step.title || '').toLowerCase() && preview.toLowerCase() !== label.toLowerCase();
          const hasDetails = !!(step.input || step.workdir || step.details_truncated || step.detail && step.detail.trim().toLowerCase() !== (step.title || '').trim().toLowerCase());
          const rowContent = <>
            <ActivityIcon operation={operation} size={16} />
            <Text style={[styles.rowText, {fontSize: size}]} numberOfLines={2}>
              {operation === 'tool' ? step.title || label : label}
              {showPreview && <Text style={[styles.preview, operation !== 'agent' && operation !== 'wait' && styles.codePreview]}> · {preview}</Text>}
            </Text>
            {hasDetails && <Chevron open={expanded} size={12} color="#898994" />}
          </>;
          return <View key={i} testID="tool-step-row" style={styles.row}>
            {hasDetails ? <TouchableOpacity
              testID={`tool-step-toggle-${i}`}
              accessibilityRole="button"
              accessibilityLabel={`${i + 1}. ${label}${showPreview ? `: ${preview}` : ''}`}
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
  const {fontScale} = useWindowDimensions();
  const input = step.input || step.detail || '';
  const [viewport, setViewport] = useState(0);
  const [content, setContent] = useState(0);
  const [offset, setOffset] = useState(0);
  const horizontal = useRef<ScrollView>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const done = copied === input;
  const partial = step.details_truncated;
  const summary = !!input && !step.input;
  const noticeTitle = partial ? (lang === 'zh' ? '部分调用详情' : 'Partial tool details') : (lang === 'zh' ? '参数摘要' : 'Argument summary');
  const scrollTo = (x: number) => {
    const next = Math.max(0, Math.min(Math.max(0, content - viewport), x));
    horizontal.current?.scrollTo({x: next, animated: false});
    setOffset(next);
  };
  return <View testID="tool-step-details" style={styles.inspector}>
    <View style={styles.inspectorHeader}>
      <Text selectable style={[styles.tool, {fontSize: size - 2}, fontScale > 1.3 && styles.toolStacked]}>{step.title || 'tool'}{step.input_kind === 'arguments' ? ' · argv (JSON)' : step.input_kind === 'parameters' ? (lang === 'zh' ? ' · 参数' : ' · Parameters') : ''}</Text>
      <View style={styles.inspectorControls}>
      {viewport > 0 && content > viewport + 1 && <HorizontalScrollHandle testID="tool-input-scroll-control" viewport={viewport} content={content} offset={offset} onScrollTo={scrollTo} lang={lang} />}
      {(partial || summary) && <TouchableOpacity testID="tool-input-info" accessibilityRole="button" accessibilityLabel={noticeTitle}
        onPress={() => Alert.alert(noticeTitle, partial
          ? (lang === 'zh' ? '当前只显示部分调用详情。复制得到的是这里显示的内容。' : 'Only part of this call’s details is available here. Copy includes the displayed text.')
          : (lang === 'zh' ? '当前会话只提供参数摘要，未记录原始输入。' : 'This session provides an argument summary rather than the original input.'))}
        style={styles.copy}><InfoIcon size={20} color="#A4A4AD" /></TouchableOpacity>}
      {!!input && <TouchableOpacity testID="tool-input-copy" accessibilityRole="button"
        accessibilityLabel={done ? (lang === 'zh' ? '已复制' : 'Copied') : (lang === 'zh' ? '复制显示内容' : 'Copy displayed input')}
        onPress={() => {Clipboard.setString(input); setCopied(input);}} style={styles.copy}>
        {done ? <CheckIcon size={20} color="#A4A4AD" /> : <CopyIcon size={20} color="#A4A4AD" />}
      </TouchableOpacity>}
      </View>
    </View>
    {!!step.workdir && <Text selectable style={[styles.workdir, {fontSize: size - 2}]}>{lang === 'zh' ? '目录' : 'Directory'} · {step.workdir}</Text>}
    {!!input && <ScrollView testID="tool-input-vertical" nestedScrollEnabled style={styles.codeHeight}>
      <ScrollView ref={horizontal} testID="tool-input-horizontal" horizontal nestedScrollEnabled
        onLayout={e => setViewport(e.nativeEvent.layout.width)}
        onContentSizeChange={width => setContent(width)}
        onScroll={event => setOffset(event.nativeEvent.contentOffset.x)} scrollEventThrottle={16}
        showsHorizontalScrollIndicator={false} contentContainerStyle={styles.codeContent}>
        <Text testID="tool-input-text" selectable style={[styles.code, {fontSize: size, lineHeight: Math.ceil(size * 1.55)}]}>{input}</Text>
      </ScrollView>
    </ScrollView>}
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
  preview: {color: '#A4A4AD'},
  codePreview: {fontFamily: 'Menlo'},
  inspector: {marginLeft: 24, marginBottom: 8, overflow: 'hidden'},
  inspectorHeader: {flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 8},
  inspectorControls: {flexDirection: 'row', alignItems: 'center', gap: 4, marginLeft: 'auto'},
  tool: {color: '#A4A4AD', fontFamily: 'Menlo', flex: 1, minWidth: 60, paddingVertical: 6},
  toolStacked: {flexBasis: '100%', flexGrow: 0},
  copy: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 10},
  workdir: {color: '#A4A4AD', fontFamily: 'Menlo', paddingBottom: 6},
  codeHeight: {maxHeight: 220},
  codeContent: {paddingBottom: 6},
  code: {color: '#B8B8C2', fontFamily: 'Menlo', flexShrink: 0},
});
