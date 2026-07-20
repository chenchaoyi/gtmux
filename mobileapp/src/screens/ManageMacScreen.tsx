// ManageMacScreen — the OWNER-only remote management surface for the connected Mac
// (owner-remote-admin, decision B). A paired phone (isGuest=false) manages SHARING
// here exactly like the menu-bar Preferences: the consent switch, per-link See/Type
// scope, minting + copying + revoking links, plus a READ-ONLY paired-device roster.
// Revoking a device and toggling the remote door stay Mac-only (a lost phone must
// not be able to re-key the machine) — so this screen never offers those, and the
// server 403s them anyway. A guest never reaches this screen (Settings hides it).

import React, {useCallback, useEffect, useState} from 'react';
import {ActivityIndicator, Alert, Share, ScrollView, StyleSheet, Switch, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {primary} from '../api/types';
import type {Agent} from '../api/types';
import type {GuestLink, PairedDevice, ShareConfig} from '../api/client';
import {SettingsGroup, SettingsRow} from '../ui/SettingsRow';
import {ContentColumn} from '../ui/ContentColumn';
import {nextLinkScope} from '../state/shareScope';

export function ManageMacScreen({navigation}: any) {
  const {lang, pal, mac} = useApp();
  const {client, agents} = useAgents();
  const zh = lang === 'zh';

  const [cfg, setCfg] = useState<ShareConfig | null>(null);
  const [guests, setGuests] = useState<GuestLink[]>([]);
  const [devices, setDevices] = useState<PairedDevice[]>([]);
  const [busy, setBusy] = useState(false);
  const [expanded, setExpanded] = useState<string>('');

  // The tmux panes a link can be scoped to (native sessions can't be shared).
  const panes: Agent[] = agents.filter(a => a.source === 'tmux' && a.pane_id);

  const load = useCallback(async () => {
    try {
      const [c, d] = await Promise.all([client.shareConfig(), client.devices()]);
      setCfg(c);
      setGuests(d.guests);
      setDevices(d.devices);
    } catch {
      // An auth/network blip leaves the last-known state; the pull-to-retry is a re-open.
    }
  }, [client]);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (fn: () => Promise<boolean>) => {
    if (busy) return;
    setBusy(true);
    try {
      await fn();
    } finally {
      await load();
      setBusy(false);
    }
  };

  const setEnabled = (on: boolean) => run(() => client.setShareEnabled(on));

  const newLink = () => {
    Alert.prompt?.(
      zh ? '新建分享链接' : 'New share link',
      zh ? '给协作者起个名（可留空）' : 'Name this collaborator (optional)',
      [
        {text: zh ? '取消' : 'Cancel', style: 'cancel'},
        {
          text: zh ? '创建' : 'Create',
          onPress: (label?: string) =>
            // Inherit the current global template so a fresh link isn't wide-open;
            // the owner then tailors it per-pane below.
            run(() => client.shareNew(label ?? '', cfg?.view_panes ?? [], cfg?.panes ?? [])),
        },
      ],
      'plain-text',
    );
  };

  const copyLink = async (g: GuestLink) => {
    const token = await client.shareLink(g.id);
    if (!token || !mac) {
      Alert.alert(zh ? '复制链接' : 'Copy link', zh ? '无法获取链接。' : "Couldn't fetch the link.");
      return;
    }
    const url = `${mac.url.replace(/\/+$/, '')}/#g=${token}`;
    Share.share({message: url});
  };

  const revoke = (g: GuestLink) =>
    Alert.alert(
      g.label || (zh ? '分享链接' : 'Share link'),
      zh ? '吊销后此链接立即失效。' : 'Revoking disables this link immediately.',
      [
        {text: zh ? '取消' : 'Cancel', style: 'cancel'},
        {text: zh ? '吊销' : 'Revoke', style: 'destructive', onPress: () => run(() => client.revokeShare(g.id))},
      ],
    );

  // Toggle one pane's See/Type on a link (Type ⊆ See enforced by nextLinkScope),
  // then persist the whole per-link scope via share/set.
  const toggleScope = (g: GuestLink, pane: string, facet: 'see' | 'type', on: boolean) => {
    const {view, input} = nextLinkScope(g.viewPanes, g.inputPanes, pane, facet, on);
    run(() => client.shareSet(g.id, view, input));
  };

  const scopeSummary = (g: GuestLink) => {
    const v = g.viewPanes.length;
    const i = g.inputPanes.length;
    if (v === 0) return zh ? '看不到任何 pane' : 'sees nothing';
    return zh ? `可见 ${v} · 可输入 ${i}` : `sees ${v} · types ${i}`;
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹ </Text>
        </TouchableOpacity>
        <Text style={[styles.title, {color: pal.fg}]}>{zh ? '管理这台 Mac' : 'Manage this Mac'}</Text>
        {busy && <ActivityIndicator style={styles.spinner} color={pal.fg3} />}
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        <ContentColumn>
          {/* CONSENT — the typing master switch. */}
          <SettingsGroup title={zh ? '分享' : 'Sharing'} pal={pal}>
            <SettingsRow
              icon="share"
              label={zh ? '允许协作者向终端输入' : 'Let a collaborator type into the terminal'}
              sub={zh ? '关闭后链接仍可查看（若已授予可见），但无法输入' : 'Off: links can still view (if allowed) but never type'}
              pal={pal}
              toggle={!!cfg?.enabled}
              onToggle={setEnabled}
            />
          </SettingsGroup>

          {/* SHARE LINKS — mint, scope, copy, revoke. */}
          <SettingsGroup title={zh ? '分享链接' : 'Share links'} pal={pal}>
            <SettingsRow
              icon="share"
              label={zh ? '新建链接…' : 'New link…'}
              pal={pal}
              chevron
              divider={guests.length > 0}
              onPress={newLink}
            />
            {guests.length === 0 ? (
              <Text style={[styles.empty, {color: pal.fg3}]}>
                {zh ? '还没有链接。新建一个邀请协作者。' : 'No links yet. Create one to invite a collaborator.'}
              </Text>
            ) : (
              guests.map((g, idx) => (
                <View key={g.id} style={idx < guests.length - 1 ? {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth} : undefined}>
                  <TouchableOpacity style={styles.linkHead} onPress={() => setExpanded(expanded === g.id ? '' : g.id)}>
                    <View style={styles.flex1}>
                      <Text style={[styles.linkLabel, {color: pal.fg}]} numberOfLines={1}>
                        {g.label || (zh ? '分享链接' : 'Share link')}
                      </Text>
                      <Text style={[styles.linkSub, {color: pal.fg3}]} numberOfLines={1}>
                        {scopeSummary(g)}
                      </Text>
                    </View>
                    <Text style={[styles.chev, {color: pal.fg3}]}>{expanded === g.id ? '⌄' : '›'}</Text>
                  </TouchableOpacity>

                  {expanded === g.id && (
                    <View style={styles.editor}>
                      {panes.length === 0 ? (
                        <Text style={[styles.hint, {color: pal.fg3}]}>{zh ? '没有 tmux pane 可分享。' : 'No tmux panes to share.'}</Text>
                      ) : (
                        panes.map(a => {
                          const see = g.viewPanes.includes(a.pane_id);
                          const type = g.inputPanes.includes(a.pane_id);
                          return (
                            <View key={a.pane_id} style={styles.paneRow}>
                              <Text style={[styles.paneName, {color: pal.fg2}]} numberOfLines={1}>
                                {primary(a)}
                              </Text>
                              <View style={styles.facet}>
                                <Text style={[styles.facetLabel, {color: pal.fg3}]}>{zh ? '可见' : 'See'}</Text>
                                <Switch value={see} onValueChange={v => toggleScope(g, a.pane_id, 'see', v)} disabled={busy} />
                              </View>
                              <View style={styles.facet}>
                                <Text style={[styles.facetLabel, {color: pal.fg3}]}>{zh ? '输入' : 'Type'}</Text>
                                <Switch value={type} onValueChange={v => toggleScope(g, a.pane_id, 'type', v)} disabled={busy || !see} />
                              </View>
                            </View>
                          );
                        })
                      )}
                      <View style={styles.linkActions}>
                        <TouchableOpacity onPress={() => copyLink(g)} hitSlop={hit}>
                          <Text style={[styles.actionLink, {color: pal.fg}]}>{zh ? '复制链接' : 'Copy link'}</Text>
                        </TouchableOpacity>
                        <TouchableOpacity onPress={() => revoke(g)} hitSlop={hit}>
                          <Text style={[styles.actionLink, styles.actionDanger]}>{zh ? '吊销' : 'Revoke'}</Text>
                        </TouchableOpacity>
                      </View>
                    </View>
                  )}
                </View>
              ))
            )}
          </SettingsGroup>

          {/* PAIRED DEVICES — read-only. Revoking one is a Mac-only operation (B). */}
          <SettingsGroup title={zh ? '已配对设备' : 'Paired devices'} pal={pal}>
            {devices.length === 0 ? (
              <Text style={[styles.empty, {color: pal.fg3}]}>{zh ? '还没有配对设备。' : 'No paired devices yet.'}</Text>
            ) : (
              devices.map((d, idx) => (
                <SettingsRow
                  key={d.id}
                  icon="server"
                  label={d.name || '—'}
                  sub={d.lastSeen ? (zh ? '最近在线' : 'recently seen') : undefined}
                  pal={pal}
                  divider={idx < devices.length - 1}
                />
              ))
            )}
            <Text style={[styles.note, {color: pal.fg3}]}>
              {zh
                ? '吊销配对设备、开关远程访问都在 Mac 上操作 —— 手机丢了也无法改动这台机器的门禁。'
                : 'Revoking a paired device and toggling remote access are done on the Mac — a lost phone can’t re-key the machine.'}
            </Text>
          </SettingsGroup>
        </ContentColumn>
      </ScrollView>
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  flex1: {flex: 1},
  spinner: {marginLeft: 8},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 10},
  back: {fontSize: 28, fontWeight: '300'},
  title: {fontSize: 20, fontWeight: '700'},
  body: {paddingVertical: 16},
  empty: {fontSize: 13, paddingHorizontal: 14, paddingVertical: 12},
  note: {fontSize: 12, paddingHorizontal: 14, paddingVertical: 10, lineHeight: 17},
  hint: {fontSize: 12, paddingVertical: 6},
  linkHead: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 12},
  linkLabel: {fontSize: 15},
  linkSub: {fontSize: 12, marginTop: 2},
  chev: {fontSize: 18, marginLeft: 8},
  editor: {paddingHorizontal: 14, paddingBottom: 12},
  paneRow: {flexDirection: 'row', alignItems: 'center', paddingVertical: 6},
  paneName: {flex: 1, fontSize: 13},
  facet: {flexDirection: 'row', alignItems: 'center', marginLeft: 10},
  facetLabel: {fontSize: 11, marginRight: 4},
  linkActions: {flexDirection: 'row', justifyContent: 'space-between', marginTop: 10, paddingHorizontal: 4},
  actionLink: {fontSize: 14, fontWeight: '500'},
  actionDanger: {color: '#EF4444'},
});
