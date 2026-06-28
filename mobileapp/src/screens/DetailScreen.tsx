// DetailScreen — a read-only view of one pane's current screen (MOBILE §4), in
// COLOR. It polls /api/pane (now `tmux capture-pane -e`) every ~1.5s and renders
// the ANSI output with a native SGR parser into colored <Text> spans — offline
// over VPN, no webview/xterm needed. Narrow-screen controls: A−/A+ font size, a
// wrap↔scroll toggle, and a jump-to-bottom FAB. "Focus on Mac" lives in the top
// bar (POST /api/focus), not the input area.

import React, {useEffect, useMemo, useState} from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import Clipboard from '@react-native-clipboard/clipboard';
import {Agent, primary, ReplyOption, secondary, TermTheme} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {StatusBadge} from '../ui/StatusBadge';
import {statusLabel} from '../i18n';
import {AnsiLine, parseAnsi} from '../ui/ansi';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {ApprovalCard} from '../ui/ApprovalCard';
import {XtermView} from '../ui/XtermView';
import {FloatingKeys} from '../ui/FloatingKeys';
import {DiffModal} from '../ui/DiffModal';
import {StatusColor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

const FONT_SIZES = [9, 11, 13];

type DetailMode = 'chat' | 'terminal';
const MODE_KEY = (paneId: string) => `detail.mode.${paneId}`;

// DetailScreen is the stack route (compact); it wraps the presentational
// DetailView, which the iPad split-view also renders directly in its main pane.
export function DetailScreen({route, navigation}: any) {
  return <DetailView agent={route.params.agent} onBack={() => navigation.goBack()} />;
}

export function DetailView({agent, onBack}: {agent: Agent; onBack?: () => void}) {
  const {client, agents, conn} = useAgents();
  const {pal, lang, fontPref, mac, returnSends, defaultDetailMode} = useApp();
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
  const [fullscreen, setFullscreen] = useState(false);
  const [keysOpen, setKeysOpen] = useState(false);
  const [pendingPrompt, setPendingPrompt] = useState(''); // optimistic chat echo
  const [diffOpen, setDiffOpen] = useState(false);
  const [options, setOptions] = useState<ReplyOption[]>([]);
  const [slow, setSlow] = useState(false); // D8: pane taking >3s to first paint
  // B1: 对话 ↔ 终端. Initial mode = the global "default mode" setting (B2, default
  // 终端 — preserves the established read-the-pane behavior; 对话 is a visible-
  // screen glance, not a full transcript), overridden by this pane's own
  // remembered choice if it has one.
  const [mode, setMode] = useState<DetailMode>(defaultDetailMode);

  useEffect(() => {
    let alive = true;
    AsyncStorage.getItem(MODE_KEY(agent.pane_id)).then(v => {
      if (alive && (v === 'chat' || v === 'terminal')) setMode(v);
    });
    return () => {
      alive = false;
    };
  }, [agent.pane_id]);

  const pickMode = (m: DetailMode) => {
    setMode(m);
    AsyncStorage.setItem(MODE_KEY(agent.pane_id), m);
    if (m === 'chat') setFullscreen(false); // full-screen is terminal-only
  };

  // D8: upgrade the loading copy if the first frame is slow to arrive.
  useEffect(() => {
    if (!loading) {
      setSlow(false);
      return;
    }
    const id = setTimeout(() => setSlow(true), 3000);
    return () => clearTimeout(id);
  }, [loading]);

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

  // D6: copy the visible screen as plain text (ANSI stripped via the parsed spans).
  const copyVisible = () => {
    const plain = lines
      .map(spans => spans.map(s => s.text).join(''))
      .join('\n')
      .replace(/\s+$/, '');
    Clipboard.setString(plain);
  };

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

      {/* B1: 对话 ↔ 终端 segmented (remembered per pane) */}
      {!fullscreen && (
        <View style={styles.segWrap}>
          <View style={[styles.seg, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
            <Seg
              label={lang === 'zh' ? '对话' : 'Chat'}
              active={mode === 'chat'}
              onPress={() => pickMode('chat')}
              testID={TestIds.detail.modeChat}
              pal={pal}
            />
            <Seg
              label={lang === 'zh' ? '终端' : 'Terminal'}
              active={mode === 'terminal'}
              onPress={() => pickMode('terminal')}
              testID={TestIds.detail.modeTerminal}
              pal={pal}
            />
          </View>
        </View>
      )}

      {/* controls: connection · (terminal-only) A− A+ · wrap · full-screen */}
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
            {mode === 'terminal' && (
              <>
                <Ctl pal={pal} label={lang === 'zh' ? '复制' : 'Copy'} onPress={copyVisible} />
                <Ctl pal={pal} label="A−" onPress={smaller} />
                <Ctl pal={pal} label="A+" onPress={bigger} />
                <Ctl pal={pal} label={wrapLabel} onPress={() => setWrap(w => !w)} />
                <Ctl pal={pal} label="⛶" onPress={() => setFullscreen(true)} />
              </>
            )}
          </View>
        </View>
      )}

      {/* body: 对话 (glance) or 终端 (raw TUI) */}
      {mode === 'chat' ? (
        <ChatView agent={live} lines={lines} status={live.status} fontSize={fontSize} pal={pal} lang={lang} client={client} paneId={agent.pane_id} pendingPrompt={pendingPrompt} />
      ) : (
      /* pane screen (colored) — xterm.js terminal emulator */
      <View style={styles.termWrap} testID={TestIds.detail.pane}>
        <XtermView text={text} fontSize={fontSize} wrap={wrap} cursor={cursor} theme={theme} fontPref={fontPref} />
        {/* D8: pane-loading feedback (until the first frame arrives) */}
        {loading && !text && (
          <View style={styles.loadingOverlay}>
            <ActivityIndicator color={pal.fg3} />
            <Text style={[styles.loadingText, {color: pal.fg3}]}>
              {slow
                ? lang === 'zh' ? '仍在连接…网络较慢' : 'Still connecting… slow network'
                : lang === 'zh' ? '正在拉取屏幕…' : 'Loading screen…'}
            </Text>
          </View>
        )}
        {/* full-screen: just a clean exit control (the config options are hidden
            for more room; adjust A−/A+ / wrap before going full-screen) */}
        {fullscreen && (
          <View style={styles.fsBar}>
            <FsBtn label={'⤡ ' + (lang === 'zh' ? '退出' : 'Exit')} onPress={() => setFullscreen(false)} />
          </View>
        )}
      </View>
      )}

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
          // optimistic echo in 对话 mode: show the sent text immediately as a
          // pending bubble until the transcript refetch confirms it.
          if (p.text) setPendingPrompt(p.text);
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

// Seg — one segment of the 对话/终端 toggle (B1).
function Seg({
  label,
  active,
  onPress,
  testID,
  pal,
}: {
  label: string;
  active: boolean;
  onPress: () => void;
  testID: string;
  pal: any;
}) {
  return (
    <TouchableOpacity
      testID={testID}
      accessibilityLabel={testID}
      onPress={onPress}
      activeOpacity={0.8}
      style={[styles.segBtn, active && {backgroundColor: pal.bg}]}>
      <Text style={[styles.segText, {color: active ? pal.fg : pal.fg3}]}>{label}</Text>
    </TouchableOpacity>
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
  segWrap: {paddingHorizontal: 12, paddingTop: 8, paddingBottom: 4},
  seg: {flexDirection: 'row', borderRadius: 9, borderWidth: StyleSheet.hairlineWidth, padding: 2},
  segBtn: {flex: 1, alignItems: 'center', paddingVertical: 6, borderRadius: 7},
  segText: {fontSize: 13, fontWeight: '600'},
  live: {flexDirection: 'row', alignItems: 'center', flexShrink: 1, minWidth: 0, marginRight: 8},
  liveDot: {width: 6, height: 6, borderRadius: 3, marginRight: 5},
  ctlRight: {flexDirection: 'row', alignItems: 'center'},
  ctl: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 9, paddingVertical: 3, marginLeft: 7},
  ctlText: {fontSize: 11.5, fontWeight: '600'},
  termWrap: {flex: 1},
  loadingOverlay: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, alignItems: 'center', justifyContent: 'center', gap: 10},
  loadingText: {fontSize: 12.5},
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
