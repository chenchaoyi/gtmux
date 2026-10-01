// ServersScreen — the connection page. Lists every Mac you've paired, shows which
// one is connected (green dot), and lets you switch, add, remove, or disconnect.
// Shown two ways: as the root when nothing is connected (no `navigation`), and
// pushed from the radar's server chip while connected (has `navigation`, so it
// can go back). Adding a Mac reuses PairingScreen in a modal.

import React, {useEffect, useState} from 'react';
import {
  Alert,
  Modal,
  ScrollView,
  StyleSheet,
  Switch,
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
import {BRAND, StatusColor} from '../ui/theme';
import {PairingScreen} from './PairingScreen';
import {DemoScreen} from './DemoScreen';
import {TestIds} from '../constants/testIds';

export function ServersScreen({navigation}: {navigation?: any}) {
  const {t, pal, servers, activeUrl, selectServer, removeServer, disconnect,
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

  // One Mac per card. Connect, notifications and removal have separate tap targets.
  const serverRow = (s: PairedMac, guest = false) => {
    const active = s.url === activeUrl;
    const connected = active && agentsCtx?.conn === 'live';
    const wantsPush = pushEnabled && (pushKinds.waiting || pushKinds.done) && s.pushEnabled !== false;
    const sync = pushSync[s.url];
    // The switch already says On/Off. Only exceptional states need a second sentence.
    const notice = sync === 'pending' ? t(wantsPush ? 'serverPushPendingOn' : 'serverPushPendingOff') :
      sync === 'syncing' ? t('serverPushSyncing') :
      s.pushEnabled !== false && (!pushEnabled || (!pushKinds.waiting && !pushKinds.done)) ? t('serverPushPaused') : null;
    const status = connected ? t('connectedLabel') : active ? t(agentsCtx?.conn === 'connecting' ? 'serverConnecting' : 'serverOffline') : t('serverConnect');
    return (
      <View key={s.url} style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
        <View style={styles.row}>
          <TouchableOpacity
            style={styles.rowMain} onPress={() => onPick(s.url)} activeOpacity={0.6}
            accessibilityRole="button" accessibilityLabel={`${s.name}, ${status}`}
            accessibilityState={{selected: active}}>
            <View style={styles.rowText}>
              <Text style={[styles.name, {color: pal.fg}]} numberOfLines={2}>{s.name}</Text>
              <Text style={[styles.url, {color: pal.fg3}]} numberOfLines={1} ellipsizeMode="middle">{s.url}</Text>
              <View style={styles.connectionStatus}>
                {active && <View style={styles.dotWrap}>
                  <View style={[styles.dot, {backgroundColor: connected ? StatusColor.idle : agentsCtx?.conn === 'connecting' ? StatusColor.working : StatusColor.waiting}]} />
                  {connected && srvOn && <View style={[styles.dotAwake, {borderColor: StatusColor.idle}]} />}
                </View>}
                <Text style={[styles.connectionLabel, {color: connected ? StatusColor.idle : pal.fg2}]}>{status}</Text>
                {connected && srvOn && <Text style={[styles.connectionLabel, {color: pal.fg3}]}>{t('serverModeShort')}</Text>}
                {guest && <Text style={[styles.connectionLabel, {color: pal.fg3}]}>{t('guestRowLabel')}</Text>}
              </View>
            </View>
            <Text style={[styles.chevron, {color: pal.fg3}]}>›</Text>
          </TouchableOpacity>
          <TouchableOpacity onPress={() => more(s)} style={styles.more} accessibilityRole="button" accessibilityLabel={`${s.name} · ${t('serverMore')}`}>
            <Text style={[styles.moreText, {color: pal.fg2}]}>•••</Text>
          </TouchableOpacity>
        </View>
        {!guest && <>
          <View style={[styles.pushRow, {borderTopColor: pal.divider}]}>
            <Text style={[styles.pushLabel, {color: pal.fg}]}>{t('serverPush')}</Text>
            <Switch value={s.pushEnabled !== false}
              onValueChange={v => setServerPushEnabled(s.url, v).catch(() => Alert.alert(t('serverPushSaveFailed')))}
              accessibilityLabel={`${s.name} · ${t('serverPush')}`} trackColor={{true: BRAND}} />
          </View>
          {!!notice && <View style={styles.noticeRow}>
            <Text style={[styles.pushStatus, {color: pal.fg2}]}>{notice}</Text>
            {sync === 'pending' && <TouchableOpacity onPress={retryPushSync}
              accessibilityRole="button" accessibilityLabel={`${s.name} · ${t('serverPushRetry')}`} style={styles.retry}>
              <Text style={[styles.retryText, {color: pal.fg}]}>{t('serverPushRetry')}</Text>
            </TouchableOpacity>}
          </View>}
        </>}
      </View>
    );
  };

  const more = (s: PairedMac) => Alert.alert(s.name, undefined, [
    ...(s.url === activeUrl ? [{text: t('disconnect'), onPress: disconnect}] : []),
    {text: t('removeMac'), style: 'destructive', onPress: () => confirmRemove(s)},
    {text: t('cancel'), style: 'cancel'},
  ]);

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
                      <View style={styles.list}>
                        {mine.map(s => serverRow(s))}
                      </View>
                    </>
                  )}
                  {guests.length > 0 && (
                    <>
                      <Text style={[styles.groupTitle, {color: pal.fg2}]}>{t('guestConnections')}</Text>
                      <View style={styles.list}>
                        {guests.map(s => serverRow(s, true))}
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
  list: {gap: 12},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  row: {flexDirection: 'row', alignItems: 'flex-start'},
  rowMain: {flex: 1, flexDirection: 'row', alignItems: 'center', gap: 12, padding: 16, minWidth: 0},
  rowText: {flex: 1, minWidth: 0},
  name: {fontSize: 16, lineHeight: 22, fontWeight: '600'},
  url: {fontSize: 12, marginTop: 3},
  connectionStatus: {flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 8, marginTop: 10},
  connectionLabel: {fontSize: 12, fontWeight: '500'},
  dotWrap: {width: 9, height: 9},
  dot: {width: 9, height: 9, borderRadius: 5},
  dotAwake: {position: 'absolute', width: 15, height: 15, borderRadius: 8, borderWidth: 1.5, left: -3, top: -3},
  chevron: {fontSize: 22},
  more: {minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center', marginTop: 6},
  moreText: {fontSize: 12, letterSpacing: 1},
  pushRow: {borderTopWidth: StyleSheet.hairlineWidth, marginHorizontal: 16, paddingVertical: 12, flexDirection: 'row', alignItems: 'center', gap: 12},
  pushLabel: {flex: 1, fontSize: 14, fontWeight: '500'},
  noticeRow: {paddingHorizontal: 16, paddingBottom: 12, gap: 4, alignItems: 'flex-start'},
  pushStatus: {fontSize: 12, lineHeight: 18},
  retry: {minHeight: 44, justifyContent: 'center'},
  retryText: {fontSize: 13, fontWeight: '600'},
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
