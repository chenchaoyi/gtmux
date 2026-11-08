import React, { useMemo, useState } from 'react';
import { Text, TouchableOpacity, View } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import type { TranscriptStep } from '../api/client';
import { CodeBlock } from './CodeBlock';
import { ChromeText } from './ChromeText';
import { CopyIcon } from './Icons';
import {
  callPage,
  callRaw,
  callSections,
  CallSection,
} from './toolPresentation';

export function ToolCallDetails({
  step,
  lang,
  dark = true,
  fontSize = 13,
  raw = false,
  disabled = false,
}: {
  step: TranscriptStep;
  lang: 'en' | 'zh';
  dark?: boolean;
  fontSize?: number;
  raw?: boolean;
  disabled?: boolean;
}) {
  const { title, input, input_kind, operation, call, detail } = step;
  const sections = useMemo(() => {
    const record = {
      kind: 'tool',
      title,
      input,
      input_kind,
      operation,
      call,
      detail,
    };
    return raw
      ? [
          {
            id: 'raw',
            label: lang === 'zh' ? '原始记录' : 'Recorded content',
            source: callRaw(record),
            hint: 'json',
          },
        ]
      : callSections(record, lang);
  }, [raw, lang, title, input, input_kind, operation, call, detail]);
  return (
    <View testID="tool-call-details" style={{ gap: 16 }}>
      {sections.map(section => (
        <Section
          key={section.id + section.source.length}
          section={section}
          lang={lang}
          dark={dark}
          fontSize={fontSize}
          disabled={disabled}
        />
      ))}
    </View>
  );
}
function Section({
  section,
  lang,
  dark,
  fontSize,
  disabled,
}: {
  section: CallSection;
  lang: 'en' | 'zh';
  dark: boolean;
  fontSize: number;
  disabled: boolean;
}) {
  const [page, setPage] = useState(0),
    [copied, setCopied] = useState(false),
    view = callPage(section.source, page),
    color = dark ? '#D8DEE9' : '#24292F',
    dim = dark ? '#A4A4AD' : '#626975',
    accent = dark ? '#86BFFC' : '#006F82';
  const button = {
    minHeight: 44,
    minWidth: 44,
    justifyContent: 'center' as const,
    alignItems: 'center' as const,
  };
  return (
    <View testID={'call-section-' + section.id} style={{ gap: 6 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
        <ChromeText style={{ color: dim, flex: 1 }}>{section.label}</ChromeText>
        {section.hint !== 'notice' && (
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={
              (lang === 'zh' ? '复制完整' : 'Copy full ') + section.label
            }
            disabled={disabled}
            style={button}
            onPress={() => {
              try {
                Clipboard.setString(section.source);
                setCopied(true);
              } catch {
                setCopied(false);
              }
            }}
          >
            {copied ? (
              <ChromeText style={{ color: accent }}>✓</ChromeText>
            ) : (
              <CopyIcon size={19} color={dim} />
            )}
          </TouchableOpacity>
        )}
      </View>
      {section.rows ? (
        section.rows.map((row, i) => (
          <View
            key={i}
            style={{
              gap: 3,
              paddingVertical: 6,
              borderBottomWidth: 1,
              borderBottomColor: dark ? '#343439' : '#DADDE3',
            }}
          >
            <ChromeText style={{ color: dim, fontSize: 12 }}>
              {row.label}
            </ChromeText>
            <Text
              selectable
              style={{ color, fontSize, lineHeight: fontSize * 1.5 }}
            >
              {row.value}
            </Text>
          </View>
        ))
      ) : section.hint === 'notice' ? (
        <Text
          selectable
          style={{ color: dim, fontSize, lineHeight: fontSize * 1.5 }}
        >
          {section.source}
        </Text>
      ) : (
        <CodeBlock
          source={view.source}
          hint={section.hint}
          lang={lang}
          dark={dark}
          fontSize={fontSize}
          allowEmbedded={section.id === 'input'}
          allowFormat={section.id !== 'raw' && view.pages === 1}
          initialFormatted={section.id !== 'raw' && view.pages === 1}
          maxHeight={360}
          disabled={disabled}
          showCopy={false}
          testIDPrefix="call-code"
        />
      )}
      {view.pages > 1 && (
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={lang === 'zh' ? '上一页' : 'Previous page'}
            disabled={view.page === 0}
            style={button}
            onPress={() => setPage(n => n - 1)}
          >
            <ChromeText style={{ color: accent }}>‹</ChromeText>
          </TouchableOpacity>
          <ChromeText style={{ color: dim }}>
            {view.page + 1} / {view.pages}
          </ChromeText>
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={lang === 'zh' ? '下一页' : 'Next page'}
            disabled={view.page === view.pages - 1}
            style={button}
            onPress={() => setPage(n => n + 1)}
          >
            <ChromeText style={{ color: accent }}>›</ChromeText>
          </TouchableOpacity>
        </View>
      )}
    </View>
  );
}
