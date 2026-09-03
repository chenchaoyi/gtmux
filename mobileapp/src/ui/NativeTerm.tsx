// NativeTerm — a READ-ONLY terminal-pane renderer built from native RN <Text>,
// not a webview/xterm.js. It renders tmux `capture-pane -e` snapshots (already a
// flat, resolved screen + SGR color — NOT a live VT stream, so no terminal
// emulator is needed) via the shared ANSI parser (src/ui/ansi.ts).
//
// Why native instead of xterm-in-webview:
//   • a tap doesn't focus a hidden <textarea> → NO soft-keyboard pop-up;
//   • long-press gives REAL range selection. iOS history (device-confirmed
//     2026-08-07): RN <Text selectable> is menu-only there (no band/handles, Copy
//     grabs the WHOLE buffer), an always-on transparent UITextView overlay both
//     MISALIGNED (its TextKit layout drifts from the Text layer's on CJK/wrapped
//     lines) and JANKED (full relayout of the 1000-line buffer on every poll),
//     and the interim full-screen/in-place select sheets each broke a different
//     way (auto-scroll drift / down-only selection). So iOS now mounts a NATIVE
//     UITextInput overlay (mobile-native-term-selection Stage 2,
//     ios/TermSelection/): a transparent view over the Stage 1 uniform row grid
//     that supplies row/char geometry (row = y ÷ rowHeight, x via Core Text
//     advances on the SAME font) to the system selection machinery — the
//     system band + both-direction lollipop handles + loupe draw directly over
//     our colored rendering, Copy comes from the system edit menu, and the
//     view passes every touch through until a long-press activates it
//     (links/scroll unaffected, zero standing cost). Android keeps the
//     transparent <Text selectable> overlay, which selects properly there;
//   • native ScrollView momentum (no DOM/canvas repaint jank);
//   • no WebGL/canvas/DOM renderer fragility (the ~10-PR webview saga);
//   • pure JS → the same renderer works on iOS AND Android.
// Input still flows through the native Composer (POST /api/send) — display-only here.
//
// Fidelity notes: capture-pane has already resolved cursor moves / clears / alt-
// screen into a flat colored grid, so we only handle SGR (fg+bg, bold/dim, 256 /
// truecolor) + the pane's text cursor (drawn as a reverse-video cell). Monospace
// alignment + CJK width rely on the system monospace (Menlo → PingFang fallback).

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Linking, NativeScrollEvent, NativeSyntheticEvent, Platform, ScrollView, StyleProp, StyleSheet, Text, View, ViewStyle, requireNativeComponent, useWindowDimensions} from 'react-native';
import {Lang} from '../i18n';
import {JumpToBottom} from './JumpToBottom';
import {Debug} from '../debug';
import {AnsiLine} from './ansi';
import {PAD, colsFor, cursorSpans, flattenGrid, linkify, linkSegsForLines, nativeFontFamily, normalizeGlyphs, renderView, rowHeightFor, tapTarget} from './term';
import {makeLineCache, parseLinesCached, wrapLinesCached} from './termLineCache';
import {TermTheme} from '../api/types';

// The Stage 2 native selection overlay (iOS only; ios/TermSelection/). A
// transparent view mounted absoluteFill over the block stack that implements a
// read-only UITextInput from the props below — the system selection UI (band +
// handles + loupe + edit menu) then draws over our colored rows. It is
// registered as a legacy view manager (`TermSelectionViewManager`), which RN's
// new-arch interop layer hosts.
interface TermSelectionProps {
  style?: StyleProp<ViewStyle>;
  pointerEvents?: 'box-none';
  text: string; // the grid's PLAIN text: visual rows joined by '\n' (raw, not cursor-spliced)
  rowHeight: number; // rowHeightFor(fontSize) — row i's top edge is i × rowHeight
  fontSize: number;
  fontName: string;
  padTop: number; // overlay-relative offsets of row (0,0) — 0: it shares the
  padLeft: number; // block stack's origin inside layerWrap (PAD sits outside)
  // "#RRGGBB" — a plain hex STRING parsed natively (a processColor number
  // arrives byte-swapped through the new-arch interop layer: ARGB read as RGBA)
  selectionTint?: string;
  menuLang: string; // 'en' | 'zh' — the edit menu's Copy label
  onSelectionActive?: (e: NativeSyntheticEvent<{active: boolean}>) => void;
}
const TermSelection = Platform.OS === 'ios' ? requireNativeComponent<TermSelectionProps>('TermSelectionView') : null;

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
  lang?: Lang; // for the iOS selection overlay's edit menu (Copy / 拷贝)
  onLiveEdge?: (atBottom: boolean) => void; // hide/show host chrome as you leave/return to the live tail
}

// The terminal surface's default (always-dark) background. Exported so the
// Detail full-screen path can paint its safe-area backdrop the SAME dark —
// regardless of the app theme — instead of showing a light band above the pane.
export const TERM_BG = '#17171a';
const DEF_BG = TERM_BG;
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

// Selection tint. iOS: the native selection overlay's tintColor — the system paints
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
//   • block (iOS) — ONE VISUAL ROW of the uniform grid as its own top-level <Text>,
//     stacked in a View. Since Stage 1 of mobile-native-term-selection, blocks are
//     pre-wrapped visual rows (cell arithmetic in JS — see wrapLine) with an explicit
//     uniform `lineHeight` (rowHeightFor), so a row can never native-wrap and CJK
//     rows are the same height as Latin ones: row geometry is y = index * rowH, pure
//     arithmetic (what Stage 2's native selection layer consumes). A changed row
//     re-measures ONLY its own block natively; the old single-paragraph form re-ran
//     TextKit over the whole 1000-line buffer on every poll of a working pane, and
//     that main-thread stall is exactly what made Composer typing hitch. Only
//     possible because the standing overlay (the Stage 2 selection layer) needs
//     ROW-exact geometry, not paragraph-exact TextKit alignment. An empty row
//     renders ' ' (an empty block collapses to zero height). All block props are
//     per-poll-stable so memo still bails.
//   • nested (Android) — a child of the single paragraph its transparent selectable
//     overlay aligns to; `last` carries the joining newline so the flattened text
//     still equals the overlay's plainText exactly. No lineHeight/wrap here —
//     Android's path is untouched by Stage 1 (its selection works as-is).
const TermLine = React.memo(function TermLine({
  spans,
  last,
  block,
  fontSize,
  lineHeight,
  color,
}: {
  spans: AnsiLine;
  last: boolean;
  block?: boolean;
  fontSize?: number;
  lineHeight?: number;
  color?: string;
}) {
  const body = spans.map((s, j) => {
        const base = {
          color: s.color,
          backgroundColor: s.bg,
          fontWeight: (s.bold ? '700' : '400') as '700' | '400',
        };
        // A span carrying a link target is ONE link — underlined + tappable (opens in
        // the browser). Two sources, one behavior: an OSC 8 hyperlink the agent declared
        // (a non-web href, e.g. the Mac's file:// image refs, stays clean text), and a
        // bare URL annotateUrls tagged on the LOGICAL line before the grid wrapped it.
        // The second is why a URL the wrap cut in two still opens whole from either
        // half — this layer only ever sees the already-wrapped row.
        const tap = tapTarget(s);
        if (tap) {
          return (
            <Text key={j} onPress={() => Linking.openURL(tap)} style={{...base, textDecorationLine: 'underline'}}>
              {s.text}
            </Text>
          );
        }
        // Fallback for spans that did not come through the line cache (so were never
        // annotated): detect a bare URL within this span alone. Cannot see past the
        // span, which is exactly the limitation annotateUrls exists to remove. The
        // common no-URL line renders as a single <Text> (fast path).
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
    // allowFontScaling OFF: the iOS grid honors Dynamic Type by folding the OS
    // fontScale into the fontSize/lineHeight props themselves (NativeTerm's
    // `fs`) — letting RN scale them AGAIN would grow the rendered pitch past
    // rowH and break the row = y ÷ rowH invariant the selection overlay's
    // geometry (and the bottom rows' selectability) depends on.
    return (
      <Text style={[styles.mono, {fontSize, lineHeight, color}]} suppressHighlighting allowFontScaling={false}>
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
  // TRUE while the native selection overlay holds an active selection. A flush
  // while it's held would swap the text under the band (the native layer clears
  // its selection when the text prop changes), so flushPending refuses to run
  // until the overlay reports the selection cleared.
  const selActive = useRef(false);
  const freeze = () => {
    frozen.current = true;
    if (thawTimer.current) {
      clearTimeout(thawTimer.current);
      thawTimer.current = null;
    }
  };
  const flushPending = () => {
    if (selActive.current) return; // never clobber a held selection
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

  // iOS uniform grid geometry (mobile-native-term-selection Stage 1). The window
  // width stands in for the terminal's width — the pane spans the screen — and
  // colsFor bakes in the content padding (−12) plus the conservative −1 cell so a
  // pre-wrapped row can NEVER native-wrap (the grid invariant Stage 2's selection
  // geometry depends on). rowH is the explicit uniform lineHeight every block row
  // gets: CJK rows and Latin rows render the SAME height, so a row's y is
  // index * rowH, pure arithmetic. A rotation/font change just recomputes both
  // (and invalidates the wrap cache via its signature).
  //
  // `fs` is the EFFECTIVE font size — the fontSize prop × the OS Dynamic Type
  // scale — and it is the ONLY size the iOS grid path may consume. RN Text
  // scales fontSize AND lineHeight by fontScale (allowFontScaling defaults
  // true), so a device set above Large rendered a taller/wider stack than the
  // unscaled arithmetic predicted: the real row pitch beat rowH, scaled glyphs
  // overflowed the computed cols into native wraps, and every press below
  // rows×rowH clamped to the trailing blank row — the REAL-DEVICE "bottom
  // screens don't select" dead zone (a clean boundary, nothing below it; the
  // 1.0-scale sim couldn't reproduce it). Folding the scale in here keeps the
  // user's Dynamic Type size EXACTLY as rendered before while every consumer —
  // wrap cols, row height, the block rows' own font, the overlay's CTLine
  // font — derives from the same scaled number; the block rows then set
  // allowFontScaling={false} so RN cannot apply the scale a SECOND time.
  // useWindowDimensions makes a Settings change re-derive everything live.
  const {width: winW, fontScale} = useWindowDimensions();
  const fs = Platform.OS === 'ios' ? fontSize * (fontScale || 1) : fontSize;
  const cols = Platform.OS === 'ios' ? colsFor(winW, fs) : 0;
  const rowH = rowHeightFor(fs);

  // Render only the last MAX_LINES of the capture (capture-pane returns up to ~2000
  // lines of scrollback; one big selectable <Text> of that many nested spans is
  // heavy enough to hitch a working pane's re-render and, at the extreme, crash the
  // app). The bottom is preserved, so the cursor's bottom-relative `up` still maps.
  // Parsed through the per-line cache: a line whose raw text is unchanged keeps its
  // exact span-array identity across polls, so the memoized <TermLine> rows below
  // bail — an in-place repaint (a spinner/footer tick on a busy pane) re-parses and
  // re-renders only the rows that actually changed (see termLineCache.ts). On iOS
  // the same cache ALSO holds each line's hard-wrapped visual rows for the current
  // grid (cols × fontSize), identity-stable for unchanged lines too.
  const cacheRef = useRef(makeLineCache());
  const parsed = useMemo(() => {
    const nl = normalizeGlyphs(shown).split('\n');
    const capped = nl.length > MAX_LINES ? nl.slice(nl.length - MAX_LINES) : nl;
    const opts = {palette: theme?.palette, base: fg, bg: true};
    if (Platform.OS === 'ios') return wrapLinesCached(capped, opts, {cols, fontSize: fs}, cacheRef.current);
    return {lines: parseLinesCached(capped, opts, cacheRef.current), rows: null};
  }, [shown, theme?.palette, fg, cols, fs]);
  const lines = parsed.lines;

  // Trim the trailing all-blank rows from the RENDER and place the cursor.
  // capture-pane keeps the pane grid's blank tail (the bottom-anchored cursor
  // math needs the row count), but rendering it pinned a mostly-empty tall
  // pane's few content lines above the viewport — the all-black-pane bug. So
  // renderView computes the cursor's logical line on the UNTRIMMED array
  // (capture-pane ends rows with "\n" — trailing empty line — and `up` counts
  // LOGICAL lines, not visual rows) and how many leading lines to keep: down to
  // the last visible content, never cutting the cursor's own row. The spliced
  // line is wrapped fresh below.
  const view = useMemo(() => renderView(lines, shownCursor), [lines, shownCursor]);
  const vLines = useMemo(() => (view.keep < lines.length ? lines.slice(0, view.keep) : lines), [lines, view.keep]);
  const vRows = useMemo(
    () => (parsed.rows ? (view.keep < parsed.rows.length ? parsed.rows.slice(0, view.keep) : parsed.rows) : null),
    [parsed.rows, view.keep],
  );
  const rendered = useMemo(() => {
    if (view.cursorRow < 0 || !shownCursor) return vLines;
    const copy = vLines.slice();
    copy[view.cursorRow] = cursorSpans(copy[view.cursorRow] || [], shownCursor.x | 0, curColor, bg);
    return copy;
  }, [vLines, view.cursorRow, shownCursor, curColor, bg]);

  // iOS: flatten logical lines into the VISUAL ROWS the block stack renders (row
  // index = grid row). Unchanged lines reuse their cached rows arrays (per-row
  // memo identity); ONLY the cursor-spliced line — recognizable because its spans
  // identity differs from the cached logical line — is wrapped fresh each render
  // (one line, cheap). Keys are lineIdx-rowIdx: stable for a line whose wrap
  // count changes above others (in-place repaint keeps keys aligned).
  //
  // Alongside the rows it builds the selection overlay's TEXT (see flattenGrid
  // in term.ts — extracted so the overlay-rows == stack-rows invariant is
  // unit-tested against the real builder).
  const grid = useMemo(
    () => (Platform.OS === 'ios' ? flattenGrid(rendered, vLines, vRows, cols) : null),
    [rendered, vLines, vRows, cols],
  );

  // ANDROID-ONLY (iOS has no standing overlay — links tap through to the color layer
  // directly, selection lives in the select sheet): the transparent overlay is the TOP
  // layer and the ONLY one that receives touches there, so EVERY link the color layer
  // draws must be tappable on it — both a bare URL (linkify) and an OSC 8 hyperlink
  // (span.href). Built from the same `lines` spans as plainText so the flattened text
  // still equals plainText exactly (same wrapping/alignment). Cursor cell excluded.
  const plainText = useMemo(
    () => (Platform.OS === 'android' ? vLines.map(spans => spans.map(s => s.text).join('')).join('\n') : ''),
    [vLines],
  );
  const overlaySegs = useMemo(() => (Platform.OS === 'android' ? linkSegsForLines(vLines) : []), [vLines]);
  const overlayHasLink = useMemo(() => overlaySegs.some(s => s.url), [overlaySegs]);

  // atBottom drives the jump-to-bottom FAB (a ref can't re-render); stick keeps the
  // follow-live behavior. Both track the same "near the tail" test.
  const [atBottom, setAtBottom] = useState(true);
  // TEMPORARY instrumentation (jump-to-bottom investigation) — remove before merging.
  // True while the user's own finger is driving the scroll (a drag, and the momentum it
  // hands off to). Programmatic scrolls never set it, which is what keeps them from
  // cancelling the follow they are performing.
  const dragging = useRef(false);
  const beginDrag = () => {
    dragging.current = true;
    freeze();
  };
  const endDrag = () => {
    dragging.current = false;
    thawByPosition();
  };
  const probe = useRef(Debug.logNet);
  // Route through the harness's own recorder (Documents/gtmux-debug.jsonl, read back by
  // readDebugLog) rather than console: a `log stream` on the simulator kept being
  // group-killed with the Appium server and lost every line.
  const say = (m: string, extra: Record<string, unknown> = {}) => {
    if (probe.current) Debug.record({event: 'termprobe', at: m, ...extra});
  };
  const onScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    const gap = contentSize.height - contentOffset.y - layoutMeasurement.height;
    say('scroll', {gap: +gap.toFixed(1), content: +contentSize.height.toFixed(1), off: +contentOffset.y.toFixed(1), view: +layoutMeasurement.height.toFixed(1), stick: stick.current});
    const bottom = gap < 40;
    // Following the live tail is the USER's intent, so only the user may withdraw it.
    //
    // This used to be a plain `stick.current = bottom`, which let a PROGRAMMATIC scroll
    // cancel the very follow it was performing. Measured on the simulator: tapping
    // jump-to-bottom set stick=true, and the animation's own first onScroll — reporting
    // gap=1046 while it was still a thousand points from the tail — set it straight back
    // to false. It stayed false for the whole flight, so when the content grew (and when
    // the viewport resized under the collapsing header, which the same trace shows moving
    // 692.7 → 684.3 → 691.3 mid-scroll) nothing re-pinned, and the view came to rest
    // short of the tail. That is the reported "the arrow doesn't quite get me back".
    //
    // Arriving at the bottom always re-sticks, whoever caused it.
    if (bottom) {
      stick.current = true;
    } else if (dragging.current) {
      stick.current = false;
    }
    setAtBottom(bottom);
    onLiveEdge?.(bottom);
  };
  // Snap back to the live tail: resume following, flush any frozen snapshot (so you
  // land on the newest output, not a stale frame), and scroll down.
  const jumpToBottom = () => {
    stick.current = true;
    setAtBottom(true);
    onLiveEdge?.(true);
    say('jump', {pending: pending.current !== null, frozen: frozen.current});
    flushPending();
    ref.current?.scrollToEnd({animated: true});
  };
  const onContentSizeChange = (_w: number, h: number) => {
    say('contentSize', {h: +h.toFixed(1), stick: stick.current});
    if (stick.current) ref.current?.scrollToEnd({animated: false});
  };

  // iOS selection — the native overlay reports activation so JS can freeze the
  // snapshot under it (a text-prop change while a selection is held would clear
  // the band). The overlay itself owns ALL selection behavior natively: a
  // long-press (recognized on the enclosing scroll view, so the pass-through
  // overlay never blocks links or scrolling) selects the word under the finger,
  // the system draws the band + both-direction handles + loupe + edit menu over
  // the colored rows, Copy writes the exact banded range, tap-away clears.
  const onSelectionActive = (e: NativeSyntheticEvent<{active: boolean}>) => {
    const active = !!e.nativeEvent.active;
    selActive.current = active;
    if (active) freeze();
    else thawSoon();
  };

  // LAYERS. iOS: a STACK of per-ROW <Text> blocks (see TermLine's block shape) —
  // the uniform grid: pre-wrapped visual rows at an explicit shared lineHeight, the
  // only always-on layer; link taps land on it directly (the selection overlay
  // above it passes touches through until a long-press activates it). Android:
  // the single paragraph its FLAT transparent <Text selectable> overlay aligns to —
  // the overlay draws the selection band properly there and carries the link taps.
  const colorLayer =
    Platform.OS === 'ios' ? (
      <View>
        {grid!.rows.map(r => (
          <TermLine key={r.key} spans={r.spans} last block fontSize={fs} lineHeight={rowH} color={fg} />
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
    <View style={[styles.fill, {backgroundColor: bg}]}>
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
        onScrollBeginDrag={beginDrag}
        onScrollEndDrag={endDrag}
        onMomentumScrollEnd={endDrag}
        scrollEventThrottle={16}
        onContentSizeChange={onContentSizeChange}>
        <View style={styles.layerWrap}>
          {colorLayer}
          {Platform.OS === 'ios' && TermSelection && grid && (
            /* The Stage 2 native selection layer, absoluteFill over the block
               stack (same origin — row i's top edge is exactly i × rowH in its
               coordinates, so padTop/padLeft are 0; PAD lives OUTSIDE layerWrap
               on the scroll content). pointerEvents box-none + the native
               point(inside:) gate make it invisible to touches until its
               long-press activates a selection. */
            <TermSelection
              style={StyleSheet.absoluteFill}
              pointerEvents="box-none"
              text={grid.text}
              rowHeight={rowH}
              fontSize={fs}
              fontName={MONO}
              padTop={0}
              padLeft={0}
              selectionTint={SELECTION_TINT}
              menuLang={lang}
              onSelectionActive={onSelectionActive}
            />
          )}
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
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  pad: {padding: PAD}, // couples to colsFor's usable-width math — change PAD in term.ts
  mono: {fontFamily: MONO},
  // Auto-detected URL: inherits the span's terminal color/weight, adds the underline
  // that marks it tappable (mirrors the OSC 8 hyperlink treatment).
  link: {textDecorationLine: 'underline'},
  // color layer (bottom) defines the height; the transparent selectable layer is
  // absolutely overlaid on top, same width/font → same wrapping → exact alignment.
  layerWrap: {position: 'relative', overflow: 'visible'},
  overlay: {position: 'absolute', top: 0, left: 0, right: 0},
});
