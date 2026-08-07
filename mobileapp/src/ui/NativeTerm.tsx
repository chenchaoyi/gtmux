// NativeTerm — a READ-ONLY terminal-pane renderer built from native RN <Text>,
// not a webview/xterm.js. It renders tmux `capture-pane -e` snapshots (already a
// flat, resolved screen + SGR color — NOT a live VT stream, so no terminal
// emulator is needed) via the shared ANSI parser (src/ui/ansi.ts).
//
// Why native instead of xterm-in-webview:
//   • a tap doesn't focus a hidden <textarea> → NO soft-keyboard pop-up;
//   • long-press gives REAL range selection. iOS history (device-confirmed
//     2026-08-07): RN <Text selectable> is menu-only there (no band/handles, Copy
//     grabs the WHOLE buffer), and an always-on transparent UITextView overlay
//     both MISALIGNED (its TextKit layout drifts from the Text layer's on
//     CJK/wrapped lines — the band painted up-right of the touch) and JANKED
//     (full relayout of the 1000-line buffer on every poll). So on iOS a
//     long-press opens a SELECT SHEET: a read-only UITextView of the same plain
//     text windowed from the first visible line — native band + handles + range
//     Copy, no alignment constraint (it's the only visible layer), zero standing
//     cost. Android keeps the transparent <Text selectable> overlay, which
//     selects properly there;
//   • native ScrollView momentum (no DOM/canvas repaint jank);
//   • no WebGL/canvas/DOM renderer fragility (the ~10-PR webview saga);
//   • pure JS → the same renderer works on iOS AND Android.
// Input still flows through the native Composer (POST /api/send) — display-only here.
//
// Fidelity notes: capture-pane has already resolved cursor moves / clears / alt-
// screen into a flat colored grid, so we only handle SGR (fg+bg, bold/dim, 256 /
// truecolor) + the pane's text cursor (drawn as a reverse-video cell). Monospace
// alignment + CJK width rely on the system monospace (Menlo → PingFang fallback).

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {Linking, Modal, NativeScrollEvent, NativeSyntheticEvent, Platform, ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Lang} from '../i18n';
import {JumpToBottom} from './JumpToBottom';
import {AnsiLine} from './ansi';
import {cursorSpans, linkify, linkSegsForLines, nativeFontFamily, normalizeGlyphs} from './term';
import {makeLineCache, parseLinesCached} from './termLineCache';
import {TermTheme} from '../api/types';

interface PaneCursor {
  x: number;
  up: number;
  visible: boolean;
}

interface Props {
  text: string;
  fontSize?: number;
  cursor?: PaneCursor;
  theme?: TermTheme;
  fontPref?: string; // accepted for config-parity with the font picker; native uses Menlo
  lang?: Lang; // for the iOS select sheet's chrome
  onLiveEdge?: (atBottom: boolean) => void; // hide/show host chrome as you leave/return to the live tail
}

const DEF_BG = '#17171a';
const DEF_FG = '#d4d2cc';

// iOS system monospace (covers Latin + falls back to PingFang for CJK at 2-cell
// width), via the shared resolver so the chat view uses the exact same font. The
// bundled woff2 picker fonts are webview-only; native would need them linked as
// .ttf — a later follow-up, not needed for the read-only viewer.
const MONO = nativeFontFamily();
// cap how many trailing capture lines we render as one selectable <Text> — enough
// scrollback for a phone glance, light enough not to hitch/crash. Deeper history
// lives in Chat mode (the full transcript).
const MAX_LINES = 1000; // dual-layer (color + selectable overlay) makes each line
// cost twice; 1000 gives a deep phone scrollback (the server capture sends 2000, so
// this stays half the buffer) while holding under the flat-ScrollView mount hitch —
// dial down if a fast-updating pane janks on older hardware. The bottom is preserved
// so the bottom-anchored cursor still maps; the full transcript lives in Chat mode.

// Selection tint. iOS: the select sheet's UITextView tintColor — the system paints
// the range band (at its own ~20% alpha) and the drag handles from it, so pass it
// OPAQUE. Android: its transparent <Text selectable> overlay draws the band verbatim
// behind the glyphs, so keep it translucent there (opaque would slab over the color
// layer beneath).
const SELECTION_TINT = Platform.select({ios: '#3478F7', default: 'rgba(52,120,247,0.5)'});

// One line of the color layer. Memoized: the per-line parse cache hands back a STABLE
// spans identity for a line whose raw text didn't change, so React bails on that row —
// a busy pane's in-place repaint re-renders only the changed rows (plus the cursor
// row, whose spans are freshly spliced each poll).
//
// Two render shapes:
//   • block (iOS) — its OWN top-level <Text>, stacked in a View. A changed row then
//     re-measures ONLY its own block natively; the old single-paragraph form re-ran
//     TextKit over the whole 1000-line buffer on every poll of a working pane, and
//     that main-thread stall is exactly what made Composer typing hitch. Blocks also
//     carry the long-press per line, so the select sheet knows the EXACT touched
//     line (no coordinate math). Only possible because iOS has no overlay needing
//     paragraph-exact alignment. An empty line renders ' ' (an empty block collapses
//     to zero height). All block props are per-poll-stable so memo still bails.
//   • nested (Android) — a child of the single paragraph its transparent selectable
//     overlay aligns to; `last` carries the joining newline so the flattened text
//     still equals the overlay's plainText exactly.
const TermLine = React.memo(function TermLine({
  spans,
  last,
  block,
  fontSize,
  color,
  index = 0,
  onLongPressLine,
}: {
  spans: AnsiLine;
  last: boolean;
  block?: boolean;
  fontSize?: number;
  color?: string;
  index?: number;
  onLongPressLine?: (line: number) => void;
}) {
  const body = spans.map((s, j) => {
        const base = {
          color: s.color,
          backgroundColor: s.bg,
          fontWeight: (s.bold ? '700' : '400') as '700' | '400',
        };
        // An OSC 8 hyperlink (tiered/terminal-hyperlink): the WHOLE span is one
        // agent-declared link — underlined + tappable (opens in the browser). A
        // non-web href (e.g. file:// image refs from the Mac) shows as clean text.
        const oscWeb = !!s.href && /^https?:\/\//i.test(s.href);
        if (oscWeb) {
          return (
            <Text key={j} onPress={() => Linking.openURL(s.href!)} style={{...base, textDecorationLine: 'underline'}}>
              {s.text}
            </Text>
          );
        }
        // Otherwise auto-detect BARE http(s) URLs the agent merely printed as text
        // and make each one tappable (open in the system browser), same as an OSC 8
        // link. The common no-URL line renders as a single <Text> (fast path).
        const segs = linkify(s.text);
        if (segs.length === 1 && !segs[0].url) {
          return (
            <Text key={j} style={base}>
              {s.text}
            </Text>
          );
        }
        return (
          <Text key={j} style={base}>
            {segs.map((seg, k) =>
              seg.url ? (
                <Text key={k} onPress={() => Linking.openURL(seg.url!)} style={styles.link}>
                  {seg.text}
                </Text>
              ) : (
                <Text key={k}>{seg.text}</Text>
              ),
            )}
          </Text>
        );
      });
  if (block) {
    return (
      <Text
        style={[styles.mono, {fontSize, color}]}
        suppressHighlighting
        onLongPress={onLongPressLine ? () => onLongPressLine(index) : undefined}>
        {spans.length ? body : ' '}
      </Text>
    );
  }
  return (
    <Text>
      {body}
      {last ? '' : '\n'}
    </Text>
  );
});

export function NativeTerm({text, fontSize = 12, cursor, theme, lang = 'en', onLiveEdge}: Props) {
  const bg = theme?.background || DEF_BG;
  const fg = theme?.foreground || DEF_FG;
  const curColor = theme?.cursor || '#bbc1ff';
  const insets = useSafeAreaInsets();
  const ref = useRef<ScrollView>(null);
  const stick = useRef(true); // follow the bottom unless the user scrolled up
  const frozen = useRef(false);
  const pending = useRef<{text: string; cursor?: PaneCursor} | null>(null);
  const thawTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // `shown`/`shownCursor` are the snapshot actually rendered. While the user is
  // TOUCHING the pane — scrolling OR holding a text selection — we FREEZE BOTH: a
  // working pane streams a new snapshot AND a moving cursor every poll, and either
  // one changing re-renders the <Text> tree, which both hitches a scroll AND wipes
  // an in-progress selection (native selection can't survive a re-render). The
  // cursor matters as much as the text here — it ticks every second on a busy pane,
  // so freezing only the text still let the selection get cleared. We stay frozen a
  // few seconds after release so the selection lives long enough to Copy.
  const [shown, setShown] = useState(text);
  const [shownCursor, setShownCursor] = useState(cursor);
  useEffect(() => {
    if (frozen.current) pending.current = {text, cursor};
    else {
      setShown(text);
      setShownCursor(cursor);
    }
  }, [text, cursor]);
  const freeze = () => {
    frozen.current = true;
    if (thawTimer.current) {
      clearTimeout(thawTimer.current);
      thawTimer.current = null;
    }
  };
  const flushPending = () => {
    frozen.current = false;
    if (thawTimer.current) {
      clearTimeout(thawTimer.current);
      thawTimer.current = null;
    }
    if (pending.current !== null) {
      const p = pending.current;
      pending.current = null;
      setShown(p.text);
      setShownCursor(p.cursor);
    }
  };
  const thawSoon = () => {
    if (thawTimer.current) clearTimeout(thawTimer.current);
    thawTimer.current = setTimeout(flushPending, 3500);
  };
  // End-of-scroll thaw: if the gesture ended AT the bottom, flush the latest
  // snapshot IMMEDIATELY and resume following live (you scrolled down to see the
  // newest output — no selection to protect there). If it ended scrolled UP, keep
  // the snapshot frozen a few seconds (a held selection / reading history). Without
  // this, scrolling to the bottom while a working pane streamed left you stuck on a
  // stale frame, unable to reach the live tail.
  const thawByPosition = () => {
    if (stick.current) flushPending();
    else thawSoon();
  };
  useEffect(() => () => {
    if (thawTimer.current) clearTimeout(thawTimer.current);
  }, []);

  // Render only the last MAX_LINES of the capture (capture-pane returns up to ~2000
  // lines of scrollback; one big selectable <Text> of that many nested spans is
  // heavy enough to hitch a working pane's re-render and, at the extreme, crash the
  // app). The bottom is preserved, so the cursor's bottom-relative `up` still maps.
  // Parsed through the per-line cache: a line whose raw text is unchanged keeps its
  // exact span-array identity across polls, so the memoized <TermLine> rows below
  // bail — an in-place repaint (a spinner/footer tick on a busy pane) re-parses and
  // re-renders only the rows that actually changed (see termLineCache.ts).
  const cacheRef = useRef(makeLineCache());
  const lines = useMemo(() => {
    const nl = normalizeGlyphs(shown).split('\n');
    const capped = nl.length > MAX_LINES ? nl.slice(nl.length - MAX_LINES) : nl;
    return parseLinesCached(capped, {palette: theme?.palette, base: fg, bg: true}, cacheRef.current);
  }, [shown, theme?.palette, fg]);

  // place the cursor: capture-pane ends rows with "\n" (trailing empty line), and
  // `up` = rows above the bottom content line.
  const rendered = useMemo(() => {
    if (!shownCursor || shownCursor.visible === false) return lines;
    let last = lines.length - 1;
    if (last > 0 && lines[last].length === 0) last--; // skip the trailing-newline blank
    const row = Math.max(0, Math.min(last, last - (shownCursor.up | 0)));
    const copy = lines.slice();
    copy[row] = cursorSpans(copy[row] || [], shownCursor.x | 0, curColor, bg);
    return copy;
  }, [lines, shownCursor, curColor, bg]);

  // ANDROID-ONLY (iOS has no standing overlay — links tap through to the color layer
  // directly, selection lives in the select sheet): the transparent overlay is the TOP
  // layer and the ONLY one that receives touches there, so EVERY link the color layer
  // draws must be tappable on it — both a bare URL (linkify) and an OSC 8 hyperlink
  // (span.href). Built from the same `lines` spans as plainText so the flattened text
  // still equals plainText exactly (same wrapping/alignment). Cursor cell excluded.
  const plainText = useMemo(
    () => (Platform.OS === 'android' ? lines.map(spans => spans.map(s => s.text).join('')).join('\n') : ''),
    [lines],
  );
  const overlaySegs = useMemo(() => (Platform.OS === 'android' ? linkSegsForLines(lines) : []), [lines]);
  const overlayHasLink = useMemo(() => overlaySegs.some(s => s.url), [overlaySegs]);

  // atBottom drives the jump-to-bottom FAB (a ref can't re-render); stick keeps the
  // follow-live behavior. Both track the same "near the tail" test.
  const [atBottom, setAtBottom] = useState(true);
  // Last-seen scroll metrics — the select sheet estimates the first VISIBLE line
  // from them so its content can line up with what's on screen (in-place morph).
  const viewport = useRef({y: 0, contentH: 0});
  const onScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    viewport.current = {y: contentOffset.y, contentH: contentSize.height};
    const bottom = contentSize.height - contentOffset.y - layoutMeasurement.height < 40;
    stick.current = bottom;
    setAtBottom(bottom);
    onLiveEdge?.(bottom);
  };
  // Snap back to the live tail: resume following, flush any frozen snapshot (so you
  // land on the newest output, not a stale frame), and scroll down.
  const jumpToBottom = () => {
    stick.current = true;
    setAtBottom(true);
    onLiveEdge?.(true);
    flushPending();
    ref.current?.scrollToEnd({animated: true});
  };
  const onContentSizeChange = (_w: number, h: number) => {
    viewport.current.contentH = h;
    if (stick.current) ref.current?.scrollToEnd({animated: false});
  };

  // iOS select sheet — an IN-PLACE morph, not a jump: the sheet's content starts at
  // the terminal's first VISIBLE line and its text is offset to the terminal's own
  // on-screen position (measureInWindow), the modal swaps in with NO animation, and
  // the long-pressed line opens pre-selected. Visually the terminal just "freezes
  // into selection mode" with the band appearing under your finger — the earlier
  // put-the-pressed-line-at-top version yanked the whole screen and read as a jarring
  // jump. linesRef + [] deps keep the callback identity STABLE so the memoized
  // TermLine blocks never re-render just because this component did.
  const [selText, setSelText] = useState<string | null>(null);
  const [selRange, setSelRange] = useState<{start: number; end: number} | undefined>(undefined);
  const [selTop, setSelTop] = useState(0);
  const pendingSel = useRef<{start: number; end: number} | undefined>(undefined);
  const selInputRef = useRef<TextInput>(null);
  const rootRef = useRef<View>(null);
  const linesRef = useRef(lines);
  linesRef.current = lines;
  // The pre-selection is NOT set at mount: a controlled `selection` races the fresh
  // UITextView (native reports its own initial selection and clobbers ours — the
  // band landed on the wrong text). It's applied in onShow, after the view exists.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const openSelectAt = useCallback((touched: number) => {
    const all = linesRef.current;
    const n = all.length;
    if (n === 0) return;
    const line = Math.max(0, Math.min(n - 1, touched));
    // First visible line (proportional estimate — lines are near-uniform height);
    // never below the pressed line so the selection is always inside the slice.
    const {y, contentH} = viewport.current;
    let first = contentH > 0 ? Math.floor((y / contentH) * n) : Math.max(0, n - 40);
    first = Math.max(0, Math.min(line, first));
    const plain = all.slice(first).map(spans => spans.map(s => s.text).join(''));
    let off = 0;
    for (let i = 0; i < line - first; i++) off += plain[i].length + 1;
    pendingSel.current = {start: off, end: off + (plain[line - first]?.length ?? 0)};
    setSelRange(undefined); // uncontrolled at mount — see the race note above
    const open = () => {
      setSelText(plain.join('\n'));
      freeze();
    };
    // Align the sheet's text with the terminal's on-screen position so the morph
    // is seamless; fall back to opening unaligned if the measure never fires.
    const node = rootRef.current;
    if (node?.measureInWindow) {
      node.measureInWindow((_x, winY) => {
        setSelTop(Math.max(0, winY));
        open();
      });
    } else {
      setSelTop(0);
      open();
    }
  }, []);
  const onSelectShown = () => {
    selInputRef.current?.focus(); // first responder → the band + handles render
    setTimeout(() => setSelRange(pendingSel.current), 120);
  };
  const closeSelect = () => {
    setSelText(null);
    setSelRange(undefined);
    pendingSel.current = undefined;
    thawSoon();
  };

  // LAYERS. iOS: a STACK of per-line <Text> blocks (see TermLine's block shape) —
  // the only always-on layer; link taps land on it directly, and each block carries
  // the long-press that opens the select sheet at exactly that line. Android: the
  // single paragraph its FLAT transparent <Text selectable> overlay aligns to — the
  // overlay draws the selection band properly there and carries the link taps.
  const colorLayer =
    Platform.OS === 'ios' ? (
      <View>
        {rendered.map((spans, i) => (
          <TermLine key={i} spans={spans} last block index={i} fontSize={fontSize} color={fg} onLongPressLine={openSelectAt} />
        ))}
      </View>
    ) : (
      <Text style={[styles.mono, {fontSize, color: fg}]}>
        {rendered.map((spans, i) => (
          <TermLine key={i} spans={spans} last={i === rendered.length - 1} />
        ))}
      </Text>
    );

  return (
    <View ref={rootRef} style={[styles.fill, {backgroundColor: bg}]}>
      {/* Always wrap to the phone width (character-grid stays aligned in monospace).
          A no-wrap + horizontal-scroll mode is a later addition — a horizontal
          ScrollView nested in this vertical one collapses/blank-renders on iOS. */}
      <ScrollView
        ref={ref}
        style={styles.fill}
        contentContainerStyle={styles.pad}
        onScroll={onScroll}
        onTouchStart={freeze}
        onTouchEnd={thawSoon}
        onTouchCancel={thawSoon}
        onScrollBeginDrag={freeze}
        onScrollEndDrag={thawByPosition}
        onMomentumScrollEnd={thawByPosition}
        scrollEventThrottle={16}
        onContentSizeChange={onContentSizeChange}>
        <View style={styles.layerWrap}>
          {colorLayer}
          {Platform.OS === 'android' && (
            /* Android-only: FLAT selectable Text draws the band properly there;
               nested Text keeps OSC 8 / bare-URL taps. iOS has NO standing overlay
               — selection lives in the long-press select sheet. */
            <Text selectable selectionColor={SELECTION_TINT} style={[styles.mono, styles.overlay, {fontSize, color: 'transparent'}]}>
              {!overlayHasLink
                ? plainText
                : overlaySegs.map((seg, i) =>
                    seg.url ? (
                      <Text key={i} onPress={() => Linking.openURL(seg.url!)}>
                        {seg.text}
                      </Text>
                    ) : (
                      seg.text
                    ),
                  )}
            </Text>
          )}
        </View>
      </ScrollView>
      <JumpToBottom visible={!atBottom} onPress={jumpToBottom} />
      {selText != null && (
        /* iOS select sheet: an in-place morph — no animation, content aligned to the
           terminal's on-screen position (selTop), first line = first visible line —
           so the band appears under your finger instead of the screen jumping. The
           ONLY visible layer while up, so UITextView's own layout is the truth; the
           snapshot is frozen, so polls can't yank it mid-selection. */
        <Modal visible animationType="none" onRequestClose={closeSelect} onShow={onSelectShown}>
          <View style={[styles.fill, {backgroundColor: bg}]}>
            {/* Opens with the long-pressed line pre-selected (`selection`); focus on
                show makes UITextView first responder so the band + handles render.
                onSelectionChange keeps the controlled prop following the user's drag
                instead of fighting it. editable=false → no keyboard. */}
            <TextInput
              ref={selInputRef}
              multiline
              editable={false}
              scrollEnabled
              caretHidden
              autoCorrect={false}
              spellCheck={false}
              selectionColor={SELECTION_TINT}
              selection={selRange}
              onSelectionChange={e => setSelRange(e.nativeEvent.selection)}
              value={selText}
              style={[styles.mono, styles.selInput, {fontSize, color: fg, backgroundColor: bg, marginTop: selTop}]}
            />
            <TouchableOpacity
              onPress={closeSelect}
              hitSlop={{top: 10, bottom: 10, left: 12, right: 12}}
              style={[styles.selDoneChip, {bottom: Math.max(16, insets.bottom + 10)}]}>
              <Text style={styles.selDone}>{lang === 'zh' ? '完成' : 'Done'}</Text>
            </TouchableOpacity>
          </View>
        </Modal>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  pad: {padding: 6},
  mono: {fontFamily: MONO},
  // Auto-detected URL: inherits the span's terminal color/weight, adds the underline
  // that marks it tappable (mirrors the OSC 8 hyperlink treatment).
  link: {textDecorationLine: 'underline'},
  // color layer (bottom) defines the height; the transparent selectable layer is
  // absolutely overlaid on top, same width/font → same wrapping → exact alignment.
  layerWrap: {position: 'relative', overflow: 'visible'},
  overlay: {position: 'absolute', top: 0, left: 0, right: 0},
  // iOS select sheet chrome. selInput's padding mirrors the terminal's content
  // padding (styles.pad = 6) so the morphed text sits where the terminal's did.
  selDone: {fontSize: 15, fontWeight: '600', color: '#3478F7'},
  selDoneChip: {
    position: 'absolute',
    right: 16,
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 16,
    backgroundColor: 'rgba(120,120,128,0.24)',
  },
  selInput: {flex: 1, paddingHorizontal: 6, paddingTop: 6},
});
