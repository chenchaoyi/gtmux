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
import { CopyableText, CopyFeedbackContext } from '../ui/CopyableText';
import { CopyIcon } from '../ui/Icons';
import { ChromeText } from '../ui/ChromeText';
import { SIcon } from '../ui/SettingsIcons';
import { StatusBadge } from '../ui/StatusBadge';
import { nativeDocumentPreview } from '../documents/nativePreview';
import { DocumentError, documentErrorMessage } from '../documents/references';
import { build, filter, TraceEvent, TraceKind } from './model';

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
    [source, setSource] = useState(false);
  const [limitsOpen, setLimitsOpen] = useState(false);
  const [follow, setFollow] = useState(agent.status === 'working'),
    [profile, setProfile] = useState<ProfileTarget | null>(null),
    [copied, setCopied] = useState(false);
  const rows = useRef(new Map<string, View>()),
    searchRef = useRef<TextInput>(null);
  const list = useRef<FlatList<TraceEvent>>(null),
    followRef = useRef(follow),
    copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  followRef.current = follow;
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
  }, [client, scope]);
  useEffect(() => () => clearTimeout(copyTimer.current), []);
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
  const current =
    selection && index.events.find(event => event.key === selection.event.key);
  const chosen = sameIdentity && current ? selection : null;
  const showDetails = detailsOpen && !!chosen;
  const activeSession = index.sessions.find(value => value.key === session);
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
    setFollow(false);
    setSelection({ event, turn: turns[event.sourceIndex] });
    setDetailsOpen(true);
    setSource(false);
    setCopied(false);
  }
  function closeDetail() {
    setDetailsOpen(false);
    if (chosen) {
      const at = events.findIndex(event => event.key === chosen.event.key);
      if (at >= 0)
        list.current?.scrollToIndex({
          index: at,
          animated: false,
          viewPosition: 0.3,
        });
    }
    requestAnimationFrame(() => {
      const target = findNodeHandle(
        chosen
          ? rows.current.get(chosen.event.key) || searchRef.current
          : searchRef.current,
      );
      if (target) AccessibilityInfo.setAccessibilityFocus(target);
    });
  }
  function changeFilter(action: () => void) {
    action();
    setFollow(false);
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
    const value = chosen.event.body || chosen.event.title;
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
      JSON.stringify(chosen.event.step) !== JSON.stringify(current.step));
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
          <ChromeText style={[styles.title, { color: pal.fg }]}>
            {zh ? '轨迹' : 'Trace'}
          </ChromeText>
          <ChromeText
            numberOfLines={1}
            style={[styles.caption, { color: pal.fg2 }]}
          >
            {primary(agent)}
          </ChromeText>
        </View>
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
      {listVisible && (
        <View style={styles.toolbar}>
          {!history && (
            <StatusBadge
              status={agent.status}
              size={16}
              errored={agent.status === 'idle' && !!agent.error}
            />
          )}
          <ChromeText style={[styles.caption, { color: pal.fg2 }]}>
            {history
              ? zh
                ? '历史记录'
                : 'History'
              : statusLabel(
                  agent.status === 'idle' && agent.error
                    ? 'errored'
                    : agent.status,
                  lang,
                )}{' '}
            · {events.length}
            {zh ? ' 条记录' : events.length === 1 ? ' record' : ' records'}
          </ChromeText>
          {!history && (
            <>
              <TouchableOpacity
                testID="trace-follow"
                accessibilityRole="button"
                accessibilityState={{ selected: follow }}
                accessibilityLabel={zh ? '跟随最新' : 'Follow latest'}
                style={[
                  styles.pill,
                  {
                    backgroundColor: follow ? pal.rowSelected : pal.surface,
                    borderColor: pal.divider,
                  },
                ]}
                onPress={() => {
                  setFollow(!follow);
                  if (!follow) {
                    setDetailsOpen(false);
                    list.current?.scrollToEnd({ animated: false });
                  }
                }}
              >
                <ChromeText style={{ color: pal.fg2 }}>
                  {zh ? '跟随最新' : 'Follow latest'}
                </ChromeText>
              </TouchableOpacity>
            </>
          )}
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
          <FilterMenu
            label={kindLabels[kind][zh ? 1 : 0]}
            pal={pal}
            lang={lang}
            testID="trace-kind"
            items={Object.entries(kindLabels).map(([value, names]) => ({
              key: value,
              label: names[zh ? 1 : 0],
              icon: 'history',
              selected: value === kind,
              onPress: () =>
                changeFilter(() => setKind(value as TraceKind | 'all')),
            }))}
          />
          {index.sessions.length > 1 && (
            <FilterMenu
              label={
                session === 'all'
                  ? zh
                    ? '全部会话'
                    : 'All sessions'
                  : activeSession
                  ? [activeSession.agent, activeSession.session]
                      .filter(Boolean)
                      .join(' · ')
                  : session
              }
              pal={pal}
              lang={lang}
              testID="trace-session"
              items={[
                {
                  key: 'all',
                  label: zh ? '全部会话' : 'All sessions',
                  icon: 'history',
                  selected: session === 'all',
                  onPress: () => changeFilter(() => setSession('all')),
                },
                ...index.sessions.map(value => ({
                  key: value.key,
                  label: value.agent + ' · ' + value.session,
                  icon: 'history' as const,
                  selected: session === value.key,
                  onPress: () => changeFilter(() => setSession(value.key)),
                })),
              ]}
            />
          )}
        </View>
      )}
      {listVisible && dropped > 0 && (
        <ChromeText style={[styles.notice, { color: pal.fg2 }]}>
          {zh
            ? `更早的 ${dropped} 轮未载入`
            : `${dropped} earlier turns are not loaded`}
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
            data={events}
            keyExtractor={event => event.key}
            keyboardShouldPersistTaps="handled"
            style={[styles.list, !listVisible && styles.covered]}
            maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
            initialNumToRender={14}
            maxToRenderPerBatch={16}
            windowSize={7}
            onScrollBeginDrag={() => setFollow(false)}
            onContentSizeChange={() => {
              if (followRef.current)
                list.current?.scrollToEnd({ animated: false });
            }}
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
                  {query || kind !== 'all' || session !== 'all'
                    ? zh
                      ? '没有匹配的记录'
                      : 'No matching records'
                    : zh
                    ? '暂无会话记录'
                    : 'No conversation records yet'}
                </ChromeText>
              )
            }
            renderItem={({ item, index: rowIndex }) => (
              <TouchableOpacity
                ref={node => {
                  if (node) rows.current.set(item.key, node);
                  else rows.current.delete(item.key);
                }}
                testID={'trace-record-' + item.key}
                accessibilityRole="button"
                accessibilityState={{
                  selected: chosen?.event.key === item.key,
                }}
                onPress={() => select(item)}
                style={[
                  styles.record,
                  {
                    borderBottomColor: pal.divider,
                    backgroundColor:
                      chosen?.event.key === item.key ? pal.rowSelected : pal.bg,
                  },
                ]}
              >
                {(rowIndex === 0 ||
                  events[rowIndex - 1].sessionKey !== item.sessionKey) && (
                  <ChromeText style={[styles.caption, { color: pal.fg2 }]}>
                    {item.agent} · {item.session}
                    {item.boundary?.kind ? ' · /' + item.boundary.kind : ''}
                  </ChromeText>
                )}
                <View style={styles.recordHead}>
                  <ChromeText style={[styles.kind, { color: pal.fg2 }]}>
                    {kindLabels[item.kind][zh ? 1 : 0]}
                  </ChromeText>
                  <ChromeText
                    numberOfLines={1}
                    style={[styles.caption, styles.flex, { color: pal.fg2 }]}
                  >
                    {item.agent} ·{' '}
                    {zh ? '第 ' + item.turn + ' 轮' : 'Turn ' + item.turn}
                  </ChromeText>
                </View>
                <ChromeText
                  numberOfLines={1}
                  style={[styles.recordTitle, { color: pal.fg }]}
                >
                  {item.title ||
                    (item.kind === 'user'
                      ? zh
                        ? '用户消息'
                        : 'User message'
                      : item.agent)}
                </ChromeText>
                <ChromeText
                  numberOfLines={2}
                  style={[styles.preview, { color: pal.fg2 }]}
                >
                  {item.body.replace(/\s+/g, ' ').slice(0, 240)}
                </ChromeText>
              </TouchableOpacity>
            )}
          />
        </View>
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
                  zh ? '查看说话人资料' : 'View speaker profile'
                }
                onPress={openProfile}
                style={styles.iconButton}
              >
                {chosen.event.kind === 'user' ? (
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
                {chosen.event.title || (zh ? '用户消息' : 'User message')}
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
                  chosen.event.session,
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
              {chosen.event.step?.truncated && (
                <ChromeText style={[styles.notice, { color: pal.fg2 }]}>
                  {zh
                    ? '参数较长，仅显示已记录的部分。'
                    : 'Only the recorded portion of this input is available.'}
                </ChromeText>
              )}
              {chosen.event.step && !chosen.event.step.input && (
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
              {chosen.event.kind !== 'tool' && (
                <TouchableOpacity
                  testID="trace-source-toggle"
                  accessibilityRole="button"
                  accessibilityLabel={
                    source
                      ? zh
                        ? '显示预览'
                        : 'Show preview'
                      : zh
                      ? '显示原文'
                      : 'Show source'
                  }
                  onPress={() => setSource(!source)}
                  style={styles.update}
                >
                  <ChromeText style={{ color: BRAND }}>
                    {source
                      ? zh
                        ? '预览'
                        : 'Preview'
                      : zh
                      ? '原文'
                      : 'Source'}
                  </ChromeText>
                </TouchableOpacity>
              )}
              <CopyFeedbackContext.Provider value>
                {chosen.event.kind === 'tool' || source ? (
                  <ScrollView
                    horizontal={!!chosen.event.step?.input}
                    style={styles.codeScroll}
                  >
                    <CopyableText
                      testID="trace-source"
                      lang={lang}
                      selectable
                      rangeUnbounded
                      style={[
                        styles.source,
                        {
                          color: pal.fg,
                          fontSize,
                          lineHeight: fontSize * 1.65,
                        },
                        chosen.event.step?.input && styles.mono,
                      ]}
                    >
                      {chosen.event.body}
                    </CopyableText>
                  </ScrollView>
                ) : (
                  <MarkdownView
                    source={chosen.event.body}
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
      <TouchableOpacity
        testID="trace-limits"
        accessibilityRole="button"
        accessibilityState={{ expanded: limitsOpen }}
        onPress={() => setLimitsOpen(!limitsOpen)}
        style={[
          styles.limits,
          {
            borderTopColor: pal.divider,
            minHeight: 44,
            justifyContent: 'center',
          },
        ]}
      >
        <ChromeText style={{ color: pal.fg2 }}>
          {zh ? '记录范围' : 'About these records'}
        </ChromeText>
      </TouchableOpacity>
      {limitsOpen && (
        <ScrollView
          testID="trace-limits-content"
          style={{ maxHeight: 200, flexGrow: 0 }}
        >
          <Text style={[styles.limits, { color: pal.fg2, borderTopWidth: 0 }]}>
            {zh
              ? '展示会话正文与调用参数，轮次按已加载记录编号。不含工具输出、调用耗时或 Token 用量。'
              : 'Conversation text and call input. Turn numbers refer to loaded history. Tool output, call timing and token usage are not recorded.'}
          </Text>
        </ScrollView>
      )}
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
  toolbar: {
    padding: 12,
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
  source: { lineHeight: 24 },
  mono: { fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace' },
  codeScroll: { flexGrow: 0 },
  update: { minHeight: 44, justifyContent: 'center', paddingVertical: 10 },
  loadEarlier: {
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 16,
  },
  empty: { margin: 24, textAlign: 'center', fontSize: 14, lineHeight: 22 },
  limits: {
    fontSize: 11,
    lineHeight: 17,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
});
