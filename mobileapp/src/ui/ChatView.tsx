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
import {ActivityIndicator, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {AnsiLine} from './ansi';
import {agentMark} from './agentMark';
import {Agent, StatusName} from '../api/types';
import {GtmuxClient, TranscriptTurn} from '../api/client';
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
  client: GtmuxClient;
  paneId: string;
}

function dotColor(status: StatusName): string {
  return status === 'waiting'
    ? StatusColor.waiting
    : status === 'working'
    ? StatusColor.working
    : status === 'idle'
    ? StatusColor.idle
    : StatusColor.running;
}

export function ChatView({agent, lines, status, fontSize, pal, lang, client, paneId}: Props) {
  const [turns, setTurns] = React.useState<TranscriptTurn[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [expanded, setExpanded] = React.useState<Record<number, boolean>>({});
  const scrollRef = React.useRef<ScrollView>(null);

  // Fetch history on mount, on pane change, and when the status flips (a turn has
  // likely just completed — working→idle/waiting), so the final response lands.
  React.useEffect(() => {
    let alive = true;
    client
      .transcript(paneId)
      .then(ts => {
        if (!alive) return;
        setTurns(ts);
        setLoading(false);
        requestAnimationFrame(() => scrollRef.current?.scrollToEnd({animated: false}));
      })
      .catch(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [client, paneId, status]);

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
    <ScrollView
      ref={scrollRef}
      testID={TestIds.detail.chat}
      style={styles.body}
      contentContainerStyle={styles.content}>
      {/* current-state row: avatar + agent + status dot */}
      <View style={styles.stateRow}>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>{agentMark(agent.agent)}</Text>
        </View>
        <View style={styles.stateText}>
          <Text style={[styles.agentName, {color: pal.fg}]} numberOfLines={1}>
            {agent.agent}
          </Text>
          <View style={styles.statusLine}>
            <View style={[styles.dot, {backgroundColor: dotColor(status)}]} />
            <Text style={[styles.statusText, {color: pal.fg3}]} numberOfLines={1}>
              {sub}
            </Text>
          </View>
        </View>
      </View>

      {loading && turns.length === 0 && (
        <View style={styles.center}>
          <ActivityIndicator color={pal.fg3} />
        </View>
      )}

      {!loading && turns.length === 0 && (
        <Text style={[styles.empty, {color: pal.fg3}]}>
          {lang === 'zh'
            ? '暂无对话历史。\n历史来自 agent 的会话记录（需已装 gtmux hooks）——开始对话后即会出现。切到「终端」可看当前屏幕。'
            : 'No conversation history yet.\nHistory comes from the agent’s session log (needs the gtmux hooks). It appears once you start talking. Switch to Terminal for the current screen.'}
        </Text>
      )}

      {/* the conversation: prompt → collapsed steps → final response */}
      {turns.map((t, i) => (
        <View key={i} style={styles.turn}>
          {!!t.prompt && (
            <View style={styles.userRow}>
              <View style={styles.userBubble}>
                <Text style={styles.userText}>{t.prompt}</Text>
              </View>
            </View>
          )}

          {!!t.steps?.length && (
            <TouchableOpacity
              style={styles.stepsToggle}
              activeOpacity={0.7}
              onPress={() => setExpanded(e => ({...e, [i]: !e[i]}))}>
              <Text style={styles.stepsToggleText}>
                {expanded[i] ? '▾ ' : '▸ '}
                {lang === 'zh' ? `${t.steps.length} 个步骤` : `${t.steps.length} step${t.steps.length > 1 ? 's' : ''}`}
              </Text>
            </TouchableOpacity>
          )}
          {expanded[i] &&
            t.steps?.map((s, j) => (
              <View key={j} style={styles.stepRow}>
                <Text style={styles.stepName}>{s.title}</Text>
                {!!s.detail && (
                  <Text style={styles.stepDetail} numberOfLines={1}>
                    {s.detail}
                  </Text>
                )}
              </View>
            ))}

          {!!t.response && (
            <View style={styles.agentRow}>
              <View style={styles.agentAvatar}>
                <Text style={styles.agentAvatarText}>{agentMark(agent.agent)}</Text>
              </View>
              <View style={styles.agentBubble}>
                <Text style={[styles.agentText, {color: pal.fg}]}>{t.response}</Text>
              </View>
            </View>
          )}
        </View>
      ))}

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
  );
}

const styles = StyleSheet.create({
  body: {flex: 1, backgroundColor: '#0D0D0F'},
  content: {padding: 12, gap: 10},
  center: {paddingVertical: 24, alignItems: 'center'},
  empty: {fontSize: 12.5, textAlign: 'center', lineHeight: 19, paddingHorizontal: 16, paddingVertical: 20},
  stateRow: {flexDirection: 'row', alignItems: 'center', gap: 9},
  avatar: {
    width: 26,
    height: 26,
    borderRadius: 7,
    backgroundColor: '#1C1C1F',
    alignItems: 'center',
    justifyContent: 'center',
  },
  avatarText: {fontSize: 10, fontWeight: '700', color: 'rgba(235,235,245,0.7)'},
  stateText: {flex: 1, minWidth: 0},
  agentName: {fontSize: 13.5, fontWeight: '700'},
  statusLine: {flexDirection: 'row', alignItems: 'center', marginTop: 2},
  dot: {width: 6, height: 6, borderRadius: 3, marginRight: 5},
  statusText: {fontSize: 12, flexShrink: 1},

  turn: {gap: 6},
  // user prompt — right-aligned accent bubble.
  userRow: {flexDirection: 'row', justifyContent: 'flex-end'},
  userBubble: {
    maxWidth: '88%',
    backgroundColor: 'rgba(6,182,212,0.16)',
    borderRadius: 13,
    borderTopRightRadius: 3,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  userText: {color: '#E6F7FB', fontSize: 14, lineHeight: 20},

  // collapsed middle steps.
  stepsToggle: {alignSelf: 'flex-start', marginLeft: 35, paddingVertical: 2},
  stepsToggleText: {fontSize: 11.5, color: 'rgba(235,235,245,0.45)', fontWeight: '600'},
  stepRow: {marginLeft: 35, flexDirection: 'row', alignItems: 'baseline', gap: 6},
  stepName: {fontSize: 11, fontWeight: '700', color: '#27C7E6', fontFamily: 'Menlo'},
  stepDetail: {fontSize: 11, color: 'rgba(235,235,245,0.55)', fontFamily: 'Menlo', flexShrink: 1},

  // agent final response — left, with avatar.
  agentRow: {flexDirection: 'row', gap: 8, alignItems: 'flex-start'},
  agentAvatar: {
    width: 26,
    height: 26,
    borderRadius: 7,
    backgroundColor: '#1C1C1F',
    alignItems: 'center',
    justifyContent: 'center',
  },
  agentAvatarText: {fontSize: 10, fontWeight: '700', color: 'rgba(235,235,245,0.7)'},
  agentBubble: {
    flex: 1,
    backgroundColor: '#1C1C1F',
    borderRadius: 13,
    borderTopLeftRadius: 3,
    paddingHorizontal: 12,
    paddingVertical: 9,
  },
  agentText: {fontSize: 14, lineHeight: 20},

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
