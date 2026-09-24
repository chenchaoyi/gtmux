// RouteScreen — which Direct route this Mac takes, chosen from the owner's phone
// (openspec/changes/phone-moves-the-route).
//
// The paired app is the owner's Mac at a distance: it types into panes and sends work, so
// choosing the route is the same act performed from somewhere else. A GUEST connection
// never reaches this screen (Settings hides it) and the Mac refuses it anyway.
//
// Every round trip here is measured BY THIS PHONE. The Mac's own figure answers a
// different question: the person holding the phone is asking what their connection costs
// from where they are standing.

import React, {useCallback, useEffect, useState} from 'react';
import {ActivityIndicator, Alert, RefreshControl, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ContentColumn} from '../ui/ContentColumn';
import {SettingsGroup} from '../ui/SettingsRow';
import {StatusColor} from '../ui/theme';
import {macName} from './connectionGroup';
import {
  MeasuredRoute,
  measureRoutes,
  orderRoutes,
  pickable,
  roundTripText,
  routeLabel,
} from './routeModel';

export function RouteScreen({navigation}: any) {
  const {pal, lang, mac} = useApp();
  const {client, isGuest} = useAgents();
  const zh = lang === 'zh';
  const [routes, setRoutes] = useState<MeasuredRoute[]>([]);
  const [measuring, setMeasuring] = useState(true);
  const [moving, setMoving] = useState<string | null>(null);
  const [measuredAt, setMeasuredAt] = useState<number | null>(null);

  const load = useCallback(async () => {
    setMeasuring(true);
    const list = await client.routes();
    // Time each one from HERE. A bare fetch of the route's own health is enough: any
    // answer means the server is there, a 404 from an older one included.
    const measured = await measureRoutes(list, async url => {
      const r = await fetch(`${url}/api/health`);
      return !!r;
    });
    setRoutes(orderRoutes(measured));
    setMeasuredAt(Date.now());
    setMeasuring(false);
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  // When these figures were taken. A number with no time on it is not a measurement, and
  // the menu bar says the same thing in the same place (docs/design/DESIGN.md §13).
  const measuredText = measuring
    ? zh
      ? '这台手机正在测…'
      : 'Measuring from this phone…'
    : measuredAt === null
    ? zh
      ? '这台手机测的'
      : 'Measured from this phone'
    : (() => {
        const secs = Math.floor((Date.now() - measuredAt) / 1000);
        if (secs < 10) return zh ? '这台手机测的，刚刚' : 'Measured from this phone, just now';
        if (secs < 60) return zh ? `这台手机测的，${secs} 秒前` : `Measured from this phone, ${secs}s ago`;
        return zh
          ? `这台手机测的，${Math.floor(secs / 60)} 分钟前`
          : `Measured from this phone, ${Math.floor(secs / 60)}m ago`;
      })();

  const move = (r: MeasuredRoute) => {
    Alert.alert(
      zh ? `把 ${macName(mac, zh)} 换到「${routeLabel(r, zh)}」？` : `Move ${macName(mac, zh)} to ${routeLabel(r, zh)}?`,
      zh
        ? `连着 ${macName(mac, zh)} 的设备都会跟着换。其他已配对的设备会断几秒，然后自己恢复；只扫过码、还没连上来过的设备要重新扫一次；换之前发出的分享链接会失效。`
        : `Every device on ${macName(mac, zh)} moves with it. Other paired devices drop for a few seconds and come back on their own; a device that paired but never connected has to scan again; guest links made before the move stop working.`,
      [
        {text: zh ? '取消' : 'Cancel', style: 'cancel'},
        {
          text: zh ? '换过去' : 'Move',
          onPress: async () => {
            setMoving(r.id);
            try {
              await client.moveRoute(r.id);
            } catch {
              setMoving(null);
              Alert.alert(
                zh ? '没能换过去' : 'The move did not go through',
                zh ? '这台 Mac 拒绝了这次切换。稍后再试一次。' : 'The Mac refused it. Try again in a moment.',
              );
              return;
            }
            // The Mac is reconnecting on the new route; this phone finds it there through
            // the addresses it already keeps. Re-read once it has had a moment.
            setTimeout(() => {
              setMoving(null);
              void load();
            }, 6000);
          },
        },
      ],
    );
  };

  return (
    <SafeAreaView style={[s.fill, {backgroundColor: pal.bg}]} edges={['top', 'bottom']}>
      <View style={s.head}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}>
          <Text style={[s.back, {color: pal.fg2}]}>{zh ? '‹ 设置' : '‹ Settings'}</Text>
        </TouchableOpacity>
        <Text style={[s.title, {color: pal.fg}]}>{zh ? '线路' : 'Route'}</Text>
        <View style={s.backSpacer} />
      </View>
      <ScrollView
        contentContainerStyle={s.body}
        refreshControl={<RefreshControl refreshing={measuring} onRefresh={() => void load()} tintColor={pal.fg3} />}>
        <ContentColumn>
          <SettingsGroup
            title={zh ? `${macName(mac, zh)} 能用的线路` : `Routes ${macName(mac, zh)} can use`}
            pal={pal}>
            {routes.map((r, i) => (
              <TouchableOpacity
                key={r.id}
                disabled={!pickable(r) || moving !== null || isGuest}
                onPress={() => move(r)}
                style={[s.row, i > 0 && {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider}]}>
                <View
                  style={[s.dot, {backgroundColor: r.ms === null ? StatusColor.waiting : StatusColor.idle}]}
                />
                <Text style={[s.name, {color: pal.fg}]} numberOfLines={1}>
                  {routeLabel(r, zh)}
                </Text>
                <Text style={[s.ms, {color: pal.fg3}]}>{roundTripText(r, zh, measuring)}</Text>
                {r.current ? (
                  <Text style={[s.mark, {color: pal.fg3}]}>{zh ? '正在使用' : 'in use'}</Text>
                ) : moving === r.id ? (
                  <ActivityIndicator size="small" />
                ) : null}
              </TouchableOpacity>
            ))}
            {routes.length > 0 && (
              <View style={[s.row, {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider}]}>
                <Text style={[s.measured, {color: pal.fg3}]}>{measuredText}</Text>
                <TouchableOpacity onPress={() => void load()} disabled={measuring}>
                  <Text style={[s.again, {color: pal.fg2}]}>{zh ? '重新测' : 'Measure again'}</Text>
                </TouchableOpacity>
              </View>
            )}
            {routes.length === 0 && (
              <Text style={[s.empty, {color: pal.fg3}]}>
                {measuring
                  ? zh
                    ? '正在读这台 Mac 的线路…'
                    : 'Reading this Mac’s routes…'
                  : zh
                  ? '这台 Mac 只有一条线路。'
                  : 'This Mac has one route.'}
              </Text>
            )}
          </SettingsGroup>
          <Text style={[s.note, {color: pal.fg3}]}>
            {zh
              ? `延迟是这台手机测的，不是 ${macName(mac, zh)} 测的。你在哪儿，这个数字就是从哪儿看到的。`
              : `Measured from this phone, not from ${macName(mac, zh)}: it is what your own connection costs.`}
          </Text>
          <Text style={[s.note, {color: pal.fg3}]}>
            {zh
              ? '从这台手机测不到的线路不能选 —— 换过去也一样连不上。'
              : 'A route this phone cannot reach cannot be picked: moving there would not help.'}
          </Text>

        </ContentColumn>
      </ScrollView>
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  fill: {flex: 1},
  head: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 10},
  back: {fontSize: 15, width: 90},
  backSpacer: {width: 90},
  title: {flex: 1, fontSize: 15, fontWeight: '600', textAlign: 'center'},
  body: {paddingBottom: 28, gap: 10},
  row: {flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 13},
  dot: {width: 7, height: 7, borderRadius: 4},
  name: {flex: 1, fontSize: 14},
  ms: {fontSize: 12},
  mark: {fontSize: 11},
  empty: {fontSize: 12, paddingHorizontal: 14, paddingVertical: 13},
  measured: {flex: 1, fontSize: 11},
  again: {fontSize: 12},
  note: {fontSize: 11, lineHeight: 16, paddingHorizontal: 18},
});
