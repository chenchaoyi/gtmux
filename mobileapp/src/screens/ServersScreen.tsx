// ServersScreen — the connection page. Lists every Mac you've paired, one row each, Wi-Fi
// style: a check on the open one, a bell for its notifications, ••• for the rest, and
// under the name a status line saying whether that Mac answers (servers-reachability),
// and lets you switch, add, rename, remove, or disconnect, and put them in your own
// order: hold a row and drag it (ReorderableList), or VoiceOver's Move up / Move down.
// Shown two ways: as the root when nothing is connected (no `navigation`), and
// pushed from the radar's server chip while connected (has `navigation`, so it
// can go back). Adding a Mac reuses PairingScreen in a modal.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {
  AccessibilityInfo,
  Alert,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaProvider, SafeAreaView} from 'react-native-safe-area-context';
import {useApp} from '../state/AppContext';
import {splitServers} from '../pairing/store';
import {PairedMac} from '../pairing/qr';
import type {ServerMode} from '../api/types';
import {useAgentsOptional} from '../state/AgentsContext';
import {BrandMark} from '../ui/BrandMark';
import {ContentColumn} from '../ui/ContentColumn';
import {SIcon} from '../ui/SettingsIcons';
import {BRAND, StatusColor} from '../ui/theme';
import {PairingScreen} from './PairingScreen';
import {DemoScreen} from './DemoScreen';
import {TestIds} from '../constants/testIds';
import {ReorderableList, ScrollHost} from '../ui/ReorderableList';
import {Reach, rowStatus, RowTone, useReachability} from './serverReachability';
import {MODAL_ORIENTATIONS} from '../ui/modalOrientations';
import {ServerDetailsSheet} from './ServerDetailsSheet';
import {AnchoredMenu, MenuAnchor, MenuItem} from '../ui/AnchoredMenu';
import {cachedHost, hostSummary, hostSystem, loadHost} from '../state/hostInfo';
import type {HostAnswer} from '../api/types';
import {Debug} from '../debug';
import {APP_VERSION} from '../version';

export function ServersScreen({navigation}: {navigation?: any}) {
  const {t, pal, lang, servers, activeUrl, selectServer, removeServer, renameServer, moveServer, disconnect,
    pushEnabled, pushKinds, pushSync, setServerPushEnabled, retryPushSync} = useApp();

  // The scroll view, as the reorderable list needs it: held still while a row is lifted,
  // scrolled by the list near its edges, and its visible band on screen.
  const scrollRef = useRef<ScrollView>(null);
  const [scrollEnabled, setScrollEnabled] = useState(true);
  const scroll = useRef({y: 0, content: 0, view: 0, top: 0}).current;
  const host: ScrollHost = {
    setScrollEnabled,
    scrollBy: dy => {
      const max = Math.max(0, scroll.content - scroll.view);
      const next = Math.max(0, Math.min(max, scroll.y + dy));
      const went = next - scroll.y;
      if (went !== 0) {
        scroll.y = next;
        scrollRef.current?.scrollTo({y: next, animated: false});
      }
      return went;
    },
    band: () => (scroll.view > 0 ? {top: scroll.top, bottom: scroll.top + scroll.view} : null),
  };
  // A move that did not save is put back by the list, and said here.
  const move = (url: string, to: number) =>
    moveServer(url, to).catch(e => {
      Alert.alert(t('serverOrderSaveFailed'));
      throw e;
    });
  // May be null: this page also renders before anything is connected.
  const agentsCtx = useAgentsOptional();
  const client = agentsCtx?.client;

  // Server mode for the Mac we are CONNECTED to. Only that one — a paired Mac we are
  // not talking to right now cannot be asked, and inventing a state for it would be
  // worse than showing none. Read at once when the Mac says it changed (serverModeRev)
  // or the stream came back; the slow poll stays as the fallback, since the hint is
  // not guaranteed to arrive (a Mac that sleeps takes the stream with it).
  const serverModeRev = agentsCtx?.serverModeRev ?? 0;
  const [srv, setSrv] = useState<{url: string; mode: ServerMode} | null>(null);
  useEffect(() => {
    if (!client) return;
    let alive = true;
    const tick = () => {
      client.serverMode().then(m => alive && setSrv(m ? {url: activeUrl ?? '', mode: m} : null)).catch(() => {});
    };
    tick();
    const id = setInterval(tick, 30000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, activeUrl, serverModeRev]);
  const srvOn = srv?.url === activeUrl && !!srv && (srv.mode.system_disablesleep || srv.mode.state === 'lapsed');
  // First run (no servers) opens the add sheet straight away — same as before.
  const [adding, setAdding] = useState(servers.length === 0);
  // The read-only demo tour (from the pairing screen's "See a demo"). Only ever
  // shown on this no-server connection page — a paired user never reaches it.
  const [demo, setDemo] = useState(false);

  const onPick = async (url: string) => {
    if (url === activeUrl) {
      navigation?.goBack?.(); // already connected → just dismiss
      return;
    }
    await selectServer(url); // switching active remounts the radar (App key=url)
  };

  // The device-wide switch or both alert kinds are off: every bell is moot until they're
  // back. Said once under the list, not once per Mac.
  const pushPaused = !pushEnabled || (!pushKinds.waiting && !pushKinds.done);

  // Whether each Mac answers, asked only while this page is shown. The open Mac also
  // speaks for its live link; the probe still runs for it, for when that link is down.
  const probed = useReachability(servers.map(s => s.url), true);
  // SHOT_MODE, store screenshots only: the seeded Macs are addresses nothing answers, so the
  // probe read Checking… and then Can't reach, and the open Mac's link stayed Connecting…
  // (%6, the 1.0.97 captures). A capture shows them answering instead: the open Mac
  // Connected, the rest Available, each with what it is. The shipped app never sets it.
  const shot = Debug.shotMode;
  const reach = useMemo<Record<string, Reach | undefined>>(
    () => (shot ? Object.fromEntries(servers.map(s => [s.url, 'reachable' as Reach])) : probed),
    [shot, servers, probed],
  );
  const conn = shot ? 'live' : agentsCtx?.conn;
  // A Mac whose notification setting is pending gets it again the moment a probe finds it
  // answering. That is what "Retry sync" asked the reader to do by hand, for a Mac they
  // could not see was off (2026-10-05).
  const lastReach = useRef<Record<string, Reach | undefined>>({});
  useEffect(() => {
    let resync = false;
    for (const s of servers) {
      const now = reach[s.url];
      if (now === 'reachable' && lastReach.current[s.url] !== 'reachable' && pushSync[s.url] === 'pending') resync = true;
      lastReach.current[s.url] = now;
    }
    if (resync) retryPushSync();
  }, [reach, servers, pushSync, retryPushSync]);

  // What each owned Mac actually is (GET /api/host), asked when a probe finds it answering
  // and checked every minute while the page is shown; loadHost keeps an answer five
  // minutes, so the check reaches the Mac only once it is stale (a restarted serve reports
  // a new version and uptime). Keyed by the credential, and a share link is never asked
  // nor shown one (an owner's answer cached for the same address must not reach its row).
  const hostKey = (s: PairedMac) => `${s.url}\n${s.token}`;
  const [hosts, setHosts] = useState<Record<string, HostAnswer | undefined>>(() =>
    Object.fromEntries(servers.filter(s => s.scope !== 'guest').map(s => [hostKey(s), cachedHost(s.url, s.token)])));
  useEffect(() => {
    if (shot) return; // a capture is never asked: shotHost answers for it
    let live = true;
    const ask = () => {
      for (const s of servers) {
        if (s.scope === 'guest' || reach[s.url] !== 'reachable') continue;
        loadHost(s.url, s.token).then(a => {
          if (live) setHosts(h => (h[hostKey(s)] === a ? h : {...h, [hostKey(s)]: a}));
        });
      }
    };
    ask();
    const id = setInterval(ask, 60_000);
    return () => {
      live = false;
      clearInterval(id);
    };
  }, [reach, servers, shot]);
  const [details, setDetails] = useState<PairedMac | null>(null);

  // What a Mac's status line says, and in what colour. Its row says it, and so does the
  // head of its Details sheet, which must not tell a different story. The sheet takes the
  // short form, the state and the system: its This Mac group names the computer just below,
  // and the whole line wrapped to two lines under the title on an iPhone (%6, #1532).
  const statusOf = (s: PairedMac, guest = s.scope === 'guest') => {
    const active = s.url === activeUrl;
    const what = guest ? undefined : shot ? shotHost(s) : hosts[hostKey(s)];
    const st = rowStatus({
      active,
      conn: active ? conn : undefined,
      reach: reach[s.url],
      pending: !guest && pushSync[s.url] === 'pending',
      mayNotify: !pushPaused && s.pushEnabled !== false,
      rejected: what?.ok === false && what.why === 'auth',
    });
    const state = t(st.key) + (st.pending ? ` · ${t(st.pending)}` : '');
    const text = state + (what?.ok ? ` · ${hostSummary(what.info)}` : '');
    const short = state + (what?.ok ? ` · ${hostSystem(what.info)}` : '');
    return {st, text, short, tone: toneColor(st.tone, pal.fg3)};
  };

  // One row per Mac, always two lines: the name with its bell and •••, and a status line.
  // Tapping the row connects; the bell and ••• are their own targets. The address lives
  // in ••• — it tells two Macs apart only when their names don't. Nothing is ever added
  // under a row: a pending setting is a clause on the status line, and a sync in flight
  // is not shown, so no tap and no probe moves the list (it jumped on every tap: each
  // change set every Mac "syncing", and each row grew a line and shrank again). What the
  // Mac is ("Studio · macOS 26.1") is a clause on that same line, for the same reason.
  const serverRow = (
    s: PairedMac,
    i: number,
    guest: boolean,
    count: number,
    drag: {lift: () => void; onPressOut: () => void; lifted: boolean},
  ) => {
    const active = s.url === activeUrl;
    const connected = active && conn === 'live';
    const muted = s.pushEnabled === false;
    const {st, text: status, tone} = statusOf(s, guest);
    const awake = connected && srvOn;
    return (
      <View key={s.url} style={drag.lifted ? {backgroundColor: pal.surface} : undefined}>
        {i > 0 && !drag.lifted && <View style={[styles.sep, {backgroundColor: pal.divider}]} />}
        <View style={styles.row}>
          <TouchableOpacity
            style={styles.rowMain} onPress={() => onPick(s.url)} activeOpacity={0.6}
            // Held, the row lifts to be dragged; a tap still connects, and no drag taps.
            onLongPress={count > 1 ? drag.lift : undefined} delayLongPress={300} onPressOut={drag.onPressOut}
            accessibilityRole="button"
            accessibilityLabel={`${s.name}${active ? `, ${t('serverCurrent')}` : ''}, ${status}${awake ? `, ${t('serverModeShort')}` : ''}`}
            accessibilityState={{selected: active}}
            accessibilityActions={[
              ...(i > 0 ? [{name: 'moveUp', label: t('serverMoveUp')}] : []),
              ...(i < count - 1 ? [{name: 'moveDown', label: t('serverMoveDown')}] : []),
            ]}
            onAccessibilityAction={e => {
              const to = e.nativeEvent.actionName === 'moveUp' ? i - 1 : i + 1;
              move(s.url, to)
                .then(() => AccessibilityInfo.announceForAccessibility(
                  t('serverMovedTo').replace('{name}', s.name).replace('{n}', String(to + 1)).replace('{total}', String(count))))
                .catch(() => {});
            }}>
            {/* The check says which Mac is open; its slot is always there, so no row shifts. */}
            <View style={styles.checkSlot} testID={active ? `server-current-${i}` : undefined}>
              {active && <SIcon name="check" size={17} color={BRAND} />}
            </View>
            <View style={styles.rowText}>
              <Text style={[styles.name, {color: pal.fg}]} numberOfLines={1}>{s.name}</Text>
              {/* The status line: whether this Mac answers. Filled dot on the open Mac (its
                  live link), hollow on the others (the probe). One line, always. */}
              <View style={styles.statusLine}>
                <View style={styles.dotSlot}>
                  <View style={[styles.dot, st.filled ? {backgroundColor: tone} : [styles.dotHollow, {borderColor: tone}]]} />
                  {awake && <View style={[styles.dotAwake, {borderColor: tone}]} />}
                </View>
                <Text style={[styles.sub, {color: pal.fg2}]} numberOfLines={1}>{status}</Text>
              </View>
            </View>
          </TouchableOpacity>
          {!guest && <TouchableOpacity
            onPress={() => setServerPushEnabled(s.url, muted).catch(() => Alert.alert(t('serverPushSaveFailed')))}
            style={styles.iconBtn}
            accessibilityRole="switch" accessibilityState={{checked: !muted}}
            accessibilityLabel={`${s.name} · ${t('serverPush')}`}>
            <SIcon name={muted ? 'bellOff' : 'bell'} size={20} color={muted ? pal.fg3 : BRAND} />
          </TouchableOpacity>}
          <TouchableOpacity
            ref={r => {
              moreButtons.current[s.url] = r;
            }}
            onPress={() => openMore(s)} style={styles.iconBtn} accessibilityRole="button" accessibilityLabel={`${s.name} · ${t('serverMore')}`}>
            <Text style={[styles.moreText, {color: pal.fg2}]}>•••</Text>
          </TouchableOpacity>
        </View>
      </View>
    );
  };

  // ••• opens a menu that drops from the button (AnchoredMenu). Its header carries the
  // address, and the Mac's own name once it was renamed. Then Details and Rename;
  // Disconnect, neutral and only on the open Mac, since it keeps the Mac saved; and Remove,
  // last and in red, which still asks first. It opens at once and is placed under the
  // button when the button's measurement arrives (AnchoredMenu waits for it, briefly).
  const moreButtons = useRef<Record<string, any>>({});
  const [menu, setMenu] = useState<{mac: PairedMac; anchor: MenuAnchor | null} | null>(null);
  // What the menu shows while it fades out, after `menu` has gone null.
  const shownMenu = useRef(menu);
  if (menu) shownMenu.current = menu;
  const openMore = (s: PairedMac) => {
    setMenu({mac: s, anchor: null});
    moreButtons.current[s.url]?.measureInWindow?.((x: number, y: number, width: number, height: number) =>
      setMenu(m => (m && m.mac.url === s.url ? {mac: m.mac, anchor: {x, y, width, height}} : m)));
  };
  const shownStatus = details ? statusOf(details) : undefined;
  const detailsStatus = shownStatus && {st: shownStatus.st, tone: shownStatus.tone, text: shownStatus.short};
  const menuSections = (s: PairedMac): MenuItem[][] => [
    [
      {key: 'details', label: t('serverDetails'), icon: 'info', onPress: () => setDetails(s)},
      {key: 'rename', label: t('renameServer'), icon: 'pencil', onPress: () => rename(s)},
    ],
    s.url === activeUrl ? [{key: 'disconnect', label: t('disconnect'), icon: 'disconnect', onPress: () => disconnect()}] : [],
    [{key: 'remove', label: t('removeServerMenu'), icon: 'trash', danger: true, onPress: () => confirmRemove(s)}],
  ];

  // A name on this phone only: the Mac keeps its own, and pushes are still matched by it.
  const rename = (s: PairedMac) => Alert.prompt?.(
    t('renameServer'),
    t('renameServerHint').replace('{name}', s.macName ?? s.name),
    [
      {text: t('cancel'), style: 'cancel'},
      {text: t('renameServerSave'), onPress: (v?: string) => {
        renameServer(s.url, v ?? '').catch(() => Alert.alert(t('renameServerFailed')));
      }},
    ],
    'plain-text',
    s.name,
  );

  const confirmRemove = (m: {url: string; name: string}) =>
    Alert.alert(m.name, t('removeServerQ'), [
      {text: t('cancel'), style: 'cancel'},
      {text: t('removeMac'), style: 'destructive', onPress: () => {
        removeServer(m.url).catch(() => Alert.alert(t('removeServerFailed')));
      }},
    ]);

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.servers.screen}>
      {/* header: back (only when pushed) + title */}
      <ContentColumn>
        <View style={styles.header}>
          {navigation?.canGoBack?.() && (
            <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit} style={styles.back}>
              <Text style={[styles.backText, {color: pal.fg2}]}>‹</Text>
            </TouchableOpacity>
          )}
          <Text style={[styles.title, {color: pal.fg}]}>{t('servers')}</Text>
        </View>
      </ContentColumn>

      <ScrollView
        ref={scrollRef}
        contentContainerStyle={styles.body}
        scrollEnabled={scrollEnabled}
        scrollEventThrottle={16}
        onScroll={e => {
          scroll.y = e.nativeEvent.contentOffset.y;
        }}
        onContentSizeChange={(_w, h) => {
          scroll.content = h;
        }}
        onLayout={e => {
          scroll.view = e.nativeEvent.layout.height;
          const target: any = scrollRef.current ?? e.currentTarget;
          target?.measureInWindow?.((_x: number, y: number) => {
            scroll.top = y;
          });
        }}>
        <ContentColumn>
        {servers.length === 0 ? (
          <View style={styles.empty}>
            <BrandMark size={48} neutral={pal.fg3} />
            <Text style={[styles.emptyText, {color: pal.fg2}]}>{t('noServers')}</Text>
          </View>
        ) : (
          <>
            <Text style={[styles.hint, {color: pal.fg2}]}>{t('serverPushHint')}</Text>
            {/* Two-track model (pair-share): my own paired Macs (full control) vs
                guest connections via share links (least privilege) — never mixed. */}
            {(() => {
              const {mine, guests} = splitServers(servers);
              return (
                <>
                  {mine.length > 0 && (
                    <>
                      <Text style={[styles.groupTitle, {color: pal.fg2}]}>{t('myMacs')}</Text>
                      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
                        <ReorderableList
                          testID="servers-mine"
                          keys={mine.map(s => s.url)}
                          host={host}
                          onMove={move}
                          render={(url, i, drag) => serverRow(mine.find(s => s.url === url)!, i, false, mine.length, drag)}
                        />
                      </View>
                      {pushPaused && <Text style={[styles.footnote, {color: pal.fg2}]}>{t('serverPushPaused')}</Text>}
                    </>
                  )}
                  {guests.length > 0 && (
                    <>
                      <Text style={[styles.groupTitle, {color: pal.fg2}]}>{t('guestConnections')}</Text>
                      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
                        <ReorderableList
                          testID="servers-guests"
                          keys={guests.map(s => s.url)}
                          host={host}
                          onMove={move}
                          render={(url, i, drag) => serverRow(guests.find(s => s.url === url)!, i, true, guests.length, drag)}
                        />
                      </View>
                    </>
                  )}
                  {(mine.length > 1 || guests.length > 1) && (
                    <Text style={[styles.footnote, {color: pal.fg2}]}>{t('serverReorderHint')}</Text>
                  )}
                </>
              );
            })()}
          </>
        )}

        <TouchableOpacity
          testID={TestIds.servers.add}
          accessibilityLabel={t('addMac')}
          style={[styles.add, {borderColor: pal.divider, backgroundColor: pal.surface}]}
          onPress={() => setAdding(true)}>
          <Text style={[styles.addText, {color: pal.fg2}]}>＋  {t('addMac')}</Text>
        </TouchableOpacity>

        {/* Disconnect: only when something is connected — drops the live link and
            lands you back here (the connection page). */}
        {!!activeUrl && (
          <TouchableOpacity
            testID={TestIds.servers.disconnect}
            accessibilityLabel={t('disconnect')}
            style={styles.disconnect}
            onPress={disconnect}
            hitSlop={hit}>
            <Text style={[styles.disconnectText, {color: StatusColor.waiting}]}>{t('disconnect')}</Text>
          </TouchableOpacity>
        )}
        </ContentColumn>
      </ScrollView>

      <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={adding} animationType="slide" onRequestClose={() => setAdding(false)}>
        {/* Fresh provider: inside a RN Modal, safe-area insets are otherwise zero,
            so PairingScreen's Cancel collided with the status-bar clock (REVIEW P0). */}
        <SafeAreaProvider>
          <PairingScreen onCancel={() => setAdding(false)} onDemo={() => { setAdding(false); setDemo(true); }} />
        </SafeAreaProvider>
      </Modal>

      <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={demo} animationType="slide" onRequestClose={() => setDemo(false)}>
        <SafeAreaProvider>
          <DemoScreen onExit={() => setDemo(false)} onPair={() => { setDemo(false); setAdding(true); }} />
        </SafeAreaProvider>
      </Modal>
      <AnchoredMenu
        visible={!!menu}
        anchor={shownMenu.current?.anchor ?? null}
        title={shownMenu.current?.mac.name ?? ''}
        // The Mac's own name while a rename is in effect, then the address, by host: the
        // scheme says nothing a reader acts on, and it pushed the host into an ellipsis.
        subtitle={shownMenu.current
          ? [shownMenu.current.mac.macName && shownMenu.current.mac.macName !== shownMenu.current.mac.name ? shownMenu.current.mac.macName : '', shownMenu.current.mac.url.replace(/^https?:\/\//, '').replace(/\/$/, '')]
          : []}
        sections={shownMenu.current ? menuSections(shownMenu.current.mac) : []}
        pal={pal}
        closeLabel={t('cancel')}
        onClose={() => setMenu(null)}
        testID="server-menu"
        lift={<Text style={[styles.moreText, {color: pal.fg}]}>•••</Text>}
      />
      <ServerDetailsSheet
        mac={details}
        status={detailsStatus}
        pal={pal}
        lang={lang}
        t={t}
        onClose={() => setDetails(null)}
      />
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

/** What a seeded Mac is in a store capture (SHOT_MODE): its own name and a current macOS. */
function shotHost(s: PairedMac): HostAnswer {
  const name = s.macName ?? s.name;
  return {ok: true, info: {hostname: `${name.replace(/[^A-Za-z0-9]+/g, '-')}.local`, computer_name: name, os: 'macOS',
    os_version: '26.1', arch: 'arm64', cores: 12, gtmux_version: APP_VERSION, serve_started: 0}};
}

/** The status dot's colour: the connection colours (green / amber / red), grey unknown. */
function toneColor(tone: RowTone, unknown: string): string {
  return tone === 'ok' ? StatusColor.idle : tone === 'busy' ? AMBER : tone === 'bad' ? StatusColor.waiting : unknown;
}
// Reconnecting amber, the same as the radar's connection dot.
const AMBER = '#F59E0B';

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 12},
  back: {paddingRight: 4},
  backText: {fontSize: 30, fontWeight: '300', lineHeight: 30},
  title: {fontSize: 26, fontWeight: '700'},
  body: {padding: 16, paddingTop: 4},
  groupTitle: {fontSize: 12, fontWeight: '600', marginTop: 14, marginBottom: 6, textTransform: 'uppercase', letterSpacing: 0.4},
  hint: {fontSize: 12.5, lineHeight: 18, marginBottom: 12, marginLeft: 2},
  empty: {alignItems: 'center', paddingVertical: 28, gap: 12},
  emptyText: {fontSize: 14, textAlign: 'center'},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  // Inset to the name, so the check column reads as one gutter.
  sep: {height: StyleSheet.hairlineWidth, marginLeft: 42},
  // Fixed height: two lines whatever the row says, so nothing in the list ever moves.
  row: {flexDirection: 'row', alignItems: 'center', height: 62, paddingRight: 4},
  rowMain: {flex: 1, flexDirection: 'row', alignItems: 'center', gap: 8, paddingLeft: 16, height: '100%', minWidth: 0},
  checkSlot: {width: 18, height: 18, alignItems: 'center', justifyContent: 'center'},
  rowText: {flex: 1, minWidth: 0},
  name: {fontSize: 16, lineHeight: 21, fontWeight: '500'},
  statusLine: {flexDirection: 'row', alignItems: 'center', gap: 6, marginTop: 2, height: 18},
  dotSlot: {width: 13, height: 13, alignItems: 'center', justifyContent: 'center'},
  dot: {width: 8, height: 8, borderRadius: 4},
  dotHollow: {borderWidth: 1.5},
  dotAwake: {position: 'absolute', width: 13, height: 13, borderRadius: 7, borderWidth: 1.5},
  sub: {fontSize: 12.5, lineHeight: 17, flexShrink: 1},
  iconBtn: {width: 44, height: 44, alignItems: 'center', justifyContent: 'center'},
  moreText: {fontSize: 12, letterSpacing: 1},
  footnote: {fontSize: 12.5, lineHeight: 18, marginTop: 6, marginHorizontal: 16},
  add: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 12,
    paddingVertical: 15,
    alignItems: 'center',
    marginTop: 18,
  },
  addText: {fontSize: 15.5, fontWeight: '600'},
  disconnect: {alignItems: 'center', paddingVertical: 18, marginTop: 6},
  disconnectText: {fontSize: 14.5, fontWeight: '600'},
});
