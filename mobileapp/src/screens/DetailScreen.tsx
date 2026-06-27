// DetailScreen — a read-only view of one pane's current screen (MOBILE §4), in
// COLOR. It polls /api/pane (now `tmux capture-pane -e`) every ~1.5s and renders
// the ANSI output with a native SGR parser into colored <Text> spans — offline
// over VPN, no webview/xterm needed. Narrow-screen controls: A−/A+ font size, a
// wrap↔scroll toggle, and a jump-to-bottom FAB. "Focus on Mac" lives in the top
// bar (POST /api/focus), not the input area.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, primary, ReplyOption, secondary, TermTheme} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {StatusBadge} from '../ui/StatusBadge';
import {statusLabel} from '../i18n';
import {AnsiLine, parseAnsi} from '../ui/ansi';
import {Composer} from '../ui/Composer';
import {ApprovalCard} from '../ui/ApprovalCard';
import {XtermView} from '../ui/XtermView';
import {FloatingKeys} from '../ui/FloatingKeys';
import {DiffModal} from '../ui/DiffModal';
import {StatusColor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

const FONT_SIZES = [9, 11, 13];

// DetailScreen is the stack route (compact); it wraps the presentational
// DetailView, which the iPad split-view also renders directly in its main pane.
export function DetailScreen({route, navigation}: any) {
  return <DetailView agent={route.params.agent} onBack={() => navigation.goBack()} />;
}

export function DetailView({agent, onBack}: {agent: Agent; onBack?: () => void}) {
  const {client, agents, conn} = useAgents();
  const {pal, lang, xtermEnabled, fontPref, mac, returnSends} = useApp();
  // `agent` is a static snapshot from the navigation params; resolve the LIVE agent
  // from the polled store by pane_id so the header badge/status follow status changes
  // (working→waiting→idle) while you're on this screen. Fall back to the snapshot if
  // it's momentarily absent from the list (e.g. between polls / pane just closed).
  const live = agents.find(a => a.pane_id === agent.pane_id) ?? agent;
  const [text, setText] = useState('');
  const [cursor, setCursor] = useState<{x: number; up: number; visible: boolean} | undefined>();
  const [theme, setTheme] = useState<TermTheme | undefined>();
  const [loading, setLoading] = useState(true);
  const [fontIdx, setFontIdx] = useState(1);
  const [wrap, setWrap] = useState(true);
  const [atBottom, setAtBottom] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const [keysOpen, setKeysOpen] = useState(false);
  const [diffOpen, setDiffOpen] = useState(false);
  const [options, setOptions] = useState<ReplyOption[]>([]);
  const scrollRef = useRef<ScrollView>(null);

  // Fetch the host terminal's appearance once so the pane matches it (global, not per-pane).
  useEffect(() => {
    let alive = true;
    client.theme().then(t => { if (alive && t) setTheme(t); });
    return () => { alive = false; };
  }, [client]);

  const smaller = () => setFontIdx(i => Math.max(0, i - 1));
  const bigger = () => setFontIdx(i => Math.min(FONT_SIZES.length - 1, i + 1));
  const wrapLabel = wrap ? (lang === 'zh' ? '换行' : 'Wrap') : lang === 'zh' ? '滚动' : 'Scroll';

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const r = await client.pane(agent.pane_id);
        if (alive) {
          // Skip the update when the screen is unchanged so a re-render doesn't
          // clobber an in-progress text selection (React bails on an equal value).
          setText(prev => (prev === (r.text || '') ? prev : r.text || ''));
          setCursor(r.cursor);
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

  // Approval card (B1): only while waiting (cardinal rule), poll the pane's 1/2/3
  // choices from the shared parser. Cleared the moment it's no longer waiting.
  useEffect(() => {
    if (live.status !== 'waiting') {
      setOptions([]);
      return;
    }
    let alive = true;
    const load = () => client.options(agent.pane_id).then(o => alive && setOptions(o));
    load();
    const id = setInterval(load, 2000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, agent.pane_id, live.status]);

  const lines: AnsiLine[] = useMemo(() => parseAnsi(text), [text]);
  const fontSize = FONT_SIZES[fontIdx];
  const lineHeight = Math.round(fontSize * 1.36);

  const onScroll = (e: any) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    setAtBottom(contentOffset.y + layoutMeasurement.height >= contentSize.height - 24);
  };

  // The whole screen is ONE selectable <Text> (lines joined by '\n'), so a
  // long-press starts a real drag selection that spans lines — extend the
  // handles over any region and use iOS's own Copy. (Per-line <Text>s, as before,
  // could only ever select within a single line, so copy was useless.)
  const term = (
    <Text style={[styles.mono, {fontSize, lineHeight}]} selectable>
      {lines.map((spans, i) => (
        <Text key={i}>
          {i > 0 ? '\n' : ''}
          {spans.length === 0
            ? ' '
            : spans.map((s, j) => (
                <Text key={j} style={{color: s.color, fontWeight: s.bold ? '700' : '400'}}>
                  {s.text}
                </Text>
              ))}
        </Text>
      ))}
    </Text>
  );

  return (
    <KeyboardAvoidingView
      testID={TestIds.detail.screen}
      style={[styles.safe, {backgroundColor: pal.bg}]}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <StatusBar hidden={fullscreen} />
      {/* keep the top safe-area inset even in full-screen so the floating control
          isn't hidden under the notch / Dynamic Island */}
      <SafeAreaView style={styles.safe} edges={['top']}>
      {/* header: back · badge · title/sub · Focus on Mac (hidden in full-screen) */}
      {!fullscreen && (
        <View style={[styles.header, {borderBottomColor: pal.divider}]}>
          {onBack && (
            <TouchableOpacity
              testID={TestIds.detail.back}
              accessibilityLabel={TestIds.detail.back}
              onPress={onBack}
              hitSlop={hit}
              style={styles.back}>
              <Text style={[styles.backText, {color: pal.fg2}]}>‹</Text>
            </TouchableOpacity>
          )}
          <View style={styles.badgeWrap}>
            <StatusBadge status={live.status} size={18} />
          </View>
          <View style={styles.headerText}>
            <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
              {primary(live)}
            </Text>
            <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
              {live.agent} · {statusLabel(live.status, lang)} · {secondary(live)}
            </Text>
          </View>
        </View>
      )}

      {/* controls: connection · A− A+ · wrap · full-screen (hidden in full-screen) */}
      {!fullscreen && (
        <View style={[styles.controls, {borderBottomColor: pal.divider}]}>
          {/* D9: server name + status dot (no "live" text); only abnormal states add a word. */}
          <View style={styles.live}>
            <View
              style={[
                styles.liveDot,
                {backgroundColor: conn === 'live' ? StatusColor.idle : conn === 'offline' ? StatusColor.waiting : '#F59E0B'},
              ]}
            />
            <Text style={[styles.ctlText, {color: pal.fg3}]} numberOfLines={1}>
              {mac?.name || (lang === 'zh' ? '服务器' : 'server')}
              {conn === 'offline'
                ? lang === 'zh' ? ' · 离线' : ' · offline'
                : conn === 'connecting'
                ? lang === 'zh' ? ' · 重连中' : ' · reconnecting'
                : ''}
            </Text>
          </View>
          <View style={styles.ctlRight}>
            <Ctl pal={pal} label={lang === 'zh' ? '改动' : 'Diff'} onPress={() => setDiffOpen(true)} />
            <Ctl pal={pal} label="A−" onPress={smaller} />
            <Ctl pal={pal} label="A+" onPress={bigger} />
            <Ctl pal={pal} label={wrapLabel} onPress={() => setWrap(w => !w)} />
            <Ctl pal={pal} label="⛶" onPress={() => setFullscreen(true)} />
          </View>
        </View>
      )}

      {/* pane screen (colored) — xterm.js emulator (opt-in) or the classic renderer */}
      <View style={styles.termWrap} testID={TestIds.detail.pane}>
        {xtermEnabled ? (
          <XtermView text={text} fontSize={fontSize} wrap={wrap} cursor={cursor} theme={theme} fontPref={fontPref} />
        ) : (
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
        )}
        {!xtermEnabled && !atBottom && (
          <TouchableOpacity
            style={styles.fab}
            onPress={() => scrollRef.current?.scrollToEnd({animated: true})}>
            <Text style={styles.fabText}>↓</Text>
          </TouchableOpacity>
        )}
        {/* full-screen: just a clean exit control (the config options are hidden
            for more room; adjust A−/A+ / wrap before going full-screen) */}
        {fullscreen && (
          <View style={styles.fsBar}>
            <FsBtn label={'⤡ ' + (lang === 'zh' ? '退出' : 'Exit')} onPress={() => setFullscreen(false)} />
          </View>
        )}
      </View>

      {/* approval card (B1): waiting → the agent's own 1/2/3 as big buttons */}
      <ApprovalCard
        options={options}
        pal={pal}
        lang={lang}
        onSend={n => {
          client.send(agent.pane_id, {text: String(n), enter: true});
          setOptions([]);
        }}
      />

      {/* input — types into the pane via POST /api/send (MOBILE §4) */}
      <Composer
        status={live.status}
        pal={pal}
        lang={lang}
        returnSends={returnSends}
        onSend={p => {
          client.send(agent.pane_id, p);
        }}
        onUpload={(uri, name, type) => client.upload(uri, name, type)}
        onOpenKeys={() => setKeysOpen(true)}
      />

      {/* Moshi-style draggable nav keypad, floating over the terminal */}
      {/* "what did the agent change" — git diff of the pane's cwd */}
      <DiffModal
        visible={diffOpen}
        paneId={agent.pane_id}
        client={client}
        pal={pal}
        lang={lang}
        onClose={() => setDiffOpen(false)}
      />

      <FloatingKeys
        visible={keysOpen}
        pal={pal}
        lang={lang}
        onKey={key => client.send(agent.pane_id, {key})}
        onClose={() => setKeysOpen(false)}
      />
      </SafeAreaView>
    </KeyboardAvoidingView>
  );
}

function Ctl({pal, label, onPress}: {pal: any; label: string; onPress: () => void}) {
  return (
    <TouchableOpacity onPress={onPress} style={[styles.ctl, {borderColor: pal.divider}]}>
      <Text style={[styles.ctlText, {color: pal.fg2}]}>{label}</Text>
    </TouchableOpacity>
  );
}

// FsBtn — a button in the floating full-screen control pill (over the terminal).
function FsBtn({label, onPress}: {label: string; onPress: () => void}) {
  return (
    <TouchableOpacity onPress={onPress} style={styles.fsBtn} hitSlop={hit}>
      <Text style={styles.fsBtnText}>{label}</Text>
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
  fsBar: {
    position: 'absolute',
    top: 8,
    right: 10,
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: 'rgba(20,20,22,0.82)',
    borderRadius: 18,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: 'rgba(255,255,255,0.14)',
    paddingHorizontal: 4,
  },
  fsBtn: {paddingHorizontal: 11, paddingVertical: 7},
  fsBtnText: {color: 'rgba(255,255,255,0.88)', fontSize: 13, fontWeight: '600'},
});
