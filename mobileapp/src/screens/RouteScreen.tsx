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

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {ActivityIndicator, Alert, RefreshControl, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ContentColumn} from '../ui/ContentColumn';
import {SettingsGroup} from '../ui/SettingsRow';
import {StatusColor} from '../ui/theme';
import {macName} from './connectionGroup';
import {useRouteChoices} from './useRouteChoices';
import {
  MeasuredRoute,
  pickable,
  roundTripText,
  routeLabel,
} from './routeModel';

export function RouteScreen({navigation}: any) {
  const {pal, lang, mac} = useApp();
  const {client, isGuest, conn} = useAgents();
  const zh = lang === 'zh';
  const {routes, loading, measuring, error, measuredAt, load, markCurrent} = useRouteChoices(client, conn === 'live' && !isGuest);
  const [moving, setMoving] = useState<string | null>(null);
  const alive = useRef(true);
  const moveRequest = useRef(0);
  const moveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const currentRoute = routes.find(r => r.current);
  const cancelMove = useCallback(() => {
    alive.current = false;
    moveRequest.current++;
    if (moveTimer.current !== null) clearTimeout(moveTimer.current);
  }, []);
  useEffect(() => {
    alive.current = true;
    setMoving(null);
    return cancelMove;
  }, [client, cancelMove]);
  const busy = loading || measuring;

  // When these figures were taken. A number with no time on it is not a measurement, and
  // the menu bar says the same thing in the same place (docs/design/DESIGN.md §13).
  const measuredText = busy
    ? (zh ? '正在测量延迟' : 'Measuring latency…')
    : measuredAt === null ? (zh ? '尚未测量' : 'Not measured')
    : `${zh ? '测量于' : 'Measured at'} ${new Date(measuredAt).toLocaleTimeString(zh ? 'zh-CN' : 'en', {hour: '2-digit', minute: '2-digit'})}`;

  const move = (r: MeasuredRoute) => {
    const confirmationVersion = moveRequest.current;
    Alert.alert(
      zh ? `切换至${routeLabel(r, zh)}？` : `Switch to ${routeLabel(r, zh)}?`,
      zh
        ? `${macName(mac, zh)} 的所有配对设备将短暂断开并自动重连。尚未完成连接的设备需重新扫码。现有分享链接将失效。`
        : `Paired devices on ${macName(mac, zh)} will briefly disconnect and reconnect automatically. Devices that have not connected yet must scan again. Existing share links will stop working.`,
      [
        {text: zh ? '取消' : 'Cancel', style: 'cancel'},
        {
          text: zh ? '切换' : 'Switch',
          onPress: async () => {
            if (!alive.current || confirmationVersion !== moveRequest.current) return;
            const ticket = ++moveRequest.current;
            setMoving(r.id);
            try {
              await client.moveRoute(r.id);
            } catch {
              if (!alive.current || ticket !== moveRequest.current) return;
              setMoving(null);
              Alert.alert(
                zh ? '线路切换失败' : 'Could not switch routes',
                zh ? '请求未完成，请检查连接后重试。' : 'The request did not complete. Check the connection and try again.',
              );
              return;
            }
            if (!alive.current || ticket !== moveRequest.current) return;
            // Discard a measurement started before the move. Its old `current` marker
            // otherwise redraws the previous route while the Mac is reconnecting.
            markCurrent(r.id);
            // The Mac is reconnecting on the new route; this phone finds it there through
            // the addresses it already keeps. Re-read once it has had a moment.
            moveTimer.current = setTimeout(() => {
              moveTimer.current = null;
              if (!alive.current || ticket !== moveRequest.current) return;
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
      <ContentColumn>
        <View style={s.head}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}>
          <Text style={[s.back, {color: pal.fg2}]}>{zh ? '‹ 设置' : '‹ Settings'}</Text>
        </TouchableOpacity>
        <Text style={[s.title, {color: pal.fg}]}>{zh ? '线路' : 'Route'}</Text>
        <View style={s.backSpacer} />
        </View>
      </ContentColumn>
      <ScrollView
        contentContainerStyle={s.body}
        refreshControl={<RefreshControl refreshing={busy} onRefresh={() => { if (!moving) void load(); }} tintColor={pal.fg3} />}>
        <ContentColumn>
          <SettingsGroup
            title={macName(mac, zh)}
            pal={pal}>
            {routes.length > 0 && (
              <Text style={[s.current, {color: pal.fg2}]}>
                {currentRoute
                  ? `${zh ? '当前线路：' : 'Current route: '}${routeLabel(currentRoute, zh)}`
                  : zh ? '正在确认当前线路' : 'Checking the current route'}
              </Text>
            )}
            {routes.map((r, i) => (
              <TouchableOpacity
                key={r.id}
                disabled={!pickable(r) || moving !== null || isGuest || conn !== 'live' || error}
                accessibilityRole="button"
                accessibilityState={{selected: r.current, disabled: !pickable(r) || moving !== null || isGuest || conn !== 'live' || error}}
                accessibilityLabel={`${routeLabel(r, zh)}，${r.current ? (zh ? '正在使用' : 'in use') : roundTripText(r, zh, busy)}`}
                onPress={() => move(r)}
                style={[s.row, i > 0 && {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider}]}>
                <View
                  style={[s.dot, {backgroundColor: r.ms === null ? (busy ? pal.fg3 : StatusColor.waiting) : StatusColor.idle}]}
                />
                <Text style={[s.name, {color: pal.fg}]} numberOfLines={1}>
                  {routeLabel(r, zh)}
                </Text>
                <Text style={[s.ms, {color: pal.fg3}]}>{roundTripText(r, zh, busy)}</Text>
                {r.current ? (
                  <Text style={[s.mark, {color: pal.fg2}]}>{zh ? '✓ 正在使用' : '✓ In use'}</Text>
                ) : moving === r.id ? (
                  <ActivityIndicator size="small" />
                ) : null}
              </TouchableOpacity>
            ))}
            {routes.length > 0 && (
              <View style={[s.row, {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider}]}>
                <Text style={[s.measured, {color: pal.fg3}]}>{measuredText}</Text>
                <TouchableOpacity onPress={() => void load()} disabled={busy || moving !== null || conn !== 'live'}>
                  <Text style={[s.again, {color: pal.fg2}]}>{zh ? '重新测量' : 'Measure again'}</Text>
                </TouchableOpacity>
              </View>
            )}
            {(routes.length === 0 || error || conn !== 'live') && (
              <View style={s.empty}>
                <Text style={{color: pal.fg2}}>
                  {conn !== 'live' ? (zh ? '连接 Mac 后可查看和切换线路。' : 'Connect to the Mac to view and change routes.') :
                    error ? (zh ? '无法加载线路，请重试。' : 'Could not load routes. Please retry.') :
                    loading ? (zh ? '正在加载线路…' : 'Loading routes…') :
                    (zh ? 'Mac 未提供可切换的直连线路。请检查 Mac 的直连设置。' : 'No Direct routes available. Check Direct settings on the Mac.')}
                </Text>
                {conn === 'live' && !busy && !moving && (
                  <TouchableOpacity accessibilityRole="button" onPress={() => void load()} style={s.retry}>
                    <Text style={{color: pal.fg}}>{zh ? '重新加载' : 'Reload routes'}</Text>
                  </TouchableOpacity>
                )}
              </View>
            )}
          </SettingsGroup>
          <Text style={[s.note, {color: pal.fg3}]}>
            {zh ? '延迟由当前设备测量。仅可切换至设备能访问的线路。' : 'Latency is measured from this device. Only reachable routes can be selected.'}
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
  current: {fontSize: 12, fontWeight: '600', paddingHorizontal: 14, paddingVertical: 10},
  dot: {width: 7, height: 7, borderRadius: 4},
  name: {flex: 1, fontSize: 14},
  ms: {fontSize: 12},
  mark: {fontSize: 11},
  empty: {paddingHorizontal: 14, paddingVertical: 16, gap: 12},
  retry: {alignSelf: 'flex-start', minHeight: 44, justifyContent: 'center'},
  measured: {flex: 1, fontSize: 11},
  again: {fontSize: 12},
  note: {fontSize: 11, lineHeight: 16, paddingHorizontal: 18},
});
