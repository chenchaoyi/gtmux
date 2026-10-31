import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  AccessibilityInfo,
  ActivityIndicator,
  Alert,
  FlatList,
  findNodeHandle,
  Keyboard,
  Modal,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import Clipboard from '@react-native-clipboard/clipboard';
import Svg, { Path } from 'react-native-svg';
import { GtmuxClient, TranscriptTurn } from '../api/client';
import { Agent, primary } from '../api/types';
import { Lang, statusLabel } from '../i18n';
import { Palette, BRAND } from '../ui/theme';
import { AnchoredMenu, MenuAnchor, MenuItem } from '../ui/AnchoredMenu';
import { MODAL_ORIENTATIONS } from '../ui/modalOrientations';
import { useTraceSplit } from '../ui/layout';
import { AgentAvatar } from '../ui/AgentAvatar';
import { UserAvatar } from '../ui/UserAvatar';
import { SenderAvatar } from '../ui/SenderAvatar';
import { ParticipantProfileModal } from '../ui/ParticipantProfileModal';
import { ProfileTarget } from '../ui/participantProfile';
import { MarkdownView } from '../ui/MarkdownView';
import { CodeText } from '../ui/CodeText';
import { darkCodeBackground, toolCodeHint } from '../ui/codeReading';
import { CopyFeedbackContext } from '../ui/CopyableText';
import { ArrowToBottomIcon, CopyIcon, SearchIcon, Chevron } from '../ui/Icons';
import { ChromeText } from '../ui/ChromeText';
import { SIcon } from '../ui/SettingsIcons';
import { StatusBadge } from '../ui/StatusBadge';
import { nativeDocumentPreview } from '../documents/nativePreview';
import { DocumentError, documentErrorMessage } from '../documents/references';
import {
  build,
  filter,
  groupTurns,
  sourceInfo,
  overviewFacts,
  previewText,
  TraceEvent,
  TraceKind,
} from './model';
import { timelineRows, TimelineRow } from './timeline';
import {
  TraceTimelineRow,
  traceEventLabel,
  traceEventPreview,
} from './TraceTimelineRow';

type Props = {
  visible: boolean;
  agent: Agent;
  client?: GtmuxClient;
  turns: TranscriptTurn[];
  loading: boolean;
  dropped?: number;
  pal: Palette;
  lang: Lang;
  fontSize?: number;
  onClose: () => void;
  failed?: boolean;
  onRetry?: () => void;
  earlierAvailable?: boolean;
  loadingEarlier?: boolean;
  onLoadEarlier?: () => void;
};
type Selection = { event: TraceEvent; turn: TranscriptTurn };
const kindLabels = {
  all: ['All records', '全部记录'],
  user: ['Messages', '用户消息'],
  assistant: ['Agent replies', 'Agent 回复'],
  tool: ['Tool calls', '工具调用'],
  context: ['Context', '上下文'],
};
const NO_TURNS: TranscriptTurn[] = [];
export function TraceModal(props: Props) {
  // The modal owns no reader: its parent supplies the existing transcript subscription.
  // Native modals need their own safe-area root, as in the pairing scanner.
  return (
    <Modal
      visible={props.visible}
      animationType="slide"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={props.onClose}
    >
      {props.visible && (
        <SafeAreaProvider style={styles.root}>
          <TraceReader {...props} />
        </SafeAreaProvider>
      )}
    </Modal>
  );
}
export function TraceReader({
  agent,
  client,
  turns = NO_TURNS,
  loading,
  dropped = 0,
  pal,
  lang,
  fontSize = 15,
  onClose,
  failed,
  onRetry,
  earlierAvailable,
  loadingEarlier,
  onLoadEarlier,
}: Props) {
  const zh = lang === 'zh',
    [width, setWidth] = useState(0),
    split = useTraceSplit(width);
  const [query, setQuery] = useState(''),
    [kind, setKind] = useState<TraceKind | 'all'>('all'),
    [session, setSession] = useState('all');
  const [selection, setSelection] = useState<Selection | null>(null),
    [detailsOpen, setDetailsOpen] = useState(false),
    [detailTab, setDetailTab] = useState<
      'overview' | 'preview' | 'raw' | 'origin'
    >('preview');
  const [atTail, setAtTail] = useState(true),
    [searchOpen, setSearchOpen] = useState(false),
    [expandedTurns, setExpandedTurns] = useState<Record<string, boolean>>({}),
    [expandedCalls, setExpandedCalls] = useState<Record<string, boolean>>({}),
    [profile, setProfile] = useState<ProfileTarget | null>(null),
    [copied, setCopied] = useState(false);
  const [turnDefault, setTurnDefault] = useState<boolean | undefined>(),
    [callDefault, setCallDefault] = useState<boolean | undefined>();
  const rows = useRef(new Map<string, View>()),
    searchRef = useRef<TextInput>(null);
  const list = useRef<FlatList<TimelineRow>>(null),
    stick = useRef(true),
    disclosureHold = useRef(false),
    dragging = useRef(false),
    gapRef = useRef(0),
    tailFrame = useRef<number | null>(null),
    copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scope = JSON.stringify([
    agent.pane_id,
    agent.session_id,
    agent.home_id,
    agent.agent,
  ]);
  const [identity, setIdentity] = useState({ client, scope });
  const sameIdentity = identity.client === client && identity.scope === scope;
  useEffect(() => {
    setIdentity({ client, scope });
    setSelection(null);
    setDetailsOpen(false);
    setProfile(null);
    setQuery('');
    setKind('all');
    setSession('all');
    setSearchOpen(false);
    setExpandedTurns({});
    setExpandedCalls({});
    setTurnDefault(undefined);
    setCallDefault(undefined);
    stick.current = true;
    disclosureHold.current = false;
    setAtTail(true);
    if (tailFrame.current !== null) cancelAnimationFrame(tailFrame.current);
  }, [client, scope]);
  useEffect(
    () => () => {
      clearTimeout(copyTimer.current);
      if (tailFrame.current !== null) cancelAnimationFrame(tailFrame.current);
    },
    [],
  );
  const safeTurns = sameIdentity ? turns : NO_TURNS;
  const index = useMemo(
    () =>
      build(safeTurns, {
        pane_id: agent.pane_id,
        session_id: agent.session_id,
        home_id: agent.home_id,
        agent: agent.agent,
      }),
    [safeTurns, agent.pane_id, agent.session_id, agent.home_id, agent.agent],
  );
  const events = useMemo(
    () => filter(index.events, query, kind, session),
    [index, query, kind, session],
  );
  const groups = useMemo(() => groupTurns(events), [events]);
  const searching = !!query.trim() || kind !== 'all' || session !== 'all';
  const latestGroup = groups.at(-1);
  useEffect(() => {
    if (
      sameIdentity &&
      !searching &&
      stick.current &&
      latestGroup &&
      turnDefault === undefined
    )
      setExpandedTurns(value =>
        value[latestGroup.key] ? value : { ...value, [latestGroup.key]: true },
      );
  }, [sameIdentity, searching, latestGroup?.key, turnDefault]); // eslint-disable-line react-hooks/exhaustive-deps
  const timeline = useMemo(
    () =>
      timelineRows(groups, expandedTurns, expandedCalls, searching, {
        turns: turnDefault,
        calls: callDefault,
      }),
    [groups, expandedTurns, expandedCalls, searching, turnDefault, callDefault],
  );
  const allTurnsOpen =
    groups.length > 0 &&
    groups.every(group => expandedTurns[group.key] ?? turnDefault ?? false);
  const callBatches = timelineRows(groups, {}, expandedCalls, false, {
    turns: true,
    calls: callDefault,
  }).filter(row => row.type === 'calls');
  const allCallsOpen = callBatches.every(
    row => row.type === 'calls' && row.open,
  );
  function foldAll(level: 'turns' | 'calls') {
    disclosureHold.current = true;
    stopFollowing();
    if (level === 'turns') {
      setTurnDefault(!allTurnsOpen);
      setExpandedTurns({});
    } else {
      setCallDefault(!allCallsOpen);
      setExpandedCalls({});
    }
  }
  const latestEvent = latestGroup?.events.at(-1);
  function stopFollowing() {
    stick.current = false;
    setAtTail(false);
    if (tailFrame.current !== null) cancelAnimationFrame(tailFrame.current);
  }
  function pinTail() {
    if (!stick.current || searching || detailsOpen || !sameIdentity) return;
    if (tailFrame.current !== null) cancelAnimationFrame(tailFrame.current);
    tailFrame.current = requestAnimationFrame(() => {
      tailFrame.current = null;
      if (stick.current && !dragging.current)
        list.current?.scrollToEnd({ animated: false });
    });
  }
  function jumpLatest() {
    disclosureHold.current = false;
    if (latestGroup)
      setExpandedTurns(value => ({ ...value, [latestGroup.key]: true }));
    stick.current = true;
    setAtTail(true);
    pinTail();
  }
  function toggleTimeline(
    row: Extract<TimelineRow, { type: 'turn' | 'calls' }>,
  ) {
    disclosureHold.current = true;
    stopFollowing();
    if (row.type === 'turn')
      setExpandedTurns(value => ({ ...value, [row.group.key]: !row.open }));
    else setExpandedCalls(value => ({ ...value, [row.key]: !row.open }));
  }
  const current =
    selection && index.events.find(event => event.key === selection.event.key);
  const chosen = sameIdentity && current ? selection : null;
  const showDetails = detailsOpen && !!chosen;
  const history = session !== 'all' && session !== index.sessions.at(-1)?.key;
  useEffect(() => {
    if (selection && !current) {
      setSelection(null);
      setDetailsOpen(false);
      setProfile(null);
    }
  }, [current, selection]);
  useEffect(() => {
    if (
      session !== 'all' &&
      !index.sessions.some(value => value.key === session)
    )
      setSession('all');
  }, [index.sessions, session]);
  function select(event: TraceEvent) {
    Keyboard.dismiss();
    stopFollowing();
    setSelection({ event, turn: turns[event.sourceIndex] });
    setDetailsOpen(true);
    setDetailTab('preview');
    setCopied(false);
  }
  function closeDetail() {
    setDetailsOpen(false);
    requestAnimationFrame(() => {
      const target = findNodeHandle(
        chosen
          ? rows.current.get(chosen.event.key) ||
              rows.current.get('turn:' + chosen.event.turnKey) ||
              searchRef.current
          : searchRef.current,
      );
      if (target) AccessibilityInfo.setAccessibilityFocus(target);
    });
  }
  function changeFilter(action: () => void) {
    action();
    list.current?.scrollToOffset({ offset: 0, animated: false });
    stopFollowing();
    setDetailsOpen(false);
  }
  async function openFile(reference: string) {
    if (!client || !chosen) return;
    try {
      await nativeDocumentPreview.open(
        client.documentRequest(
          reference,
          chosen.event.session,
          agent.home_id || '',
          lang,
        ),
      );
    } catch (error) {
      Alert.alert(
        zh ? '无法打开文件' : 'Could not open file',
        documentErrorMessage(
          error instanceof DocumentError ? error.reason : 'failed',
          zh,
        ),
      );
    }
  }
  function openProfile() {
    if (!chosen) return;
    const event = chosen.event;
    setProfile({
      kind: event.kind === 'user' ? (event.from ? 'sender' : 'human') : 'agent',
      agent: { ...agent, agent: event.agent },
      turn: chosen.turn,
      sender: event.from ?? undefined,
    });
  }
  function copy() {
    if (!chosen) return;
    const value =
      detailTab === 'origin'
        ? JSON.stringify(sourceInfo(chosen.event), null, 2)
        : chosen.event.body || chosen.event.title;
    if (!value) return;
    try {
      Clipboard.setString(value);
    } catch {
      setCopied(false);
      Alert.alert(
        zh ? '无法复制' : 'Could not copy',
        zh ? '请稍后重试。' : 'Please try again.',
      );
      return;
    }
    setCopied(true);
    clearTimeout(copyTimer.current);
    copyTimer.current = setTimeout(() => setCopied(false), 1600);
    AccessibilityInfo.announceForAccessibility(zh ? '已复制' : 'Copied');
  }
  const colors = {
    text: pal.fg,
    dim: pal.fg2,
    code: pal.fg,
    codeBg: pal.raised,
    border: pal.divider,
    link: BRAND,
  };
  const changed =
    !!chosen &&
    !!current &&
    (chosen.event.body !== current.body ||
      JSON.stringify(chosen.event.step) !== JSON.stringify(current.step) ||
      JSON.stringify(chosen.event.context) !== JSON.stringify(current.context));
  const twoColumns = split && showDetails;
  const listVisible = twoColumns || !showDetails;
  const turnTime = chosen ? validTime(chosen.event.time, lang) : '';
  return (
    <SafeAreaView
      accessibilityViewIsModal
      onAccessibilityEscape={showDetails ? closeDetail : onClose}
      edges={['top', 'bottom', 'left', 'right']}
      style={[styles.root, { backgroundColor: pal.bg }]}
      onLayout={event => setWidth(event.nativeEvent.layout.width)}
    >
      <View style={[styles.header, { borderBottomColor: pal.divider }]}>
        <View style={styles.flex}>
          <ChromeText
            testID="trace-session-title"
            accessibilityRole="header"
            numberOfLines={2}
            style={[styles.title, { color: pal.fg }]}
          >
            {primary(agent) || (zh ? '轨迹' : 'Trace')}
          </ChromeText>
          <ChromeText
            numberOfLines={1}
            style={[styles.caption, { color: pal.fg2 }]}
          >
            {zh ? '轨迹' : 'Trace'}
          </ChromeText>
        </View>
        {listVisible && (
          <TouchableOpacity
            testID="trace-search-toggle"
            accessibilityRole="button"
            accessibilityLabel={zh ? '搜索轨迹' : 'Search trace'}
            accessibilityState={{ expanded: searchOpen }}
            onPress={() => {
              setSearchOpen(value => !value);
            }}
            style={styles.iconButton}
          >
            <SearchIcon size={22} color={query ? BRAND : pal.fg2} />
          </TouchableOpacity>
        )}
        <TouchableOpacity
          testID="trace-close"
          accessibilityRole="button"
          accessibilityLabel={zh ? '关闭轨迹' : 'Close trace'}
          onPress={onClose}
          style={styles.iconButton}
        >
          <CloseIcon color={pal.fg2} />
        </TouchableOpacity>
      </View>
      {listVisible && latestEvent && (
        <View
          testID="trace-progress"
          style={[
            styles.progress,
            { backgroundColor: pal.surface, borderColor: pal.divider },
          ]}
        >
          <View style={styles.progressHead}>
            {!history && !searching && (
              <StatusBadge
                status={agent.status}
                size={18}
                errored={agent.status === 'idle' && !!agent.error}
              />
            )}
            <ChromeText style={[styles.progressTitle, { color: pal.fg }]}>
              {searching && !history
                ? zh
                  ? '筛选结果'
                  : 'Filtered records'
                : history
                ? zh
                  ? '历史会话'
                  : 'History'
                : statusLabel(
                    agent.status === 'idle' && agent.error
                      ? 'errored'
                      : agent.status,
                    lang,
                  )}{' '}
              · {zh ? `第 ${latestEvent.turn} 轮` : `Turn ${latestEvent.turn}`}
            </ChromeText>
            <ChromeText
              numberOfLines={1}
              style={[styles.caption, styles.flex, { color: pal.fg2 }]}
            >
              {traceEventLabel(latestEvent, lang)}
            </ChromeText>
          </View>
          <ChromeText
            numberOfLines={1}
            style={[styles.progressText, { color: pal.fg }]}
          >
            {traceEventPreview(latestEvent, lang)}
          </ChromeText>
        </View>
      )}
      {listVisible && (
        <View style={styles.toolbar}>
          {searchOpen && (
            <TextInput
              ref={searchRef}
              testID="trace-search"
              accessibilityLabel={zh ? '搜索轨迹' : 'Search trace'}
              placeholder={zh ? '搜索轨迹' : 'Search trace'}
              placeholderTextColor={pal.fg2}
              value={query}
              onChangeText={value => changeFilter(() => setQuery(value))}
              style={[
                styles.search,
                {
                  backgroundColor: pal.surface,
                  color: pal.fg,
                  borderColor: pal.divider,
                },
              ]}
              returnKeyType="search"
              autoCorrect={false}
              clearButtonMode="while-editing"
            />
          )}
          <ChromeText
            testID="trace-count"
            style={[styles.caption, styles.flex, { color: pal.fg2 }]}
          >
            {groups.length}
            {zh ? ' 轮' : groups.length === 1 ? ' turn' : ' turns'} ·{' '}
            {events.length}
            {zh ? ' 条记录' : events.length === 1 ? ' record' : ' records'}
          </ChromeText>
          {(['turns', 'calls'] as const).map(level => {
            const open = level === 'turns' ? allTurnsOpen : allCallsOpen;
            const name =
              level === 'turns'
                ? zh
                  ? '轮次'
                  : 'Turns'
                : zh
                ? '调用'
                : 'Calls';
            const disabled =
              searching ||
              !groups.length ||
              (level === 'calls' && !groups.some(group => group.calls));
            return (
              <TouchableOpacity
                key={level}
                testID={'trace-fold-' + level}
                accessibilityRole="button"
                accessibilityLabel={
                  zh
                    ? `${open ? '收起' : '展开'}所有${name}`
                    : `${
                        open ? 'Collapse' : 'Expand'
                      } all ${name.toLowerCase()}`
                }
                accessibilityState={{ expanded: open, disabled }}
                disabled={disabled}
                onPress={() => foldAll(level)}
                style={[styles.foldButton, { opacity: disabled ? 0.45 : 1 }]}
              >
                <Chevron open={open} size={16} color={pal.fg2} />
                <ChromeText numberOfLines={1} style={{ color: pal.fg }}>
                  {name}
                </ChromeText>
              </TouchableOpacity>
            );
          })}
          <FilterMenu
            label={
              kind === 'all' && session === 'all'
                ? zh
                  ? '筛选'
                  : 'Filter'
                : kindLabels[kind][zh ? 1 : 0]
            }
            pal={pal}
            lang={lang}
            testID="trace-kind"
            items={[
              ...Object.entries(kindLabels).map(([value, names]) => ({
                key: 'kind-' + value,
                label: names[zh ? 1 : 0],
                icon: 'history' as const,
                selected: value === kind,
                onPress: () =>
                  changeFilter(() => setKind(value as TraceKind | 'all')),
              })),
              ...(index.sessions.length > 1
                ? [
                    {
                      key: 'session-all',
                      label: zh ? '全部会话' : 'All sessions',
                      icon: 'history' as const,
                      selected: session === 'all',
                      onPress: () => changeFilter(() => setSession('all')),
                    },
                    ...index.sessions.map((value, i) => ({
                      key: 'session-' + value.key,
                      label:
                        value.agent +
                        ' · ' +
                        (zh ? '会话 ' : 'Session ') +
                        (i + 1),
                      icon: 'history' as const,
                      selected: value.key === session,
                      onPress: () => changeFilter(() => setSession(value.key)),
                    })),
                  ]
                : []),
            ]}
          />
        </View>
      )}
      {listVisible && dropped > 0 && (
        <ChromeText style={[styles.notice, { color: pal.fg2 }]}>
          {zh
            ? `更早的 ${dropped} 轮未载入`
            : `${dropped} earlier ${
                dropped === 1 ? 'turn is' : 'turns are'
              } not loaded`}
        </ChromeText>
      )}
      {listVisible && earlierAvailable && onLoadEarlier && (
        <TouchableOpacity
          testID="trace-load-earlier"
          accessibilityRole="button"
          accessibilityState={{ disabled: !!loadingEarlier }}
          disabled={loadingEarlier}
          onPress={onLoadEarlier}
          style={styles.loadEarlier}
        >
          <ChromeText style={{ color: pal.fg2 }}>
            {loadingEarlier
              ? zh
                ? '正在加载…'
                : 'Loading…'
              : zh
              ? '加载更早会话'
              : 'Load earlier sessions'}
          </ChromeText>
        </TouchableOpacity>
      )}
      {failed && (
        <View style={styles.errorBar}>
          <ChromeText style={[styles.flex, styles.notice, { color: pal.fg2 }]}>
            {index.events.length
              ? zh
                ? '更新失败，当前显示上次加载的记录。'
                : 'Could not update. Showing loaded records.'
              : zh
              ? '暂时无法读取轨迹。'
              : 'Could not load trace records.'}
          </ChromeText>
          {onRetry && (
            <TouchableOpacity
              accessibilityRole="button"
              onPress={onRetry}
              style={styles.iconButton}
            >
              <ChromeText style={{ color: BRAND }}>
                {zh ? '重试' : 'Retry'}
              </ChromeText>
            </TouchableOpacity>
          )}
        </View>
      )}
      <View
        testID="trace-body"
        style={[styles.body, twoColumns && styles.split]}
      >
        <View
          testID="trace-list-pane"
          style={[
            twoColumns ? styles.splitList : styles.listPane,
            { borderRightColor: pal.divider },
          ]}
        >
          <FlatList
            ref={list}
            accessibilityElementsHidden={!listVisible}
            importantForAccessibility={
              listVisible ? 'auto' : 'no-hide-descendants'
            }
            pointerEvents={listVisible ? 'auto' : 'none'}
            testID="trace-list"
            data={timeline}
            keyExtractor={row => row.key}
            keyboardShouldPersistTaps="handled"
            style={[styles.list, !listVisible && styles.covered]}
            maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
            initialNumToRender={14}
            maxToRenderPerBatch={16}
            windowSize={7}
            onScrollBeginDrag={() => {
              disclosureHold.current = false;
              dragging.current = true;
              stopFollowing();
            }}
            onScrollEndDrag={() => {
              dragging.current = false;
              if (gapRef.current < 60 && !searching) {
                stick.current = true;
                setAtTail(true);
              }
            }}
            onMomentumScrollBegin={() => {
              dragging.current = true;
            }}
            onMomentumScrollEnd={() => {
              dragging.current = false;
            }}
            onScroll={event => {
              const { contentOffset, contentSize, layoutMeasurement } =
                event.nativeEvent;
              gapRef.current =
                contentSize.height - contentOffset.y - layoutMeasurement.height;
              if (
                gapRef.current < 60 &&
                !searching &&
                !detailsOpen &&
                !disclosureHold.current
              ) {
                stick.current = true;
                setAtTail(true);
              } else if (dragging.current) stopFollowing();
            }}
            scrollEventThrottle={16}
            onContentSizeChange={pinTail}
            onLayout={pinTail}
            onScrollToIndexFailed={info => {
              list.current?.scrollToOffset({
                offset: Math.max(0, info.averageItemLength * info.index),
                animated: false,
              });
            }}
            ListEmptyComponent={
              loading ? (
                <ActivityIndicator color={pal.fg2} style={styles.empty} />
              ) : failed ? null : (
                <ChromeText style={[styles.empty, { color: pal.fg2 }]}>
                  {kind === 'context' &&
                  !index.events.some(event => event.kind === 'context')
                    ? zh
                      ? '此会话记录未包含可读取的上下文'
                      : 'No readable context was recorded in this conversation log'
                    : query || kind !== 'all' || session !== 'all'
                    ? zh
                      ? '没有匹配的记录'
                      : 'No matching records'
                    : zh
                    ? '暂无会话记录'
                    : 'No conversation records yet'}
                </ChromeText>
              )
            }
            renderItem={({ item }) => (
              <TraceTimelineRow
                row={item}
                sessionNumber={
                  item.type === 'turn' && index.sessions.length > 1
                    ? index.sessions.findIndex(
                        value => value.key === item.group.sessionKey,
                      ) + 1
                    : undefined
                }
                pal={pal}
                lang={lang}
                selected={chosen?.event.key || ''}
                onSelect={select}
                onToggle={toggleTimeline}
                foldingDisabled={searching}
                onRef={(key, node) => {
                  if (node) rows.current.set(key, node);
                  else rows.current.delete(key);
                }}
              />
            )}
          />
        </View>
        {!atTail && !searching && !showDetails && !!events.length && (
          <TouchableOpacity
            testID="trace-latest"
            accessibilityRole="button"
            accessibilityLabel={zh ? '回到最新记录' : 'Jump to latest'}
            onPress={jumpLatest}
            style={[
              styles.latest,
              { backgroundColor: pal.surface, borderColor: pal.divider },
            ]}
          >
            <ArrowToBottomIcon size={22} color={BRAND} />
          </TouchableOpacity>
        )}
        {showDetails && chosen && (
          <View
            testID="trace-detail"
            style={[
              styles.detail,
              !twoColumns && styles.detailOverlay,
              { backgroundColor: pal.surface },
            ]}
          >
            <View style={styles.detailHeader}>
              <TouchableOpacity
                testID="trace-profile"
                accessibilityRole="button"
                accessibilityLabel={
                  chosen.event.kind === 'context'
                    ? zh
                      ? '查看来源'
                      : 'View source'
                    : zh
                    ? '查看说话人资料'
                    : 'View speaker profile'
                }
                onPress={
                  chosen.event.kind === 'context'
                    ? () => setDetailTab('origin')
                    : openProfile
                }
                style={styles.iconButton}
              >
                {chosen.event.kind === 'context' ? (
                  <SIcon name="info" size={24} color={pal.fg2} />
                ) : chosen.event.kind === 'user' ? (
                  chosen.event.from ? (
                    <SenderAvatar from={chosen.event.from} size={28} />
                  ) : (
                    <UserAvatar size={28} />
                  )
                ) : (
                  <AgentAvatar
                    agent={{ ...agent, agent: chosen.event.agent }}
                    size={28}
                    radius={7}
                    bg={pal.raised}
                    fg={pal.fg}
                  />
                )}
              </TouchableOpacity>
              <ChromeText
                numberOfLines={2}
                style={[styles.recordTitle, styles.flex, { color: pal.fg }]}
              >
                {chosen.event.kind === 'context'
                  ? traceEventLabel(chosen.event, lang)
                  : chosen.event.title || (zh ? '用户消息' : 'User message')}
              </ChromeText>
              <TouchableOpacity
                testID="trace-copy"
                accessibilityRole="button"
                accessibilityLabel={zh ? '复制' : 'Copy'}
                onPress={copy}
                style={styles.iconButton}
              >
                <CopyIcon color={pal.fg2} size={22} />
              </TouchableOpacity>
              <TouchableOpacity
                testID="trace-detail-close"
                accessibilityRole="button"
                accessibilityLabel={zh ? '关闭详情' : 'Close details'}
                onPress={closeDetail}
                style={styles.iconButton}
              >
                <CloseIcon color={pal.fg2} />
              </TouchableOpacity>
            </View>
            {copied && (
              <ChromeText
                accessibilityLiveRegion="polite"
                style={[styles.notice, { color: pal.fg2 }]}
              >
                {zh ? '已复制' : 'Copied'}
              </ChromeText>
            )}
            <View accessibilityRole="tablist" style={styles.detailTabs}>
              {(['overview', 'preview', 'raw', 'origin'] as const).map(
                (tab, i) => (
                  <TouchableOpacity
                    key={tab}
                    testID={'trace-tab-' + tab}
                    accessibilityRole="tab"
                    accessibilityState={{ selected: tab === detailTab }}
                    onPress={() => setDetailTab(tab)}
                    style={[
                      styles.detailTab,
                      {
                        borderBottomColor:
                          tab === detailTab ? BRAND : 'transparent',
                      },
                    ]}
                  >
                    <ChromeText
                      style={{ color: tab === detailTab ? BRAND : pal.fg2 }}
                    >
                      {
                        (zh
                          ? ['概述', '预览', '原始内容', '来源']
                          : ['Overview', 'Preview', 'Raw content', 'Source'])[i]
                      }
                    </ChromeText>
                  </TouchableOpacity>
                ),
              )}
            </View>
            <ScrollView
              testID="trace-detail-scroll"
              style={styles.flex}
              contentContainerStyle={styles.detailContent}
              keyboardShouldPersistTaps="handled"
            >
              <Text selectable style={[styles.meta, { color: pal.fg2 }]}>
                {[
                  chosen.event.agent,
                  zh
                    ? '第 ' + chosen.event.turn + ' 轮'
                    : 'Turn ' + chosen.event.turn,
                  turnTime
                    ? (zh ? '本轮开始于 ' : 'Turn started ') + turnTime
                    : '',
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </Text>
              {!!chosen.event.step?.workdir && (
                <Text selectable style={[styles.meta, { color: pal.fg2 }]}>
                  {zh ? '工作目录：' : 'Working directory: '}
                  {chosen.event.step.workdir}
                </Text>
              )}
              {(detailTab === 'overview' || detailTab === 'origin') &&
                (chosen.event.step?.truncated ||
                  chosen.event.context?.truncated) && (
                  <ChromeText style={[styles.notice, { color: pal.fg2 }]}>
                    {zh
                      ? '内容较长，仅显示已记录的部分。'
                      : 'Only the recorded portion is available.'}
                  </ChromeText>
                )}
              {(detailTab === 'overview' || detailTab === 'origin') &&
                chosen.event.step &&
                !chosen.event.step.input && (
                  <ChromeText style={[styles.notice, { color: pal.fg2 }]}>
                    {zh
                      ? '此调用仅记录了摘要。'
                      : 'Only a call summary was recorded.'}
                  </ChromeText>
                )}
              {changed && (
                <TouchableOpacity
                  testID="trace-update-detail"
                  accessibilityRole="button"
                  onPress={() => {
                    setSelection({
                      event: current!,
                      turn: turns[current!.sourceIndex],
                    });
                    setCopied(false);
                  }}
                  style={styles.update}
                >
                  <ChromeText style={{ color: BRAND }}>
                    {zh ? '有新内容，更新详情' : 'New content · Update details'}
                  </ChromeText>
                </TouchableOpacity>
              )}
              <CopyFeedbackContext.Provider value>
                {detailTab === 'overview' ? (
                  <View testID="trace-overview-facts" style={{ gap: 12 }}>
                    {overviewFacts(chosen.event, lang).map(fact => (
                      <View key={fact.label}>
                        <ChromeText style={{ color: pal.fg2 }}>
                          {fact.label}
                        </ChromeText>
                        <Text
                          selectable
                          style={{
                            color: pal.fg,
                            fontSize,
                            lineHeight: fontSize * 1.6,
                          }}
                        >
                          {fact.value}
                        </Text>
                      </View>
                    ))}
                    <Text
                      selectable
                      numberOfLines={4}
                      style={{ color: pal.fg, fontSize }}
                    >
                      {traceEventPreview(chosen.event, lang)}
                    </Text>
                  </View>
                ) : detailTab === 'origin' ? (
                  <View testID="trace-origin">
                    <CodeText
                      source={JSON.stringify(sourceInfo(chosen.event), null, 2)}
                      hint="json"
                      dark={darkCodeBackground(pal.surface)}
                      lang={lang}
                      fontSize={Math.max(12, fontSize - 1)}
                    />
                  </View>
                ) : chosen.event.kind === 'tool' ||
                  (chosen.event.kind === 'context' &&
                    chosen.event.title === 'Environment') ||
                  detailTab === 'raw' ? (
                  <View
                    style={[
                      styles.sourceBlock,
                      {
                        backgroundColor: pal.raised,
                        borderRadius: 6,
                        padding: 12,
                      },
                    ]}
                  >
                    <CodeText
                      testID="trace-source"
                      source={
                        detailTab === 'preview'
                          ? previewText(chosen.event)
                          : chosen.event.body
                      }
                      hint={
                        detailTab === 'raw'
                          ? 'text'
                          : toolCodeHint(
                              chosen.event.title,
                              chosen.event.step?.inputKind,
                              chosen.event.step?.operation,
                            )
                      }
                      dark={darkCodeBackground(pal.surface)}
                      lang={lang}
                      fontSize={Math.max(12, fontSize - 1)}
                    />
                  </View>
                ) : (
                  <MarkdownView
                    source={previewText(chosen.event)}
                    colors={colors}
                    fontSize={fontSize}
                    selectable
                    lang={lang}
                    openFile={client ? openFile : undefined}
                  />
                )}
              </CopyFeedbackContext.Provider>
            </ScrollView>
          </View>
        )}
      </View>
      {profile && sameIdentity && chosen && (
        <ParticipantProfileModal
          target={profile}
          client={client}
          lang={lang}
          onClose={() => setProfile(null)}
        />
      )}
    </SafeAreaView>
  );
}
function validTime(value: string, lang: Lang) {
  const date = new Date(value);
  return value && !isNaN(date.getTime())
    ? date.toLocaleString(lang === 'zh' ? 'zh-CN' : 'en-US')
    : '';
}
function CloseIcon({ color }: { color: string }) {
  return (
    <Svg width={22} height={22} viewBox="0 0 24 24" fill="none">
      <Path
        d="m6 6 12 12M6 18 18 6"
        stroke={color}
        strokeWidth={1.8}
        strokeLinecap="round"
      />
    </Svg>
  );
}
function FilterMenu({
  label,
  items,
  pal,
  lang,
  testID,
}: {
  label: string;
  items: MenuItem[];
  pal: Palette;
  lang: Lang;
  testID: string;
}) {
  const ref = useRef<View>(null),
    [visible, setVisible] = useState(false),
    [anchor, setAnchor] = useState<MenuAnchor | null>(null);
  return (
    <>
      <TouchableOpacity
        ref={ref}
        testID={testID}
        accessibilityRole="button"
        accessibilityLabel={label}
        style={[
          styles.pill,
          { backgroundColor: pal.surface, borderColor: pal.divider },
        ]}
        onPress={() => {
          setAnchor(null);
          ref.current?.measureInWindow((x, y, width, height) =>
            setAnchor({ x, y, width, height }),
          );
          setVisible(true);
        }}
      >
        <ChromeText numberOfLines={1} style={{ color: pal.fg2 }}>
          {label}
        </ChromeText>
        <SIcon name="chevronDown" size={16} color={pal.fg2} />
      </TouchableOpacity>
      <AnchoredMenu
        visible={visible}
        anchor={anchor}
        title={label}
        sections={[items]}
        pal={pal}
        closeLabel={lang === 'zh' ? '关闭' : 'Close'}
        onClose={() => setVisible(false)}
      />
    </>
  );
}
const styles = StyleSheet.create({
  progress: {
    marginHorizontal: 12,
    marginTop: 10,
    padding: 12,
    borderRadius: 12,
    borderWidth: StyleSheet.hairlineWidth,
    gap: 6,
  },
  progressHead: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  progressTitle: { fontSize: 13, fontWeight: '600', flexShrink: 1 },
  progressText: { fontSize: 14, lineHeight: 21, minHeight: 29 },
  latest: {
    position: 'absolute',
    right: 16,
    bottom: 16,
    width: 44,
    height: 44,
    borderRadius: 22,
    borderWidth: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  errorBar: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 8,
  },
  root: { flex: 1 },
  flex: { flex: 1, minWidth: 0 },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  title: { fontSize: 20, fontWeight: '600' },
  caption: { fontSize: 12, lineHeight: 18 },
  iconButton: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  foldButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    minHeight: 44,
    paddingHorizontal: 6,
  },
  toolbar: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    gap: 8,
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
  },
  search: {
    flexBasis: '100%',
    minHeight: 44,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    paddingHorizontal: 14,
    fontSize: 15,
  },
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    minHeight: 44,
    maxWidth: '100%',
    paddingHorizontal: 14,
    paddingVertical: 10,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
    justifyContent: 'center',
  },
  notice: {
    fontSize: 12,
    lineHeight: 18,
    paddingHorizontal: 14,
    paddingVertical: 8,
  },
  body: { flex: 1, minHeight: 0 },
  split: { flexDirection: 'row' },
  list: { flex: 1 },
  listPane: { flex: 1, minWidth: 0, minHeight: 0 },
  covered: { opacity: 0 },
  detailOverlay: { position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 },
  splitList: {
    flexGrow: 0,
    flexShrink: 0,
    width: 320,
    minWidth: 320,
    maxWidth: 320,
    borderRightWidth: StyleSheet.hairlineWidth,
  },
  record: {
    paddingHorizontal: 16,
    paddingVertical: 14,
    gap: 5,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  recordHead: { flexDirection: 'row', gap: 10, alignItems: 'center' },
  kind: { fontSize: 11, fontWeight: '600' },
  recordTitle: { fontSize: 15, fontWeight: '600', lineHeight: 22 },
  preview: { fontSize: 13, lineHeight: 20 },
  detail: { flex: 1, minWidth: 0 },
  detailHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    padding: 8,
  },
  detailContent: { padding: 16, paddingBottom: 36 },
  meta: { fontSize: 12, lineHeight: 20, marginBottom: 8 },
  detailTabs: { flexDirection: 'row', flexWrap: 'wrap', marginVertical: 8 },
  detailTab: {
    minHeight: 44,
    paddingHorizontal: 10,
    justifyContent: 'center',
    borderBottomWidth: 2,
  },
  source: { lineHeight: 24 },
  mono: { fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace' },
  sourceBlock: { width: '100%', minWidth: 0 },
  update: { minHeight: 44, justifyContent: 'center', paddingVertical: 10 },
  loadEarlier: {
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 16,
  },
  empty: { margin: 24, textAlign: 'center', fontSize: 14, lineHeight: 22 },
});
