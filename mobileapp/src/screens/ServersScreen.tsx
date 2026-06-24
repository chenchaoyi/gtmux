// ServersScreen — the connection page. Lists every Mac you've paired, shows which
// one is connected (green dot), and lets you switch, add, remove, or disconnect.
// Shown two ways: as the root when nothing is connected (no `navigation`), and
// pushed from the radar's server chip while connected (has `navigation`, so it
// can go back). Adding a Mac reuses PairingScreen in a modal.

import React, {useState} from 'react';
import {
  Alert,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {StatusColor} from '../ui/theme';
import {PairingScreen} from './PairingScreen';

export function ServersScreen({navigation}: {navigation?: any}) {
  const {t, pal, servers, activeUrl, selectServer, removeServer, disconnect} = useApp();
  // First run (no servers) opens the add sheet straight away — same as before.
  const [adding, setAdding] = useState(servers.length === 0);

  const onPick = async (url: string) => {
    if (url === activeUrl) {
      navigation?.goBack?.(); // already connected → just dismiss
      return;
    }
    await selectServer(url); // switching active remounts the radar (App key=url)
  };

  const confirmRemove = (m: {url: string; name: string}) =>
    Alert.alert(m.name, t('removeServerQ'), [
      {text: t('cancel'), style: 'cancel'},
      {text: t('removeMac'), style: 'destructive', onPress: () => removeServer(m.url)},
    ]);

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      {/* header: back (only when pushed) + title */}
      <View style={styles.header}>
        {navigation?.canGoBack?.() && (
          <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit} style={styles.back}>
            <Text style={[styles.backText, {color: pal.fg2}]}>‹</Text>
          </TouchableOpacity>
        )}
        <Text style={[styles.title, {color: pal.fg}]}>{t('servers')}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        {servers.length === 0 ? (
          <View style={styles.empty}>
            <BrandMark size={48} neutral={pal.fg3} />
            <Text style={[styles.emptyText, {color: pal.fg2}]}>{t('noServers')}</Text>
          </View>
        ) : (
          <>
            <Text style={[styles.hint, {color: pal.fg3}]}>{t('serversHint')}</Text>
            <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              {servers.map((s, i) => {
                const active = s.url === activeUrl;
                return (
                  <View
                    key={s.url}
                    style={[
                      styles.row,
                      i < servers.length - 1 && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth},
                    ]}>
                    <TouchableOpacity style={styles.rowMain} onPress={() => onPick(s.url)} hitSlop={hit}>
                      <View
                        style={[
                          styles.dot,
                          {backgroundColor: active ? StatusColor.idle : pal.fg3, opacity: active ? 1 : 0.35},
                        ]}
                      />
                      <View style={styles.rowText}>
                        <Text style={[styles.name, {color: pal.fg}]} numberOfLines={1}>
                          {s.name}
                        </Text>
                        <Text style={[styles.url, {color: pal.fg3}]} numberOfLines={1}>
                          {active ? `${t('connectedLabel')} · ` : ''}
                          {s.url}
                        </Text>
                      </View>
                    </TouchableOpacity>
                    <TouchableOpacity onPress={() => confirmRemove(s)} hitSlop={hit} style={styles.remove}>
                      <Text style={[styles.removeText, {color: pal.fg3}]}>✕</Text>
                    </TouchableOpacity>
                  </View>
                );
              })}
            </View>
          </>
        )}

        <TouchableOpacity
          style={[styles.add, {borderColor: pal.divider, backgroundColor: pal.surface}]}
          onPress={() => setAdding(true)}>
          <Text style={[styles.addText, {color: pal.fg2}]}>＋  {t('addMac')}</Text>
        </TouchableOpacity>

        {/* Disconnect: only when something is connected — drops the live link and
            lands you back here (the connection page). */}
        {!!activeUrl && (
          <TouchableOpacity style={styles.disconnect} onPress={disconnect} hitSlop={hit}>
            <Text style={[styles.disconnectText, {color: StatusColor.waiting}]}>{t('disconnect')}</Text>
          </TouchableOpacity>
        )}
      </ScrollView>

      <Modal visible={adding} animationType="slide" onRequestClose={() => setAdding(false)}>
        <PairingScreen onCancel={() => setAdding(false)} />
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
  hint: {fontSize: 12.5, lineHeight: 18, marginBottom: 12, marginLeft: 2},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  row: {flexDirection: 'row', alignItems: 'center'},
  rowMain: {flex: 1, flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 14, minWidth: 0},
  dot: {width: 9, height: 9, borderRadius: 4.5, marginRight: 12},
  rowText: {flex: 1, minWidth: 0},
  name: {fontSize: 15.5, fontWeight: '600'},
  url: {fontSize: 12.5, marginTop: 2},
  remove: {paddingHorizontal: 14, paddingVertical: 14},
  removeText: {fontSize: 15, fontWeight: '600'},
  empty: {alignItems: 'center', paddingVertical: 34},
  emptyText: {fontSize: 14, marginTop: 14, textAlign: 'center'},
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
