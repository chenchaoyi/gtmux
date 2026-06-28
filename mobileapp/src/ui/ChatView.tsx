// ChatView — 对话模式 (B1 / mockup §10): a glance-friendly rendering of the pane,
// as opposed to the raw TUI in 终端 mode. It shows the agent's CURRENT screen as a
// soft bubble + an optional tool card; the approval card (when waiting) and the
// composer live in DetailView, below.
//
// HONESTY NOTE: tmux `capture-pane` only exposes the VISIBLE screen — there is no
// scrollback transcript to reconstruct a full Moshi-style chat log from. So this
// is a faithful "what's on screen right now" glance (cleaner than the raw TUI),
// not a fabricated message history. Switch to 终端 for the complete buffer.

import React from 'react';
import {ScrollView, StyleSheet, Text, View} from 'react-native';
import {AnsiLine} from './ansi';
import {agentMark} from './agentMark';
import {Agent, StatusName} from '../api/types';
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
}

// Detect the most recent tool invocation on screen (Claude Code prints
// "Tool use: Bash(cmd)" / "⏺ Bash(cmd)"). Best-effort, additive — absent → no card.
const TOOL_RE = /(?:Tool use:|⏺|●)\s*([A-Za-z][\w-]*)\s*\(([^)]*)\)/;

function dotColor(status: StatusName): string {
  return status === 'waiting'
    ? StatusColor.waiting
    : status === 'working'
    ? StatusColor.working
    : status === 'idle'
    ? StatusColor.idle
    : StatusColor.running;
}

export function ChatView({agent, lines, status, fontSize, pal, lang}: Props) {
  const plain = lines.map(spans => spans.map(s => s.text).join(''));

  // most-recent tool-use line, scanning from the bottom.
  let tool: {name: string; arg: string} | undefined;
  for (let i = plain.length - 1; i >= 0; i--) {
    const m = plain[i].match(TOOL_RE);
    if (m) {
      tool = {name: m[1], arg: m[2].trim()};
      break;
    }
  }

  // visible-screen tail: drop trailing blank lines, keep the last ~18 rows.
  let end = lines.length;
  while (end > 0 && plain[end - 1].trim() === '') end--;
  const start = Math.max(0, end - 18);
  const shown = lines.slice(start, end);

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

  return (
    <ScrollView
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

      {/* optional tool card — the most recent tool the agent invoked */}
      {tool && (
        <View style={styles.toolCard}>
          <View style={styles.toolHead}>
            <Text style={styles.toolIcon}>🔧</Text>
            <Text style={styles.toolName}>{tool.name}</Text>
          </View>
          {!!tool.arg && (
            <Text style={styles.toolArg} numberOfLines={2}>
              {tool.arg}
            </Text>
          )}
        </View>
      )}

      {/* the current screen, as a soft agent bubble (ANSI-colored, monospace) */}
      <View style={styles.bubbleRow}>
        <View style={styles.bubble}>
          <Text style={[styles.mono, {fontSize, lineHeight}]}>
            {shown.length === 0 ? (
              <Text style={{color: 'rgba(235,235,245,0.4)'}}>
                {lang === 'zh' ? '（屏幕为空）' : '(screen is empty)'}
              </Text>
            ) : (
              shown.map((spans, i) => (
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
              ))
            )}
          </Text>
        </View>
      </View>

      <Text style={[styles.hint, {color: pal.fg3}]}>
        {lang === 'zh'
          ? '对话模式只显示当前屏幕概览 — 切到「终端」看完整缓冲与滚动历史'
          : 'Chat shows the current screen only — switch to Terminal for the full buffer & scrollback'}
      </Text>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  body: {flex: 1, backgroundColor: '#0D0D0F'},
  content: {padding: 12, gap: 11},
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
  toolCard: {
    marginLeft: 35,
    backgroundColor: 'rgba(255,255,255,0.04)',
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: 'rgba(255,255,255,0.09)',
    borderRadius: 10,
    padding: 10,
  },
  toolHead: {flexDirection: 'row', alignItems: 'center', gap: 7, marginBottom: 4},
  toolIcon: {fontSize: 11},
  toolName: {fontSize: 11.5, fontWeight: '700', color: '#27C7E6', fontFamily: 'Menlo'},
  toolArg: {fontSize: 11, color: 'rgba(235,235,245,0.6)', fontFamily: 'Menlo'},
  bubbleRow: {flexDirection: 'row'},
  bubble: {
    flex: 1,
    backgroundColor: '#1C1C1F',
    borderRadius: 12,
    borderTopLeftRadius: 3,
    padding: 11,
  },
  mono: {color: '#D6D6DA', fontFamily: 'Menlo'},
  hint: {fontSize: 11, textAlign: 'center', lineHeight: 16, marginTop: 2},
});
