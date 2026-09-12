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

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {Animated, View, Text, ScrollView, TouchableOpacity, StyleSheet, KeyboardAvoidingView, Platform} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent} from '../api/types';
import {Debug} from '../debug';
import {DigestRow, HQBoard, HQEvent, KnowledgeIndex, SendPayload, TranscriptTurn, UsageReport} from '../api/client';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {ERRORED_COLOR, StatusColor} from '../ui/theme';
import {Composer} from '../ui/Composer';
import {historyScope} from '../state/history';
import {acts, tally} from './hqActsModel';
import {AnsiLine, parseAnsi} from '../ui/ansi';
import {SessionReset} from '../ui/chatWindow';
import {ChatView} from '../ui/ChatView';
import {CHROME_ANIM_MS, ChromeState, chromeDecision} from '../ui/liveEdge';
import {READING_WIDTH, SizeClass} from '../ui/layout';
import {KeyBus} from '../keys/bus';
import {BoardSheet} from './BoardSheet';
import {KnowledgeSheet} from './KnowledgeSheet';
import {UsageSheet} from './UsageSheet';
import {knowledgeValue, knowledgeOverdue} from './knowledgeModel';
import {SendFailedBar} from '../ui/SendFailedBar';
import {useWorkspace} from '../state/WorkspaceContext';
import {parseBoardSections} from './boardSections';
import {ActsView, HQActs} from './HQActs';
import {acts as supervisorActs} from './hqActsModel';
import {HQHeader} from './HQHeader';
import {ResourceState, WindowPct, headerModel, usageDoorValue} from './hqHeaderModel';
import {
  Zone,
  running,
  assessment,
  askOf,
  boardAge,
  decisions,
  eventMark,
  hasNewActivity,
  initialZone,
  relTime,
  sessionName,
  windowNo,
} from './hqZones';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

/** The phone's route: the view with a back button. The iPad's main pane renders HQView. */
export function HQScreen({route, navigation}: any) {
  return <HQView agent={route.params.agent} prefill={route.params.prefill} onBack={() => navigation.goBack()} />;
}

export function HQView({agent: hq, onBack, layout = 'compact'}: {agent: Agent; prefill?: string; onBack?: () => void; layout?: SizeClass}) {
  const {select} = useWorkspace();
  // The regular shell (D5): the report header spans the main pane, the console takes the
  // width beneath it, and the two zones the phone puts behind tabs sit in an inspector on
  // the right. Nothing folds — the header is not competing with a phone's height.
  const regular = layout === 'regular';
  const {client, agents, conn, demo} = useAgents();
  const {pal, lang} = useApp();
  const zh = lang === 'zh';
  const t = (en: string, cn: string) => (zh ? cn : en);

  const [digest, setDigest] = useState<DigestRow[]>([]);
  const [week, setWeek] = useState<WindowPct[]>([]);
  const [usageFull, setUsageFull] = useState<UsageReport | null>(null);
  const [usageOpen, setUsageOpen] = useState(false);
  const [res, setRes] = useState<ResourceState | null>(null);
  const [board, setBoard] = useState<HQBoard>({exists: false});
  const [ledger, setLedger] = useState<HQEvent[]>([]);
  // The supervisor's own acts (a separate, narrowed feed — see the poll below).
  const [actFeed, setActFeed] = useState<HQEvent[]>([]);
  const [actsView, setActsView] = useState<ActsView>('acts');
  const [turns, setTurns] = useState<TranscriptTurn[]>([]);
  // Set when the supervisor's session began with a `/clear`/`/new` — including the
  // `gtmux hq --rotate` HQ performs on itself. The console then shows this shift whole
  // and the shift before it not at all, which without a word reads as a broken app: on
  // 2026-08-09 a cleared 400-turn shift showed three bubbles and cost a diagnosis.
  const [sessionReset, setSessionReset] = useState<SessionReset | undefined>();
  const [loaded, setLoaded] = useState(false);
  const [pending, setPending] = useState<string | undefined>();
  const [selected, setSelected] = useState<DigestRow | null>(null);
  const [zone, setZone] = useState<Zone | null>(null); // null until the first digest picks it
  const [boardOpen, setBoardOpen] = useState(false);
  // The knowledge base —HQ's long-term memory, beside the board's working memory
  // (hq-knowledge-on-phone). Polled with the rest: the promotion queue is a debt whose
  // count belongs on the header row, so it cannot wait until the sheet is opened.
  const [knowledge, setKnowledge] = useState<KnowledgeIndex>({
    entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0},
  });
  const [knowledgeOpen, setKnowledgeOpen] = useState(false);
  // esc (keys/keymap) closes whichever sheet is open.
  useEffect(
    () =>
      KeyBus.on('sheet.close', () => {
        setKnowledgeOpen(false);
        setBoardOpen(false);
        setUsageOpen(false);
      }),
    [],
  );
  // The board's outline. Parsed here because the sheet is unmounted until opened, and
  // re-parsing 48k characters on every open would be work the poll already did.
  const boardSections = useMemo(() => parseBoardSections(board.text ?? ''), [board.text]);
  // The assessment headline is the supervisor's synthesized conclusion — it can run
  // long. Collapsed to 2 lines by default; tapping it expands to the full text so it
  // is never stranded truncated with no way to read the rest.
  // The verdict's disclosure — everything the old header showed standing.
  const [briefOpen, setBriefOpen] = useState(false);
  // Collapsing top (like the terminal/Detail header): the header (verdict, counts, the
  // three doors) and the zone tabs hide as you scroll into a zone's body, reclaiming the
  // height for content, and reappear at the top. `collapse` 0 = shown, 1 = hidden.
  //
  // THE TOP CHROME FLOATS, exactly as Detail's does since 2026-09-10: it is drawn over the
  // zone rather than above it in the column, and folds by sliding out on `translateY`, so
  // the scroll view underneath keeps one frame for its whole life. This page kept
  // animating the chrome's HEIGHT for a month longer, and on 2026-09-12 the user found the
  // consequence Detail had already been cured of: a small upward scroll folded the header,
  // the viewport grew by the header's height, the console's distance from its tail shrank
  // by the same amount and asked for the header back, which shrank the viewport again —
  // fold, unfold, fold, until a scroll longer than the header out-ran it ("轻轻滑动一下…
  // 反复折叠展开来回换，只有上滑比较大一截的时候才会稳定"). No decision rule can fix a
  // measurement that the decision itself changes; see `ui/liveEdge`.
  //
  // The content underneath carries a constant top padding of `chromeH`, so the oldest
  // line can still be scrolled clear of the chrome.
  const collapse = useRef(new Animated.Value(0)).current;
  const [headerH, setHeaderH] = useState(0);
  const [tabsH, setTabsH] = useState(0);
  // Both bands fold on one driver, so the distance the chrome slides out — and the padding
  // the content carries — is their SUM. A band folded but not counted would slide only
  // partway out, or leave content starting underneath it.
  const chromeH = headerH + tabsH;
  const chrome = useRef<ChromeState>({hidden: false, settledAt: 0});
  const lastGap = useRef(0);
  // One rule, shared with Detail: `ui/liveEdge`. No decision is taken from a frame
  // measured mid-animation; the newest reading is re-asked when the animation ends.
  const runEdge = useCallback(
    (gap: number) => {
      const d = chromeDecision(chrome.current, {gap, now: Date.now()});
      if (!d.change) return;
      chrome.current = d;
      Animated.timing(collapse, {
        toValue: d.hidden ? 1 : 0,
        duration: CHROME_ANIM_MS,
        // Transform + opacity only, so the fold runs on the UI thread.
        useNativeDriver: true,
      }).start(({finished}) => {
        if (finished) runEdge(lastGap.current); // answer whatever arrived mid-flight
      });
    },
    [collapse],
  );
  const onLiveEdge = useCallback(
    (gap: number) => {
      lastGap.current = gap;
      runEdge(gap);
    },
    [runEdge],
  );
  // Top-anchored zones (calls / HQ's work) do NOT fold. Their content starts under the
  // chrome and reads downward, so a fold at 72pt would leave a blank band the rest of the
  // chrome's height above the first row (seen on the simulator, 2026-09-12). The chrome
  // instead scrolls away WITH the content, in step, clamped at its own height — a plain
  // scrolling header — and comes back the same way. Driven on the UI thread from the
  // zone's own offset; no decision, so nothing to flicker.
  const zoneOffset = useRef(new Animated.Value(0)).current;
  const onZoneScroll = useMemo(
    () => Animated.event([{nativeEvent: {contentOffset: {y: zoneOffset}}}], {useNativeDriver: true}),
    [zoneOffset],
  );
  const [seenMark, setSeenMark] = useState(0);
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  // The HQ pane's live screen. Without it the console had NO progress surface: your
  // prompt echoed, then nothing until the reply landed, so a long turn was
  // indistinguishable from a dead app. ChatView renders it as the same "live" card the
  // worker Detail has —HQ was the only place passing an empty screen.
  const [paneText, setPaneText] = useState('');
  // Ticks while HQ is working, so the transcript re-reads and its intermediate reply
  // bubbles / tool steps appear AS THEY LAND rather than all at once when the turn ends.
  const [turnTick, setTurnTick] = useState(0);
  // A command the server declined to submit — held so it can be retried, not lost.
  const [failedSend, setFailedSend] = useState<string | null>(null);

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
          setUsageFull(u ?? null);
          setWeek((u?.limits?.windows ?? []).map(x => ({label: x.label, pct: x.pct_used, agent: x.agent})));
          const m = u?.resource?.machine;
          // `tier` rides along now: it is what decides whether the machine's line is
          // promoted OUT of the disclosure (hqHeader.isCritical), and dropping it here
          // was why the old header printed disk/memory unconditionally.
          setRes(m ? {warn: m.warn, diskGB: m.disk_free_gb, memTier: m.mem_tier, tier: m.tier} : null);
        })
        .catch(() => {});
      client
        .hqBoard()
        .then(b => alive && setBoard(b))
        .catch(() => {});
      client
        .hqKnowledge()
        .then(k => alive && setKnowledge(k))
        .catch(() => {});
      client
        .hqEvents('notable', 40)
        .then(e => alive && setLedger(e))
        .catch(() => {});
      // The supervisor's own acts, narrowed by the CORE before its cap — a client-side
      // filter over the mixed feed sees under four hours (see hqActsModel / the contract).
      client
        .hqEvents('', 200, true)
        .then(e => alive && setActFeed(e))
        .catch(() => {});
    };
    load();
    const id = setInterval(load, 3000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client]);

  // Poll the HQ pane's screen only WHILE IT IS WORKING — that is the only time the live
  // card renders, so an idle HQ costs no captures at all. Cleared on the way out so a
  // finished turn never leaves a stale "live" screen behind.
  useEffect(() => {
    if (live.status !== 'working') {
      setPaneText('');
      return;
    }
    let alive = true;
    const load = () =>
      client
        .pane(hq.pane_id)
        .then(p => alive && setPaneText(p?.text ?? ''))
        .catch(() => {});
    load();
    const id = setInterval(load, 1500);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, hq.pane_id, live.status]);

  const paneLines: AnsiLine[] = useMemo(() => parseAnsi(paneText), [paneText]);

  // Re-read the transcript on a slow beat while working. Slower than the screen poll:
  // the screen is what changes second to second, while a new reply SEGMENT is a rarer
  // event and re-parsing is the more expensive of the two.
  useEffect(() => {
    if (live.status !== 'working') return;
    const id = setInterval(() => setTurnTick(t => t + 1), 4000);
    return () => clearInterval(id);
  }, [live.status]);

  // The HQ conversation transcript — refetch on status flip or after a command.
  useEffect(() => {
    let alive = true;
    client
      .transcript(hq.pane_id)
      .then(({turns: ts, reset}) => {
        if (!alive) return;
        setTurns(ts);
        setSessionReset(reset);
        setLoaded(true);
      })
      .catch(() => alive && setLoaded(true));
    return () => {
      alive = false;
    };
  }, [client, hq.pane_id, live.status, pending, turnTick]);

  // Retire the optimistic echo the moment the real turn is in the transcript, with a
  // long safety net so a send that never lands can't pin a ghost bubble forever.
  useEffect(() => {
    if (!pending) return;
    if (turns.length > 0 && turns[turns.length - 1].prompt === pending) {
      setPending(undefined);
      return;
    }
    const id = setTimeout(() => setPending(undefined), 10 * 60_000);
    return () => clearTimeout(id);
  }, [turns, pending]);

  const calls = useMemo(() => decisions(digest), [digest]);
  const working = useMemo(() => running(digest), [digest]);
  const actList = useMemo(() => supervisorActs(actFeed, zh), [actFeed, zh]);

  // Land on the block when there is one — that's why you opened HQ. Decided ONCE, from
  // the first digest that arrives, so a later state change never yanks the zone out from
  // under the user mid-read.
  useEffect(() => {
    if (zone === null && digest.length > 0) setZone(initialZone(digest));
  }, [digest, zone]);
  const activeZone: Zone = zone ?? 'console';

  // Reveal the top when switching zones — each zone starts at its resting edge, so a
  // stale collapsed state from the previous zone would hide the header with nothing
  // scrolled.
  useEffect(() => {
    collapse.setValue(0);
    zoneOffset.setValue(0); // the new zone's scroll view mounts at 0
    chrome.current = {hidden: false, settledAt: 0}; // don't carry a fold across a zone switch
    lastGap.current = 0;
  }, [activeZone, collapse, zoneOffset]);

  // Reading the feed marks it read; leaving it doesn't un-mark.
  useEffect(() => {
    if (activeZone === 'acts' && actFeed.length > 0) {
      setSeenMark(m => Math.max(m, eventMark(actFeed[0])));
    }
  }, [activeZone, actFeed]);
  // The zone's own signal reports UNSEEN ACTS. It used to report unseen fleet events,
  // which is the other tab's content now — a dot that lights for something the zone does
  // not lead with sends the reader to the wrong place.
  const actsNew = hasNewActivity(actFeed, seenMark);

  // Every command routes through gtmux HQ (send to the supervisor pane).
  const command = useCallback(
    (text: string) => {
      const body = text.trim();
      if (!body) return;
      setPending(body);
      setFailedSend(null);
      setZone('console'); // you asked HQ something — show you the answer arriving
      // The echo is cleared by the effect below — when the transcript actually carries
      // the turn — NOT on a timer. A blind 4s clear could erase your message while HQ
      // was still thinking, leaving the console showing nothing at all.
      //
      // A null result means the server refused to submit (it couldn't confirm the full
      // command reached HQ's input box). Say so and keep the text — otherwise the
      // console shows your command echoed forever, waiting on a turn that never started.
      client
        .send(hq.pane_id, {text: body, enter: true})
        .then(snap => {
          if (!snap) {
            setFailedSend(body);
            setPending(undefined);
          }
        })
        .catch(() => {
          setFailedSend(body);
          setPending(undefined);
        });
    },
    [client, hq.pane_id],
  );
  const onSend = useCallback((p: SendPayload) => p.text && command(p.text), [command]);

  // Open a worker's own Detail — the ONLY place direct input to a worker lives.
  const openWorker = useCallback(
    (row: DigestRow) => {
      const a = agents.find(x => x.pane_id === row.pane_id);
      if (a) select({kind: 'pane', agent: a});
    },
    [agents, select],
  );

  // Quick-command chips: per-selected-target when a decision card is picked, else fleet-wide.
  const chips: {label: string; cmd: string}[] = selected
    ? [
        {label: t('Reply for me', '帮我回复'), cmd: t(`${selected.loc} is waiting — recommend a reply.`, `${selected.loc} 在等待,给我一个回复建议。`)},
        {label: t('Inspect', '看它在干嘛'), cmd: t(`What is ${selected.loc} doing right now?`, `${selected.loc} 现在在干什么?`)},
        {label: t('Continue it', '让它继续'), cmd: t(`Tell ${selected.loc} to continue.`, `让 ${selected.loc} 继续。`)},
      ]
    : [
        {label: t('Brief', '简报'), cmd: t('Give me a one-line brief of the whole fleet, needs-you first.', '给我一句话的舰队简报,先说需要我的。')},
        {label: t("Who's waiting", '谁在等我'), cmd: t('Which agents are waiting on me, and what for?', '哪些 agent 在等我?分别等什么?')},
        {label: t("What's important", '要事'), cmd: t('What are the important events I should know about?', '有哪些我该知道的要紧事?')},
        {label: t('My call', '该我拍板'), cmd: t('What needs my decision right now?', '现在有什么需要我拍板的?')},
      ];

  const tabs: {key: Zone; label: string; badge?: string; dot?: boolean}[] = [
    {key: 'calls', label: t('Your call', '该你拍板'), badge: calls.length > 0 ? String(calls.length) : undefined},
    {key: 'acts', label: t("HQ's work", 'HQ 动作'), dot: actsNew},
    ...(regular ? [] : [{key: 'console' as Zone, label: t('Console', '对话')}]),
  ];
  // On the regular shell the console is always on screen; the inspector shows a zone.
  const inspectorZone: Zone = activeZone === 'console' ? 'calls' : activeZone;

  // The "your call" zone's body, rendered once: the phone scrolls it as a zone, the
  // regular shell shows it in the inspector.
  const callsBody = (
    <>
            {calls.length === 0 ? (
              /* The quiet state is the COMMON state, and it was one grey sentence over an
                 empty screen — this zone saying nothing at the moment it is most often
                 read (user report, 2026-09-09). Say the thing that is true, then show
                 what IS happening: the lines that are running do not need you, but they
                 are where the money is going. */
              <View testID="hq-calls-quiet" style={styles.quiet}>
                <Text style={[styles.quietTitle, {color: pal.fg}]}>
                  {t('Nobody is waiting on you', '没有人在等你')}
                </Text>
                {working.length > 0 ? (
                  <>
                    <Text style={[styles.quietSub, {color: pal.fg2}]}>
                      {zh
                        ? `下面是这会儿在跑的 ${working.length} 条线。它们不需要你,只是让你知道钱花在哪。`
                        : `These ${working.length} are running right now. They do not need you — this is where the time is going.`}
                    </Text>
                    <View style={[styles.quietList, {backgroundColor: pal.surface}]}>
                      {working.slice(0, 5).map((r, i) => (
                        <TouchableOpacity
                          key={r.pane_id || r.loc}
                          testID={`hq-quiet-${r.loc}`}
                          activeOpacity={0.6}
                          onPress={() => openWorker(r)}
                          style={[styles.quietRow, i > 0 && {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
                          <View style={[styles.rowDot, {backgroundColor: StatusColor.working}]} />
                          <Text style={[styles.quietName, {color: pal.fg}]} numberOfLines={1}>
                            {sessionName(r)}
                          </Text>
                          <Text style={[styles.quietSince, {color: pal.fg3}]}>{relTime(r.since, now)}</Text>
                        </TouchableOpacity>
                      ))}
                    </View>
                  </>
                ) : (
                  <Text style={[styles.quietSub, {color: pal.fg2}]}>
                    {t('Nothing is running either — the fleet is idle.', '也没有在跑的线 —— 舰队是空闲的。')}
                  </Text>
                )}
                <TouchableOpacity
                  testID="hq-quiet-ask"
                  activeOpacity={0.8}
                  onPress={() => command(t('What is the situation right now?', '现在什么情况?'))}
                  style={[styles.quietAsk, {borderColor: StatusColor.working}]}>
                  <Text style={[styles.quietAskText, {color: StatusColor.working}]}>
                    {t('Ask HQ what the situation is', '问 HQ 现在什么情况')}
                  </Text>
                </TouchableOpacity>
              </View>
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
                        style={[styles.action, styles.actionPrimary, {borderColor: StatusColor.working}]}
                        onPress={() => openWorker(row)}>
                        <Text style={[styles.actionText, {color: StatusColor.working, fontWeight: '600'}]}>
                          {t('Open session', '打开会话')}
                        </Text>
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
                        <Text style={[styles.actionText, {color: pal.fg}]}>{t('Ask HQ', '问 HQ')}</Text>
                      </TouchableOpacity>
                    </View>
                  </TouchableOpacity>
                );
              })
            )}
    </>
  );
  // The supervisor's own acts, likewise (topPad / onScroll differ per host).
  const actsProps = {acts: actList, ledger, view: actsView, onView: setActsView, now, pal, zh};
  const consoleEl = (topPad: number, onEdge?: (gap: number) => void) => (
    <ChatView
              agent={live}
              lines={paneLines}
              status={live.status}
              workingSince={live.since}
              fontSize={13}
              pal={pal}
              lang={lang}
              turns={turns}
              sessionReset={sessionReset}
              // Where the earlier record still IS. The event ledger behind ACTIVITY is
              // fed by gtmux, not by the conversation, so a reset cannot empty it — the
              // one place on this page a cleared history is still readable.
              resetElsewhere={t('Activity', '动态')}
              loading={!loaded}
              pendingPrompt={pending}
              // Header shows at the live tail (newest), hides when you scroll up into
              // history. The old loop (showing the header bumped you off the bottom →
              // it re-hid → …) is broken in ChatView: it re-pins to the bottom when its
              // viewport changes while at the tail, so atBottom stays true and the header
              // stays put.
              onLiveEdge={onEdge}
              topPad={topPad}
              maxWidth={regular ? READING_WIDTH : undefined}
            />
  );
  const composerEl = (
    <>
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
        {failedSend && (
          <SendFailedBar
            text={failedSend}
            pal={pal}
            lang={lang}
            onRetry={() => command(failedSend)}
            onDismiss={() => setFailedSend(null)}
          />
        )}
        <Composer
            pal={pal}
            lang={lang}
            demo={demo}
            draftKey={hq.pane_id}
            historyScope={historyScope(hq)}
            onSend={onSend}
          />
    </>
  );
  const headerEl = (
    <HQHeader
            model={headerModel({
              verdict: assessment(digest, zh),
              urgent: calls.length > 0,
              digest,
              turns,
              week,
              res,
              // What HQ did in the last day — its own acts, already ranked by the
              // zone that lists them, so the header and that zone cannot disagree.
              did: tally(acts(actFeed, zh), now, 24 * 3600),
              owed: {
                pending: knowledge.promotions?.pending ?? 0,
                oldestLabel: knowledge.promotions?.oldest_sec
                  ? relTime(now - knowledge.promotions.oldest_sec, now)
                  : undefined,
                overdue: knowledgeOverdue(knowledge),
              },
              nowSecs: now,
              zh,
            })}
            conn={conn}
            demo={demo && !Debug.shotMode}
            boardValue={board.exists ? boardAge(board.updated_at, now, zh) : null}
            usageValue={usageDoorValue(week, zh)}
            open={briefOpen}
            onToggle={() => setBriefOpen(v => !v)}
            onOpenActs={() => setZone('acts')}
            onOpenUsage={() => setUsageOpen(true)}
            onBack={onBack}
            onOpenBoard={() => setBoardOpen(true)}
            knowledgeValue={knowledgeValue(knowledge, zh)}
            onOpenKnowledge={() => setKnowledgeOpen(true)}
            pal={pal}
            zh={zh}
          />
  );
  const tabsEl = (
    <View
          onLayout={e => {
            const h = e.nativeEvent.layout.height;
            if (tabsH === 0 && h > 0) setTabsH(h);
          }}
          style={[styles.tabs, {borderBottomColor: pal.divider}]}>
          {tabs.map(tab => {
            const on = tab.key === activeZone;
            return (
              <TouchableOpacity
                key={tab.key}
                testID={`hq-tab-${tab.key}`}
                style={[
                  styles.tab,
                  {backgroundColor: pal.surface, borderColor: 'transparent'},
                  on && (tab.badge
                    ? {backgroundColor: 'rgba(239,68,68,0.14)', borderColor: StatusColor.waiting}
                    : {backgroundColor: pal.rowSelected ?? pal.surface, borderColor: pal.divider}),
                ]}
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
  );
  const sheetsEl = (
    <>
      {/* The situation board, read-only — the supervisor's own working memory. */}
      {/* The situation board, read-only.
          `pageSheet`, not a full-screen modal. A Modal renders in its OWN native
          hierarchy, where SafeAreaView resolves every inset to ZERO — so the header and
          its close control were drawn UNDERNEATH the system status bar, overlapping the
          clock and the battery, and the ✕ could not reliably be tapped. A page sheet is
          laid out below the status bar by iOS itself, and it adds a second way out
          (swipe down) so leaving never depends on hitting one glyph. */}
      <KnowledgeSheet
        layout={layout}
        visible={knowledgeOpen}
        index={knowledge}
        nowSecs={now}
        pal={pal}
        zh={zh}
        onClose={() => setKnowledgeOpen(false)}
        loadEntry={id => client.hqKnowledgeEntry(id)}
        act={async a => {
          const r = await client.hqKnowledgeAct(a);
          // A mutation changes the base, so the index a reader is looking at is stale the
          // moment it succeeds — refetch rather than patching a copy locally.
          if (r.ok) client.hqKnowledge().then(setKnowledge).catch(() => {});
          return r;
        }}
      />

      <UsageSheet
        visible={usageOpen}
        usage={usageFull}
        agents={agents}
        pal={pal}
        lang={lang}
        onClose={() => setUsageOpen(false)}
      />

      <BoardSheet
        visible={boardOpen}
        sections={boardSections}
        age={boardAge(board.updated_at, now, zh)}
        pal={pal}
        zh={zh}
        onClose={() => setBoardOpen(false)}
      />
    </>
  );

  if (regular) {
    return (
      <SafeAreaView style={[styles.root, {backgroundColor: pal.bg}]} edges={['top', 'bottom']}>
        {headerEl}
        <View style={styles.wide}>
          <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
            <View style={styles.flex}>{consoleEl(0)}</View>
            {composerEl}
          </KeyboardAvoidingView>
          <View testID="hq-inspector" style={[styles.inspector, {borderLeftColor: pal.divLoud}]}>
            {tabsEl}
            {inspectorZone === 'acts' ? (
              <HQActs {...actsProps} />
            ) : (
              <ScrollView style={styles.flex} keyboardShouldPersistTaps="handled" contentContainerStyle={styles.pad}>
                {callsBody}
              </ScrollView>
            )}
          </View>
        </View>
        {sheetsEl}
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={[styles.root, {backgroundColor: pal.bg}]} edges={['top', 'bottom']}>
      {/* The stack: the zone underneath, the floating chrome over it (drawn last, so it
          is on top). See the collapse driver above for why the chrome does not live in
          the column. */}
      <View style={styles.stack}>
      <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        {/* YOUR CALL — one decision card per blocked session. */}
        {activeZone === 'calls' && (
          <Animated.ScrollView style={styles.flex} keyboardShouldPersistTaps="handled" contentContainerStyle={[styles.pad, {paddingTop: chromeH + styles.pad.paddingVertical}]} onScroll={onZoneScroll} scrollEventThrottle={16}>
            {callsBody}
          </Animated.ScrollView>
        )}

        {/* WHAT HQ DID — the supervisor's own acts, with the fleet ledger beside them.
            The old zone gave this height to the fleet's lifecycle and left the chief of
            staff's own work with no surface anywhere in the app. */}
        {activeZone === 'acts' && <HQActs {...actsProps} onScroll={onZoneScroll} topPad={chromeH} />}

        {/* CONSOLE — the conversation with gtmux HQ. */}
        {activeZone === 'console' && <View style={styles.flex}>{consoleEl(chromeH, onLiveEdge)}</View>}

        {/* Quick-command chips + command bar — available on every zone. */}
        {composerEl}
      </KeyboardAvoidingView>

      <Animated.View
        pointerEvents="box-none"
        style={[
          styles.chrome,
          {
            // Opaque, because it FLOATS: the zone scrolls underneath it.
            backgroundColor: pal.bg,
            opacity: collapse.interpolate({inputRange: [0, 1], outputRange: [1, 0]}),
            // Two drivers, one at a time: the console's fold (collapse), or a top-anchored
            // zone's scroll offset (zoneOffset). The other is 0 whenever this one moves.
            transform: [{
              translateY: Animated.add(
                collapse.interpolate({inputRange: [0, 1], outputRange: [0, -chromeH]}),
                zoneOffset.interpolate({inputRange: [0, Math.max(chromeH, 1)], outputRange: [0, -Math.max(chromeH, 1)], extrapolate: 'clamp'}),
              ),
            }],
          },
        ]}>
        {/* The header measures its NATURAL height on every layout while shown (not once):
            the assessment text arrives after mount and can grow to two lines, and the
            disclosure opens in place. Not while hidden — the chrome is off screen then,
            and a re-sync when it comes back is enough. */}
        <View
          onLayout={e => {
            if (chrome.current.hidden) return;
            const h = e.nativeEvent.layout.height;
            if (h > 0 && Math.abs(h - headerH) > 1) setHeaderH(h);
          }}>
          {headerEl}
        </View>

        {/* Zone selector — each tab carries its own signal so a hidden zone still reports
            itself. Folds with the header on the same driver: one gesture, all of the top
            chrome, as on Detail. Measured once at natural height. */}
        {tabsEl}
      </Animated.View>
      </View>

      {sheetsEl}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1},
  flex: {flex: 1},
  // Clipped: the chrome slides up out of this box, and without the clip a partly scrolled
  // header shows through the status bar (the title under the clock, 2026-09-12).
  stack: {flex: 1, overflow: 'hidden'},
  // The regular shell: console column + inspector (D5).
  wide: {flex: 1, flexDirection: 'row', minHeight: 0},
  inspector: {width: 360, borderLeftWidth: StyleSheet.hairlineWidth},
  chrome: {position: 'absolute', top: 0, left: 0, right: 0, zIndex: 10},
  pad: {paddingHorizontal: 14, paddingVertical: 10},
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  back: {fontSize: 30, fontWeight: '300', marginRight: 10, marginTop: -4},
  close: {fontSize: 18, fontWeight: '400', paddingHorizontal: 4},
  stripDetail: {paddingHorizontal: 14, paddingTop: 6, paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  titleRow: {flexDirection: 'row', alignItems: 'center', flex: 1},
  demoPill: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 0.5, marginLeft: 8},
  demoPillText: {fontSize: 9, fontWeight: '700', letterSpacing: 0.06},
  dot: {width: 8, height: 8, borderRadius: 4, marginLeft: 8},

  assess: {paddingHorizontal: 14, paddingTop: 9, paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  assessText: {fontSize: 14, fontWeight: '600', lineHeight: 19},
  boardRow: {
    flexDirection: 'row', alignItems: 'center', marginTop: 8,
    borderWidth: StyleSheet.hairlineWidth, borderRadius: 8,
    paddingHorizontal: 10, paddingVertical: 7, gap: 7,
  },
  boardIcon: {fontSize: 13},
  boardLink: {fontSize: 12.5, fontWeight: '600', flex: 1},
  boardChevron: {fontSize: 15},
  boardText: {fontSize: 12.5, lineHeight: 19, fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace'},

  // Pills, not a three-up underline bar. The three are different KINDS of thing — a
  // queue, a log, a conversation — and only the queue is ever urgent; the active pill
  // carries that, in the status language the rest of the product uses.
  tabs: {flexDirection: 'row', gap: 8, paddingHorizontal: 14, paddingTop: 9, paddingBottom: 9,
    borderBottomWidth: StyleSheet.hairlineWidth},
  tab: {flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 6,
    minHeight: 32, paddingHorizontal: 12, borderRadius: 16, borderWidth: 1},
  tabText: {fontSize: 13},
  badge: {minWidth: 18, height: 18, borderRadius: 9, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 5},
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
  // 44pt, the floor for a hit target. These were ~30pt (paddingVertical 8 around 13pt
  // text) on the card that carries the page's whole purpose — the one thing you came to
  // do (user report, 2026-09-09).
  action: {flex: 1, minHeight: 44, alignItems: 'center', justifyContent: 'center',
    borderRadius: 10, borderWidth: StyleSheet.hairlineWidth},
  actionPrimary: {borderWidth: 1},
  actionText: {fontSize: 12.5, fontWeight: '600'},

  eventRow: {flexDirection: 'row', alignItems: 'flex-start', paddingVertical: 7},
  eventTime: {fontSize: 11, width: 34, fontVariant: ['tabular-nums']},
  eventDot: {width: 6, height: 6, borderRadius: 3, marginTop: 5, marginRight: 9},
  eventHead: {fontSize: 13, fontWeight: '600'},
  eventSummary: {fontSize: 11.5, marginTop: 2, lineHeight: 16},

  empty: {fontSize: 13, textAlign: 'center', paddingVertical: 30, lineHeight: 19},
  quiet: {paddingTop: 18, gap: 12},
  quietTitle: {fontSize: 15, fontWeight: '600', textAlign: 'center'},
  quietSub: {fontSize: 13, lineHeight: 19, textAlign: 'center', paddingHorizontal: 6},
  quietList: {borderRadius: 12, overflow: 'hidden'},
  quietRow: {flexDirection: 'row', alignItems: 'center', gap: 9, minHeight: 44, paddingHorizontal: 13},
  quietName: {flex: 1, fontSize: 13.5},
  quietSince: {fontSize: 11.5},
  quietAsk: {minHeight: 44, alignItems: 'center', justifyContent: 'center', borderRadius: 11, borderWidth: 1},
  quietAskText: {fontSize: 14, fontWeight: '600'},
  chips: {paddingHorizontal: 10, paddingVertical: 6, gap: 6},
  selPill: {flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', paddingHorizontal: 8, paddingVertical: 3, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, marginBottom: 6, gap: 8, maxWidth: '70%'},
  selText: {fontSize: 12},
  chip: {paddingHorizontal: 12, paddingVertical: 7, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth, marginRight: 8},
  chipText: {fontSize: 13, fontWeight: '600'},
});
