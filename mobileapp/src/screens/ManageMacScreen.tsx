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
import type {ServerMode} from '../api/types';
import {serverModeNeedsAttention} from '../api/types';
import {displayDeviceName} from '../pairing/deviceName';
import {SettingsGroup, SettingsRow} from '../ui/SettingsRow';
import {SIcon, IconName} from '../ui/SettingsIcons';
import {ContentColumn} from '../ui/ContentColumn';
import {nextLinkScope} from '../state/shareScope';

// The paired-device subtitle: platform (e.g. "iOS 17.5", "Safari · macOS") joined
// with a relative last-seen ("2m ago"). Either half may be missing on an older Mac
// serve (no platform capture) or a device that never checked in.
function relSeen(unixSec: number, zh: boolean): string {
  const s = Math.max(0, Math.floor(Date.now() / 1000) - unixSec);
  if (s < 60) return zh ? '刚刚在线' : 'just now';
  if (s < 3600) return zh ? `${Math.floor(s / 60)} 分钟前` : `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return zh ? `${Math.floor(s / 3600)} 小时前` : `${Math.floor(s / 3600)}h ago`;
  return zh ? `${Math.floor(s / 86400)} 天前` : `${Math.floor(s / 86400)}d ago`;
}

function deviceSub(platform: string | undefined, lastSeen: number | undefined, zh: boolean): string | undefined {
  const parts: string[] = [];
  if (platform) parts.push(platform);
  if (lastSeen) parts.push(relSeen(lastSeen, zh));
  return parts.length ? parts.join(' · ') : undefined;
}

// deviceIcon picks a leading glyph matching what the device IS, so the roster reads at a
// glance: a phone, a browser, or a Mac/other. Prefers the platform tag ("iOS 17.5",
// "Safari · macOS"); falls back to the name for a device that hasn't reported one yet.
function deviceIcon(name: string, platform: string | undefined): IconName {
  const p = (platform ?? '').toLowerCase();
  const n = (name ?? '').toLowerCase();
  if (/^(ios|ipados|android)/.test(p) || /iphone|ipad|android/.test(n)) return 'phone';
  if (p.includes(' · ') || /safari|chrome|firefox|edge/.test(p) || n === 'browser') return 'globe';
  return 'server';
}

export function ManageMacScreen({navigation}: any) {
  const {lang, pal, mac} = useApp();
  const {client, agents} = useAgents();
  const zh = lang === 'zh';

  const [cfg, setCfg] = useState<ShareConfig | null>(null);
  const [guests, setGuests] = useState<GuestLink[]>([]);
  const [devices, setDevices] = useState<PairedDevice[]>([]);
  const [srv, setSrv] = useState<ServerMode | null>(null);
  const [busy, setBusy] = useState(false);
  const [expanded, setExpanded] = useState<string>('');

  // The tmux panes a link can be scoped to (native sessions can't be shared).
  const panes: Agent[] = agents.filter(a => a.source === 'tmux' && a.pane_id);

  const load = useCallback(async () => {
    try {
      const [c, d, sm] = await Promise.all([
        client.shareConfig(),
        client.devices(),
        client.serverMode(),
      ]);
      setCfg(c);
      setGuests(d.guests);
      setDevices(d.devices);
      setSrv(sm);
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
        <Text style={[styles.title, {color: pal.fg}]}>{zh ? '分享与设备' : 'Sharing & devices'}</Text>
        {busy && <ActivityIndicator style={styles.spinner} color={pal.fg3} />}
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        <ContentColumn>
          {/* Server mode: SHOWN, never controlled. Enabling needs an administrator
              password typed at the Mac, so a remote switch that can only turn it off
              would send you back to the laptop anyway. The radar carries the ambient
              signal (a ring on the connection dot); this states it in words for
              someone who came here to check on the machine. */}
          {srv && (srv.system_disablesleep || srv.state === 'lapsed') && (
            <SettingsGroup title={zh ? '服务器模式' : 'Server mode'} pal={pal}>
              <SettingsRow
                icon="server"
                label={
                  srv.state === 'lapsed'
                    ? zh ? '已失效 —— 合盖会休眠' : 'Lapsed — the lid will sleep it'
                    : zh ? '开启中 —— 合盖不会休眠' : 'On — the lid may stay closed'
                }
                sub={serverModeSub(srv, zh)}
                pal={pal}
                danger={serverModeNeedsAttention(srv)}
              />
            </SettingsGroup>
          )}


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
                    <View style={styles.iconWrap}>
                      <SIcon name="person" size={21} color={pal.fg2} />
                    </View>
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
                  icon={deviceIcon(d.name, d.platform)}
                  label={displayDeviceName(d.name)}
                  sub={deviceSub(d.platform, d.lastSeen, zh)}
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
  linkHead: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 13, minHeight: 52},
  iconWrap: {width: 30, alignItems: 'center', marginRight: 8},
  linkLabel: {fontSize: 16},
  linkSub: {fontSize: 12.5, marginTop: 2},
  chev: {fontSize: 20, fontWeight: '300', marginLeft: 8},
  editor: {paddingLeft: 52, paddingRight: 14, paddingBottom: 12},
  paneRow: {flexDirection: 'row', alignItems: 'center', paddingVertical: 6},
  paneName: {flex: 1, fontSize: 13},
  facet: {flexDirection: 'row', alignItems: 'center', marginLeft: 10},
  facetLabel: {fontSize: 11, marginRight: 4},
  linkActions: {flexDirection: 'row', justifyContent: 'space-between', marginTop: 10, paddingHorizontal: 4},
  actionLink: {fontSize: 14, fontWeight: '500'},
  actionDanger: {color: '#EF4444'},
});

// serverModeSub carries what someone away from their Mac actually needs: how long it
// has been on, and — on battery — how much is left before sleep returns by itself.
function serverModeSub(m: ServerMode, zh: boolean): string {
  const parts: string[] = [];
  if (m.since) {
    const mins = Math.max(0, Math.floor(Date.now() / 1000) - m.since) / 60;
    const dur = mins < 60 ? `${Math.floor(mins)}m` : `${Math.floor(mins / 60)}h${Math.floor(mins % 60)}m`;
    parts.push(zh ? `已开启 ${dur}` : `on for ${dur}`);
  }
  if (m.power === 'battery' && m.battery_pct != null) {
    parts.push(zh ? `电池 ${m.battery_pct}% · 到 20% 自动恢复睡眠`
                  : `battery ${m.battery_pct}% · sleep returns at 20%`);
  } else if (m.power === 'ac') {
    parts.push(zh ? '接通电源' : 'on power');
  }
  if (m.state === 'lapsed') parts.push(zh ? '设置已不再生效' : 'the setting is no longer in force');
  else if (!m.guard.healthy) parts.push(zh ? '⚠ 恢复睡眠的守护缺失' : '⚠ the safety guard is missing');
  return parts.join(' · ');
}
