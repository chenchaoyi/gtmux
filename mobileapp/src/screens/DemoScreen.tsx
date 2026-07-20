// DemoScreen — a fully-clickable, no-server tour: the REAL radar + DetailView
// rendered over a fake client (demoClient) through DemoAgentsProvider. Reached only
// via "See a demo" on the pairing screen. Tap any row to open its real detail
// (terminal + chat + composer, all canned); a DEMO chip follows you into detail so
// sample output is never mistaken for a live Mac; a sticky "Pair your Mac" stays on
// screen. Everything resets on exit. Doubles as the App Review path — Apple sanctions
// a fully-featured demo mode with demonstration data in lieu of a demo account.
//
// F7: the fake world is now ALIVE enough to show the core loop — approving the
// hero's permission walks it waiting→working→idle(+latest) on this very radar
// (demoClient pushes fresh agents through setAgents), and the chief-of-staff card
// (ui/HQCard) opens the real HQScreen over a canned digest + one preset exchange.

import React, {useMemo, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, SectionKey} from '../api/types';
import {useApp} from '../state/AppContext';
import {DemoAgentsProvider} from '../state/AgentsContext';
import {HQCard} from '../ui/HQCard';
import {SectionList} from '../ui/SectionList';
import {StatusColor} from '../ui/theme';
import {sampleAgents} from '../ui/demoData';
import {makeDemoClient} from '../ui/demoClient';
import {DetailView} from './DetailScreen';
import {HQScreen} from './HQScreen';

export function DemoScreen({onExit, onPair}: {onExit: () => void; onPair: () => void}) {
  const {pal, lang} = useApp();
  // The scripted world lives in state so demoClient's status arc re-renders the
  // real radar; a fresh client per Demo session resets everything on reopen.
  const [agents, setAgents] = useState<Agent[]>(() => sampleAgents());
  const client = useMemo(() => makeDemoClient(lang === 'zh' ? 'zh' : 'en', setAgents), [lang]);
  const [selected, setSelected] = useState<Agent | null>(null);
  const [showHQ, setShowHQ] = useState(false);
  const [collapsed, setCollapsed] = useState<Set<SectionKey>>(new Set());
  const onToggle = (s: SectionKey) =>
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(s) ? next.delete(s) : next.add(s);
      return next;
    });

  const hq = agents.find(a => a.role === 'supervisor');

  const Header = (
    <View>
      <View style={styles.banner}>
        <View style={[styles.pill, {borderColor: StatusColor.working}]}>
          <Text style={[styles.pillText, {color: StatusColor.working}]}>DEMO</Text>
        </View>
        <Text style={[styles.bannerText, {color: pal.fg2}]}>
          {lang === 'zh' ? '样例数据 —— 不是真的 Mac。点任意一行进去看看。' : 'Sample data — not a real Mac. Tap any row to explore.'}
        </Text>
      </View>
      {hq && (
        <View style={styles.hqWrap}>
          <HQCard hq={hq} agents={agents} pal={pal} lang={lang} onPress={() => setShowHQ(true)} />
        </View>
      )}
    </View>
  );

  // A minimal navigation shim for the inline HQScreen: back closes it; a fleet-row
  // long-press "jump to Detail" swaps to that worker's demo detail.
  const hqNav = {
    goBack: () => setShowHQ(false),
    navigate: (_screen: string, params?: any) => {
      setShowHQ(false);
      if (params?.agent) setSelected(params.agent);
    },
  };

  return (
    <DemoAgentsProvider client={client} agents={agents}>
      {showHQ && hq ? (
        // The REAL HQ command center over the canned digest (DEMO chip via context).
        <HQScreen route={{params: {agent: hq}}} navigation={hqNav} />
      ) : selected ? (
        // The REAL detail screen, over the fake client. Back returns to the radar.
        <DetailView agent={selected} onBack={() => setSelected(null)} />
      ) : (
        <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
          <View style={styles.topbar}>
            <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '演示' : 'Demo'}</Text>
            <TouchableOpacity
              onPress={onExit}
              hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}
              accessibilityRole="button"
              accessibilityLabel={lang === 'zh' ? '关闭演示' : 'Close demo'}>
              <Text style={[styles.close, {color: pal.fg3}]}>✕</Text>
            </TouchableOpacity>
          </View>

          <SectionList
            agents={agents}
            pal={pal}
            lang={lang}
            onPressAgent={setSelected}
            refreshing={false}
            onRefresh={() => {}}
            collapsed={collapsed}
            onToggle={onToggle}
            ListHeaderComponent={Header}
          />

          <TouchableOpacity
            style={[styles.cta, {backgroundColor: StatusColor.working}]}
            onPress={onPair}
            accessibilityRole="button"
            accessibilityLabel={lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}>
            <Text style={styles.ctaText}>{lang === 'zh' ? '配对你的 Mac' : 'Pair your Mac'}</Text>
          </TouchableOpacity>
        </SafeAreaView>
      )}
    </DemoAgentsProvider>
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
  hqWrap: {paddingHorizontal: 14, paddingBottom: 4, marginTop: -6},
  cta: {margin: 16, borderRadius: 12, paddingVertical: 14, alignItems: 'center'},
  ctaText: {color: '#04141a', fontSize: 15, fontWeight: '700'},
});
