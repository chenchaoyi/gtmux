// DetailScreen — a read-only view of one pane's current screen, plus a "focus on
// Mac" action. It polls /api/pane every ~1.5s and refetches on the SSE `agents`
// signal (via the rev-driven AgentsContext refresh that re-renders Radar).
//
// Rendering note: /api/pane is `tmux capture-pane -p` — PLAIN text, no ANSI. The
// app must also work offline over VPN, where a CDN xterm.js can't load and a
// bundled one needs Xcode resource wiring. So we render the text directly in a
// native monospace view (offline, simple, visually identical). When the server
// later switches to `capture-pane -e` (ANSI colors), swap this for the
// react-native-webview + xterm.html path (the dep is already installed).

import React, {useEffect, useRef, useState} from 'react';
import {
  ActivityIndicator,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, primary, secondary} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {StatusBadge} from '../ui/StatusBadge';
import {statusLabel} from '../i18n';
import {StatusColor} from '../ui/theme';

export function DetailScreen({route, navigation}: any) {
  const agent: Agent = route.params.agent;
  const {client} = useAgents();
  const {t, pal, lang} = useApp();
  const [text, setText] = useState('');
  const [loading, setLoading] = useState(true);
  const [focusMsg, setFocusMsg] = useState('');
  const scrollRef = useRef<ScrollView>(null);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const r = await client.pane(agent.pane_id);
        if (alive) {
          setText(r.text || '');
          setLoading(false);
        }
      } catch {
        if (alive) setLoading(false);
      }
    };
    load();
    const id = setInterval(load, 1500);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, agent.pane_id]);

  const doFocus = async () => {
    const ok = await client.focus(agent.pane_id);
    setFocusMsg(ok ? t('focused') : t('focusFailed'));
    setTimeout(() => setFocusMsg(''), 2500);
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      {/* header */}
      <View style={[styles.header, {borderBottomColor: pal.divider}]}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit} style={styles.back}>
          <Text style={[styles.backText, {color: pal.fg2}]}>‹</Text>
        </TouchableOpacity>
        <View style={styles.badgeWrap}>
          <StatusBadge status={agent.status} size={18} />
        </View>
        <View style={styles.headerText}>
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
            {primary(agent)}
          </Text>
          <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
            {agent.agent} · {statusLabel(agent.status, lang)} · {secondary(agent)}
          </Text>
        </View>
      </View>

      {/* pane screen */}
      <ScrollView
        ref={scrollRef}
        style={styles.term}
        contentContainerStyle={styles.termContent}
        onContentSizeChange={() => scrollRef.current?.scrollToEnd({animated: false})}>
        {loading ? (
          <ActivityIndicator color={pal.fg3} style={styles.loading} />
        ) : (
          <Text selectable style={styles.mono}>
            {text}
          </Text>
        )}
      </ScrollView>

      {/* focus action */}
      <View style={[styles.footer, {borderTopColor: pal.divider}]}>
        {!!focusMsg && <Text style={[styles.focusMsg, {color: pal.fg2}]}>{focusMsg}</Text>}
        <TouchableOpacity style={styles.focusBtn} onPress={doFocus}>
          <Text style={styles.focusText}>{t('focusOnMac')}</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingBottom: 10,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  back: {paddingRight: 6},
  backText: {fontSize: 30, fontWeight: '300', lineHeight: 30},
  badgeWrap: {marginHorizontal: 8},
  headerText: {flex: 1, minWidth: 0},
  title: {fontSize: 16, fontWeight: '700'},
  sub: {fontSize: 12, marginTop: 1},
  term: {flex: 1, backgroundColor: '#0A0A0C'},
  termContent: {padding: 12},
  loading: {marginTop: 40},
  mono: {
    color: '#D6D6DA',
    fontFamily: 'Menlo',
    fontSize: 11,
    lineHeight: 15,
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
    padding: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  focusMsg: {fontSize: 12.5, marginRight: 12, flex: 1},
  focusBtn: {backgroundColor: '#06B6D4', borderRadius: 10, paddingHorizontal: 18, paddingVertical: 10},
  focusText: {color: '#fff', fontSize: 14, fontWeight: '700'},
});
