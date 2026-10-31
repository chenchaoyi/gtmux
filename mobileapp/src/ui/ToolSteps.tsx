import React, { useState } from 'react';
import {
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import { GtmuxClient, TranscriptStep } from '../api/client';
import { ActivityIcon } from './ActivityIcon';
import {
  activityLabel,
  activityOperation,
  activityPreview,
  activitySummary,
} from './activity';
import { CheckIcon, Chevron, CopyIcon, ExpandIcon } from './Icons';
import { CodeText } from './CodeText';
import { codeLabel, toolCodeHint } from './codeReading';
import { CodeReaderModal } from './CodeReaderModal';
import { useToolInput } from './useToolInput';

/** A call is evidence of an action, not evidence that it succeeded. */
export function ToolSteps({
  steps,
  client,
  session,
  home,
  open,
  onToggle,
  lang,
  fontSize,
}: {
  steps: TranscriptStep[];
  client?: GtmuxClient;
  session?: string;
  home?: string;
  open: boolean;
  onToggle: () => void;
  lang: 'en' | 'zh';
  fontSize: number;
}) {
  const [selected, setSelected] = useState<number | null>(null);
  const heading = activitySummary(steps, lang);
  const count =
    lang === 'zh'
      ? `${steps.length} 次调用`
      : `${steps.length} call${steps.length === 1 ? '' : 's'}`;
  const size = Math.max(12, Math.min(14, fontSize - 1));
  return (
    <View style={styles.group}>
      <TouchableOpacity
        testID="tool-steps-toggle"
        accessibilityRole="button"
        accessibilityLabel={`${heading}, ${count}`}
        accessibilityHint={
          lang === 'zh'
            ? open
              ? '收起全部工具调用'
              : '展开工具调用'
            : open
            ? 'Hide all tool calls'
            : 'Show tool calls'
        }
        accessibilityState={{ expanded: open }}
        activeOpacity={0.7}
        onPress={onToggle}
        style={styles.header}
      >
        <Chevron open={open} size={12} color="#A4A4AD" />
        <Text style={[styles.heading, { fontSize: size }]} numberOfLines={1}>
          {open ? (lang === 'zh' ? '工具调用' : 'Tool calls') : heading} ·{' '}
          {steps.length}
        </Text>
      </TouchableOpacity>
      {open && (
        <View testID="tool-steps-list" style={styles.list}>
          {steps.map((step, i) => {
            const expanded = selected === i;
            const operation = activityOperation(step);
            const label = activityLabel(operation, lang);
            const preview = activityPreview(step, lang);
            const showPreview =
              preview.toLowerCase() !== (step.title || '').toLowerCase() &&
              preview.toLowerCase() !== label.toLowerCase();
            const hasDetails = !!(
              step.input ||
              step.workdir ||
              step.details_truncated ||
              (step.detail &&
                step.detail.trim().toLowerCase() !==
                  (step.title || '').trim().toLowerCase())
            );
            const rowContent = (
              <>
                <ActivityIcon operation={operation} size={16} />
                <Text
                  style={[styles.rowText, { fontSize: size }]}
                  numberOfLines={2}
                >
                  {operation === 'tool' ? step.title || label : label}
                  {showPreview && (
                    <Text
                      style={[
                        styles.preview,
                        operation !== 'agent' &&
                          operation !== 'wait' &&
                          styles.codePreview,
                      ]}
                    >
                      {' '}
                      · {preview}
                    </Text>
                  )}
                </Text>
                {hasDetails && (
                  <Chevron open={expanded} size={12} color="#898994" />
                )}
              </>
            );
            return (
              <View key={i} testID="tool-step-row" style={styles.row}>
                {hasDetails ? (
                  <TouchableOpacity
                    testID={`tool-step-toggle-${i}`}
                    accessibilityRole="button"
                    accessibilityLabel={`${i + 1}. ${label}${
                      showPreview ? `: ${preview}` : ''
                    }`}
                    accessibilityState={{ expanded }}
                    accessibilityHint={
                      lang === 'zh'
                        ? expanded
                          ? '收起详情'
                          : '查看命令或参数详情'
                        : expanded
                        ? 'Hide details'
                        : 'Inspect command or input'
                    }
                    onPress={() => setSelected(expanded ? null : i)}
                    activeOpacity={0.7}
                    style={styles.rowButton}
                  >
                    {rowContent}
                  </TouchableOpacity>
                ) : (
                  <View
                    testID={`tool-step-static-${i}`}
                    style={styles.staticRow}
                    accessible
                    accessibilityLabel={
                      operation === 'tool' ? step.title || label : label
                    }
                  >
                    {rowContent}
                  </View>
                )}
                {hasDetails && expanded && (
                  <ToolInput
                    step={step}
                    client={client}
                    session={session}
                    home={home}
                    lang={lang}
                    size={size}
                    readerFontSize={Math.max(12, fontSize - 1)}
                  />
                )}
              </View>
            );
          })}
        </View>
      )}
    </View>
  );
}

function ToolInput({
  step: original,
  client,
  session,
  home,
  lang,
  size,
  readerFontSize,
}: {
  step: TranscriptStep;
  client?: GtmuxClient;
  session?: string;
  home?: string;
  lang: 'en' | 'zh';
  size: number;
  readerFontSize: number;
}) {
  const {
    step: loaded,
    loading,
    failed,
    retry,
  } = useToolInput(original, client, session, home);
  const step = loaded!;
  const { fontScale } = useWindowDimensions();
  const input = step.input || step.detail || '';
  const [reader, setReader] = useState<{
    input: string;
    title: string;
    hint: string;
  } | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const done = copied === input;
  const hint = toolCodeHint(step.title, step.input_kind, step.operation);
  const provenance = loading
    ? lang === 'zh'
      ? '读取完整输入…'
      : 'Loading full input…'
    : failed
    ? lang === 'zh'
      ? '读取失败'
      : 'Could not load'
    : step.details_truncated
    ? lang === 'zh'
      ? '部分'
      : 'Partial'
    : !step.input
    ? lang === 'zh'
      ? '摘要'
      : 'Summary'
    : '';
  return (
    <View testID="tool-step-details" style={styles.inspector}>
      {failed && (
        <TouchableOpacity
          testID="tool-input-retry"
          onPress={retry}
          accessibilityRole="button"
          style={{ minHeight: 44, justifyContent: 'center' }}
        >
          <Text style={{ color: '#86BFFC' }}>
            {lang === 'zh' ? '重试读取完整输入' : 'Retry full input'}
          </Text>
        </TouchableOpacity>
      )}
      <View style={styles.inspectorHeader}>
        <Text
          selectable
          style={[
            styles.tool,
            { fontSize: size - 2 },
            fontScale > 1.3 && styles.toolStacked,
          ]}
        >
          {step.title || 'tool'}
          {step.input_kind === 'arguments'
            ? ' · JSON'
            : step.input_kind === 'parameters'
            ? lang === 'zh'
              ? ' · 参数'
              : ' · Parameters'
            : ''}
        </Text>
        <View style={styles.inspectorControls}>
          {!!provenance && (
            <Text style={[styles.provenance, { fontSize: size - 2 }]}>
              {provenance}
            </Text>
          )}
          {!!input && (
            <TouchableOpacity
              testID="tool-input-open"
              disabled={loading || !!step.details_truncated}
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh' ? '打开代码阅读器' : 'Open code reader'
              }
              onPress={() =>
                setReader({
                  input,
                  title: step.title || 'tool',
                  hint: hint || '',
                })
              }
              style={styles.copy}
            >
              <ExpandIcon size={20} color="#A4A4AD" />
            </TouchableOpacity>
          )}
          {!!input && (
            <TouchableOpacity
              testID="tool-input-copy"
              disabled={loading || !!step.details_truncated}
              accessibilityRole="button"
              accessibilityLabel={
                done
                  ? lang === 'zh'
                    ? '已复制'
                    : 'Copied'
                  : lang === 'zh'
                  ? '复制调用输入'
                  : 'Copy recorded input'
              }
              onPress={() => {
                Clipboard.setString(input);
                setCopied(input);
              }}
              style={styles.copy}
            >
              {done ? (
                <CheckIcon size={20} color="#A4A4AD" />
              ) : (
                <CopyIcon size={20} color="#A4A4AD" />
              )}
            </TouchableOpacity>
          )}
        </View>
      </View>
      {!!step.workdir && (
        <Text selectable style={[styles.workdir, { fontSize: size - 2 }]}>
          {lang === 'zh' ? '目录' : 'Directory'} · {step.workdir}
        </Text>
      )}
      {!!input && (
        <Text style={{ color: '#A4A4AD', fontSize: 11, marginBottom: 6 }}>
          {codeLabel(input, hint)}
        </Text>
      )}
      {!!input && (
        <ScrollView
          testID="tool-input-vertical"
          nestedScrollEnabled
          style={styles.codeHeight}
        >
          <CodeText
            testID="tool-input-text"
            formatted
            source={input}
            hint={hint}
            lang={lang}
            fontSize={size}
          />
        </ScrollView>
      )}
      {reader && (
        <CodeReaderModal
          source={reader.input}
          title={reader.title}
          hint={reader.hint}
          lang={lang}
          fontSize={readerFontSize}
          onClose={() => setReader(null)}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  // Chat has its own dark reading canvas in either device appearance.
  group: { marginLeft: 52 },
  header: {
    minHeight: 44,
    paddingVertical: 6,
    gap: 8,
    flexDirection: 'row',
    alignItems: 'center',
  },
  heading: { color: '#A4A4AD', flexShrink: 1 },
  list: {
    paddingLeft: 6,
    marginBottom: 4,
    borderLeftWidth: StyleSheet.hairlineWidth,
    borderLeftColor: '#343439',
  },
  row: { paddingLeft: 8 },
  rowButton: {
    flexDirection: 'row',
    gap: 8,
    alignItems: 'center',
    paddingVertical: 6,
    minHeight: 44,
  },
  staticRow: {
    flexDirection: 'row',
    gap: 8,
    alignItems: 'center',
    paddingVertical: 6,
    minHeight: 32,
  },
  rowText: { color: '#A4A4AD', flex: 1, minWidth: 0 },
  preview: { color: '#A4A4AD' },
  codePreview: { fontFamily: 'Menlo' },
  inspector: {
    marginBottom: 8,
    overflow: 'hidden',
    backgroundColor: '#161618',
    borderRadius: 6,
    paddingHorizontal: 10,
  },
  inspectorHeader: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: 8,
  },
  provenance: { color: '#A4A4AD', flexShrink: 1, minWidth: 0 },
  inspectorControls: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    marginLeft: 'auto',
    maxWidth: '100%',
    flexShrink: 1,
  },
  tool: {
    color: '#A4A4AD',
    fontFamily: 'Menlo',
    flex: 1,
    minWidth: 60,
    paddingVertical: 6,
  },
  toolStacked: { flexBasis: '100%', flexGrow: 0 },
  copy: {
    minHeight: 44,
    minWidth: 44,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 10,
  },
  workdir: { color: '#A4A4AD', fontFamily: 'Menlo', paddingBottom: 6 },
  codeHeight: { maxHeight: 220 },
});
