// ChatView — 对话模式 (B1 / mockup §10): a glance-friendly conversation view.
// It shows the REAL chat history (GET /api/transcript): each turn = your prompt,
// the agent's collapsed middle steps (tap to expand), and its final response.
// Below the history, a compact "live" card shows the current screen while the
// agent is working. Switch to 终端 for the full raw TUI + scrollback.
//
// History comes from the agent's on-disk session log (parsed server-side per
// agent — Claude + Codex), so it survives across the visible-screen window that
// `capture-pane` alone can't reconstruct. It's available once the pane has a
// resume record (the gtmux hooks capture the agent + session id).

import React from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {AnsiLine} from './ansi';
import {AgentAvatar} from './AgentAvatar';
import {JumpToBottom} from './JumpToBottom';
import {UserAvatar} from './UserAvatar';
import {MarkdownView, MdColors} from './MarkdownView';
import {fmtTurnTime} from './time';
import {nativeFontFamily} from './term';
import {BrandLoader} from './BrandLoader';
import {Agent, StatusName} from '../api/types';
import {TranscriptSegment, TranscriptTurn} from '../api/client';
import {statusLabel, Lang} from '../i18n';
import {StatusColor} from './theme';
import {TestIds} from '../constants/testIds';

interface Props {
  agent: Agent;
  lines: AnsiLine[];
  status: StatusName;
  fontSize: number;
  pal: any;
  lang: Lang;
  // Transcript is fetched + cached by DetailScreen (survives mode switches), passed
  // in here so 终端→对话 shows instantly instead of re-fetching every mount.
  turns: TranscriptTurn[];
  loading: boolean;
  // The just-sent prompt, echoed optimistically as a trailing bubble until the
  // transcript refetch catches up — so sending feels instant over the tunnel.
  pendingPrompt?: string;
  // Font-family config (same value the terminal uses), so chat text matches the
  // terminal font — resolved via the shared nativeFontFamily().
  fontPref?: string;
}

// The chat surface is ALWAYS dark (terminal aesthetic — see styles.body), so its
// text is light-on-dark regardless of the app's light/dark appearance. Using the
// theme palette (pal.fg) here made the agent name + response invisible in light
// mode (dark text on the dark bubble). These fixed colors keep it readable.
// Explicit selection tint for long-press copy. iOS won't paint the DEFAULT
// highlight on the chat's colored/nested <Text selectable> (the same quirk
// NativeTerm hits), so we force a visible band — same blue the terminal overlay
// uses. Without this, long-press copies but shows no highlight.
const SEL_COLOR = 'rgba(52,120,247,0.5)';
const hitSlop = {top: 8, bottom: 8, left: 12, right: 12};
const CHAT_FG = 'rgba(255,255,255,0.92)'; // primary text on the dark chat surface
const CHAT_FG_DIM = 'rgba(235,235,245,0.5)'; // secondary / muted text

// Markdown colors for the agent response — all fixed light-on-dark (the chat
// surface is always dark), so markdown stays readable in light app mode too.
const MD_COLORS: MdColors = {
  text: CHAT_FG,
  dim: CHAT_FG_DIM,
  code: '#E6F7FB',
  codeBg: 'rgba(255,255,255,0.08)',
  border: 'rgba(255,255,255,0.16)',
  link: '#27C7E6',
};

function dotColor(status: StatusName): string {
  return status === 'waiting'
    ? StatusColor.waiting
    : status === 'working'
    ? StatusColor.working
    : status === 'idle'
    ? StatusColor.idle
    : StatusColor.running;
}

export function ChatView({agent, lines, status, fontSize, lang, turns, loading, pendingPrompt, fontPref}: Props) {
  const fontFamily = nativeFontFamily(fontPref); // match the terminal font (shared resolver)
  const [expanded, setExpanded] = React.useState<Record<string, boolean>>({}); // per step-group
  const scrollRef = React.useRef<ScrollView>(null);
  // Show the jump-to-bottom FAB once you've scrolled up away from the live tail.
  const [atBottom, setAtBottom] = React.useState(true);
  const onScroll = (e: any) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    setAtBottom(contentSize.height - contentOffset.y - layoutMeasurement.height < 60);
  };
  const jumpToBottom = () => {
    scrollRef.current?.scrollToEnd({animated: true});
    setAtBottom(true);
  };

  // Collapse/expand all agent replies — so you can scan prompts to find a turn, then
  // open just that one. `collapsedAll` is the default; `turnOpen[i]` overrides a
  // single turn while collapsed. This state lives in ChatView (kept mounted across
  // mode switches), so the collapse layout persists when you flip 终端↔对话.
  const [collapsedAll, setCollapsedAll] = React.useState(false);
  const [turnOpen, setTurnOpen] = React.useState<Record<number, boolean>>({});
  // A very long USER prompt (e.g. a pasted context dump) is auto-collapsed to a few
  // lines with a tap-to-expand toggle, so it doesn't bury the conversation.
  const [promptOpen, setPromptOpen] = React.useState<Record<number, boolean>>({});
  const togglePrompt = (i: number) => setPromptOpen(o => ({...o, [i]: !o[i]}));
  // collapse/expand-ALL re-renders every turn (expand rebuilds all markdown), which
  // blocks JS for a beat — show the branded loader over it (it animates natively, so
  // it stays smooth through the hitch). Defer the state change one frame so the
  // loader paints first, then hold it a perceptible minimum.
  const [busy, setBusy] = React.useState(false);
  const runBusy = (fn: () => void) => {
    setBusy(true);
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        fn();
        setTimeout(() => setBusy(false), 260);
      }),
    );
  };
  const collapseAll = () => runBusy(() => {
    setCollapsedAll(true);
    setTurnOpen({});
  });
  const expandAll = () => runBusy(() => {
    setCollapsedAll(false);
    setTurnOpen({});
  });
  const toggleTurn = (i: number) => setTurnOpen(o => ({...o, [i]: !o[i]}));

  // Jump to the latest turn whenever the history grows (kept in sync by the parent).
  React.useEffect(() => {
    requestAnimationFrame(() => scrollRef.current?.scrollToEnd({animated: false}));
    setAtBottom(true);
  }, [turns.length]);

  // Per-turn time labels, with adjacent duplicates blanked so a burst of turns in
  // the same minute shows the label once (a centered separator, chat-app style).
  const timeLabels = React.useMemo(() => {
    let prev = '';
    return turns.map(t => {
      const l = fmtTurnTime(t.time, lang);
      if (l && l !== prev) {
        prev = l;
        return l;
      }
      return '';
    });
  }, [turns, lang]);

  const lineHeight = Math.round(fontSize * 1.4);
  const sub =
    status === 'waiting'
      ? lang === 'zh'
        ? '等你回应 — 用下面的审批卡或直接输入'
        : 'Waiting on you — use the approval card below or type'
      : status === 'working'
      ? lang === 'zh'
        ? '正在运行…'
        : 'Working…'
      : statusLabel(status, lang);

  // current-screen tail, for the live card while working.
  const plain = lines.map(spans => spans.map(s => s.text).join(''));
  let end = lines.length;
  while (end > 0 && plain[end - 1].trim() === '') end--;
  const liveShown = lines.slice(Math.max(0, end - 14), end);

  return (
    <View style={styles.root}>
      {/* FIXED one-tap collapse / expand-all bar (outside the scroll, so it's always
          reachable even after the chat auto-scrolls to the latest turn). */}
      {turns.length > 0 && (
        <View style={styles.collapseBar}>
          <TouchableOpacity testID={TestIds.detail.collapseAll} accessibilityLabel={TestIds.detail.collapseAll} onPress={collapsedAll ? expandAll : collapseAll} activeOpacity={0.7} hitSlop={hitSlop}>
            <Text style={styles.collapseBarText}>
              {collapsedAll
                ? lang === 'zh' ? '▾ 展开全部' : '▾ Expand all'
                : lang === 'zh' ? '▸ 折叠全部' : '▸ Collapse all'}
            </Text>
          </TouchableOpacity>
        </View>
      )}
    <ScrollView
      ref={scrollRef}
      testID={TestIds.detail.chat}
      style={styles.body}
      contentContainerStyle={styles.content}
      onScroll={onScroll}
      scrollEventThrottle={16}>
      {/* current-state row: avatar + agent + status dot */}
      <View style={styles.stateRow}>
        <AgentAvatar agent={agent} size={26} radius={7} bg="#1C1C1F" fg="rgba(235,235,245,0.7)" />
        <View style={styles.stateText}>
          <Text style={[styles.agentName, {color: CHAT_FG}]} numberOfLines={1}>
            {agent.agent}
          </Text>
          <View style={styles.statusLine}>
            <View style={[styles.dot, {backgroundColor: dotColor(status)}]} />
            <Text style={[styles.statusText, {color: CHAT_FG_DIM}]} numberOfLines={1}>
              {sub}
            </Text>
          </View>
        </View>
      </View>

      {loading && turns.length === 0 && (
        <View style={styles.center}>
          <BrandLoader size={36} neutral="rgba(255,255,255,0.18)" />
        </View>
      )}

      {!loading && turns.length === 0 && (
        <Text style={[styles.empty, {color: CHAT_FG_DIM}]}>
          {lang === 'zh'
            ? '暂无对话历史。\n历史来自 agent 的会话记录（需已装 gtmux hooks）——开始对话后即会出现。切到「终端」可看当前屏幕。'
            : 'No conversation history yet.\nHistory comes from the agent’s session log (needs the gtmux hooks). It appears once you start talking. Switch to Terminal for the current screen.'}
        </Text>
      )}

      {/* the conversation: prompt → interleaved (text bubble / step group) segments */}
      {turns.map((t, i) => {
        // each segment = one assistant message's text bubble + the tool steps that
        // ran after it; rendering them in order puts intermediate process BETWEEN
        // separate speech bubbles. Fall back to the joined response when no segments.
        const segs: TranscriptSegment[] = t.segments?.length ? t.segments : t.response ? [{text: t.response}] : [];
        const firstText = segs.findIndex(s => !!s.text); // first reply bubble (preview + hasReply)
        const hasReply = firstText !== -1;
        const open = collapsedAll ? !!turnOpen[i] : true; // collapsed turns hide the reply
        // one-line reply preview, shown next to the toggle while collapsed so a
        // promptless turn is still locatable.
        const preview = !open && hasReply ? (segs[firstText].text || '').replace(/\s*\n+\s*/g, ' ').trim().slice(0, 140) : '';
        return (
          <View key={i} style={styles.turn}>
            {!!timeLabels[i] && <Text style={styles.timeLabel}>{timeLabels[i]}</Text>}
            {!!t.prompt && (() => {
              const nLines = t.prompt.split('\n').length;
              const long = t.prompt.length > 600 || nLines > 12;
              const collapsed = long && !promptOpen[i];
              return (
                <>
                  <View style={styles.userRow}>
                    <View style={styles.userBubble}>
                      <Text
                        selectable
                        selectionColor={SEL_COLOR}
                        numberOfLines={collapsed ? 8 : undefined}
                        style={[styles.userText, {fontFamily, fontSize, lineHeight}]}>
                        {t.prompt}
                      </Text>
                    </View>
                    <UserAvatar size={26} />
                  </View>
                  {long && (
                    <TouchableOpacity onPress={() => togglePrompt(i)} activeOpacity={0.7} style={styles.userToggle}>
                      <Text style={styles.userToggleText}>
                        {promptOpen[i]
                          ? lang === 'zh' ? '▴ 收起' : '▴ Collapse'
                          : lang === 'zh' ? `▾ 展开全部（${nLines} 行）` : `▾ Show all (${nLines} lines)`}
                      </Text>
                    </TouchableOpacity>
                  )}
                </>
              );
            })()}

            {/* COLLAPSED: keep the agent avatar + a tappable preview bubble, so a
                collapsed turn still reads as a normal conversation row (just short). */}
            {collapsedAll && hasReply && !open && (
              <TouchableOpacity testID={TestIds.detail.collapsedReply} onPress={() => toggleTurn(i)} activeOpacity={0.7}>
                <View style={styles.agentRow}>
                  <AgentAvatar agent={agent} size={26} radius={7} bg="#1C1C1F" fg="rgba(235,235,245,0.7)" />
                  <View style={[styles.agentBubble, styles.collapsedBubble]}>
                    <Text style={[styles.collapsedPreview, {fontFamily, fontSize: fontSize - 0.5}]} numberOfLines={2}>
                      {preview || (lang === 'zh' ? '（无文本回复）' : '(no text reply)')}
                    </Text>
                    <Text style={styles.collapsedHint}>{lang === 'zh' ? '轻点展开 ▸' : 'Tap to expand ▸'}</Text>
                  </View>
                </View>
              </TouchableOpacity>
            )}

            {open &&
            segs.map((seg, k) => {
              const key = `${i}-${k}`;
              return (
                <View key={k} style={styles.segBlock}>
                  {!!seg.text && (
                    <View style={styles.agentRow}>
                      {/* every agent bubble carries the avatar — a turn can split into
                          many bubbles across tool calls; one-per-turn left the
                          follow-ups looking orphaned. */}
                      <AgentAvatar agent={agent} size={26} radius={7} bg="#1C1C1F" fg="rgba(235,235,245,0.7)" />
                      <View style={styles.agentBubble}>
                        <MarkdownView source={seg.text} colors={MD_COLORS} fontSize={fontSize} fontFamily={fontFamily} selectable selectionColor={SEL_COLOR} />
                      </View>
                    </View>
                  )}
                  {!!seg.steps?.length && (
                    <>
                      <TouchableOpacity
                        style={styles.stepsToggle}
                        activeOpacity={0.7}
                        onPress={() => setExpanded(e => ({...e, [key]: !e[key]}))}>
                        <Text style={styles.stepsToggleText}>
                          {expanded[key] ? '▾ ' : '▸ '}
                          {lang === 'zh' ? `${seg.steps.length} 个步骤` : `${seg.steps.length} step${seg.steps.length > 1 ? 's' : ''}`}
                        </Text>
                      </TouchableOpacity>
                      {expanded[key] &&
                        seg.steps.map((s, j) => (
                          <View key={j} style={styles.stepRow}>
                            <Text style={styles.stepName}>{s.title}</Text>
                            {!!s.detail && (
                              <Text style={styles.stepDetail} numberOfLines={1}>
                                {s.detail}
                              </Text>
                            )}
                          </View>
                        ))}
                    </>
                  )}
                </View>
              );
            })}

            {/* re-collapse affordance when this turn was individually expanded */}
            {collapsedAll && hasReply && open && (
              <TouchableOpacity onPress={() => toggleTurn(i)} activeOpacity={0.7} style={styles.replyToggle}>
                <Text style={styles.replyToggleText}>{lang === 'zh' ? '▾ 收起回复' : '▾ Collapse reply'}</Text>
              </TouchableOpacity>
            )}
          </View>
        );
      })}

      {/* optimistic echo: the just-sent prompt, until the transcript catches up */}
      {!!pendingPrompt && (turns.length === 0 || turns[turns.length - 1].prompt !== pendingPrompt) && (
        <View style={styles.userRow}>
          <View style={[styles.userBubble, styles.userBubblePending]}>
            <Text style={[styles.userText, {fontFamily, fontSize, lineHeight}]}>{pendingPrompt}</Text>
          </View>
          <UserAvatar size={26} />
        </View>
      )}

      {/* live card: the current screen while the agent is working */}
      {status === 'working' && liveShown.length > 0 && (
        <View style={styles.liveCard}>
          <Text style={styles.liveLabel}>{lang === 'zh' ? '正在进行' : 'Live'}</Text>
          <Text style={[styles.mono, {fontSize, lineHeight}]}>
            {liveShown.map((spans, i) => (
              <Text key={i}>
                {i > 0 ? '\n' : ''}
                {spans.length === 0
                  ? ' '
                  : spans.map((s, j) => (
                      <Text key={j} style={{color: s.color, fontWeight: s.bold ? '700' : '400'}}>
                        {s.text}
                      </Text>
                    ))}
              </Text>
            ))}
          </Text>
        </View>
      )}
    </ScrollView>
      {/* branded loader over the list while collapse/expand-all re-renders. */}
      {busy && (
        <View style={styles.busyOverlay} pointerEvents="none">
          <BrandLoader size={40} neutral="rgba(255,255,255,0.2)" />
        </View>
      )}
      <JumpToBottom visible={!atBottom && turns.length > 0} onPress={jumpToBottom} />
    </View>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1, backgroundColor: '#0D0D0F'},
  body: {flex: 1, backgroundColor: '#0D0D0F'},
  content: {padding: 12, gap: 10},
  center: {paddingVertical: 24, alignItems: 'center'},
  empty: {fontSize: 12.5, textAlign: 'center', lineHeight: 19, paddingHorizontal: 16, paddingVertical: 20},
  stateRow: {flexDirection: 'row', alignItems: 'center', gap: 9},
  // fixed collapse/expand-all bar above the scroll — right-aligned, quiet.
  collapseBar: {flexDirection: 'row', justifyContent: 'flex-end', paddingHorizontal: 14, paddingTop: 8, paddingBottom: 4},
  collapseBarText: {fontSize: 12.5, color: '#27C7E6', fontWeight: '600'},
  // per-turn reply toggle shown while collapsed (chevron + preview).
  replyToggle: {alignSelf: 'flex-start', marginLeft: 35, paddingVertical: 3, maxWidth: '88%'},
  replyToggleText: {fontSize: 12.5, color: 'rgba(235,235,245,0.55)'},
  stateText: {flex: 1, minWidth: 0},
  agentName: {fontSize: 13.5, fontWeight: '700'},
  statusLine: {flexDirection: 'row', alignItems: 'center', marginTop: 2},
  dot: {width: 6, height: 6, borderRadius: 3, marginRight: 5},
  statusText: {fontSize: 12, flexShrink: 1},

  turn: {gap: 6},
  // centered time separator above a turn (chat-app style), deliberately quiet.
  timeLabel: {fontSize: 10.5, color: CHAT_FG_DIM, textAlign: 'center', alignSelf: 'center', letterSpacing: 0.3, marginTop: 2},
  // user prompt — right-aligned accent bubble + the human avatar on the right.
  userRow: {flexDirection: 'row', justifyContent: 'flex-end', alignItems: 'flex-start', gap: 8},
  userBubble: {
    maxWidth: '82%',
    backgroundColor: 'rgba(6,182,212,0.16)',
    borderRadius: 13,
    borderTopRightRadius: 3,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  userBubblePending: {opacity: 0.55},
  userText: {color: '#E6F7FB', fontSize: 14, lineHeight: 20},
  // expand/collapse toggle for an auto-collapsed long prompt — right-aligned under
  // the bubble (marginRight = avatar 26 + gap 8, so it sits under the bubble's edge).
  userToggle: {alignSelf: 'flex-end', marginRight: 34, marginTop: 3, paddingVertical: 2},
  userToggleText: {fontSize: 12.5, color: 'rgba(230,247,251,0.6)', fontWeight: '600'},

  // collapsed middle steps.
  stepsToggle: {alignSelf: 'flex-start', marginLeft: 35, paddingVertical: 2},
  stepsToggleText: {fontSize: 11.5, color: 'rgba(235,235,245,0.45)', fontWeight: '600'},
  stepRow: {marginLeft: 35, flexDirection: 'row', alignItems: 'baseline', gap: 6},
  stepName: {fontSize: 11, fontWeight: '700', color: '#27C7E6', fontFamily: 'Menlo'},
  stepDetail: {fontSize: 11, color: 'rgba(235,235,245,0.55)', fontFamily: 'Menlo', flexShrink: 1},

  // one reply segment = a text bubble + its trailing step group (small inner gap).
  segBlock: {gap: 4},

  // agent reply bubble — left, avatar on every bubble.
  agentRow: {flexDirection: 'row', gap: 8, alignItems: 'flex-start'},
  agentBubble: {
    flex: 1,
    backgroundColor: '#1C1C1F',
    borderRadius: 13,
    borderTopLeftRadius: 3,
    paddingHorizontal: 12,
    paddingVertical: 9,
  },
  // collapsed agent bubble — a quieter, truncated version of agentBubble.
  collapsedBubble: {backgroundColor: 'rgba(255,255,255,0.05)', paddingVertical: 8},
  collapsedPreview: {color: 'rgba(235,235,245,0.62)'},
  collapsedHint: {fontSize: 10.5, color: '#27C7E6', marginTop: 4, fontWeight: '600'},
  busyOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: 'rgba(13,13,15,0.55)',
  },

  liveCard: {
    backgroundColor: 'rgba(6,182,212,0.06)',
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: 'rgba(6,182,212,0.25)',
    borderRadius: 10,
    padding: 10,
    marginTop: 4,
  },
  liveLabel: {fontSize: 10, fontWeight: '700', color: '#27C7E6', letterSpacing: 0.5, marginBottom: 5},
  mono: {color: '#D6D6DA', fontFamily: 'Menlo'},
});
