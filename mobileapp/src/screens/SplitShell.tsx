// SplitShell — the regular shell (MOBILE §5, change ipad-universal-app): the radar as a
// sidebar beside a main pane that shows whatever is selected — a pane's detail, the HQ
// page or the All-panes browser. It composes the phone's screens and adds nothing of its
// own but the frame: the sidebar IS `RadarPanel`, the main pane IS `DetailView` /
// `HQView` / `PaneBrowserView`. `SplitScreen` (2026-07) was a copy of the radar's chrome
// instead, and drifted from the phone in every radar change since; this file must never
// grow a list, a banner or a header of its own — `shellDrift.test.ts` reads it.
//
// The sidebar can be hidden (the header's button, persisted): a Stage Manager window
// between 768 and 1000 wide, or a reader who wants the whole width for a terminal. A
// small floating button over the main pane brings it back.

import React, {useEffect, useState} from 'react';
import {StyleSheet, TouchableOpacity, View, useWindowDimensions} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {NavigationContext} from '@react-navigation/native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {agentId, paneRowToAgent} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {initialSelection, useWorkspace} from '../state/WorkspaceContext';
import {SidebarIcon} from '../ui/Icons';
import {hqInspectorFits, sidebarWidth} from '../ui/layout';
import {TestIds} from '../constants/testIds';
import {KeyBus} from '../keys/bus';
import {DetailView} from './DetailScreen';
import {HQView} from './HQScreen';
import {PaneBrowserView} from './PaneBrowserScreen';
import {DemoChrome, RadarPanel} from './RadarPanel';
import {RadarReadState} from './RadarReadState';

const SIDEBAR_KEY = 'workspace.sidebarHidden';

export function SplitShell({demo}: {demo?: DemoChrome} = {}) {
  const {agents, read, conn, isGuest, shareStale, refresh} = useAgents();
  const {pal, lang} = useApp();
  const navigation = React.useContext(NavigationContext) as {navigate: (route: string) => void} | undefined;
  const {selection, select} = useWorkspace();
  const {width} = useWindowDimensions();
  const [hidden, setHidden] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(SIDEBAR_KEY).then(raw => raw === '1' && setHidden(true));
  }, []);
  const setSidebar = (h: boolean) => {
    setHidden(h);
    AsyncStorage.setItem(SIDEBAR_KEY, h ? '1' : '0');
  };
  useEffect(() => KeyBus.on('sidebar.toggle', () => setSidebar(!hidden)), [hidden]);

  // Nothing selected yet: prefer the first ordinary session. The roster can lead with
  // HQ, but HQ has a dedicated page; treating it as a generic pane makes the same HQ
  // look like two unrelated places. Once only — roster changes must not yank the reader.
  useEffect(() => {
    if (!selection) {
      const first = initialSelection(agents);
      if (first) select(first);
    }
  }, [selection, agents, select]);

  // The selected pane, live: `selection.agent` is the snapshot the row was tapped with,
  // and the main pane's key follows the identity so a switch remounts the detail.
  const selectedPane =
    selection?.kind === 'pane' ? agents.find(a => a.pane_id === selection.agent.pane_id) ?? selection.agent : undefined;
  const sidebarSelectedId =
    selection?.kind === 'pane' && selectedPane ? agentId(selectedPane) : undefined;

  let main: React.ReactNode;
  if (selection?.kind === 'pane' && selectedPane) {
    main = (
      <DetailView
        key={agentId(selectedPane)}
        agent={selectedPane}
        initialMode={selection.mode}
        onOpenPane={row => select({kind: 'pane', agent: paneRowToAgent(row)})}
      />
    );
  } else if (selection?.kind === 'hq') {
    main = <HQView key="hq" agent={selection.agent} prefill={selection.prefill} openResource={selection.openResource} layout="regular" showInspector={hqInspectorFits(width, hidden)} />;
  } else if (selection?.kind === 'panes') {
    main = <PaneBrowserView key="panes" layout="regular" />;
  } else {
    main = (
      <RadarReadState read={read ?? {hasValue: false, refreshing: false, error: null}} conn={conn ?? 'connecting'}
        isGuest={isGuest} staleShare={shareStale} filtered={false} lang={lang} pal={pal} demo={!!demo}
        hasSessions={agents.length > 0} showQuote={false}
        onRetry={() => {void refresh();}} onConnections={navigation ? () => navigation.navigate('Servers') : undefined}
        onShowAll={() => {}} />
    );
  }

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.radar.split}>
      <View style={styles.row}>
        {!hidden && (
          <View style={[styles.sidebar, {width: sidebarWidth(width), borderRightColor: pal.divLoud}]}>
            <RadarPanel variant="sidebar" selectedId={sidebarSelectedId} onHideSidebar={() => setSidebar(true)} demo={demo} />
          </View>
        )}
        <View style={styles.main}>
          {main}
          {hidden && (
            <TouchableOpacity
              testID={TestIds.radar.showSidebar}
              accessibilityLabel={lang === 'zh' ? '显示侧栏' : 'Show sidebar'}
              onPress={() => setSidebar(false)}
              style={[styles.showSidebar, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <SidebarIcon size={18} color={pal.fg2} />
            </TouchableOpacity>
          )}
        </View>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {flex: 1},
  row: {flex: 1, flexDirection: 'row'},
  sidebar: {borderRightWidth: StyleSheet.hairlineWidth},
  main: {flex: 1},
  showSidebar: {
    position: 'absolute', left: 10, top: 10, width: 36, height: 36, borderRadius: 10,
    borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', zIndex: 20,
  },
});
