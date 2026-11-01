import React, { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, TouchableOpacity, View } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import { CodeText } from './CodeText';
import { ChromeText } from './ChromeText';
import { codeParts } from './codeFormatting';
import { codeLabel } from './codeReading';
import { CheckIcon, CopyIcon, ExpandIcon } from './Icons';
import { CopyFeedbackContext } from './CopyableText';
import { RangeSourceContext } from './RangeText';

export type CodePanelProps = {
  source: string;
  hint?: string;
  lang?: 'en' | 'zh';
  fontSize?: number;
  dark?: boolean;
  initialFormatted?: boolean;
  initialPartID?: string;
  allowFormat?: boolean;
  allowEmbedded?: boolean;
  disabled?: boolean;
  maxHeight?: number;
  testIDPrefix?: string;
  textTestID?: string;
  selectable?: boolean;
  selectionColor?: string;
  showCopy?: boolean;
};

/** One reading surface for Chat, Trace and the full-screen code reader. */
export function CodePanel({
  source,
  hint,
  lang = 'en',
  fontSize = 13,
  dark = true,
  initialFormatted = true,
  initialPartID,
  allowFormat = true,
  allowEmbedded = true,
  disabled = false,
  maxHeight,
  testIDPrefix = 'code',
  textTestID,
  selectable = true,
  selectionColor,
  showCopy = true,
  onExpand,
}: CodePanelProps & {
  onExpand?: (state: { partID: string; formatted: boolean }) => void;
}) {
  const parts = useMemo(
    () =>
      allowEmbedded
        ? codeParts(source, hint)
        : [{ id: 'original', source, hint: hint || '', path: '' }],
    [source, hint, allowEmbedded],
  );
  const [partID, setPartID] = useState<string | null>(initialPartID || null);
  const part = parts.find(p => p.id === partID) || parts[0];
  const canFormat =
    allowFormat &&
    [
      'JavaScript',
      'TypeScript',
      'JSON',
      'HTML',
      'CSS',
      'YAML',
      'Python',
    ].includes(codeLabel(part.source, part.hint));
  const [formatted, setFormatted] = useState(initialFormatted);
  const [copied, setCopied] = useState<string | null>(null);
  const fg = dark ? '#D8DEE9' : '#24292F';
  const dim = dark ? '#A4A4AD' : '#626975';
  const accent = dark ? '#86BFFC' : '#006F82';
  const border = dark ? '#343439' : '#DADDE3';
  const code = (
    <RangeSourceContext.Provider value={part.source}>
      <CopyFeedbackContext.Provider value>
        <CodeText
          source={part.source}
          hint={part.hint}
          formatted={canFormat && formatted}
          dark={dark}
          fontSize={fontSize}
          lang={lang}
          testID={textTestID}
          selectable={selectable}
          selectionColor={selectionColor}
        />
      </CopyFeedbackContext.Provider>
    </RangeSourceContext.Provider>
  );
  return (
    <View
      testID={testIDPrefix + '-panel'}
      style={[
        styles.root,
        { backgroundColor: dark ? '#1C1C21' : '#F4F4F8', borderColor: border },
      ]}
    >
      <View style={[styles.toolbar, { borderBottomColor: border }]}>
        <ChromeText numberOfLines={1} style={[styles.language, { color: dim }]}>
          {codeLabel(part.source, part.hint)}
        </ChromeText>
        <View style={styles.actions}>
          {canFormat && (
            <TouchableOpacity
              testID={testIDPrefix + '-format'}
              accessibilityRole="button"
              accessibilityState={{ selected: formatted }}
              accessibilityLabel={
                lang === 'zh'
                  ? formatted
                    ? '查看原文'
                    : '格式化代码'
                  : formatted
                  ? 'Show original'
                  : 'Format code'
              }
              onPress={() => setFormatted(v => !v)}
              style={styles.button}
            >
              <ChromeText
                style={{
                  fontSize: 18,
                  fontWeight: '600',
                  color: formatted ? accent : dim,
                }}
              >
                {'{ }'}
              </ChromeText>
            </TouchableOpacity>
          )}
          {onExpand && (
            <TouchableOpacity
              testID={testIDPrefix + '-open'}
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh' ? '打开代码阅读器' : 'Open code reader'
              }
              accessibilityState={{ disabled }}
              disabled={disabled}
              onPress={() => onExpand({ partID: part.id, formatted })}
              style={styles.button}
            >
              <ExpandIcon size={20} color={disabled ? dim : fg} />
            </TouchableOpacity>
          )}
          {showCopy && (
            <TouchableOpacity
              testID={testIDPrefix + '-copy'}
              accessibilityRole="button"
              accessibilityState={{ disabled }}
              disabled={disabled}
              accessibilityLabel={
                lang === 'zh'
                  ? copied === source
                    ? '已复制'
                    : '复制调用原文'
                  : copied === source
                  ? 'Copied'
                  : 'Copy recorded input'
              }
              onPress={() => {
                Clipboard.setString(source);
                setCopied(source);
              }}
              style={styles.button}
            >
              {copied === source ? (
                <CheckIcon size={20} color={dim} />
              ) : (
                <CopyIcon size={20} color={disabled ? dim : fg} />
              )}
            </TouchableOpacity>
          )}
        </View>
      </View>
      {parts.length > 1 && (
        <View accessibilityRole="tablist" style={styles.parts}>
          {parts.map(p => (
            <TouchableOpacity
              key={p.id}
              testID={testIDPrefix + '-part-' + p.id}
              accessibilityRole="tab"
              accessibilityState={{ selected: part.id === p.id }}
              accessibilityLabel={
                p.id === 'original'
                  ? lang === 'zh'
                    ? '完整调用原文'
                    : 'Complete recorded call'
                  : codeLabel(p.source, p.hint) + ' · ' + p.path
              }
              onPress={() => setPartID(p.id)}
              style={[
                styles.part,
                { borderColor: part.id === p.id ? accent : border },
              ]}
            >
              <ChromeText
                style={{ color: part.id === p.id ? accent : dim, fontSize: 12 }}
              >
                {p.id === 'original'
                  ? lang === 'zh'
                    ? '完整调用'
                    : 'Full call'
                  : codeLabel(p.source, p.hint) +
                    ' · ' +
                    (p.path || (lang === 'zh' ? '脚本' : 'Script'))}
              </ChromeText>
            </TouchableOpacity>
          ))}
        </View>
      )}
      {maxHeight ? (
        <ScrollView
          testID={testIDPrefix + '-vertical'}
          nestedScrollEnabled
          style={{ maxHeight }}
          contentContainerStyle={styles.content}
        >
          {code}
        </ScrollView>
      ) : (
        <View style={styles.content}>{code}</View>
      )}
    </View>
  );
}
const styles = StyleSheet.create({
  root: {
    minWidth: 0,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    overflow: 'hidden',
  },
  toolbar: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    paddingLeft: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  language: {
    fontSize: 12,
    fontWeight: '600',
    flexGrow: 1,
    flexShrink: 1,
    minWidth: 60,
    paddingVertical: 8,
  },
  actions: { flexDirection: 'row', marginLeft: 'auto' },
  button: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  content: { padding: 12 },
  parts: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 6,
    paddingHorizontal: 10,
    paddingTop: 8,
  },
  part: {
    minHeight: 44,
    paddingHorizontal: 10,
    justifyContent: 'center',
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
  },
});
