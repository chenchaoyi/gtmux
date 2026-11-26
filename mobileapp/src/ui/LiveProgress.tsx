import {useReadingPalette, readingColors} from './ReadingPalette';
import type {Palette} from './theme';
import React from 'react';
import {PanResponder, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import type {AnsiLine, Span} from './ansi';
import {MOBILE_SCROLLBACK_LINES} from './term';
import type {NativeScrollEvent, NativeSyntheticEvent} from 'react-native';
import {BrandLoader} from './BrandLoader';
import {Chevron} from './Icons';
import {TestIds} from '../constants/testIds';
import {ChromeText} from './ChromeText';
import {Lang} from '../i18n';

export const COLLAPSED_LIVE_ROW_HEIGHT = 48;
const DEFAULT_LIVE_PREVIEW_HEIGHT = 220;
const MIN_LIVE_PREVIEW_HEIGHT = 132;
const LIVE_RESIZE_STEP = 56;
const HEADER_HEIGHT = 44;
const PANEL_PADDING = 6;

/** The conversation owns disclosure state; this row can disappear between observations. */
export function LiveProgress({avatar, label, lines, fontSize, fontFamily, lang, open, onToggle, working = true, inspectAvailable = true, sourceLabel, heightLimit, savedHeight, onResize}: {
  avatar: React.ReactNode;
  label: string;
  lines: AnsiLine[];
  fontSize: number;
  fontFamily: string | undefined;
  lang: Lang;
  open: boolean;
  working?: boolean;
  onToggle: () => void;
  inspectAvailable?: boolean;
  sourceLabel?: string;
  heightLimit?: number;
  savedHeight?: number;
  onResize: (height: number) => void;
}) {
  const pal = useReadingPalette();
  const styles = React.useMemo(() => makeStyles(pal), [pal]);
  const [desiredHeight, setDesiredHeight] = React.useState(savedHeight ?? DEFAULT_LIVE_PREVIEW_HEIGHT);
  const [headerHeight, setHeaderHeight] = React.useState(HEADER_HEIGHT + (sourceLabel ? 22 : 0));
  const previewLimit = heightLimit === undefined ? DEFAULT_LIVE_PREVIEW_HEIGHT
    : Math.max(0, Math.floor(heightLimit - headerHeight - 2 * (PANEL_PADDING + StyleSheet.hairlineWidth)));
  const minHeight = Math.min(MIN_LIVE_PREVIEW_HEIGHT, previewLimit);
  const height = Math.max(minHeight, Math.min(desiredHeight, previewLimit));
  const lineHeight = Math.round(fontSize * 1.4);
  const rowLimit = Math.max(1, Math.floor((height - 8) / lineHeight));
  const canResize = lines.length > 0 && previewLimit > minHeight;
  const current = React.useRef({height, minHeight, previewLimit, onResize});
  current.current = {height, minHeight, previewLimit, onResize};
  const dragStart = React.useRef<number | undefined>(undefined);
  const resize = React.useCallback((target: number) => {
    const limits = current.current;
    const next = Math.round(Math.max(limits.minHeight, Math.min(limits.previewLimit, target)));
    setDesiredHeight(next);
    limits.onResize(next);
  }, []);
  const finish = React.useCallback(() => {
    dragStart.current = undefined;
  }, []);
  const grip = React.useMemo(() => PanResponder.create({
    onStartShouldSetPanResponder: () => current.current.previewLimit > current.current.minHeight,
    onPanResponderGrant: () => {
      dragStart.current = current.current.height;
    },
    onPanResponderMove: (_event, {dy}) => {
      if (dragStart.current !== undefined) resize(dragStart.current - dy);
    },
    onPanResponderRelease: finish,
    onPanResponderTerminate: finish,
    onPanResponderTerminationRequest: () => false,
  }), [finish, resize]);
  React.useEffect(() => {
    if (!open || !inspectAvailable || !canResize) finish();
  }, [open, inspectAvailable, canResize, finish]);
  React.useEffect(() => finish, [finish]);
  const action = lang === 'zh' ? (open ? '收起实时画面' : '展开实时画面') : (open ? 'Hide live view' : 'Show live view');
  const marker = <>
    {working && <BrandLoader size={14} neutral={pal.fg3} />}
    <ChromeText style={styles.label} numberOfLines={1}>{label}</ChromeText>
  </>;
  const actionContent = (title: string) => <View style={[styles.action, !open && styles.disclosureAction]}>
    <ChromeText style={styles.actionText} numberOfLines={open ? 1 : undefined}>{title}</ChromeText>
    <Chevron open={open} size={13} color={readingColors(pal).link} />
  </View>;
  const toggleProps = {
    testID: TestIds.detail.chatLiveToggle,
    accessibilityRole: 'button' as const,
    accessibilityLabel: `${label}, ${action}`,
    accessibilityState: {expanded: open},
    onPress: onToggle,
    activeOpacity: 0.7,
  };
  if (!open || !inspectAvailable) {
    return <View testID={TestIds.detail.chatThinking} style={styles.row}>
      {avatar}
      {inspectAvailable ? <TouchableOpacity {...toggleProps} style={[styles.disclosure, styles.closedDisclosure]}>
        {marker}
        {actionContent(action)}
      </TouchableOpacity> : <View style={styles.disclosure}>{marker}</View>}
    </View>;
  }
  return <View testID={TestIds.detail.chatThinking} style={[styles.expanded, {maxHeight: heightLimit}]}>
    <View onLayout={event => setHeaderHeight(event.nativeEvent.layout.height)}>
      <View testID="chat-live-header" style={styles.row}>
        <View style={styles.headerStatus}>{avatar}{marker}</View>
        {canResize && <View testID="chat-live-resize-grip" accessible
          {...grip.panHandlers} style={styles.grip} accessibilityRole="adjustable"
          accessibilityLabel={lang === 'zh' ? '实时终端高度' : 'Live terminal height'}
          accessibilityHint={lang === 'zh' ? '向上拉高，向下缩小' : 'Drag up to expand or down to shrink'}
          accessibilityValue={{min: minHeight, max: previewLimit, now: height, text: lang === 'zh' ? `${rowLimit} 行` : `${rowLimit} lines`}}
          accessibilityActions={[
            {name: 'increment', label: lang === 'zh' ? '拉高实时终端' : 'Expand live terminal'},
            {name: 'decrement', label: lang === 'zh' ? '缩小实时终端' : 'Shrink live terminal'},
          ]}
          onAccessibilityAction={({nativeEvent: {actionName}}) => {
            if (actionName === 'increment') resize(height + LIVE_RESIZE_STEP);
            if (actionName === 'decrement') resize(height - LIVE_RESIZE_STEP);
          }}>
          <View style={styles.gripMark} />
        </View>}
        <View style={styles.headerEnd}>
          <TouchableOpacity {...toggleProps} style={styles.close}>
            {actionContent(lang === 'zh' ? '收起' : 'Hide')}
          </TouchableOpacity>
        </View>
      </View>
      {sourceLabel && <ChromeText testID="live-pane-source" style={styles.source}>{sourceLabel}</ChromeText>}
    </View>
    {height > 0 && <View testID={TestIds.detail.chatLivePreview} style={styles.preview}>
      {lines.length ? <LiveScrollback lines={lines} height={height} fontSize={fontSize} lineHeight={lineHeight}
        fontFamily={fontFamily} /> : <Text style={historyStyles.empty}>{lang === 'zh' ? '尚无实时输出' : 'No terminal output yet'}</Text>}
    </View>}
  </View>;
}

// The viewport can resize independently of the bounded capture. Keep the displayed
// frame while browsing: a rotating buffer can otherwise replace the row under a finger.
function LiveScrollback({lines, height, fontSize, lineHeight, fontFamily}: {
  lines: AnsiLine[]; height: number; fontSize: number; lineHeight: number; fontFamily?: string;
}) {
  const live = React.useMemo(() => lines.length > MOBILE_SCROLLBACK_LINES ? lines.slice(-MOBILE_SCROLLBACK_LINES) : lines, [lines]);
  const [held, setHeld] = React.useState<AnsiLine[] | null>(null);
  const viewport = React.useRef<ScrollView>(null);
  const following = React.useRef(true);
  const measured = React.useRef({viewport: 0, content: 0});
  const displayed = held ?? live;
  const text = React.useMemo(() => {
    // Plain terminal output often has hundreds of identically styled lines.
    // Keep selection continuous without building a native Text node per line.
    const runs: Span[] = [];
    function append(value: string, color: string, bold?: boolean) {
      const last = runs[runs.length - 1];
      if (last && last.color === color && last.bold === bold) last.text += value;
      else runs.push({text: value, color, bold});
    }
    displayed.forEach((row, i) => {
      if (i > 0) runs[runs.length - 1].text += '\n';
      if (!row.length) append(' ', '#D6D6DA');
      for (const span of row) append(span.text, span.color, span.bold);
    });
    return runs.map((run, i) => <Text key={i} style={{color: run.color, fontWeight: run.bold ? '700' : '400'}}>{run.text}</Text>);
  }, [displayed]);
  const resume = () => {
    following.current = true;
    setHeld(null);
  };
  const reconcileSize = () => {
    if (!following.current) {
      // A resize can remove the last scrollable row without a scroll-end event.
      const {viewport: visible, content} = measured.current;
      if (!(visible > 0 && content > 0 && content <= visible)) return;
      resume();
    }
    viewport.current?.scrollToEnd({animated: false});
  };
  const settle = ({nativeEvent: {contentSize, layoutMeasurement, contentOffset}}: NativeSyntheticEvent<NativeScrollEvent>) => {
    const content = contentSize.height, visible = layoutMeasurement.height;
    measured.current = {content, viewport: visible};
    if (content > 0 && visible > 0 && content - visible - contentOffset.y <= 1) resume();
  };
  return <ScrollView ref={viewport} style={{...historyStyles.viewport, height, maxHeight: height}} nestedScrollEnabled
    onContentSizeChange={(_width, content) => {measured.current.content = content; reconcileSize();}}
    onLayout={({nativeEvent: {layout}}) => {measured.current.viewport = layout.height; reconcileSize();}}
    onScrollBeginDrag={() => {following.current = false; setHeld(displayed);}}
    onScrollEndDrag={settle} onMomentumScrollEnd={settle}>
    <ScrollView horizontal nestedScrollEnabled contentContainerStyle={historyStyles.screen}>
      <Text selectable style={[historyStyles.mono, {fontFamily, fontSize, lineHeight}]}>{text}</Text>
    </ScrollView>
  </ScrollView>;
}

const makeStyles = (pal: Palette) => StyleSheet.create({
  expanded: {backgroundColor: pal.surface, borderWidth: StyleSheet.hairlineWidth, borderColor: readingColors(pal).dark ? '#24606B' : '#88C3CD', borderRadius: 12, padding: PANEL_PADDING, overflow: 'hidden'},
  grip: {width: 44, minHeight: HEADER_HEIGHT, alignItems: 'center', justifyContent: 'center'},
  gripMark: {width: 28, height: 3, borderRadius: 2, backgroundColor: pal.fg2},
  row: {flexDirection: 'row', alignItems: 'center'},
  headerStatus: {flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: 4},
  headerEnd: {flex: 1, minWidth: 0, alignItems: 'flex-end'},
  close: {minHeight: HEADER_HEIGHT, minWidth: 44, maxWidth: '100%', justifyContent: 'center'},
  disclosure: {flex: 1, flexDirection: 'row', alignItems: 'center', gap: 8, minHeight: 44, paddingHorizontal: 10, paddingVertical: 6, borderRadius: 12, backgroundColor: pal.surface},
  closedDisclosure: {minHeight: COLLAPSED_LIVE_ROW_HEIGHT, borderWidth: 1, borderColor: readingColors(pal).dark ? '#24606B' : '#88C3CD'},
  label: {flex: 1, fontSize: 13, fontWeight: '500', color: pal.fg2, fontVariant: ['tabular-nums']},
  action: {flexDirection: 'row', alignItems: 'center', gap: 5, flexShrink: 1, minHeight: 32, paddingHorizontal: 8, paddingVertical: 4, borderRadius: 8, backgroundColor: pal.raised},
  disclosureAction: {maxWidth: '62%'},
  actionText: {fontSize: 13, fontWeight: '600', color: readingColors(pal).link, flexShrink: 1},
  preview: {flexShrink: 1, overflow: 'hidden', borderRadius: 6},
  source: {color: pal.fg2, fontSize: 12, lineHeight: 18, marginBottom: 4, paddingHorizontal: 6},
});

const historyStyles = StyleSheet.create({
  viewport: {backgroundColor: '#17171A'},
  screen: {paddingVertical: 4},
  mono: {color: '#D6D6DA'},
  empty: {padding: 12, backgroundColor: '#17171A', color: 'rgba(235,235,245,0.6)', fontSize: 12},
});
