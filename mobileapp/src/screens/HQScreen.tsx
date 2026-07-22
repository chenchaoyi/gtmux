// HQScreen — the gtmux HQ command page (hq-command-page). Opening the supervisor
// (role:"supervisor") lands here, NOT the generic Chat/Terminal Detail.
//
// It is built from what only the supervisor knows. It deliberately does NOT list the
// fleet session-by-session: that list is the radar's, one tap away, and the previous
// design's "fleet board" was a smaller copy of it wedged above the chat — so its own
// answer to being redundant was a collapse control, which left a bare header strip.
//
// Instead: a status strip, an ASSESSMENT line (the deterministic conclusion + the
// supervisor's own situation board), and three switchable zones each given the full body
// height — YOUR CALL (a decision card per waiting session, its ask as the body rather
// than a footnote), ACTIVITY (the severity-tagged event ledger — history, which the
// radar's present-instant view has none of), and CONSOLE (the conversation with HQ).
// The command bar spans all three; every command is HQ-mediated (the HQ page has no
// direct-send input — direct control lives in each worker's own Detail).

import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {
  View,
  Text,
  ScrollView,
  TouchableOpacity,
  StyleSheet,
  KeyboardAvoidingView,
  Modal,
  Platform,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent} from '../api/types';
import {DigestRow, HQBoard, HQEvent, SendPayload, TranscriptTurn} from '../api/client';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {ERRORED_COLOR, StatusColor} from '../ui/theme';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {
  Zone,
  assessment,
  askOf,
  boardFreshness,
  decisions,
  eventMark,
  eventPhrase,
  eventSession,
  fleetCounts,
  hasNewActivity,
  initialZone,
  planLabel,
  relTime,
  sessionName,
  windowNo,
} from './hqZones';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

export function HQScreen({route, navigation}: any) {
  const hq: Agent = route.params.agent;
  const {client, agents, conn, demo} = useAgents();
  const {pal, lang} = useApp();
  const zh = lang === 'zh';
  const t = (en: string, cn: string) => (zh ? cn : en);

  const [digest, setDigest] = useState<DigestRow[]>([]);
  const [week, setWeek] = useState<{label: string; pct: number}[]>([]);
  const [res, setRes] = useState<{warn?: string; diskGB?: number; memTier?: string} | null>(null);
  const [board, setBoard] = useState<HQBoard>({exists: false});
  const [ledger, setLedger] = useState<HQEvent[]>([]);
  const [turns, setTurns] = useState<TranscriptTurn[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [pending, setPending] = useState<string | undefined>();
  const [selected, setSelected] = useState<DigestRow | null>(null);
  const [zone, setZone] = useState<Zone | null>(null); // null until the first digest picks it
  const [boardOpen, setBoardOpen] = useState(false);
  const [seenMark, setSeenMark] = useState(0);
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));

  // The live supervisor row (status can change) — fall back to the route agent.
  const live = useMemo(() => agents.find(a => a.pane_id === hq.pane_id) ?? hq, [agents, hq]);

  // Poll everything the page reads. The board and the ledger change on a human cadence,
  // so they ride the same 3s tick as the digest rather than earning their own timer.
  useEffect(() => {
    let alive = true;
    const load = () => {
      setNow(Math.floor(Date.now() / 1000));
      client.digest().then(d => alive && setDigest(d));
      client
        .usage()
        .then(u => {
          if (!alive) return;
          setWeek((u?.limits?.windows ?? []).map(x => ({label: x.label, pct: x.pct_used})));
          const m = u?.resource?.machine;
          setRes(m ? {warn: m.warn, diskGB: m.disk_free_gb, memTier: m.mem_tier} : null);
        })
        .catch(() => {});
      client
        .hqBoard()
        .then(b => alive && setBoard(b))
        .catch(() => {});
      client
        .hqEvents('notable', 40)
        .then(e => alive && setLedger(e))
        .catch(() => {});
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
      .then(({turns: ts}) => {
        if (!alive) return;
        setTurns(ts);
        setLoaded(true);
      })
      .catch(() => alive && setLoaded(true));
    return () => {
      alive = false;
    };
  }, [client, hq.pane_id, live.status, pending]);

  const calls = useMemo(() => decisions(digest), [digest]);
  const counts = useMemo(() => fleetCounts(digest), [digest]);

  // Land on the block when there is one — that's why you opened HQ. Decided ONCE, from
  // the first digest that arrives, so a later state change never yanks the zone out from
  // under the user mid-read.
  useEffect(() => {
    if (zone === null && digest.length > 0) setZone(initialZone(digest));
  }, [digest, zone]);
  const activeZone: Zone = zone ?? 'console';

  // Reading the feed marks it read; leaving it doesn't un-mark.
  useEffect(() => {
    if (activeZone === 'activity' && ledger.length > 0) {
      setSeenMark(m => Math.max(m, eventMark(ledger[0])));
    }
  }, [activeZone, ledger]);
  const activityNew = hasNewActivity(ledger, seenMark);

  // Every command routes through gtmux HQ (send to the supervisor pane).
  const command = useCallback(
    (text: string) => {
      const body = text.trim();
      if (!body) return;
      setPending(body);
      setZone('console'); // you asked HQ something — show you the answer arriving
      client.send(hq.pane_id, {text: body, enter: true}).finally(() => {
        // Let the optimistic echo linger until the refetch swaps in the real turn.
        setTimeout(() => setPending(undefined), 4000);
      });
    },
    [client, hq.pane_id],
  );
  const onSend = useCallback((p: SendPayload) => p.text && command(p.text), [command]);

  // Open a worker's own Detail — the ONLY place direct input to a worker lives.
  const openWorker = useCallback(
    (row: DigestRow) => {
      const a = agents.find(x => x.pane_id === row.pane_id);
      if (a) navigation.navigate('Detail', {agent: a});
    },
    [agents, navigation],
  );

  // Quick-command chips: per-selected-target when a decision card is picked, else fleet-wide.
  const chips: {label: string; cmd: string}[] = selected
    ? [
        {label: t('reply for me', '帮我回复'), cmd: t(`${selected.loc} is waiting — recommend a reply.`, `${selected.loc} 在等待,给我一个回复建议。`)},
        {label: t('inspect', '看它在干嘛'), cmd: t(`What is ${selected.loc} doing right now?`, `${selected.loc} 现在在干什么?`)},
        {label: t('continue it', '让它继续'), cmd: t(`Tell ${selected.loc} to continue.`, `让 ${selected.loc} 继续。`)},
      ]
    : [
        {label: t('brief', '简报'), cmd: t('Give me a one-line brief of the whole fleet, needs-you first.', '给我一句话的舰队简报,先说需要我的。')},
        {label: t("who's waiting", '谁在等我'), cmd: t('Which agents are waiting on me, and what for?', '哪些 agent 在等我?分别等什么?')},
        {label: t("what's important", '要事'), cmd: t('What are the important events I should know about?', '有哪些我该知道的要紧事?')},
        {label: t('my call', '该我拍板'), cmd: t('What needs my decision right now?', '现在有什么需要我拍板的?')},
      ];

  const tabs: {key: Zone; label: string; badge?: string; dot?: boolean}[] = [
    {key: 'calls', label: t('your call', '该你拍板'), badge: calls.length > 0 ? String(calls.length) : undefined},
    {key: 'activity', label: t('activity', '动态'), dot: activityNew},
    {key: 'console', label: t('console', '对话')},
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
            {demo && (
              <View style={[styles.demoPill, {borderColor: StatusColor.working}]}>
                <Text style={[styles.demoPillText, {color: StatusColor.working}]}>DEMO</Text>
              </View>
            )}
            <View
              style={[
                styles.dot,
                {backgroundColor: conn === 'live' ? StatusColor.idle : conn === 'connecting' ? ERRORED_COLOR : StatusColor.waiting},
              ]}
            />
          </View>
          <Text style={[styles.sub, {color: pal.fg2}]} numberOfLines={1}>
            {t(
              `${counts.waiting} need you · ${counts.working} working · ${counts.idle} idle`,
              `${counts.waiting} 需要你 · ${counts.working} 运行 · ${counts.idle} 空闲`,
            )}
            {week.length > 0 && '  ·  ' + week.map(w => `${planLabel(w.label, zh)} ${w.pct}%`).join(' · ')}
          </Text>
          {res && (res.warn || res.diskGB != null) && (
            <Text style={[styles.sub, {color: res.warn ? ERRORED_COLOR : pal.fg3}]} numberOfLines={1}>
              {res.warn ? '⚠ ' + res.warn : `${zh ? '磁盘' : 'disk'} ${res.diskGB}GB · ${zh ? '内存' : 'mem'} ${res.memTier}`}
            </Text>
          )}
        </View>
      </View>

      {/* Assessment — the conclusion, and the supervisor's own board behind it. */}
      <View style={[styles.assess, {borderBottomColor: pal.divider}]}>
        <Text style={[styles.assessText, {color: calls.length > 0 ? ERRORED_COLOR : pal.fg}]} numberOfLines={2}>
          <Text style={{color: pal.fg3}}>⟣ </Text>
          {assessment(digest, zh)}
        </Text>
        {board.exists && (
          <TouchableOpacity testID="hq-board-open" onPress={() => setBoardOpen(true)} hitSlop={hit} style={styles.boardRow}>
            <Text style={[styles.boardLink, {color: pal.fg3}]} numberOfLines={1}>
              {boardFreshness(board.updated_at, now, zh)}
            </Text>
            <Text style={[styles.boardChevron, {color: pal.fg3}]}>›</Text>
          </TouchableOpacity>
        )}
      </View>

      <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        {/* Zone selector — each tab carries its own signal so a hidden zone still reports itself. */}
        <View style={[styles.tabs, {borderBottomColor: pal.divider}]}>
          {tabs.map(tab => {
            const on = tab.key === activeZone;
            return (
              <TouchableOpacity
                key={tab.key}
                testID={`hq-tab-${tab.key}`}
                style={[styles.tab, on && {borderBottomColor: pal.fg, borderBottomWidth: 2}]}
                onPress={() => setZone(tab.key)}>
                <Text style={[styles.tabText, {color: on ? pal.fg : pal.fg3, fontWeight: on ? '700' : '500'}]}>
                  {tab.label}
                </Text>
                {tab.badge && (
                  <View style={[styles.badge, {backgroundColor: ERRORED_COLOR}]}>
                    <Text style={styles.badgeText}>{tab.badge}</Text>
                  </View>
                )}
                {tab.dot && <View style={[styles.tabDot, {backgroundColor: StatusColor.working}]} />}
              </TouchableOpacity>
            );
          })}
        </View>

        {/* YOUR CALL — one decision card per blocked session. */}
        {activeZone === 'calls' && (
          <ScrollView style={styles.flex} keyboardShouldPersistTaps="handled" contentContainerStyle={styles.pad}>
            {calls.length === 0 ? (
              <Text style={[styles.empty, {color: pal.fg3}]}>
                {t('Nothing needs your decision right now.', '现在没有需要你拍板的事。')}
              </Text>
            ) : (
              calls.map(row => {
                const sel = selected?.pane_id === row.pane_id;
                const win = windowNo(row);
                return (
                  <TouchableOpacity
                    key={row.pane_id || row.loc}
                    testID={`hq-call-${row.loc}`}
                    activeOpacity={0.8}
                    onPress={() => setSelected(sel ? null : row)}
                    style={[
                      styles.card,
                      {backgroundColor: pal.surface, borderColor: sel ? ERRORED_COLOR : pal.divider, borderWidth: sel ? 1.5 : StyleSheet.hairlineWidth},
                    ]}>
                    <View style={styles.cardHead}>
                      <View style={[styles.rowDot, {backgroundColor: StatusColor.waiting}]} />
                      {win !== '' && <Text style={[styles.win, {color: pal.fg3, borderColor: pal.divider}]}>w{win}</Text>}
                      <Text style={[styles.cardTitle, {color: pal.fg}]} numberOfLines={1}>
                        {sessionName(row)} <Text style={{color: pal.fg3, fontWeight: '400'}}>{row.agent}</Text>
                      </Text>
                      <Text style={[styles.cardSince, {color: pal.fg3}]}>{relTime(row.since, now)}</Text>
                    </View>
                    {/* The ask IS the decision — the body of the card, not a footnote. */}
                    <Text style={[styles.ask, {color: pal.fg}]}>{askOf(row, zh)}</Text>
                    {row.goal && row.ask ? (
                      <Text style={[styles.cardGoal, {color: pal.fg3}]} numberOfLines={1}>
                        ↳ {row.goal}
                      </Text>
                    ) : null}
                    <View style={styles.actions}>
                      <TouchableOpacity
                        testID={`hq-call-open-${row.loc}`}
                        style={[styles.action, {borderColor: pal.divider}]}
                        onPress={() => openWorker(row)}>
                        <Text style={[styles.actionText, {color: pal.fg}]}>{t('open session', '打开会话')}</Text>
                      </TouchableOpacity>
                      <TouchableOpacity
                        testID={`hq-call-ask-${row.loc}`}
                        style={[styles.action, {borderColor: pal.divider}]}
                        onPress={() =>
                          command(
                            t(
                              `${row.loc} is waiting on me — what should I answer, and why?`,
                              `${row.loc} 在等我拍板 —— 我该怎么回复?为什么?`,
                            ),
                          )
                        }>
                        <Text style={[styles.actionText, {color: pal.fg}]}>{t('ask HQ', '问参谋长')}</Text>
                      </TouchableOpacity>
                    </View>
                  </TouchableOpacity>
                );
              })
            )}
          </ScrollView>
        )}

        {/* ACTIVITY — the ledger. History; the radar only has the present instant. */}
        {activeZone === 'activity' && (
          <ScrollView style={styles.flex} keyboardShouldPersistTaps="handled" contentContainerStyle={styles.pad}>
            {ledger.length === 0 ? (
              <Text style={[styles.empty, {color: pal.fg3}]}>{t('Nothing notable recently.', '最近没有值得一提的动静。')}</Text>
            ) : (
              ledger.map((e, i) => (
                <View key={`${e.seq ?? e.ts}-${i}`} testID="hq-event" style={styles.eventRow}>
                  <Text style={[styles.eventTime, {color: pal.fg3}]}>{relTime(e.ts, now)}</Text>
                  <View
                    style={[
                      styles.eventDot,
                      {backgroundColor: e.severity === 'important' ? ERRORED_COLOR : pal.fg3},
                    ]}
                  />
                  <View style={styles.flex}>
                    <Text style={[styles.eventHead, {color: pal.fg}]} numberOfLines={1}>
                      {eventSession(e)} <Text style={{color: pal.fg2, fontWeight: '400'}}>{eventPhrase(e, zh)}</Text>
                    </Text>
                    {e.summary ? (
                      <Text style={[styles.eventSummary, {color: pal.fg3}]} numberOfLines={2}>
                        {e.summary}
                      </Text>
                    ) : null}
                  </View>
                </View>
              ))
            )}
          </ScrollView>
        )}

        {/* CONSOLE — the conversation with gtmux HQ. */}
        {activeZone === 'console' && (
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
        )}

        {/* Quick-command chips + command bar — available on every zone. */}
        <View style={styles.chips}>
          {selected && (
            <View style={[styles.selPill, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <Text style={[styles.selText, {color: pal.fg2}]} numberOfLines={1}>
                ▸ {selected.loc}
              </Text>
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
        <Composer pal={pal} lang={lang} onSend={onSend} />
      </KeyboardAvoidingView>

      {/* The situation board, read-only — the supervisor's own working memory. */}
      <Modal visible={boardOpen} animationType="slide" onRequestClose={() => setBoardOpen(false)}>
        <SafeAreaView style={[styles.root, {backgroundColor: pal.bg}]} edges={['top', 'bottom']}>
          <View style={[styles.strip, {borderBottomColor: pal.divider}]}>
            <View style={styles.stripMid}>
              <Text style={[styles.title, {color: pal.fg}]}>{t('Situation board', '作战态势板')}</Text>
              <Text style={[styles.sub, {color: pal.fg3}]}>
                {boardFreshness(board.updated_at, now, zh)} · {t('read-only', '只读')}
              </Text>
            </View>
            <TouchableOpacity testID="hq-board-close" onPress={() => setBoardOpen(false)} hitSlop={hit}>
              <Text style={[styles.close, {color: pal.fg2}]}>✕</Text>
            </TouchableOpacity>
          </View>
          <ScrollView contentContainerStyle={styles.pad}>
            <Text style={[styles.boardText, {color: pal.fg2}]} selectable>
              {board.text ?? ''}
            </Text>
          </ScrollView>
        </SafeAreaView>
      </Modal>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1},
  flex: {flex: 1},
  pad: {paddingHorizontal: 14, paddingVertical: 10},
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  back: {fontSize: 30, fontWeight: '300', marginRight: 10, marginTop: -4},
  close: {fontSize: 18, fontWeight: '400', paddingHorizontal: 4},
  stripMid: {flex: 1},
  titleRow: {flexDirection: 'row', alignItems: 'center'},
  demoPill: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 0.5, marginLeft: 8},
  demoPillText: {fontSize: 9, fontWeight: '700', letterSpacing: 0.06},
  title: {fontSize: 17, fontWeight: '700'},
  dot: {width: 8, height: 8, borderRadius: 4, marginLeft: 8},
  sub: {fontSize: 12, marginTop: 1},

  assess: {paddingHorizontal: 14, paddingTop: 9, paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  assessText: {fontSize: 14, fontWeight: '600', lineHeight: 19},
  boardRow: {flexDirection: 'row', alignItems: 'center', marginTop: 4},
  boardLink: {fontSize: 11},
  boardChevron: {fontSize: 13, marginLeft: 3},
  boardText: {fontSize: 12.5, lineHeight: 19, fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace'},

  tabs: {flexDirection: 'row', borderBottomWidth: StyleSheet.hairlineWidth},
  tab: {flexDirection: 'row', alignItems: 'center', justifyContent: 'center', flex: 1, paddingVertical: 9, borderBottomColor: 'transparent', borderBottomWidth: 2},
  tabText: {fontSize: 13},
  badge: {marginLeft: 6, minWidth: 17, height: 17, borderRadius: 8.5, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 4},
  badgeText: {fontSize: 10.5, fontWeight: '700', color: '#1A1206'},
  tabDot: {width: 6, height: 6, borderRadius: 3, marginLeft: 6},

  card: {borderRadius: 12, padding: 12, marginBottom: 10},
  cardHead: {flexDirection: 'row', alignItems: 'center'},
  rowDot: {width: 8, height: 8, borderRadius: 2, marginRight: 8}, // square = waiting (状态三重编码)
  win: {fontSize: 10, fontWeight: '700', paddingHorizontal: 4, borderWidth: StyleSheet.hairlineWidth, borderRadius: 4, marginRight: 8, overflow: 'hidden'},
  cardTitle: {flex: 1, fontSize: 13.5, fontWeight: '700'},
  cardSince: {fontSize: 11, marginLeft: 8, fontVariant: ['tabular-nums']},
  ask: {fontSize: 14, lineHeight: 20, marginTop: 8, fontWeight: '500'},
  cardGoal: {fontSize: 11.5, marginTop: 5},
  actions: {flexDirection: 'row', marginTop: 11, gap: 8},
  action: {flex: 1, alignItems: 'center', paddingVertical: 8, borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  actionText: {fontSize: 12.5, fontWeight: '600'},

  eventRow: {flexDirection: 'row', alignItems: 'flex-start', paddingVertical: 7},
  eventTime: {fontSize: 11, width: 34, fontVariant: ['tabular-nums']},
  eventDot: {width: 6, height: 6, borderRadius: 3, marginTop: 5, marginRight: 9},
  eventHead: {fontSize: 13, fontWeight: '600'},
  eventSummary: {fontSize: 11.5, marginTop: 2, lineHeight: 16},

  empty: {fontSize: 13, textAlign: 'center', paddingVertical: 30, lineHeight: 19},
  chips: {paddingHorizontal: 10, paddingVertical: 6, gap: 6},
  selPill: {flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', paddingHorizontal: 8, paddingVertical: 3, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, marginBottom: 6, gap: 8, maxWidth: '70%'},
  selText: {fontSize: 12},
  chip: {paddingHorizontal: 12, paddingVertical: 7, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth, marginRight: 8},
  chipText: {fontSize: 13, fontWeight: '600'},
});
