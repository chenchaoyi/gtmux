// Adaptive workspace: the same selected reader stays mounted as the radar changes
// between a sidebar and the compact list. Selecting a different item intentionally
// replaces the reader; resizing does not.

import React, {useContext, useEffect, useRef, useState} from 'react';
import {NavigationContext} from '@react-navigation/native';
import {BackHandler, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {useWindowDimensions} from '../ui/windowMetrics';
import {SafeAreaView, useSafeAreaInsets} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {agentId, paneRowToAgent, sameAgent} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {initialSelection, useWorkspace} from '../state/WorkspaceContext';
import {BrandMark} from '../ui/BrandMark';
import {SidebarIcon} from '../ui/Icons';
import {hqInspectorFits, sidebarWidth, useSizeClass} from '../ui/layout';
import {TestIds} from '../constants/testIds';
import {KeyBus} from '../keys/bus';
import {DetailView} from './DetailScreen';
import {HQView} from './HQScreen';
import {PaneBrowserView} from './PaneBrowserScreen';
import {DemoChrome, RadarPanel} from './RadarPanel';

const SIDEBAR_KEY = 'workspace.sidebarHidden';

export function SplitShell({demo}: {demo?: DemoChrome} = {}) {
  const navigation = useContext(NavigationContext) as {push: (route: string, params: object) => void; isFocused: () => boolean} | undefined;
  const {agents} = useAgents();
  const {t, pal, lang} = useApp();
  const {selection, selectionRevision, initialize, select} = useWorkspace();
  const regular = useSizeClass() === 'regular';
  const [showList, setShowList] = useState(!regular);
  const wasRegular = useRef(regular);
  const compactHadSelection = useRef(!regular && !!selection);
  useEffect(() => {
    // Preserve whether the user was on the list or reader when they expanded.
    // A compact list that had no selection cannot preserve a reader choice.
    if (wasRegular.current && !regular && !compactHadSelection.current) setShowList(!selection);
    if (!regular) compactHadSelection.current = !!selection;
    wasRegular.current = regular;
  }, [regular, selection]);
  useEffect(() => { if (selectionRevision) setShowList(false); }, [selectionRevision]);
  useEffect(() => { if (!regular && !selection) setShowList(true); }, [regular, selection]);
  useEffect(() => {
    if (regular || showList || !selection) return;
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      if (navigation && !navigation.isFocused()) return false;
      setShowList(true);
      return true;
    });
    return () => sub.remove();
  }, [regular, showList, selection, navigation]);
  const {width} = useWindowDimensions();
  const insets = useSafeAreaInsets();
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
    if (regular && !selection) {
      const first = initialSelection(agents);
      if (first) initialize(first);
    }
  }, [regular, selection, agents, initialize]);

  // The selected pane, live: `selection.agent` is the snapshot the row was tapped with,
  // and the main pane's key follows the identity so a switch remounts the detail.
  const selectedPane =
    selection?.kind === 'pane' ? agents.find(a => sameAgent(a, selection.agent)) ?? selection.agent : undefined;
  const sidebarSelectedId =
    selection?.kind === 'pane' && selectedPane ? agentId(selectedPane) : undefined;

  let main: React.ReactNode;
  if (selection?.kind === 'pane' && selectedPane) {
    main = (
      <DetailView
        key={agentId(selectedPane)}
        agent={selectedPane}
        initialMode={selection.mode}
        openDiffRequest={selection.openDiff ? selectionRevision : undefined}
        onBack={regular ? undefined : () => setShowList(true)}
        onOpenTaskPane={agent => {
          if (!regular && navigation) navigation.push('Detail', {agent});
          else select({kind: 'pane', agent});
        }}
        onOpenPane={row => {
          const agent = paneRowToAgent(row);
          if (!regular && navigation) navigation.push('Detail', {agent});
          else select({kind: 'pane', agent});
        }}
      />
    );
  } else if (selection?.kind === 'hq') {
    main = <HQView key="hq" agent={selection.agent} prefill={selection.prefill} openResource={selection.openResource} onBack={regular ? undefined : () => setShowList(true)} layout={regular ? "regular" : "compact"} showInspector={hqInspectorFits(width, hidden)} />;
  } else if (selection?.kind === 'panes') {
    main = <PaneBrowserView key="panes" onBack={regular ? undefined : () => setShowList(true)} layout={regular ? "regular" : "compact"} />;
  } else {
    main = (
      <View style={styles.mainEmpty}>
        <BrandMark size={56} neutral={pal.fg3} />
        <Text style={[styles.mainEmptyText, {color: pal.fg3}]}>{t('noAgents')}</Text>
        <Text style={[styles.mainEmptyHint, {color: pal.fg3}]}>
          {lang === 'zh' ? '在服务器上启动一个 coding agent 就会出现在这里' : 'Start a coding agent on your server and it shows up here'}
        </Text>
      </View>
    );
  }

  return (
    <View style={[styles.safe, {backgroundColor: pal.bg}]} testID={TestIds.radar.split}>
      <View style={styles.row}>
        <SafeAreaView testID="workspace-list" edges={regular ? ['top', 'left'] : []} style={[regular ? [styles.sidebar, {width: sidebarWidth(width), borderRightColor: pal.divLoud}] : styles.main, (regular ? hidden : !showList) && styles.hidden]}>
          <RadarPanel variant={regular ? "sidebar" : "screen"} selectedId={sidebarSelectedId} onHideSidebar={() => setSidebar(true)} demo={demo} />
        </SafeAreaView>
        <View testID="workspace-reader" style={[styles.main, !regular && showList && styles.hidden]}>
          {main}
          {regular && hidden && (
            <TouchableOpacity
              testID={TestIds.radar.showSidebar}
              accessibilityLabel={lang === 'zh' ? '显示侧栏' : 'Show sidebar'}
              onPress={() => setSidebar(false)}
              style={[styles.showSidebar, {left: 10 + insets.left, backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <SidebarIcon size={18} color={pal.fg2} />
            </TouchableOpacity>
          )}
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: {flex: 1},
  row: {flex: 1, flexDirection: 'row'},
  sidebar: {borderRightWidth: StyleSheet.hairlineWidth},
  main: {flex: 1},
  hidden: {display: 'none'},
  showSidebar: {
    position: 'absolute', left: 10, top: 10, width: 36, height: 36, borderRadius: 10,
    borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', zIndex: 20,
  },
  mainEmpty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 40},
  mainEmptyText: {fontSize: 15, fontWeight: '600', marginTop: 14},
  mainEmptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
});
