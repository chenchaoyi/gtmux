import {useModalEscape} from '../keys/useModalEscape';
// DiffModal — "what did the agent change". Fetches a unified `git diff` of the
// pane's cwd from /api/diff and renders it line-coloured (additions green,
// deletions red, hunks cyan). Read-only; full-screen over the Detail.

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  ActivityIndicator,
  Animated,
  FlatList,
  Modal,
  PixelRatio,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import { GtmuxClient } from '../api/client';
import { Lang } from '../i18n';
import { Palette } from './theme';
import { diffLineColor, diffRows } from './diff';
import { diffPresentation, hunkRange } from './diffModel';
import { ChromeText } from './ChromeText';
import { CHROME_MAX_SCALE } from './textScale';
import { CodeText } from './CodeText';
import { CodeReaderModal } from './CodeReaderModal';
import { CopyIcon, Chevron, ExpandIcon } from './Icons';
import Clipboard from '@react-native-clipboard/clipboard';
import { MODAL_ORIENTATIONS } from './modalOrientations';

export function DiffModal({
  visible,
  fontSize = 13,
  paneId,
  client,
  pal,
  lang,
  onClose,
}: {
  visible: boolean;
  fontSize?: number;
  paneId: string;
  client: GtmuxClient;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
}) {
  const [text, setText] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [wrapLines, setWrapLines] = useState(true);
  const [folded, setFolded] = useState<Record<string, boolean>>({});
  const [reader, setReader] = useState<{
    source: string;
    title: string;
    hint: string;
  } | null>(null);
  useModalEscape(visible && !reader, onClose);
  const [attempt, setAttempt] = useState(0);
  const { width, fontScale } = useWindowDimensions();
  const [viewportWidth, setViewportWidth] = useState(width);
  const [viewportHeight, setViewportHeight] = useState(0);
  const [contentHeight, setContentHeight] = useState(0);
  const verticalOffset = useRef(new Animated.Value(0)).current;
  const scale = fontScale;
  const chromeScale = Math.min(scale, CHROME_MAX_SCALE);
  const baseLineHeight = Math.ceil(fontSize * 1.5);
  const fileHeight = Math.max(64, Math.ceil(34 * chromeScale + 28));
  const hunkHeight = Math.max(44, Math.ceil(18 * chromeScale + 16));
  const gutterFont = Math.max(10, Math.min(12, fontSize - 2));
  const rowHeight = PixelRatio.roundToNearestPixel(baseLineHeight * scale);

  useEffect(() => {
    if (!visible) {
      setText(null);
      return;
    }
    let alive = true;
    setLoading(true);
    setFailed(false);
    setText(null);
    setFolded({});
    setReader(null);
    verticalOffset.setValue(0);
    client
      .diff(paneId)
      .then(d => alive && (setText(d), setLoading(false)))
      .catch(() => {
        if (alive) {
          setFailed(true);
          setLoading(false);
        }
      });
    return () => {
      alive = false;
    };
  }, [visible, paneId, client, attempt, verticalOffset]);

  const structured = useMemo(
    () => diffPresentation(text ?? '', folded),
    [text, folded],
  );
  const rows = useMemo(
    () => diffRows(structured.lines.join('\n')),
    [structured],
  );
  const gutterWidth = useMemo(() => {
    let maximum = 1;
    for (const file of structured.files)
      for (const row of file.rows)
        maximum = Math.max(maximum, row.meta.old || 0, row.meta.next || 0);
    return Math.max(
      34,
      Math.ceil(String(maximum).length * gutterFont * 0.62 * chromeScale + 6),
    );
  }, [structured.files, gutterFont, chromeScale]);
  const contentWidth = Math.max(
    viewportWidth,
    rows.columns * fontSize * 0.62 * scale +
      (structured.files.length ? 2 * (gutterWidth + 6) + 42 : 24),
  );
  const listStyle = useMemo(
    () => ({
      width:
        structured.files.length && wrapLines
          ? Math.max(1, viewportWidth)
          : contentWidth,
    }),
    [contentWidth, structured.files.length, wrapLines, viewportWidth],
  );
  const layouts = useMemo(() => {
    let offset = 0;
    return rows.lines.map((_, index) => {
      const kind = structured.meta[index]?.kind;
      const length =
        kind === 'file' ? fileHeight : kind === 'hunk' ? hunkHeight : rowHeight;
      const value = { length, offset, index };
      offset += length;
      return value;
    });
  }, [rows, structured, rowHeight, fileHeight, hunkHeight]);
  const renderLine = useCallback(
    ({ item, index }: { item: string; index: number }) => {
      const meta = structured.meta[index];
      const hint = meta?.path?.split('.').pop() || 'text';
      if (meta?.kind === 'file')
        return (
          <View
            style={[
              styles.fileCard,
              { width: Math.max(1, viewportWidth - 24), height: fileHeight },
            ]}
          >
            <TouchableOpacity
              testID={'diff-file-' + meta.key}
              accessibilityRole="button"
              accessibilityState={{ expanded: meta.open }}
              onPress={() =>
                setFolded(v => ({ ...v, [meta.key]: !!meta.open }))
              }
              style={styles.fileTitle}
            >
              <Chevron open={!!meta.open} color="#B5B5BE" size={14} />
              <View style={{ flex: 1, minWidth: 0 }}>
                <ChromeText numberOfLines={1} style={styles.fileName}>
                  {meta.path?.split('/').pop()}
                </ChromeText>
                <ChromeText numberOfLines={1} style={styles.filePath}>
                  {meta.path?.split('/').slice(0, -1).join('/') || '.'}
                </ChromeText>
              </View>
              <ChromeText style={styles.added}>+{meta.added}</ChromeText>
              <ChromeText style={styles.removed}>−{meta.removed}</ChromeText>
            </TouchableOpacity>
            <TouchableOpacity
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh'
                  ? '读取完整文件改动'
                  : 'Read complete file changes'
              }
              style={styles.fileAction}
              onPress={() =>
                setReader({
                  source: structured.files.find(f => f.key === meta.key)!.raw,
                  title: meta.path || 'Diff',
                  hint,
                })
              }
            >
              <ExpandIcon size={20} color="#B5B5BE" />
            </TouchableOpacity>
          </View>
        );
      if (meta?.kind === 'hunk')
        return (
          <TouchableOpacity
            testID={'diff-hunk-' + meta.key}
            accessibilityRole="button"
            accessibilityState={{ expanded: meta.open }}
            style={[
              styles.hunk,
              { width: Math.max(1, viewportWidth - 24), height: hunkHeight },
            ]}
            onPress={() => setFolded(v => ({ ...v, [meta.key]: !!meta.open }))}
          >
            <Chevron open={!!meta.open} color="#B5B5BE" size={13} />
            <ChromeText numberOfLines={1} style={styles.hunkText}>
              {lang === 'zh' ? '行 ' : 'Lines '}
              {hunkRange(item)}
            </ChromeText>
            <ChromeText style={styles.added}>+{meta.added}</ChromeText>
            <ChromeText style={styles.removed}>−{meta.removed}</ChromeText>
          </TouchableOpacity>
        );
      if (meta?.kind === 'code')
        return (
          <View
            style={[
              styles.codeRow,
              {
                height: wrapLines ? undefined : rowHeight,
                minHeight: rowHeight,
                backgroundColor:
                  meta.role === 'add'
                    ? '#153323'
                    : meta.role === 'del'
                    ? '#392025'
                    : '#101013',
              },
            ]}
          >
            <Text
              numberOfLines={1}
              maxFontSizeMultiplier={CHROME_MAX_SCALE}
              style={[
                styles.lineNumber,
                {
                  fontSize: gutterFont,
                  width: gutterWidth,
                  lineHeight: (baseLineHeight * scale) / chromeScale,
                },
              ]}
            >
              {meta.old ?? ''}
            </Text>
            <Text
              numberOfLines={1}
              maxFontSizeMultiplier={CHROME_MAX_SCALE}
              style={[
                styles.lineNumber,
                {
                  fontSize: gutterFont,
                  width: gutterWidth,
                  lineHeight: (baseLineHeight * scale) / chromeScale,
                },
              ]}
            >
              {meta.next ?? ''}
            </Text>
            <Text
              style={[
                styles.sign,
                {
                  fontSize,
                  lineHeight: baseLineHeight,
                  color:
                    meta.role === 'add'
                      ? '#A8C997'
                      : meta.role === 'del'
                      ? '#EEA3AC'
                      : '#B5B5BE',
                },
              ]}
            >
              {item[0]}
            </Text>
            <View style={{ flex: 1 }}>
              <CodeText
                source={item.slice(1)}
                hint={hint}
                fontSize={fontSize}
                maxLines={wrapLines ? undefined : 1}
                selectable={false}
                style={{ lineHeight: baseLineHeight }}
              />
            </View>
          </View>
        );
      return (
        <Text
          numberOfLines={1}
          style={[
            styles.line,
            {
              fontSize,
              height: rowHeight,
              lineHeight: baseLineHeight,
              color: diffLineColor(item),
            },
          ]}
        >
          {item || ' '}
        </Text>
      );
    },
    [
      structured,
      viewportWidth,
      rowHeight,
      fontSize,
      lang,
      wrapLines,
      fileHeight,
      hunkHeight,
      gutterWidth,
      gutterFont,
      baseLineHeight,
      scale,
      chromeScale,
    ],
  );
  const trackHeight = Math.max(0, viewportHeight - 8);
  const thumbHeight = Math.min(
    trackHeight,
    Math.max(28, (trackHeight * viewportHeight) / Math.max(1, contentHeight)),
  );
  const indicatorTop = verticalOffset.interpolate({
    inputRange: [0, Math.max(1, contentHeight - viewportHeight)],
    outputRange: [0, Math.max(0, trackHeight - thumbHeight)],
    extrapolate: 'clamp',
  });
  const empty = !loading && (text ?? '') === '';

  return (
    <Modal
      supportedOrientations={MODAL_ORIENTATIONS}
      visible={visible}
      backdropColor={pal.bg}
      animationType="slide"
      onRequestClose={onClose}
    >
      <StatusBar hidden />
      <View style={[styles.root, { backgroundColor: pal.bg }]}>
        <View style={[styles.bar, { borderBottomColor: pal.divider }]}>
          <ChromeText
            numberOfLines={2}
            style={[styles.title, { color: pal.fg }]}
          >
            {lang === 'zh' ? '工作区改动' : 'Working tree'}
          </ChromeText>
          <TouchableOpacity
            onPress={onClose}
            hitSlop={hit}
            accessibilityRole="button"
            accessibilityLabel={lang === 'zh' ? '关闭改动' : 'Close changes'}
            style={{
              minHeight: 44,
              minWidth: 44,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <ChromeText style={styles.done}>
              {lang === 'zh' ? '完成' : 'Done'}
            </ChromeText>
          </TouchableOpacity>
        </View>

        {!!structured.files.length && (
          <View style={styles.summary}>
            <ChromeText style={styles.filePath}>
              {structured.files.length} {lang === 'zh' ? '个文件' : 'files'}
            </ChromeText>
            <ChromeText style={styles.added}>+{structured.added}</ChromeText>
            <ChromeText style={styles.removed}>
              −{structured.removed}
            </ChromeText>
            <TouchableOpacity
              testID="diff-wrap"
              accessibilityRole="button"
              accessibilityState={{ selected: wrapLines }}
              accessibilityLabel={lang === 'zh' ? '自动换行' : 'Wrap lines'}
              style={styles.fileAction}
              onPress={() => setWrapLines(v => !v)}
            >
              <Text
                style={{
                  color: wrapLines ? '#9FC7E9' : '#B5B5BE',
                  fontSize: 12,
                }}
              >
                {lang === 'zh' ? '换行' : 'Wrap'}
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={styles.fileAction}
              accessibilityRole="button"
              accessibilityLabel={
                lang === 'zh' ? '复制完整改动' : 'Copy complete diff'
              }
              onPress={() => Clipboard.setString(text || '')}
            >
              <CopyIcon size={20} color="#B5B5BE" />
            </TouchableOpacity>
          </View>
        )}
        {loading ? (
          <ActivityIndicator color={pal.fg3} style={styles.center} />
        ) : failed ? (
          <View style={styles.center}>
            <Text
              accessibilityRole="alert"
              style={[styles.empty, { color: pal.fg3 }]}
            >
              {lang === 'zh'
                ? '无法读取改动，请检查连接后重试。'
                : 'Could not load changes. Check the connection and retry.'}
            </Text>
            <TouchableOpacity
              testID="diff-retry"
              accessibilityLabel={lang === 'zh' ? '重试' : 'Retry'}
              accessibilityRole="button"
              onPress={() => setAttempt(value => value + 1)}
              style={styles.retry}
            >
              <ChromeText style={styles.done}>
                {lang === 'zh' ? '重试' : 'Retry'}
              </ChromeText>
            </TouchableOpacity>
          </View>
        ) : empty ? (
          <Text style={[styles.empty, { color: pal.fg3 }]}>
            {lang === 'zh'
              ? '没有改动（或这个目录不是 git 仓库）。'
              : 'No changes (or the directory isn’t a git repo).'}
          </Text>
        ) : (
          <View style={styles.body}>
            {rows.shortened && (
              <TouchableOpacity
                accessibilityRole="button"
                onPress={() =>
                  setReader({ source: text || '', title: 'Diff', hint: 'text' })
                }
              >
                <Text style={styles.notice}>
                  {lang === 'zh'
                    ? '长行已缩短，点此阅读完整改动。'
                    : 'Long lines are shortened. Tap to read the complete diff.'}
                </Text>
              </TouchableOpacity>
            )}
            <View style={styles.readingViewport}>
              <ScrollView
                testID="diff-horizontal"
                style={styles.horizontal}
                horizontal
                contentContainerStyle={styles.hContent}
                showsHorizontalScrollIndicator
                indicatorStyle="white"
                onLayout={event => {
                  setViewportWidth(event.nativeEvent.layout.width);
                  setViewportHeight(event.nativeEvent.layout.height);
                }}
              >
                <FlatList
                  key={rowHeight}
                  testID="diff-lines"
                  style={listStyle}
                  showsVerticalScrollIndicator={false}
                  onContentSizeChange={(_width, height) =>
                    setContentHeight(height)
                  }
                  onScroll={event =>
                    verticalOffset.setValue(event.nativeEvent.contentOffset.y)
                  }
                  scrollEventThrottle={16}
                  contentContainerStyle={styles.bodyContent}
                  data={rows.lines}
                  initialNumToRender={40}
                  maxToRenderPerBatch={24}
                  windowSize={5}
                  getItemLayout={
                    structured.files.length && wrapLines
                      ? undefined
                      : (_, index) => layouts[index]
                  }
                  keyExtractor={(_, index) => String(index)}
                  renderItem={renderLine}
                />
              </ScrollView>
              {viewportHeight > 0 && contentHeight > viewportHeight + 1 && (
                <View
                  testID="diff-scroll-track"
                  pointerEvents="none"
                  accessibilityElementsHidden
                  importantForAccessibility="no-hide-descendants"
                  style={styles.scrollTrack}
                >
                  <Animated.View
                    testID="diff-scroll-thumb"
                    style={[
                      styles.scrollThumb,
                      {
                        height: thumbHeight,
                        transform: [{ translateY: indicatorTop }],
                      },
                    ]}
                  />
                </View>
              )}
            </View>
          </View>
        )}
      </View>
      {reader && (
        <CodeReaderModal
          {...reader}
          lang={lang}
          fontSize={fontSize}
          onClose={() => setReader(null)}
        />
      )}
    </Modal>
  );
}

const hit = { top: 10, bottom: 10, left: 10, right: 10 };

const styles = StyleSheet.create({
  root: { flex: 1 },
  summary: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: 8,
    paddingLeft: 16,
    backgroundColor: '#1C1C21',
  },
  fileCard: {
    height: 64,
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#242429',
    borderTopWidth: 8,
    borderTopColor: '#0A0A0C',
    borderRadius: 8,
  },
  fileTitle: {
    flex: 1,
    minWidth: 0,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    padding: 10,
  },
  fileName: { fontSize: 14, fontWeight: '600', color: '#EBEBF5' },
  filePath: { fontSize: 12, color: '#B5B5BE' },
  added: { fontSize: 12, color: '#A8C997', fontVariant: ['tabular-nums'] },
  removed: { fontSize: 12, color: '#EEA3AC', fontVariant: ['tabular-nums'] },
  fileAction: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  hunk: {
    height: 44,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 12,
    backgroundColor: '#16161B',
  },
  hunkText: { fontSize: 12, color: '#9FC7E9', flex: 1 },
  codeRow: { flexDirection: 'row', alignItems: 'flex-start' },
  lineNumber: {
    width: 34,
    textAlign: 'right',
    color: '#A1A7B2',
    fontFamily: 'Menlo',
    marginRight: 6,
  },
  sign: { width: 18, fontFamily: 'Menlo' },
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingTop: 56,
    paddingBottom: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  title: { fontSize: 17, fontWeight: '700', flex: 1, minWidth: 0 },
  done: { fontSize: 15, fontWeight: '700', color: '#06B6D4' },
  center: { marginTop: 60 },
  retry: {
    alignSelf: 'center',
    minHeight: 44,
    justifyContent: 'center',
    paddingHorizontal: 20,
  },
  notice: { color: '#EBEBF5', padding: 12, fontSize: 12 },
  empty: {
    fontSize: 14,
    textAlign: 'center',
    marginTop: 60,
    paddingHorizontal: 24,
  },
  body: { flex: 1, backgroundColor: '#0A0A0C' },
  readingViewport: { flex: 1 },
  horizontal: { flex: 1, marginRight: 8 },
  scrollTrack: {
    position: 'absolute',
    right: 2,
    top: 4,
    bottom: 4,
    width: 4,
    borderRadius: 2,
    backgroundColor: '#242429',
  },
  scrollThumb: { width: 4, borderRadius: 2, backgroundColor: '#C4C4CE' },
  // horizontal content must at least fill the screen so a narrow diff doesn't
  // leave a void to its right (the dark body bg still covers, but edges snap).
  hContent: { flexGrow: 1, minWidth: '100%' },
  bodyContent: { paddingHorizontal: 12, paddingBottom: 40 },
  line: { fontFamily: 'Menlo', fontSize: 11, lineHeight: 15 },
});
