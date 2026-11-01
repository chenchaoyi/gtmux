import { useModalEscape } from '../keys/useModalEscape';
import React from 'react';
import {
  Modal,
  ScrollView,
  StatusBar,
  StyleSheet,
  TouchableOpacity,
  View,
} from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import Svg, { Path } from 'react-native-svg';
import { CodePanel } from './CodePanel';
import { ChromeText } from './ChromeText';
import { MODAL_ORIENTATIONS } from './modalOrientations';
import { paletteFor } from './theme';

export function CodeReaderModal({
  source,
  title,
  hint,
  lang,
  fontSize,
  dark = true,
  initialPartID,
  initialFormatted = true,
  allowEmbedded = true,
  allowFormat = true,
  onClose,
}: {
  source: string;
  title: string;
  hint?: string;
  lang: 'en' | 'zh';
  fontSize: number;
  dark?: boolean;
  initialPartID?: string;
  initialFormatted?: boolean;
  allowEmbedded?: boolean;
  allowFormat?: boolean;
  onClose: () => void;
}) {
  useModalEscape(true, onClose);
  const pal = paletteFor(dark ? 'dark' : 'light');
  return (
    <Modal
      visible
      backdropColor={pal.bg}
      animationType="slide"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={onClose}
    >
      <StatusBar barStyle={dark ? 'light-content' : 'dark-content'} />
      <SafeAreaProvider>
        <SafeAreaView style={[styles.root, { backgroundColor: pal.bg }]}>
          <View style={[styles.header, { borderBottomColor: pal.divider }]}>
            <ChromeText
              accessibilityRole="header"
              style={[styles.title, { color: pal.fg }]}
              numberOfLines={2}
            >
              {title}
            </ChromeText>
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
                  stroke={pal.fg}
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
            <CodePanel
              source={source}
              hint={hint}
              lang={lang}
              fontSize={fontSize}
              dark={dark}
              initialPartID={initialPartID}
              initialFormatted={initialFormatted}
              allowEmbedded={allowEmbedded}
              allowFormat={allowFormat}
              testIDPrefix="code-reader"
              textTestID="code-reader-text"
            />
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
});
