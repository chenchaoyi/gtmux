import React, { useMemo } from 'react';
import { Platform, StyleSheet, Text, TextStyle } from 'react-native';
import { CopyableText } from './CopyableText';
import { formatCode } from './codeFormatting';
import { codeLanguage, codeTokens, syntaxColors } from './codeReading';

export function CodeText({
  source: original,
  formatted = false,
  hint,
  dark = true,
  fontSize = 13,
  lang = 'en',
  testID,
  maxLines,
  style,
  selectable = true,
  selectionColor,
}: {
  source: string;
  formatted?: boolean;
  hint?: string;
  dark?: boolean;
  fontSize?: number;
  lang?: 'en' | 'zh';
  testID?: string;
  maxLines?: number;
  style?: TextStyle;
  selectable?: boolean;
  selectionColor?: string;
}) {
  const source = useMemo(
    () =>
      formatted
        ? formatCode(original, hint || codeLanguage(original))
        : original,
    [original, hint, formatted],
  );
  const tokens = useMemo(
    () => codeTokens(source, codeLanguage(source, hint)),
    [source, hint],
  );
  const colors = syntaxColors[dark ? 'dark' : 'light'];
  return (
    <CopyableText
      testID={testID}
      lang={lang}
      selectable={selectable}
      selectionColor={selectionColor}
      numberOfLines={maxLines}
      style={[
        styles.code,
        {
          color: colors.plain,
          fontSize,
          lineHeight: Math.ceil(fontSize * 1.55),
        },
        style,
      ]}
    >
      {tokens.map((token, i) => (
        <Text key={i} style={{ color: colors[token.kind] }}>
          {token.text}
        </Text>
      ))}
    </CopyableText>
  );
}
const styles = StyleSheet.create({
  code: {
    fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace',
    flexShrink: 1,
    minWidth: 0,
    width: '100%',
  },
});
