// NativeTerm — a READ-ONLY terminal-pane renderer built from native RN <Text>,
// not a webview/xterm.js. It renders tmux `capture-pane -e` snapshots (already a
// flat, resolved screen + SGR color — NOT a live VT stream, so no terminal
// emulator is needed) via the shared ANSI parser (src/ui/ansi.ts).
//
// Why native instead of xterm-in-webview:
//   • a tap doesn't focus a hidden <textarea> → NO soft-keyboard pop-up;
//   • long-press gives iOS text selection + the Copy callout (web-like arbitrary
//     selection — the thing xterm can't do on touch). NOTE: a colored, deeply-nested
//     <Text selectable> selects+copies but draws NO visible highlight on-device, so
//     selection rides a separate FLAT, single-color <Text selectable> with
//     TRANSPARENT glyphs overlaid on the colored layer — the iOS highlight (its own
//     translucent layer) then tints the colors behind it (see the dual-layer body);
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
import {NativeScrollEvent, NativeSyntheticEvent, ScrollView, StyleSheet, Text, View} from 'react-native';
import {parseAnsi} from './ansi';
import {cursorSpans, nativeFontFamily, normalizeGlyphs} from './term';
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
  fontPref?: string; // accepted for API-parity with XtermView; native uses Menlo
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
const MAX_LINES = 350; // dual-layer (color + selectable overlay) makes each line
// cost twice; 350 keeps a deep-enough phone glance while cutting the mount hitch on
// a mode switch (full history lives in Chat mode). The bottom is preserved so the
// bottom-anchored cursor still maps.

export function NativeTerm({text, fontSize = 12, cursor, theme}: Props) {
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
  const lines = useMemo(() => {
    const nl = normalizeGlyphs(shown).split('\n');
    const capped = nl.length > MAX_LINES ? nl.slice(nl.length - MAX_LINES).join('\n') : nl.join('\n');
    return parseAnsi(capped, {palette: theme?.palette, base: fg, bg: true});
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

  // Plain (ANSI-stripped) text of the same capped lines — the content of the
  // transparent selection layer. Cursor cell is intentionally excluded.
  const plainText = useMemo(() => lines.map(spans => spans.map(s => s.text).join('')).join('\n'), [lines]);

  const onScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    stick.current = contentSize.height - contentOffset.y - layoutMeasurement.height < 40;
  };
  const onContentSizeChange = () => {
    if (stick.current) ref.current?.scrollToEnd({animated: false});
  };

  // Two STACKED, always-on layers solve "read in color, select with a VISIBLE
  // highlight" without any mode switch (the earlier TextInput-toggle jumped the
  // layout and only showed a cursor loupe, no real selection):
  //   • color layer (bottom) — the colored <Text>, display only, defines height.
  //   • selection layer (top) — a FLAT, single-color <Text selectable> of the same
  //     plain text, absolutely overlaid, with TRANSPARENT glyphs. iOS draws the
  //     selection HIGHLIGHT as its own translucent layer independent of glyph color,
  //     so the blue band shows and tints the colored text behind it; Copy works
  //     (Text selectable gives the callout). Crucially the selection text is FLAT
  //     (not the per-span nested <Text> that suppresses the highlight on-device).
  // Both layers are <Text> with identical font/size/wrapping, so they align exactly
  // — no ghosting, no jump. selectionColor is translucent so colors stay readable.
  const colorLayer = (
    <Text style={[styles.mono, {fontSize, color: fg}]}>
      {rendered.map((spans, i) => (
        <Text key={i}>
          {spans.map((s, j) => (
            <Text key={j} style={{color: s.color, backgroundColor: s.bg, fontWeight: s.bold ? '700' : '400'}}>
              {s.text}
            </Text>
          ))}
          {i < rendered.length - 1 ? '\n' : ''}
        </Text>
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
        onScrollBeginDrag={freeze}
        onScrollEndDrag={thawByPosition}
        onMomentumScrollEnd={thawByPosition}
        scrollEventThrottle={16}
        onContentSizeChange={onContentSizeChange}>
        <View style={styles.layerWrap}>
          {colorLayer}
          {/* transparent selectable layer on top → the blue selection highlight
              tints the colored text behind it; Copy works; FLAT text so the
              highlight paints on-device. */}
          <Text selectable selectionColor="rgba(52,120,247,0.5)" style={[styles.mono, styles.overlay, {fontSize, color: 'transparent'}]}>
            {plainText}
          </Text>
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  pad: {padding: 6},
  mono: {fontFamily: MONO},
  // color layer (bottom) defines the height; the transparent selectable layer is
  // absolutely overlaid on top, same width/font → same wrapping → exact alignment.
  layerWrap: {position: 'relative', overflow: 'visible'},
  overlay: {position: 'absolute', top: 0, left: 0, right: 0},
});
