// DetailScreen — a read-only view of one pane's current screen (MOBILE §4), in
// COLOR. It polls /api/pane (now `tmux capture-pane -e`) every ~1.5s and renders
// the ANSI output with a native SGR parser into colored <Text> spans — offline
// over VPN, no webview/xterm needed. Narrow-screen controls: A−/A+ font size, a
// wrap↔scroll toggle, and a jump-to-bottom FAB. "Focus on Mac" lives in the top
// bar (POST /api/focus), not the input area.

import React, {useEffect, useMemo, useRef, useState} from 'react';
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
import {AnsiLine, parseAnsi} from '../ui/ansi';
import {Composer} from '../ui/Composer';
import {StatusColor} from '../ui/theme';

const FONT_SIZES = [9, 11, 13];

// DetailScreen is the stack route (compact); it wraps the presentational
// DetailView, which the iPad split-view also renders directly in its main pane.
export function DetailScreen({route, navigation}: any) {
  return <DetailView agent={route.params.agent} onBack={() => navigation.goBack()} />;
}

export function DetailView({agent, onBack}: {agent: Agent; onBack?: () => void}) {
  const {client} = useAgents();
  const {t, pal, lang} = useApp();
  const [text, setText] = useState('');
  const [loading, setLoading] = useState(true);
  const [focusMsg, setFocusMsg] = useState('');
  const [fontIdx, setFontIdx] = useState(1);
  const [wrap, setWrap] = useState(true);
  const [atBottom, setAtBottom] = useState(true);
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

  const lines: AnsiLine[] = useMemo(() => parseAnsi(text), [text]);
  const fontSize = FONT_SIZES[fontIdx];
  const lineHeight = Math.round(fontSize * 1.36);

  const doFocus = async () => {
    const ok = await client.focus(agent.pane_id);
    setFocusMsg(ok ? t('focused') : t('focusFailed'));
    setTimeout(() => setFocusMsg(''), 2500);
  };

  const onScroll = (e: any) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    setAtBottom(contentOffset.y + layoutMeasurement.height >= contentSize.height - 24);
  };

  const term = (
    <>
      {lines.map((spans, i) => (
        <Text key={i} style={[styles.mono, {fontSize, lineHeight}]} selectable>
          {spans.length === 0 ? ' ' : spans.map((s, j) => (
            <Text key={j} style={{color: s.color, fontWeight: s.bold ? '700' : '400'}}>
              {s.text}
            </Text>
          ))}
        </Text>
      ))}
    </>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      {/* header: back · badge · title/sub · Focus on Mac */}
      <View style={[styles.header, {borderBottomColor: pal.divider}]}>
        {onBack && (
          <TouchableOpacity onPress={onBack} hitSlop={hit} style={styles.back}>
            <Text style={[styles.backText, {color: pal.fg2}]}>‹</Text>
          </TouchableOpacity>
        )}
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
        <TouchableOpacity onPress={doFocus} style={[styles.focusTop, {borderColor: pal.divider}]}>
          <Text style={[styles.focusTopText, {color: StatusColor.working}]}>{t('focusOnMac')}</Text>
        </TouchableOpacity>
      </View>

      {/* controls: live · A− A+ · wrap/scroll */}
      <View style={[styles.controls, {borderBottomColor: pal.divider}]}>
        <View style={styles.live}>
          <View style={[styles.liveDot, {backgroundColor: StatusColor.idle}]} />
          <Text style={[styles.ctlText, {color: pal.fg3}]}>live</Text>
        </View>
        <View style={styles.ctlRight}>
          <Ctl pal={pal} label="A−" onPress={() => setFontIdx(i => Math.max(0, i - 1))} />
          <Ctl pal={pal} label="A+" onPress={() => setFontIdx(i => Math.min(FONT_SIZES.length - 1, i + 1))} />
          <Ctl
            pal={pal}
            label={wrap ? (lang === 'zh' ? '换行' : 'Wrap') : (lang === 'zh' ? '滚动' : 'Scroll')}
            onPress={() => setWrap(w => !w)}
          />
        </View>
      </View>

      {/* pane screen (colored) */}
      <View style={styles.termWrap}>
        <ScrollView
          ref={scrollRef}
          style={styles.term}
          contentContainerStyle={styles.termContent}
          scrollEventThrottle={80}
          onScroll={onScroll}
          onContentSizeChange={() => {
            if (atBottom) scrollRef.current?.scrollToEnd({animated: false});
          }}>
          {loading ? (
            <ActivityIndicator color={pal.fg3} style={styles.loading} />
          ) : wrap ? (
            term
          ) : (
            <ScrollView horizontal showsHorizontalScrollIndicator>
              <View>{term}</View>
            </ScrollView>
          )}
        </ScrollView>
        {!atBottom && (
          <TouchableOpacity
            style={styles.fab}
            onPress={() => scrollRef.current?.scrollToEnd({animated: true})}>
            <Text style={styles.fabText}>↓</Text>
          </TouchableOpacity>
        )}
      </View>

      {!!focusMsg && (
        <View style={[styles.footer, {borderTopColor: pal.divider}]}>
          <Text style={[styles.focusMsg, {color: pal.fg2}]}>{focusMsg}</Text>
        </View>
      )}

      {/* input — types into the pane via POST /api/send (MOBILE §4) */}
      <Composer
        status={agent.status}
        pal={pal}
        lang={lang}
        onSend={p => {
          client.send(agent.pane_id, p);
        }}
      />
    </SafeAreaView>
  );
}

function Ctl({pal, label, onPress}: {pal: any; label: string; onPress: () => void}) {
  return (
    <TouchableOpacity onPress={onPress} style={[styles.ctl, {borderColor: pal.divider}]}>
      <Text style={[styles.ctlText, {color: pal.fg2}]}>{label}</Text>
    </TouchableOpacity>
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
  focusTop: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 10, paddingVertical: 5, marginLeft: 8},
  focusTopText: {fontSize: 12, fontWeight: '700'},
  controls: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 12,
    paddingVertical: 7,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  live: {flexDirection: 'row', alignItems: 'center'},
  liveDot: {width: 6, height: 6, borderRadius: 3, marginRight: 5},
  ctlRight: {flexDirection: 'row', alignItems: 'center'},
  ctl: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 9, paddingVertical: 3, marginLeft: 7},
  ctlText: {fontSize: 11.5, fontWeight: '600'},
  termWrap: {flex: 1},
  term: {flex: 1, backgroundColor: '#0A0A0C'},
  termContent: {padding: 12},
  loading: {marginTop: 40},
  mono: {color: '#D6D6DA', fontFamily: 'Menlo'},
  fab: {
    position: 'absolute',
    right: 16,
    bottom: 16,
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: '#06B6D4',
    alignItems: 'center',
    justifyContent: 'center',
  },
  fabText: {color: '#fff', fontSize: 20, fontWeight: '700'},
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  focusMsg: {fontSize: 12.5, flex: 1},
});
