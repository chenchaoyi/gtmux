// RadarPanel — the agent list, ONCE (change ipad-universal-app, D3).
//
// The phone's radar screen and the iPad's sidebar are this one component. They differ
// by props only: `variant` ('screen' floats the HQ disc over the list and owns the safe
// area; 'sidebar' carries the HQ card in its header, a hide button, and the selected row's
// accent). Opening a row, HQ or All panes goes through the workspace (`select`), which
// the shell turns into navigation (phone) or a main-pane change (iPad) — the panel never
// knows which. Before this, `SplitScreen` was a copy of this file's chrome, and every
// radar change since 2026-07 landed on the phone only.
//
// Initial fetch + SSE-driven refetch (via AgentsContext), pull-to-refresh, and an in-app
// alert banner. Status language mirrors the menu-bar app.

import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {NavigationContext} from '@react-navigation/native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Alert as AlertType, SectionKey} from '../api/types';
import type {ServerMode} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {Agent} from '../api/types';
import {useApp} from '../state/AppContext';
import {Debug} from '../debug';
import {BrandMark} from '../ui/BrandMark';
import {PanesIcon} from '../ui/Icons';
import {HQDisc} from '../ui/HQDisc';
import {HQCard} from '../ui/HQCard';
import {SidebarIcon} from '../ui/Icons';
import {useWorkspace} from '../state/WorkspaceContext';
import {OfflineBanner} from '../ui/OfflineBanner';
import {SectionList} from '../ui/SectionList';
import {RowSheet} from '../ui/RowSheet';
import {RadarSummary} from '../ui/RadarSummary';
import {SettingsIcon} from '../ui/SettingsIcon';
import {StatusColor, counts} from '../ui/theme';
import {TestIds} from '../constants/testIds';

const COLLAPSED_KEY = 'radar.collapsed';

export type RadarVariant = 'screen' | 'sidebar';

/** The demo's two affordances the real radar does not have (MOBILE §18): a way out, and
 * the "pair your Mac" call to action. Their presence is what makes a panel a demo. */
export interface DemoChrome {
  onExit: () => void;
  onPair: () => void;
}

export function RadarPanel({
  variant,
  selectedId,
  onHideSidebar,
  demo: demoChrome,
}: {
  variant: RadarVariant;
  /** The row wearing the accent bar (sidebar). */
  selectedId?: string;
  /** The sidebar's hide button (regular shell only). */
  onHideSidebar?: () => void;
  /** Present in demo mode: the same panel over the fake client, plus the demo's exit and
   * pairing controls. The demo renders OUTSIDE the navigator, so nothing here may need one. */
  demo?: DemoChrome;
}) {
  // Outside the navigator (the demo) this is undefined and the routes it reaches — Servers,
  // Settings — are not offered; inside, it is the screen's own navigation.
  const navigation = React.useContext(NavigationContext) as {navigate: (r: string, p?: unknown) => void} | undefined;
  const {select, cursor} = useWorkspace();
  const sidebar = variant === 'sidebar';
  // The long-press sheet. The row IS the whole state (null = closed), so a stale sheet
  // cannot linger after a refresh replaces the list.
  const [sheetAgent, setSheetAgent] = useState<Agent | null>(null);
  const {agents, conn, lastUpdated, banner, dismissBanner, refresh, isGuest, client} = useAgents();
  const {t, pal, lang, mac} = useApp();
  const [refreshing, setRefreshing] = useState(false);
  // Genuine resource BOTTLENECK (the machine's "red" tier: disk critically low / memory
  // critical / load pinned) feeds the HQ disc's red "resource" state. A soft "amber"
  // heads-up (e.g. 37GB free — below the amber line but nowhere near empty) does NOT
  // redden the disc: red is reserved for "act now / needs your call", so a background
  // disk note must not read like a session waiting on you (低噪). Polled slowly (it
  // changes on a human/machine cadence, not per frame), owner only — /api/usage is an
  // owner surface. `false` = no bottleneck.
  const [resCrit, setResCrit] = useState(false);
  useEffect(() => {
    if (isGuest || demoChrome) return;
    let alive = true;
    const load = () =>
      client
        .usage()
        .then(u => {
          if (!alive) return;
          setResCrit(u?.resource?.machine?.tier === 'red');
        })
        .catch(() => {});
    load();
    const id = setInterval(load, 25000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, isGuest, demoChrome]);
  // Collapsed sections persist across launches (MOBILE §3).
  const [collapsed, setCollapsed] = useState<Set<SectionKey>>(new Set());
  // "Waiting only" narrows the list to the panes needing you (MOBILE §3/§8) — a fast
  // triage lever when many agents are running. Session-scoped (not persisted): it's a
  // transient filter, and auto-clears itself once nothing is waiting (see below).
  const [waitingOnly, setWaitingOnly] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(COLLAPSED_KEY).then(raw => {
      if (!raw) return;
      try {
        setCollapsed(new Set(JSON.parse(raw) as SectionKey[]));
      } catch {}
    });
  }, []);

  const onToggle = (st: SectionKey) => {
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(st) ? next.delete(st) : next.add(st);
      AsyncStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  };

  const c = counts(agents);
  // Auto-clear the filter once nothing is waiting, so it never strands the user on an
  // empty list after they answer the last pane.
  useEffect(() => {
    if (waitingOnly && c.waiting === 0) setWaitingOnly(false);
  }, [waitingOnly, c.waiting]);
  // The list the sections are built from — narrowed to waiting when the filter is on.
  const shown = waitingOnly ? agents.filter(a => a.status === 'waiting') : agents;

  const onRefresh = () => {
    // Probe (debug builds only): did the pull actually reach the list? A collapsed
    // radar is shorter than the screen, and a list sized to its content leaves the
    // blank half unreachable — see ui/SectionList.test.tsx.
    if (Debug.logNet) Debug.record({event: 'radar-refresh'});
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  // The supervisor (中控) is the meta-layer, so on the phone it renders as a FLOATING
  // DISC over the list (ui/HQDisc), NOT a section row (theme.sections excludes role
  // rows) and NOT a top card (which read too much like a session). Tap → HQScreen;
  // absent → no disc (starting one needs the Mac; no dead control).
  const hq = agents.find(a => a.role === 'supervisor');

  // Server mode is a MACHINE state, not an agent state, so it never becomes a radar
  // row — it rides the header as a quiet chip, present only while true. This is the
  // "you can always tell" half of the feature: the menu bar has its own indicator,
  // and the phone needs one too, or a user away from their Mac has no way to know
  // it is being kept awake. Slow poll: it changes when a human decides it does.
  const [srv, setSrv] = useState<ServerMode | null>(null);
  useEffect(() => {
    // The demo's client deliberately does not answer host-state calls (demoClient
    // `NotInDemo`): a stub that looked like a Mac would be worse than no call.
    if (demoChrome) return;
    let alive = true;
    const tick = () => {
      client.serverMode().then(m => alive && setSrv(m)).catch(() => {});
    };
    tick();
    const id = setInterval(tick, 30000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, demoChrome]);
  const srvOn = !!srv && (srv.system_disablesleep || srv.state === 'lapsed');

  const Header = (
    <View style={styles.header}>
      <View style={styles.headerTop}>
        {/* server chip: the connected Mac's name + a switch glyph → Servers page */}
        <TouchableOpacity
          testID={TestIds.radar.serverChip}
          accessibilityLabel={TestIds.radar.serverChip}
          style={styles.serverChip}
          disabled={!!demoChrome}
          onPress={() => navigation?.navigate('Servers')}
          hitSlop={hit}>
          <Text style={[styles.brand, {color: pal.fg}]} numberOfLines={1}>
            {demoChrome && !Debug.shotMode ? (lang === 'zh' ? '演示' : 'Demo') : mac?.name || 'gtmux'}
          </Text>
          {/* a bordered ⇄ chip reads as a tappable control (the bare glyph looked like
              a decoration next to the title, so switching went unnoticed). */}
          {!demoChrome && (
            <Text style={[styles.switchGlyph, {color: pal.fg2, borderColor: pal.divider, backgroundColor: pal.surface}]}>
              ⇄
            </Text>
          )}
        </TouchableOpacity>
        <View style={styles.headerRight}>
          <ConnDot conn={conn} t={t} pal={pal} lang={lang} awake={srvOn} />
          {/* Browse ALL panes (tiered-pane-control): the opt-in secondary surface —
              reach a pane in a session with no agent. Kept off the radar itself so the
              agent-first list stays clean. Guests reach only their shared panes. */}
          {/* Each button owns a 40pt square and NOTHING claims space outside it. These
              two used to sit 14pt apart with 10pt of hitSlop on every side, so their
              touch areas OVERLAPPED by 6pt in the middle — and the overlap belonged to
              whichever came last in the tree, so aiming at the right half of "all panes"
              opened Settings instead. A gap you can see is not a gap you can hit. */}
          <TouchableOpacity
            testID={TestIds.radar.panes}
            accessibilityLabel={lang === 'zh' ? '所有 pane' : 'All panes'}
            onPress={() => select({kind: 'panes'})}
            style={styles.headBtn}>
            <PanesIcon size={19} color={pal.fg2} />
          </TouchableOpacity>
          {sidebar && onHideSidebar && (
            <TouchableOpacity
              testID={TestIds.radar.hideSidebar}
              accessibilityLabel={lang === 'zh' ? '收起侧栏' : 'Hide sidebar'}
              onPress={onHideSidebar}
              style={styles.headBtn}>
              <SidebarIcon size={19} color={pal.fg2} />
            </TouchableOpacity>
          )}
          {demoChrome && Debug.shotMode ? null : demoChrome ? (
            <TouchableOpacity
              onPress={demoChrome.onExit}
              style={styles.headBtn}
              accessibilityRole="button"
              accessibilityLabel={lang === 'zh' ? '关闭演示' : 'Close demo'}>
              <Text style={[styles.closeDemo, {color: pal.fg3}]}>✕</Text>
            </TouchableOpacity>
          ) : (
            <TouchableOpacity
              testID={TestIds.radar.settings}
              accessibilityLabel={TestIds.radar.settings}
              onPress={() => navigation?.navigate('Settings')}
              style={styles.headBtn}>
              <SettingsIcon size={20} color={pal.fg2} />
            </TouchableOpacity>
          )}
        </View>
      </View>
      <RadarSummary
        c={c}
        agentsWord={t('agents')}
        lang={lang}
        pal={pal}
        waitingOnly={waitingOnly}
        onToggleWaitingOnly={() => setWaitingOnly(v => !v)}
      />
      {/* Demo: the sample-data banner, always shown in the shipped demo (App Review requires
          it), hidden in SHOT_MODE for clean store captures. */}
      {demoChrome && !Debug.shotMode && (
        <View style={styles.demoBanner}>
          <Text style={[styles.demoChip, {color: StatusColor.working, borderColor: StatusColor.working}]}>DEMO</Text>
          <Text style={[styles.demoBannerText, {color: pal.fg3}]} numberOfLines={2}>
            {lang === 'zh' ? '演示数据 —— 每一屏都能点。配对你的 Mac 就是真的了。' : 'Sample data — every screen is live. Pair your Mac and it is real.'}
          </Text>
        </View>
      )}
      {/* The sidebar keeps the framed HQ card (a floating disc suits a phone list, a
          resident column suits a card); the phone floats the disc below. */}
      {sidebar && hq && !isGuest && (
        <HQCard hq={hq} agents={agents} pal={pal} lang={lang} onPress={() => select({kind: 'hq', agent: hq})} />
      )}
    </View>
  );

  const Empty = (
    <View style={styles.empty}>
      <BrandMark size={52} neutral={pal.fg3} />
      <Text style={[styles.emptyText, {color: pal.fg2}]}>{t('noAgents')}</Text>
      <Text style={[styles.emptyHint, {color: pal.fg3}]}>
        {lang === 'zh' ? '在服务器上启动一个 coding agent 就会出现在这里' : 'Start a coding agent on your server and it shows up here'}
      </Text>
    </View>
  );

  // The screen owns its safe area; the sidebar sits inside the shell's.
  const Root: React.ComponentType<any> = sidebar ? View : SafeAreaView;
  const rootProps = sidebar ? {} : {edges: ['top']};
  return (
    <Root style={[styles.safe, {backgroundColor: pal.bg}]} {...rootProps} testID={TestIds.radar.screen}>
      {banner && <Banner alert={banner} t={t} onClose={dismissBanner} />}
      {conn === 'offline' && (
        <OfflineBanner serverName={mac?.name} lastUpdated={lastUpdated} lang={lang} onRetry={refresh} pal={pal} />
      )}
      {conn === 'unauthorized' && (
        <TouchableOpacity
          style={[styles.authBanner, {backgroundColor: '#3a1720', borderColor: StatusColor.waiting}]}
          onPress={() => navigation?.navigate('Servers')}
          accessibilityRole="button"
          accessibilityLabel={lang === 'zh' ? '访问被拒，重新配对' : 'Access rejected, re-pair'}>
          <View style={[styles.connDot, {backgroundColor: StatusColor.waiting}]} />
          <Text style={styles.authBannerText}>
            {lang === 'zh'
              ? '访问被拒 —— 这台服务器的 token 已吊销或更改。点此重新配对。'
              : 'Access rejected — this server’s token was revoked or changed. Tap to re-pair.'}
          </Text>
        </TouchableOpacity>
      )}
      {isGuest && (
        <View
          testID="radar-guest-banner"
          accessibilityLabel="radar-guest-banner"
          style={[styles.guestBanner, {backgroundColor: pal.surface, borderBottomColor: pal.divider}]}>
          <Text style={[styles.guestBannerText, {color: pal.fg2}]} numberOfLines={1}>
            {lang === 'zh'
              ? `以访客身份连到 ${mac?.name || '服务器'} · ${agents.length} 个会话`
              : `Guest on ${mac?.name || 'server'} · ${agents.length} session${agents.length === 1 ? '' : 's'}`}
          </Text>
        </View>
      )}
      <SectionList
        agents={shown}
        pal={pal}
        lang={lang}
        onPressAgent={a => { if (a.source !== 'native') select({kind: 'pane', agent: a}); }}
        onLongPressAgent={setSheetAgent}
        refreshing={refreshing}
        onRefresh={onRefresh}
        collapsed={collapsed}
        onToggle={onToggle}
        selectedId={sidebar ? selectedId : cursor ?? undefined}
        ListHeaderComponent={Header}
        ListEmptyComponent={Empty}
      />
      <RowSheet
        agent={sheetAgent}
        pal={pal}
        lang={lang}
        onClose={() => setSheetAgent(null)}
        onJump={a => { client.focus(a.pane_id).catch(() => {}); }}
        onDiff={a => select({kind: 'pane', agent: a, openDiff: true})}
        loadOptions={a => client.options(a.pane_id)}
        onAct={(a, act) => {
          // Every talking action lands here. A numbered choice is a KEYSTROKE (an agent's
          // menu commits on the digit, and a bracketed paste of "1" selects nothing);
          // "stop" is Escape; the rest are text. `reply` with no number has nothing to
          // send yet, so it opens the session where the composer lives.
          if (act.kind === 'option' && act.n) {
            client.send(a.pane_id, {text: String(act.n)}).catch(() => {});
          } else if (act.kind === 'continue') {
            client.send(a.pane_id, {text: lang === 'zh' ? '继续' : 'continue', enter: true}).catch(() => {});
          } else if (act.kind === 'stop') {
            client.send(a.pane_id, {key: 'Escape'}).catch(() => {});
          } else if (act.kind === 'ask-hq') {
            if (hq) select({kind: 'hq', agent: hq, prefill: `${a.pane_id} `});
          } else {
            select({kind: 'pane', agent: a});
          }
        }}
      />
      {/* HQ floats over the list as a disc (the meta-layer, above the fleet) — always
          reachable, never scrolls away. Owner-only (a guest has no HQ). Shown even when
          HQ isn't started yet (a grey disc that explains how to start it), but only once
          connected so it doesn't flash "not started" mid-connect. */}
      {demoChrome && !Debug.shotMode && (
        <TouchableOpacity
          style={[styles.cta, {backgroundColor: StatusColor.working}]}
          onPress={demoChrome.onPair}
          accessibilityRole="button"
          accessibilityLabel={lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}>
          <Text style={styles.ctaText}>{lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}</Text>
        </TouchableOpacity>
      )}
      {!sidebar && !isGuest && (hq || conn === 'live') && (
        <HQDisc
          hq={hq}
          agents={agents}
          pal={pal}
          lang={lang}
          resourceCritical={resCrit}
          onOpen={() => hq && select({kind: 'hq', agent: hq})}
        />
      )}
    </Root>
  );
}

function ConnDot({conn, t, lang, awake}: any) {
  // D9: server name (shown in the chip) + a status dot — no "live" word; only an
  // abnormal state adds text (amber reconnecting / red offline / red rejected).
  const isRed = conn === 'offline' || conn === 'unauthorized';
  const color = conn === 'live' ? StatusColor.idle : isRed ? StatusColor.waiting : '#F59E0B';
  // The WORD appears only for a state nothing else announces. offline and unauthorized
  // each already own a full-width banner directly above this row — saying it twice cost
  // the header's width, squeezing the Mac's name to "ccy MBP2024 M4…" and crowding the
  // buttons beside it. Reconnecting has no banner, so it keeps its word.
  const label = conn === 'reconnecting' ? t('reconnecting') : '';
  // A meaningful VoiceOver label for the status dot (the coloured dot alone is
  // invisible to screen readers).
  const a11y =
    conn === 'live' ? (lang === 'zh' ? '已连接' : 'connected') :
    conn === 'unauthorized' ? (lang === 'zh' ? '访问被拒' : 'access rejected') :
    conn === 'offline' ? (lang === 'zh' ? '离线' : 'offline') :
    (lang === 'zh' ? '重连中' : 'reconnecting');
  return (
    <View
      style={styles.conn}
      // `accessible` is what turns this into an element a screen reader can land on.
      // Without it RN leaves the View transparent to accessibility on iOS and the label
      // below never reaches the tree at all — measured 2026-09-04: the whole radar header
      // exposed `radar-server-chip` and no "Connection:" node anywhere. The connection
      // state was then carried by the coloured dot ALONE, which is the one channel a
      // screen-reader user does not have.
      accessible
      accessibilityRole="text"
      accessibilityLabel={
        (lang === 'zh' ? '连接：' : 'Connection: ') + a11y +
        (awake ? (lang === 'zh' ? '，服务器模式开启' : ', server mode on') : '')
      }>
      <View style={[styles.connDot, {backgroundColor: color}]} />
      {/* Server mode: a hairline ring around the connection dot, present only while
          it is on. Read-only by design — enabling needs a password typed at the Mac,
          and even turning it OFF from here would leave a prompt on an unattended
          screen. So the phone SHOWS the state and never touches it. */}
      {awake ? <View style={[styles.connAwake, {borderColor: color}]} /> : null}
      {/* The word is shown only for an abnormal state (reconnecting/offline/rejected);
          color it with the STATE color (amber/red, same as the dot) so it is actually
          READABLE and severity-coded — pal.fg3 (34% opacity) rendered it nearly invisible. */}
      {label ? <Text style={[styles.connText, {color}]}>{label}</Text> : null}
    </View>
  );
}

function Banner({alert, t, onClose}: {alert: AlertType; t: any; onClose: () => void}) {
  const isWaiting = alert.kind === 'waiting';
  const verb = isWaiting ? t('alertWaiting') : t('alertDone');
  const name = alert.agent || t('agents');
  return (
    <TouchableOpacity
      onPress={onClose}
      activeOpacity={0.9}
      style={[styles.banner, {backgroundColor: isWaiting ? StatusColor.waiting : StatusColor.idle}]}>
      <Text style={styles.bannerText} numberOfLines={1}>
        {name} {verb}
        {alert.task ? ` — ${alert.task}` : ''}
      </Text>
    </TouchableOpacity>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {paddingHorizontal: 14, paddingTop: 8, paddingBottom: 4},
  headerTop: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between'},
  serverChip: {flexDirection: 'row', alignItems: 'center', flexShrink: 1, marginRight: 8},
  brand: {fontSize: 22, fontWeight: '800', flexShrink: 1},
  switchGlyph: {
    fontSize: 14,
    fontWeight: '600',
    marginLeft: 9,
    paddingHorizontal: 6,
    paddingVertical: 1,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 7,
    overflow: 'hidden',
  },
  headerRight: {flexDirection: 'row', alignItems: 'center'},
  // 40pt square, centred icon, 4pt apart: a finger-sized target that cannot overlap its
  // neighbour. (iOS asks for 44; 40 is what the header's height allows, and the two no
  // longer fight each other for the space between them.)
  headBtn: {width: 40, height: 40, alignItems: 'center', justifyContent: 'center', marginLeft: 4},
  conn: {flexDirection: 'row', alignItems: 'center'},
  connDot: {width: 7, height: 7, borderRadius: 3.5, marginRight: 5},
  // A ring OUTSIDE the dot: same colour, so it reads as the same indicator in a
  // different state rather than as a second thing to decode.
  connAwake: {
    position: 'absolute', left: -3, top: -3, width: 13, height: 13,
    borderRadius: 6.5, borderWidth: 1, opacity: 0.75,
  },
  connText: {fontSize: 11},
  // Always-dark banner → FIXED light text (never pal.fg, which is near-black in light mode).
  authBanner: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 10, borderTopWidth: 2, gap: 4},
  authBannerText: {flex: 1, fontSize: 12, color: '#F3D9DE', fontWeight: '600'},
  guestBanner: {paddingHorizontal: 16, paddingVertical: 7, borderBottomWidth: 1},
  guestBannerText: {fontSize: 12, fontWeight: '600'},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 70, paddingHorizontal: 40},
  emptyText: {fontSize: 15, fontWeight: '600', marginTop: 16},
  emptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
  banner: {paddingHorizontal: 14, paddingVertical: 10},
  bannerText: {color: '#fff', fontSize: 13, fontWeight: '600'},
  closeDemo: {fontSize: 20, fontWeight: '400'},
  demoBanner: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingTop: 6, paddingBottom: 2},
  demoChip: {fontSize: 10, fontWeight: '800', letterSpacing: 0.6, borderWidth: 1, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 1, overflow: 'hidden'},
  demoBannerText: {flex: 1, fontSize: 12, lineHeight: 16},
  cta: {marginHorizontal: 14, marginBottom: 12, paddingVertical: 12, borderRadius: 12, alignItems: 'center'},
  ctaText: {color: '#fff', fontSize: 15, fontWeight: '700'},
});
