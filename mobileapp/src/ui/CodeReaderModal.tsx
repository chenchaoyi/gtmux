import {useModalEscape} from '../keys/useModalEscape';
import React, { useState } from 'react';
import {
  Modal,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import Svg, { Path } from 'react-native-svg';
import Clipboard from '@react-native-clipboard/clipboard';
import { CodeText } from './CodeText';
import { codeLabel } from './codeReading';
import { CheckIcon, CopyIcon } from './Icons';
import { MODAL_ORIENTATIONS } from './modalOrientations';
import { RangeSourceContext } from './RangeText';
import { CopyFeedbackContext } from './CopyableText';

export function CodeReaderModal({
  source,
  title,
  hint,
  lang,
  fontSize,
  onClose,
}: {
  source: string;
  title: string;
  hint?: string;
  lang: 'en' | 'zh';
  fontSize: number;
  onClose: () => void;
}) {
  useModalEscape(true, onClose);
  const [formatted, setFormatted] = useState(true);
  const [copied, setCopied] = useState(false);
  return (
    <Modal
      visible
      backdropColor="#0D0D0F"
      animationType="slide"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={onClose}
    >
      <StatusBar barStyle="light-content" />
      <SafeAreaProvider>
        <SafeAreaView style={styles.root}>
          <View style={styles.header}>
            <Text
              accessibilityRole="header"
              style={styles.title}
              numberOfLines={2}
            >
              {title}
            </Text>
            <TouchableOpacity
              testID="code-reader-format"
              style={styles.button}
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh'
                  ? formatted
                    ? '查看原文'
                    : '格式化代码'
                  : formatted
                  ? 'Show original'
                  : 'Format code'
              }
              accessibilityState={{ selected: formatted }}
              onPress={() => setFormatted(v => !v)}
            >
              <Text
                style={{
                  color: formatted ? '#86BFFC' : '#D8DEE9',
                  fontSize: 18,
                  fontWeight: '600',
                }}
              >
                {'{ }'}
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              testID="code-reader-copy"
              style={styles.button}
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh'
                  ? copied
                    ? '已复制'
                    : '复制调用输入'
                  : copied
                  ? 'Copied'
                  : 'Copy recorded input'
              }
              onPress={() => {
                Clipboard.setString(source);
                setCopied(true);
              }}
            >
              {copied ? (
                <CheckIcon size={20} color="#D8DEE9" />
              ) : (
                <CopyIcon size={20} color="#D8DEE9" />
              )}
            </TouchableOpacity>
            <TouchableOpacity
              testID="code-reader-close"
              style={styles.button}
              accessibilityRole="button"
              accessibilityLabel={lang === 'zh' ? '关闭代码' : 'Close code'}
              onPress={onClose}
            >
              <Svg width={22} height={22} viewBox="0 0 24 24">
                <Path
                  d="M6 6l12 12M18 6L6 18"
                  stroke="#D8DEE9"
                  strokeWidth={1.7}
                  strokeLinecap="round"
                />
              </Svg>
            </TouchableOpacity>
          </View>
          <ScrollView
            testID="code-reader-scroll"
            contentContainerStyle={styles.content}
          >
            <View style={styles.card}>
              <Text accessibilityRole="header" style={styles.language}>
                {codeLabel(source, hint)}
              </Text>
              <RangeSourceContext.Provider value={source}>
                <CopyFeedbackContext.Provider value>
                  <CodeText
                    source={source}
                    formatted={formatted}
                    hint={hint}
                    fontSize={fontSize}
                    lang={lang}
                    testID="code-reader-text"
                  />
                </CopyFeedbackContext.Provider>
              </RangeSourceContext.Provider>
            </View>
          </ScrollView>
        </SafeAreaView>
      </SafeAreaProvider>
    </Modal>
  );
}
const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#0D0D0F' },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 4,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: '#343439',
  },
  title: {
    flex: 1,
    minWidth: 0,
    fontSize: 16,
    fontWeight: '600',
    color: '#D8DEE9',
  },
  button: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  content: { padding: 12 },
  card: {
    padding: 14,
    borderRadius: 12,
    backgroundColor: '#1C1C21',
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: '#343439',
  },
  language: {
    color: '#B5B5BE',
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 12,
  },
});
