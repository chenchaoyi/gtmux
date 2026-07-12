// DetailScreen — a read-only view of one pane's current screen (MOBILE §4), in
// COLOR. It polls /api/pane (now `tmux capture-pane -e`) every ~1.5s and renders
// the ANSI output with a native SGR parser into colored <Text> spans — offline
// over VPN, no webview/xterm needed. Narrow-screen controls: A−/A+ font size, a
// wrap↔scroll toggle, and a jump-to-bottom FAB. (The phone-side "Focus on Mac"
// action was removed in #85 — little value when you're remote.)

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {
  Alert,
  Animated,
  KeyboardAvoidingView,
  Platform,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Agent, primary, ReplyOption, secondary, TermTheme} from '../api/types';
import {SendPayload, TranscriptTurn} from '../api/client';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {StatusBadge} from '../ui/StatusBadge';
import {AgentAvatar} from '../ui/AgentAvatar';
import {statusLabel} from '../i18n';
import {AnsiLine, parseAnsi} from '../ui/ansi';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {BrandLoader} from '../ui/BrandLoader';
import {ApprovalCard} from '../ui/ApprovalCard';
import {NativeTerm} from '../ui/NativeTerm';
import {DiffModal} from '../ui/DiffModal';
import {StatusColor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

// Shared by BOTH the terminal renderer and the chat view (A−/A+ adjusts both, in
// either mode) so switching modes never jumps the text size. Middle = default.
const FONT_SIZES = [11, 13, 15];

type DetailMode = 'chat' | 'terminal';
const MODE_KEY = (paneId: string) => `detail.mode.${paneId}`;
// Font size is a GLOBAL config (not per-pane): shared by both chat + terminal and
// remembered across panes/sessions, so A−/A+ in either mode adjusts both and sticks.
const FONT_IDX_KEY = 'detail.fontIdx';

// DetailScreen is the stack route (compact); it wraps the presentational
// DetailView, which the iPad split-view also renders directly in its main pane.
export function DetailScreen({route, navigation}: any) {
  return <DetailView agent={route.params.agent} initialMode={route.params.mode} onBack={() => navigation.goBack()} />;
}

export function DetailView({agent, onBack, initialMode}: {agent: Agent; onBack?: () => void; initialMode?: DetailMode}) {
  const {client, agents, conn} = useAgents();
  const {pal, lang, fontPref, mac, returnSends, defaultDetailMode} = useApp();
  // ≥768 means we're embedded in the iPad split-view's main pane (never a narrow
  // phone). Constrain content so the chat/segmented don't stretch across ~1000pt,
  // and drop the connection chip the sidebar already shows.
  const {width} = useWindowDimensions();
  const isWide = width >= 768;
  // `agent` is a static snapshot from the navigation params; resolve the LIVE agent
  // from the polled store by pane_id so the header badge/status follow status changes
  // (working→waiting→idle) while you're on this screen. Fall back to the snapshot if
  // it's momentarily absent from the list (e.g. between polls / pane just closed).
  const live = agents.find(a => a.pane_id === agent.pane_id) ?? agent;
  const [text, setText] = useState('');
  const [cursor, setCursor] = useState<{x: number; up: number; visible: boolean} | undefined>();
  const [theme, setTheme] = useState<TermTheme | undefined>();
  const [loading, setLoading] = useState(true);
  // Auto-hide the header info block to reclaim space while you browse history /
  // scrollback (i.e. away from the live tail); reveal it when you flick back to the
  // bottom. `collapse` 0 = shown, 1 = hidden; `headerH` is measured once for the
  // height animation. Driven by ChatView/NativeTerm's onLiveEdge.
  const collapse = useRef(new Animated.Value(0)).current;
  const lastEdge = useRef(true); // last atBottom → animate only on a real change
  const [headerH, setHeaderH] = useState(0);
  const onLiveEdge = useCallback(
    (atBottom: boolean) => {
      if (atBottom === lastEdge.current) return; // fires every scroll frame — dedupe so
      lastEdge.current = atBottom; // one clean animation runs (no restart-creep)
      Animated.timing(collapse, {toValue: atBottom ? 0 : 1, duration: 200, useNativeDriver: false}).start();
    },
    [collapse],
  );
  const [fontIdx, setFontIdx] = useState(1);
  const [fullscreen, setFullscreen] = useState(false);
  const [pendingPrompt, setPendingPrompt] = useState(''); // optimistic chat echo
  const [diffOpen, setDiffOpen] = useState(false);
  const [options, setOptions] = useState<ReplyOption[]>([]);
  // When an approval choice was just tapped (epoch ms). For a short settle window
  // after, the options poll ignores what it parses: the pane is mid-transition, so a
  // capture catches transient/redrawing frames (the just-answered menu lingering, or
  // a half-rendered "1234") that would flicker the card back up. Once the window
  // passes, polling resumes and a genuinely new menu shows normally.
  const answeredAt = useRef(0);
  const [slow, setSlow] = useState(false); // D8: pane taking >3s to first paint
  // Chat transcript lives HERE (DetailScreen stays mounted across mode switches) so
  // flipping 终端→对话 shows the cached history instantly instead of re-fetching +
  // spinning every time. Polled on status/prompt change; turns are never cleared.
  const [turns, setTurns] = useState<TranscriptTurn[]>([]);
  const [chatLoaded, setChatLoaded] = useState(false);
  // B1: 对话 ↔ 终端. Initial mode = the global "default mode" setting (B2, default
  // 终端 — preserves the established read-the-pane behavior; 对话 is a visible-
  // screen glance, not a full transcript), overridden by this pane's own
  // remembered choice if it has one.
  // An explicit route mode (the HQ card opens CHAT — talking to the supervisor is
  // the point) beats the global default.
  const [mode, setMode] = useState<DetailMode>(initialMode ?? defaultDetailMode);
  // Each view (terminal = hundreds of dual-layer <Text> rows; chat = many markdown
  // turns) is expensive to MOUNT, so we keep BOTH mounted once visited and just
  // toggle visibility with display:none — a switch is then instant (no re-mount, no
  // re-parse) and even preserves each view's scroll position. `seen{Chat,Term}`
  // lazily mounts a view the first time its mode is opened (no upfront cost for a
  // mode you never visit); the spinner below only covers that one first mount.
  const [seenChat, setSeenChat] = useState((initialMode ?? defaultDetailMode) === 'chat');
  const [seenTerm, setSeenTerm] = useState((initialMode ?? defaultDetailMode) === 'terminal');
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    if (mode === 'chat') setSeenChat(true);
    else setSeenTerm(true);
    collapse.setValue(0); // reveal the header when switching Chat↔Terminal
    lastEdge.current = true;
  }, [mode, collapse]);

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
    if (m === mode) return;
    AsyncStorage.setItem(MODE_KEY(agent.pane_id), m);
    // full-screen carries across modes now (both 对话 + 终端 support it)
    // Show the spinner on EVERY switch: the first mount is slow (heavy layout) and
    // even a subsequent opacity swap has a slight composite delay — the spinner
    // (native-animated, survives the JS hitch) covers both. Paint it for 2 frames
    // before swapping, then hold it a perceptible minimum so it never just flickers
    // (a 4-frame clear was invisible on a fast switch → "loading only the first time").
    setSwitching(true);
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        setMode(m);
        setTimeout(() => setSwitching(false), 280);
      }),
    );
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

  // Load the remembered global font size once; persist on every change. Both modes
  // read the same fontSize, so this is the single source of truth for either A−/A+.
  useEffect(() => {
    let alive = true;
    AsyncStorage.getItem(FONT_IDX_KEY).then(v => {
      const n = v == null ? NaN : parseInt(v, 10);
      if (alive && Number.isInteger(n) && n >= 0 && n < FONT_SIZES.length) setFontIdx(n);
    });
    return () => {
      alive = false;
    };
  }, []);
  const setFont = (i: number) => {
    setFontIdx(i);
    AsyncStorage.setItem(FONT_IDX_KEY, String(i));
  };
  const smaller = () => setFont(Math.max(0, fontIdx - 1));
  const bigger = () => setFont(Math.min(FONT_SIZES.length - 1, fontIdx + 1));

  const loadPane = useCallback(async () => {
    try {
      const r = await client.pane(agent.pane_id);
      // Skip the update when the screen is unchanged so a re-render doesn't
      // clobber an in-progress text selection (React bails on an equal value).
      setText(prev => (prev === (r.text || '') ? prev : r.text || ''));
      // Same for the cursor: r.cursor is a fresh object every poll, so setting
      // it unconditionally re-rendered the terminal every 1.5s and wiped any
      // active selection. Keep the previous object when the values are equal.
      setCursor(prev => {
        const c = r.cursor;
        if (prev === c) return prev;
        if (prev && c && prev.x === c.x && prev.up === c.up && prev.visible === c.visible) return prev;
        return c;
      });
      setLoading(false);
    } catch {
      setLoading(false);
    }
  }, [client, agent.pane_id]);

  // Late safety polls for a redraw slower than the server's post-send settle (the
  // snapshot below already covers the common case in ONE round-trip). The base 1.5s
  // poll keeps running regardless.
  const bumpPane = useCallback(() => {
    [300, 750].forEach(d => setTimeout(loadPane, d));
  }, [loadPane]);

  // sendPane = type into the pane. /api/send now returns the post-send screen, so
  // we render the echo straight from its response — one round-trip instead of two
  // (the big win over a remote tunnel). Late bumps catch a slow TUI redraw.
  const sendPane = useCallback(
    (payload: SendPayload) => {
      client
        .send(agent.pane_id, payload)
        .then(snap => {
          if (snap && snap.text) {
            setText(prev => (prev === snap.text ? prev : snap.text));
            if (snap.cursor) setCursor(snap.cursor);
          }
        })
        .finally(bumpPane);
    },
    [client, agent.pane_id, bumpPane],
  );

  useEffect(() => {
    loadPane();
    const id = setInterval(loadPane, 1500);
    return () => clearInterval(id);
  }, [loadPane]);

  // Approval card (B1): only while waiting (cardinal rule), poll the pane's 1/2/3
  // choices from the shared parser. Cleared the moment it's no longer waiting.
  useEffect(() => {
    if (live.status !== 'waiting') {
      setOptions([]);
      return;
    }
    let alive = true;
    // Ignore poll results inside the post-answer settle window (see answeredAt) so a
    // mid-transition capture can't flicker the card back with stale/partial options.
    const load = () =>
      client.options(agent.pane_id).then(o => {
        if (alive && Date.now() - answeredAt.current > 1500) setOptions(o);
      });
    load();
    const id = setInterval(load, 2000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, agent.pane_id, live.status]);

  // Fetch the transcript on mount + when the status flips or a prompt is sent (a
  // turn likely completed). Never clears `turns`, so a background refetch swaps in
  // fresh history without flashing the spinner — and a mode switch is instant.
  useEffect(() => {
    let alive = true;
    client
      .transcript(agent.pane_id)
      .then(ts => {
        if (!alive) return;
        setTurns(ts);
        setChatLoaded(true);
      })
      .catch(() => alive && setChatLoaded(true));
    return () => {
      alive = false;
    };
  }, [client, agent.pane_id, live.status, pendingPrompt]);

  const lines: AnsiLine[] = useMemo(() => parseAnsi(text), [text]);
  const fontSize = FONT_SIZES[fontIdx];

  // Memoize the two HEAVY views by their real data deps so a mode switch (which
  // changes only `mode`/`switching`, not these deps) reuses the SAME element refs
  // → React skips reconciling their trees entirely → the switch is instant. Without
  // this, every setState (the switch itself AND each 1.5s poll) re-ran both render
  // trees in JS even when nothing changed — that was the "停顿 on unchanging content".
  const chatEl = useMemo(
    () => (
      <ChatView agent={live} lines={lines} status={live.status} fontSize={fontSize} pal={pal} lang={lang} turns={turns} loading={!chatLoaded} pendingPrompt={pendingPrompt} fontPref={fontPref} onLiveEdge={onLiveEdge} />
    ),
    [live, lines, fontSize, pal, lang, turns, chatLoaded, pendingPrompt, fontPref, onLiveEdge],
  );
  const termEl = useMemo(
    () => <NativeTerm text={text} fontSize={fontSize} cursor={cursor} theme={theme} fontPref={fontPref} onLiveEdge={onLiveEdge} />,
    [text, fontSize, cursor, theme, fontPref, onLiveEdge],
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
      {/* header: back · badge · title/sub. Auto-collapses to reclaim space while you
          browse history/scrollback; reveals on flicking back to the live tail. */}
      {!fullscreen && (
        <Animated.View
          style={{
            opacity: collapse.interpolate({inputRange: [0, 1], outputRange: [1, 0]}),
            height: headerH > 0 ? collapse.interpolate({inputRange: [0, 1], outputRange: [headerH, 0]}) : undefined,
            overflow: 'hidden',
          }}>
        <View
          onLayout={e => {
            // Measure ONCE at natural height (collapse starts at 0=shown). Never
            // re-measure — a layout during collapse would clobber it with a shrunken
            // value and the header would only re-expand partway.
            const h = e.nativeEvent.layout.height;
            if (headerH === 0 && h > 0) setHeaderH(h);
          }}
          style={[styles.header, {borderBottomColor: pal.divider}]}>
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
          <View style={styles.avatarWrap}>
            <AgentAvatar agent={live} size={26} radius={7} bg={pal.surface} fg={pal.fg2} border={pal.divider} />
            <View style={styles.headerBadge}>
              <StatusBadge status={live.status} size={15} errored={!!live.error} />
            </View>
          </View>
          {/* The title is 1-line for space; tap it to read the FULL task (and error)
              in an alert — the row/header stay compact but nothing is unreachable. */}
          <TouchableOpacity
            activeOpacity={0.6}
            style={styles.headerText}
            onPress={() => {
              const body = live.error && live.error_text
                ? `${primary(live)}\n\n⚠ ${live.error_text}`
                : primary(live);
              Alert.alert(live.agent || 'Agent', body, [{text: lang === 'zh' ? '关闭' : 'Close'}]);
            }}>
            <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
              {primary(live)}
            </Text>
            <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
              {live.agent} · {statusLabel(live.status, lang)} · {secondary(live)}
            </Text>
          </TouchableOpacity>
        </View>
        </Animated.View>
      )}

      {/* B1: 对话 ↔ 终端 segmented (remembered per pane) */}
      {!fullscreen && (
        <View style={styles.segWrap}>
          <View style={[styles.seg, {backgroundColor: pal.surface, borderColor: pal.divider}, isWide && styles.segWide]}>
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
          {/* D9: server name + status dot (no "live" text); only abnormal states add a
              word. Hidden on iPad (isWide): the split-view sidebar already shows the
              connection, so repeating it in the main pane is redundant noise. */}
          {isWide ? (
            <View style={styles.live} />
          ) : (
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
          )}
          <View style={styles.ctlRight}>
            <Ctl pal={pal} label={lang === 'zh' ? '代码改动' : 'Diff'} onPress={() => setDiffOpen(true)} />
            {/* font size + full-screen both apply to either mode (consistent behavior). */}
            <Ctl pal={pal} label="A−" onPress={smaller} />
            <Ctl pal={pal} label="A+" onPress={bigger} />
            <Ctl pal={pal} label="⛶" glyph onPress={() => setFullscreen(true)} testID={TestIds.detail.fullscreen} />
          </View>
        </View>
      )}

      {/* body: 对话 (glance) + 终端 (raw TUI). Both stay MOUNTED AND LAID OUT once
          visited — they're absolutely stacked and we toggle only opacity/zIndex/
          pointerEvents (compositor props, NO Yoga relayout), so after a mode's
          first mount, switching is genuinely instant (display:none would relayout
          hundreds of <Text> nodes — that was the lag). Each is lazily mounted. */}
      <View style={styles.body}>
      {seenChat && (
        <View style={[styles.layer, mode === 'chat' ? styles.layerOn : styles.layerOff]} pointerEvents={mode === 'chat' ? 'auto' : 'none'}>
          {chatEl}
        </View>
      )}
      {seenTerm && (
      /* pane screen (colored) — native RN <Text> renderer (selectable, no keyboard) */
      <View
        style={[styles.layer, mode === 'terminal' ? styles.layerOn : styles.layerOff]}
        pointerEvents={mode === 'terminal' ? 'auto' : 'none'}
        testID={TestIds.detail.pane}>
        {termEl}
        {/* D8: pane-loading feedback (until the first frame arrives) */}
        {loading && !text && (
          <View style={styles.loadingOverlay}>
            <BrandLoader
              size={40}
              neutral={pal.fg3}
              label={
                slow
                  ? lang === 'zh' ? '仍在连接…网络较慢' : 'Still connecting… slow network'
                  : lang === 'zh' ? '正在拉取屏幕…' : 'Loading screen…'
              }
              labelColor={pal.fg3}
            />
          </View>
        )}
      </View>
      )}
      {/* mode-switch loader — only the FIRST mount of a mode is slow (subsequent
          switches are an instant opacity toggle), so it covers just that. */}
      {switching && (
        <View style={[styles.loadingOverlay, {backgroundColor: pal.bg, zIndex: 5}]} pointerEvents="none">
          <BrandLoader size={40} neutral={pal.fg3} />
        </View>
      )}
      {/* full-screen exit pill — at body level so it floats over EITHER mode (the
          top chrome is hidden in full-screen; this is the way back). */}
      {fullscreen && (
        <View style={styles.fsBar}>
          <FsBtn label={'⤡ ' + (lang === 'zh' ? '退出' : 'Exit')} onPress={() => setFullscreen(false)} testID={TestIds.detail.fsExit} />
        </View>
      )}
      </View>

      {/* approval card (B1): waiting → the agent's choices as number chips (1..N) */}
      <ApprovalCard
        options={options}
        pal={pal}
        lang={lang}
        onSend={n => {
          // Send JUST the digit, no Enter. Claude's numbered menus commit on the
          // digit alone; a trailing Enter is harmless for a single choice (it lands
          // on the now-empty input) but on CONSECUTIVE prompts it leaks onto the
          // NEXT menu and auto-confirms its default — the "second choice gets
          // skipped / auto-selected" bug. An Enter-required menu (rare) still has
          // the ⏎ key in the composer's resting row.
          answeredAt.current = Date.now(); // open the settle window (see the poll)
          setOptions([]); // hide the card immediately, don't wait for the next poll
          sendPane({text: String(n)});
        }}
      />

      {/* input — types into the pane via POST /api/send (MOBILE §4) */}
      <Composer
        pal={pal}
        lang={lang}
        returnSends={returnSends}
        onSend={p => {
          sendPane(p);
          // optimistic echo in 对话 mode: show the sent text immediately as a
          // pending bubble until the transcript refetch confirms it.
          if (p.text) setPendingPrompt(p.text);
        }}
        onUpload={(uri, name, type, onProgress) => client.upload(uri, name, type, onProgress)}
      />

      {/* "what did the agent change" — git diff of the pane's cwd */}
      <DiffModal
        visible={diffOpen}
        paneId={agent.pane_id}
        client={client}
        pal={pal}
        lang={lang}
        onClose={() => setDiffOpen(false)}
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

function Ctl({pal, label, onPress, testID, glyph}: {pal: any; label: string; onPress: () => void; testID?: string; glyph?: boolean}) {
  return (
    <TouchableOpacity testID={testID} accessibilityLabel={testID} onPress={onPress} style={[styles.ctl, glyph && styles.ctlGlyphBtn, {borderColor: pal.divider}]}>
      <Text style={[glyph ? styles.ctlGlyphText : styles.ctlText, {color: pal.fg2}]}>{label}</Text>
    </TouchableOpacity>
  );
}

// FsBtn — a button in the floating full-screen control pill (over the terminal).
function FsBtn({label, onPress, testID}: {label: string; onPress: () => void; testID?: string}) {
  return (
    <TouchableOpacity testID={testID} accessibilityLabel={testID} onPress={onPress} style={styles.fsBtn} hitSlop={hit}>
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
    paddingBottom: 6,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  back: {paddingRight: 6},
  backText: {fontSize: 28, fontWeight: '300', lineHeight: 28},
  avatarWrap: {marginHorizontal: 8, marginLeft: 4},
  headerBadge: {position: 'absolute', right: -3, bottom: -3},
  headerText: {flex: 1, minWidth: 0},
  title: {fontSize: 15, fontWeight: '700'},
  sub: {fontSize: 11.5, marginTop: 1},
  controls: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 12,
    paddingVertical: 5,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  segWrap: {paddingHorizontal: 12, paddingTop: 5, paddingBottom: 5},
  seg: {flexDirection: 'row', borderRadius: 9, borderWidth: StyleSheet.hairlineWidth, padding: 2},
  segWide: {maxWidth: 460, alignSelf: 'center', width: '100%'}, // iPad: don't span the whole main pane
  segBtn: {flex: 1, alignItems: 'center', paddingVertical: 5, borderRadius: 7},
  segText: {fontSize: 13, fontWeight: '600'},
  live: {flexDirection: 'row', alignItems: 'center', flexShrink: 1, minWidth: 0, marginRight: 8},
  liveDot: {width: 6, height: 6, borderRadius: 3, marginRight: 5},
  ctlRight: {flexDirection: 'row', alignItems: 'center'},
  ctl: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 9, paddingVertical: 3, marginLeft: 7},
  ctlText: {fontSize: 11.5, fontWeight: '600'},
  // Fullscreen ⛶ is a glyph, not text — render it bigger + in a tighter box so it
  // doesn't look dwarfed next to the A−/A+ text buttons.
  ctlGlyphBtn: {paddingHorizontal: 6, paddingVertical: 1},
  ctlGlyphText: {fontSize: 18, fontWeight: '400', lineHeight: 20},
  body: {flex: 1},
  // Stacked, always-laid-out mode layers (see the body comment). Toggling opacity/
  // zIndex never relayouts — that's what makes switching instant after first mount.
  layer: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0},
  layerOn: {opacity: 1, zIndex: 1},
  layerOff: {opacity: 0, zIndex: 0},
  termWrap: {flex: 1},
  loadingOverlay: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, alignItems: 'center', justifyContent: 'center', gap: 10},
  fsBar: {
    position: 'absolute',
    top: 8,
    left: 10, // top-LEFT so it doesn't collide with the chat's top-right collapse bar
    zIndex: 10, // above the mode layers (layerOn uses zIndex:1) so it's tappable
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
