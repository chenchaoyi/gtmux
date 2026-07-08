// DemoScreen — a read-only tour of the radar with SAMPLE data, so someone without a
// Mac running `gtmux serve` (an App Store reviewer, a curious first-run user) can see
// what gtmux does. Reached ONLY via an explicit "See a demo" tap on the pairing screen
// and never by a paired user. It reuses the real SectionList, so the status language,
// ordering, dark mode, and bilingual copy are all the genuine article — only the data
// is canned, and every row makes that clear (a tap explains it's a demo).

import React, {useMemo, useState} from 'react';
import {Alert, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {SectionKey} from '../api/types';
import {useApp} from '../state/AppContext';
import {SectionList} from '../ui/SectionList';
import {StatusColor} from '../ui/theme';
import {sampleAgents} from '../ui/demoData';

export function DemoScreen({onExit, onPair}: {onExit: () => void; onPair: () => void}) {
  const {pal, lang} = useApp();
  const agents = useMemo(() => sampleAgents(), []);
  const [collapsed, setCollapsed] = useState<Set<SectionKey>>(new Set());
  const onToggle = (s: SectionKey) =>
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(s) ? next.delete(s) : next.add(s);
      return next;
    });

  const explain = () =>
    Alert.alert(
      lang === 'zh' ? '这是 demo' : 'This is a demo',
      lang === 'zh'
        ? '配对一台跑着 `gtmux serve` 的 Mac，就能打开实时窗格、回复、收到推送。'
        : 'Pair a Mac running `gtmux serve` to open live panes, reply, and get push alerts.',
    );

  const Header = (
    <View style={styles.banner}>
      <View style={[styles.pill, {borderColor: StatusColor.working}]}>
        <Text style={[styles.pillText, {color: StatusColor.working}]}>DEMO</Text>
      </View>
      <Text style={[styles.bannerText, {color: pal.fg2}]}>
        {lang === 'zh'
          ? '样例数据 —— 不是真的 Mac。配对后即为实时。'
          : 'Sample data — not a real Mac. Pair one for the live thing.'}
      </Text>
    </View>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.topbar}>
        <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '演示' : 'Demo'}</Text>
        <TouchableOpacity onPress={onExit} hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}
          accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '关闭演示' : 'Close demo'}>
          <Text style={[styles.close, {color: pal.fg3}]}>✕</Text>
        </TouchableOpacity>
      </View>

      <SectionList
        agents={agents}
        waitingOnly={false}
        pal={pal}
        lang={lang}
        onPressAgent={explain}
        refreshing={false}
        onRefresh={() => {}}
        collapsed={collapsed}
        onToggle={onToggle}
        ListHeaderComponent={Header}
      />

      <TouchableOpacity style={[styles.cta, {backgroundColor: StatusColor.working}]} onPress={onPair}
        accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}>
        <Text style={styles.ctaText}>{lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}</Text>
      </TouchableOpacity>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {flex: 1},
  topbar: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 16, paddingVertical: 12},
  title: {fontSize: 22, fontWeight: '700'},
  close: {fontSize: 20, fontWeight: '400'},
  banner: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingBottom: 10, gap: 8},
  pill: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 6, paddingVertical: 1},
  pillText: {fontSize: 10, fontWeight: '700', letterSpacing: 0.06},
  bannerText: {flex: 1, fontSize: 12},
  cta: {margin: 16, borderRadius: 12, paddingVertical: 14, alignItems: 'center'},
  ctaText: {color: '#04141a', fontSize: 15, fontWeight: '700'},
});
