// ServersScreen — the connection page. Lists every Mac you've paired, one line each
// (Wi-Fi style: a dot on the open one, a bell for its notifications, ••• for the rest),
// and lets you switch, add, rename, remove, or disconnect.
// Shown two ways: as the root when nothing is connected (no `navigation`), and
// pushed from the radar's server chip while connected (has `navigation`, so it
// can go back). Adding a Mac reuses PairingScreen in a modal.

import React, {useEffect, useState} from 'react';
import {
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

export function ServersScreen({navigation}: {navigation?: any}) {
  const {t, pal, servers, activeUrl, selectServer, removeServer, renameServer, disconnect,
    pushEnabled, pushKinds, pushSync, setServerPushEnabled, retryPushSync} = useApp();
  // May be null: this page also renders before anything is connected.
  const agentsCtx = useAgentsOptional();
  const client = agentsCtx?.client;

  // Server mode for the Mac we are CONNECTED to. Only that one — a paired Mac we are
  // not talking to right now cannot be asked, and inventing a state for it would be
  // worse than showing none. Slow poll: this changes when a human decides it does.
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
  }, [client, activeUrl]);
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

  // One line per Mac. Tapping the row connects; the bell and ••• are their own targets.
  // The address lives in ••• — it tells two Macs apart only when their names don't.
  // A second line appears only when something needs reading: the open Mac isn't
  // connected, or this Mac's notification setting hasn't reached it yet.
  const serverRow = (s: PairedMac, i: number, guest = false) => {
    const active = s.url === activeUrl;
    const connected = active && agentsCtx?.conn === 'live';
    const muted = s.pushEnabled === false;
    const sync = pushSync[s.url];
    const notice = sync === 'pending' ? t(!pushPaused && !muted ? 'serverPushPendingOn' : 'serverPushPendingOff') :
      sync === 'syncing' ? t('serverPushSyncing') : null;
    const status = connected ? t('connectedLabel') : active ? t(agentsCtx?.conn === 'connecting' ? 'serverConnecting' : agentsCtx?.conn === 'unauthorized' ? 'serverRejected' : 'serverOffline') : t('serverConnect');
    const awake = connected && srvOn;
    return (
      <View key={s.url}>
        {i > 0 && <View style={[styles.sep, {backgroundColor: pal.divider}]} />}
        <View style={styles.row}>
          <TouchableOpacity
            style={styles.rowMain} onPress={() => onPick(s.url)} activeOpacity={0.6}
            accessibilityRole="button" accessibilityLabel={`${s.name}, ${status}${awake ? `, ${t('serverModeShort')}` : ''}`}
            accessibilityState={{selected: active}}>
            <View style={styles.dotSlot}>
              {active && <>
                <View style={[styles.dot, {backgroundColor: connected ? StatusColor.idle : agentsCtx?.conn === 'connecting' ? StatusColor.working : StatusColor.waiting}]} />
                {awake && <View style={[styles.dotAwake, {borderColor: StatusColor.idle}]} />}
              </>}
            </View>
            <View style={styles.rowText}>
              <Text style={[styles.name, {color: pal.fg}]} numberOfLines={1}>{s.name}</Text>
              {active && !connected && <Text style={[styles.sub, {color: pal.fg2}]}>{status}</Text>}
            </View>
          </TouchableOpacity>
          {!guest && <TouchableOpacity
            onPress={() => setServerPushEnabled(s.url, muted).catch(() => Alert.alert(t('serverPushSaveFailed')))}
            style={styles.iconBtn}
            accessibilityRole="switch" accessibilityState={{checked: !muted}}
            accessibilityLabel={`${s.name} · ${t('serverPush')}`}>
            <SIcon name={muted ? 'bellOff' : 'bell'} size={20} color={muted ? pal.fg3 : BRAND} />
          </TouchableOpacity>}
          <TouchableOpacity onPress={() => more(s)} style={styles.iconBtn} accessibilityRole="button" accessibilityLabel={`${s.name} · ${t('serverMore')}`}>
            <Text style={[styles.moreText, {color: pal.fg2}]}>•••</Text>
          </TouchableOpacity>
        </View>
        {!guest && !!notice && <View style={styles.noticeRow}>
          <Text style={[styles.notice, {color: pal.fg2}]}>{notice}</Text>
          {sync === 'pending' && <TouchableOpacity onPress={retryPushSync}
            accessibilityRole="button" accessibilityLabel={`${s.name} · ${t('serverPushRetry')}`} style={styles.retry}>
            <Text style={[styles.retryText, {color: pal.fg}]}>{t('serverPushRetry')}</Text>
          </TouchableOpacity>}
        </View>}
      </View>
    );
  };

  // The address lives here, and so does the Mac's own name once it was renamed.
  const more = (s: PairedMac) => Alert.alert(s.name, s.macName ? `${s.macName}\n${s.url}` : s.url, [
    {text: t('renameServer'), onPress: () => rename(s)},
    ...(s.url === activeUrl ? [{text: t('disconnect'), onPress: disconnect}] : []),
    {text: t('removeMac'), style: 'destructive', onPress: () => confirmRemove(s)},
    {text: t('cancel'), style: 'cancel'},
  ]);

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
      {text: t('removeMac'), style: 'destructive', onPress: () => removeServer(m.url)},
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

      <ScrollView contentContainerStyle={styles.body}>
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
                        {mine.map((s, i) => serverRow(s, i))}
                      </View>
                      {pushPaused && <Text style={[styles.footnote, {color: pal.fg2}]}>{t('serverPushPaused')}</Text>}
                    </>
                  )}
                  {guests.length > 0 && (
                    <>
                      <Text style={[styles.groupTitle, {color: pal.fg2}]}>{t('guestConnections')}</Text>
                      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
                        {guests.map((s, i) => serverRow(s, i, true))}
                      </View>
                    </>
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

      <Modal visible={adding} animationType="slide" onRequestClose={() => setAdding(false)}>
        {/* Fresh provider: inside a RN Modal, safe-area insets are otherwise zero,
            so PairingScreen's Cancel collided with the status-bar clock (REVIEW P0). */}
        <SafeAreaProvider>
          <PairingScreen onCancel={() => setAdding(false)} onDemo={() => { setAdding(false); setDemo(true); }} />
        </SafeAreaProvider>
      </Modal>

      <Modal visible={demo} animationType="slide" onRequestClose={() => setDemo(false)}>
        <SafeAreaProvider>
          <DemoScreen onExit={() => setDemo(false)} onPair={() => { setDemo(false); setAdding(true); }} />
        </SafeAreaProvider>
      </Modal>
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

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
  // Inset to the name, so the dot column reads as one gutter.
  sep: {height: StyleSheet.hairlineWidth, marginLeft: 41},
  row: {flexDirection: 'row', alignItems: 'center', minHeight: 52, paddingRight: 4},
  rowMain: {flex: 1, flexDirection: 'row', alignItems: 'center', gap: 10, paddingLeft: 16, paddingVertical: 10, minWidth: 0},
  dotSlot: {width: 15, height: 15, alignItems: 'center', justifyContent: 'center'},
  dot: {width: 9, height: 9, borderRadius: 5},
  dotAwake: {position: 'absolute', width: 15, height: 15, borderRadius: 8, borderWidth: 1.5},
  rowText: {flex: 1, minWidth: 0},
  name: {fontSize: 16, lineHeight: 22, fontWeight: '500'},
  sub: {fontSize: 12.5, marginTop: 1},
  iconBtn: {width: 44, height: 44, alignItems: 'center', justifyContent: 'center'},
  moreText: {fontSize: 12, letterSpacing: 1},
  noticeRow: {paddingLeft: 41, paddingRight: 16, paddingBottom: 6, marginTop: -4, flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', columnGap: 12},
  notice: {fontSize: 12.5, lineHeight: 18, flexShrink: 1},
  retry: {minHeight: 44, justifyContent: 'center'},
  retryText: {fontSize: 13, fontWeight: '600'},
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
