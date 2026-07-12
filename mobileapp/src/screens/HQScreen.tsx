// HQScreen — the gtmux HQ command center (hq-command-center change). Opening the
// supervisor (role:"supervisor") lands here, NOT the generic Chat/Terminal
// Detail. Three zones: a status strip (fleet counts + subscription-window %), a
// fleet board (the /api/digest situational-awareness list), and a command
// console (the conversation WITH gtmux HQ + a command bar). All commands are
// HQ-mediated — the command bar addresses the supervisor, which drives the
// fleet; long-pressing a fleet row jumps to that worker's own Detail (where
// direct send already lives), so HQ has no bypass input of its own.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {View, Text, ScrollView, TouchableOpacity, StyleSheet, KeyboardAvoidingView, Platform} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent} from '../api/types';
import {DigestRow, SendPayload, TranscriptTurn} from '../api/client';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {StatusColor} from '../ui/theme';
import {StatusName} from '../api/types';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

// Section rank for the fleet board — needs-you first, exactly like the radar.
const RANK: Record<string, number> = {waiting: 0, working: 1, idle: 2, running: 3};

// planLabel compacts a usage-window label for the status strip: "week (all
// models)" → wk/周, "week (fable)" → the model name, "session" → 5h.
function planLabel(label: string, lang: string): string {
  if (label.includes('all models')) return lang === 'zh' ? '周' : 'wk';
  const m = label.match(/\(([^)]+)\)/);
  if (m) return m[1].charAt(0).toUpperCase() + m[1].slice(1);
  if (label.startsWith('session')) return '5h';
  return label;
}

export function HQScreen({route, navigation}: any) {
  const hq: Agent = route.params.agent;
  const {client, agents, conn} = useAgents();
  const {pal, lang} = useApp();

  const [digest, setDigest] = useState<DigestRow[]>([]);
  const [week, setWeek] = useState<{label: string; pct: number}[]>([]);
  const [res, setRes] = useState<{warn?: string; diskGB?: number; memTier?: string} | null>(null);
  const [turns, setTurns] = useState<TranscriptTurn[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [pending, setPending] = useState<string | undefined>();
  const [selected, setSelected] = useState<DigestRow | null>(null);
  const [boardOpen, setBoardOpen] = useState(true);
  const answeredAt = useRef(0);

  // The live supervisor row (status can change) — fall back to the route agent.
  const live = useMemo(() => agents.find(a => a.pane_id === hq.pane_id) ?? hq, [agents, hq]);

  const t = (en: string, zh: string) => (lang === 'zh' ? zh : en);

  // Poll the fleet digest + subscription windows for the board and status strip.
  useEffect(() => {
    let alive = true;
    const load = () => {
      client.digest().then(d => alive && setDigest(d));
      client.usage().then(u => {
        if (!alive) return;
        const w = (u?.limits?.windows ?? []).map(x => ({label: x.label, pct: x.pct_used}));
        setWeek(w);
        const m = u?.resource?.machine;
        setRes(m ? {warn: m.warn, diskGB: m.disk_free_gb, memTier: m.mem_tier} : null);
      });
    };
    load();
    const id = setInterval(load, 3000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client]);

  // The HQ conversation transcript — refetch on status flip or after a command.
  useEffect(() => {
    let alive = true;
    client
      .transcript(hq.pane_id)
      .then(ts => {
        if (!alive) return;
        setTurns(ts);
        setLoaded(true);
      })
      .catch(() => alive && setLoaded(true));
    return () => {
      alive = false;
    };
  }, [client, hq.pane_id, live.status, pending]);

  // Every command routes through gtmux HQ (send to the supervisor pane).
  const command = useCallback(
    (text: string) => {
      const body = text.trim();
      if (!body) return;
      setPending(body);
      answeredAt.current = Date.now();
      client.send(hq.pane_id, {text: body, enter: true}).finally(() => {
        // Let the optimistic echo linger until the refetch swaps in the real turn.
        setTimeout(() => setPending(undefined), 4000);
      });
    },
    [client, hq.pane_id],
  );

  const onSend = useCallback((p: SendPayload) => p.text && command(p.text), [command]);

  // Fleet board sections (needs-you → working → idle → running), supervisor excluded.
  const sections = useMemo(() => {
    const rows = digest.filter(r => r.role !== 'supervisor');
    const by: Record<string, DigestRow[]> = {};
    for (const r of rows) (by[r.status] ??= []).push(r);
    return Object.keys(by)
      .sort((a, b) => (RANK[a] ?? 9) - (RANK[b] ?? 9))
      .map(s => ({status: s, rows: by[s]}));
  }, [digest]);

  const counts = useMemo(() => {
    const rows = digest.filter(r => r.role !== 'supervisor');
    return {
      waiting: rows.filter(r => r.status === 'waiting').length,
      working: rows.filter(r => r.status === 'working').length,
      idle: rows.filter(r => r.status === 'idle').length,
    };
  }, [digest]);

  // Quick-command chips: always-available + per-selected-target.
  const chips: {label: string; cmd: string}[] = selected
    ? [
        {label: t('continue it', '让它继续'), cmd: t(`Tell ${selected.loc} to continue.`, `让 ${selected.loc} 继续。`)},
        {label: t('inspect', '看它在干嘛'), cmd: t(`What is ${selected.loc} doing right now?`, `${selected.loc} 现在在干什么?`)},
        {label: t('reply for me', '帮我回复'), cmd: t(`${selected.loc} is waiting — recommend a reply.`, `${selected.loc} 在等待,给我一个回复建议。`)},
      ]
    : [
        {label: t('status', '现状'), cmd: t('Status of the whole fleet, needs-you first.', '给我整个舰队的现状,先说需要我的。')},
        {label: t("who's waiting", '谁在等我'), cmd: t('Which agents are waiting on me?', '哪些 agent 在等我?')},
        {label: t('usage', '用量额度'), cmd: t('Token usage + subscription window left.', 'token 用量 + 订阅额度还剩多少。')},
      ];

  return (
    <SafeAreaView style={[styles.root, {backgroundColor: pal.bg}]} edges={['top', 'bottom']}>
      {/* Status strip */}
      <View style={[styles.strip, {borderBottomColor: pal.divider}]}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹</Text>
        </TouchableOpacity>
        <View style={styles.stripMid}>
          <View style={styles.titleRow}>
            <Text style={[styles.title, {color: pal.fg}]}>gtmux HQ</Text>
            <View style={[styles.dot, {backgroundColor: conn === 'live' ? StatusColor.idle : conn === 'connecting' ? '#F59E0B' : StatusColor.waiting}]} />
          </View>
          <Text style={[styles.sub, {color: pal.fg2}]} numberOfLines={1}>
            {t(
              `${counts.waiting} need you · ${counts.working} working · ${counts.idle} idle`,
              `${counts.waiting} 需要你 · ${counts.working} 运行 · ${counts.idle} 空闲`,
            )}
            {week.length > 0 && '  ·  ' + week.map(w => `${planLabel(w.label, lang)} ${w.pct}%`).join(' · ')}
          </Text>
          {res && (res.warn || res.diskGB != null) && (
            <Text style={[styles.sub, {color: res.warn ? '#F59E0B' : pal.fg3}]} numberOfLines={1}>
              {res.warn
                ? '⚠ ' + res.warn
                : `${lang === 'zh' ? '磁盘' : 'disk'} ${res.diskGB}GB · ${lang === 'zh' ? '内存' : 'mem'} ${res.memTier}`}
            </Text>
          )}
        </View>
      </View>

      <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        {/* Fleet board */}
        <View style={[styles.board, boardOpen && {maxHeight: 220}, {borderBottomColor: pal.divider}]}>
          <TouchableOpacity style={styles.boardHead} onPress={() => setBoardOpen(o => !o)} hitSlop={hit}>
            <Text style={[styles.boardTitle, {color: pal.fg2}]}>{t('FLEET', '舰队态势')}</Text>
            <Text style={[styles.boardToggle, {color: pal.fg3}]}>{boardOpen ? '▾' : '▸'}</Text>
          </TouchableOpacity>
          {boardOpen && (
            <ScrollView keyboardShouldPersistTaps="handled">
              {sections.map(sec =>
                sec.rows.map(r => {
                  const sel = selected?.pane_id === r.pane_id;
                  return (
                    <TouchableOpacity
                      key={r.pane_id || r.loc}
                      testID={`hq-fleet-${r.loc}`}
                      activeOpacity={0.7}
                      onPress={() => setSelected(sel ? null : r)}
                      onLongPress={() => {
                        const a = agents.find(x => x.pane_id === r.pane_id);
                        if (a) navigation.navigate('Detail', {agent: a});
                      }}
                      style={[styles.row, sel && {backgroundColor: pal.surface}]}>
                      <View style={[styles.rowDot, {backgroundColor: StatusColor[(r.status as StatusName)] ?? StatusColor.running}]} />
                      <View style={styles.rowMid}>
                        <Text style={[styles.rowLoc, {color: pal.fg}]} numberOfLines={1}>
                          {r.loc} <Text style={{color: pal.fg3}}>{r.agent}</Text>
                          {r.usage_warn ? <Text style={{color: '#F59E0B'}}> · {r.usage_warn}</Text> : null}
                        </Text>
                        <Text style={[styles.rowGoal, {color: pal.fg2}]} numberOfLines={1}>
                          {r.ask ? '⏸ ' + r.ask : r.goal || '—'}
                        </Text>
                      </View>
                    </TouchableOpacity>
                  );
                }),
              )}
              {digest.filter(r => r.role !== 'supervisor').length === 0 && (
                <Text style={[styles.empty, {color: pal.fg3}]}>{t('No other agents.', '暂无其它 agent。')}</Text>
              )}
            </ScrollView>
          )}
        </View>

        {/* Command console (conversation with gtmux HQ) */}
        <View style={styles.flex}>
          <ChatView
            agent={live}
            lines={[]}
            status={live.status}
            fontSize={13}
            pal={pal}
            lang={lang}
            turns={turns}
            loading={!loaded}
            pendingPrompt={pending}
          />
        </View>

        {/* Quick-command chips */}
        <View style={styles.chips}>
          {selected && (
            <View style={[styles.selPill, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <Text style={[styles.selText, {color: pal.fg2}]} numberOfLines={1}>▸ {selected.loc}</Text>
              <TouchableOpacity onPress={() => setSelected(null)} hitSlop={hit}>
                <Text style={{color: pal.fg3}}>×</Text>
              </TouchableOpacity>
            </View>
          )}
          <ScrollView horizontal showsHorizontalScrollIndicator={false} keyboardShouldPersistTaps="handled">
            {chips.map(c => (
              <TouchableOpacity
                key={c.label}
                testID={`hq-chip-${c.label}`}
                style={[styles.chip, {backgroundColor: pal.surface, borderColor: pal.divider}]}
                onPress={() => command(c.cmd)}>
                <Text style={[styles.chipText, {color: pal.fg}]}>{c.label}</Text>
              </TouchableOpacity>
            ))}
          </ScrollView>
        </View>

        {/* Command bar → gtmux HQ */}
        <Composer pal={pal} lang={lang} onSend={onSend} />
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1},
  flex: {flex: 1},
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  back: {fontSize: 30, fontWeight: '300', marginRight: 10, marginTop: -4},
  stripMid: {flex: 1},
  titleRow: {flexDirection: 'row', alignItems: 'center'},
  title: {fontSize: 17, fontWeight: '700'},
  dot: {width: 8, height: 8, borderRadius: 4, marginLeft: 8},
  sub: {fontSize: 12, marginTop: 1},
  board: {borderBottomWidth: StyleSheet.hairlineWidth},
  boardHead: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 14, paddingTop: 8, paddingBottom: 4},
  boardTitle: {fontSize: 11, fontWeight: '700', letterSpacing: 0.5},
  boardToggle: {fontSize: 12},
  row: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 7},
  rowDot: {width: 8, height: 8, borderRadius: 4, marginRight: 10},
  rowMid: {flex: 1},
  rowLoc: {fontSize: 13, fontWeight: '600'},
  rowGoal: {fontSize: 12, marginTop: 1},
  empty: {fontSize: 12, textAlign: 'center', paddingVertical: 16},
  chips: {paddingHorizontal: 10, paddingVertical: 6, gap: 6},
  selPill: {flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', paddingHorizontal: 8, paddingVertical: 3, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, marginBottom: 6, gap: 8, maxWidth: '70%'},
  selText: {fontSize: 12},
  chip: {paddingHorizontal: 12, paddingVertical: 7, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth, marginRight: 8},
  chipText: {fontSize: 13, fontWeight: '600'},
});
