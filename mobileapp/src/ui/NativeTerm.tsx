// NativeTerm — a READ-ONLY terminal-pane renderer built from native RN <Text>,
// not a webview/xterm.js. It renders tmux `capture-pane -e` snapshots (already a
// flat, resolved screen + SGR color — NOT a live VT stream, so no terminal
// emulator is needed) via the shared ANSI parser (src/ui/ansi.ts).
//
// Why native instead of xterm-in-webview:
//   • a tap doesn't focus a hidden <textarea> → NO soft-keyboard pop-up;
//   • <Text selectable> gives iOS long-press selection + the Copy callout for free
//     (web-like arbitrary selection — the thing xterm can't do on touch);
//   • native ScrollView momentum (no DOM/canvas repaint jank);
//   • no WebGL/canvas/DOM renderer fragility (the ~10-PR webview saga);
//   • pure JS → the same renderer works on iOS AND Android.
// Input still flows through the native Composer (POST /api/send) — display-only here.
//
// Fidelity notes: capture-pane has already resolved cursor moves / clears / alt-
// screen into a flat colored grid, so we only handle SGR (fg+bg, bold/dim, 256 /
// truecolor) + the pane's text cursor (drawn as a reverse-video cell). Monospace
// alignment + CJK width rely on the system monospace (Menlo → PingFang fallback).

import React, {useMemo, useRef} from 'react';
import {ScrollView, StyleSheet, Text, View, NativeSyntheticEvent, NativeScrollEvent} from 'react-native';
import {AnsiLine, parseAnsi, Span} from './ansi';
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

// iOS Core Text renders U+23FA "⏺ BLACK CIRCLE FOR RECORD" (Claude Code's tool-call
// marker) as the glossy RED record-button COLOR EMOJI, ignoring the ANSI color.
// Swap it for U+25CF "● BLACK CIRCLE" (no emoji presentation) so it renders as a
// clean text glyph tinted by the surrounding SGR color — same fix as the xterm
// path (gen-xterm-asset.mjs normalizeGlyphs).
const DOT_REC = '⏺';
const DOT_CIRCLE = '●';
function normalizeGlyphs(t: string): string {
  return t.indexOf(DOT_REC) === -1 ? t : t.split(DOT_REC).join(DOT_CIRCLE);
}
// iOS system monospace (covers Latin + falls back to PingFang for CJK at 2-cell
// width). The bundled woff2 picker fonts are webview-only; native would need them
// linked as .ttf — a later follow-up, not needed for the read-only viewer.
const MONO = 'Menlo';

// cursorSpans rewrites one line's spans to paint a reverse-video block at column x
// (the pane's text cursor). Approximated on CHAR offset (the cursor is near the
// input line, ~ASCII, so char≈cell); pads with spaces when x is past the content.
function cursorSpans(spans: AnsiLine, x: number, curColor: string, bg: string): AnsiLine {
  const lineLen = spans.reduce((n, s) => n + s.text.length, 0);
  const cell = (ch: string): Span => ({text: ch || ' ', color: bg, bg: curColor});
  if (x >= lineLen) {
    const out = [...spans];
    if (x > lineLen) out.push({text: ' '.repeat(x - lineLen), color: bg});
    out.push(cell(' '));
    return out;
  }
  const out: AnsiLine = [];
  let col = 0;
  for (const s of spans) {
    const end = col + s.text.length;
    if (x < col || x >= end) {
      out.push(s);
      col = end;
      continue;
    }
    const i = x - col;
    if (i > 0) out.push({...s, text: s.text.slice(0, i)});
    out.push({...cell(s.text[i]), bold: s.bold});
    if (i + 1 < s.text.length) out.push({...s, text: s.text.slice(i + 1)});
    col = end;
  }
  return out;
}

export function NativeTerm({text, fontSize = 12, cursor, theme}: Props) {
  const bg = theme?.background || DEF_BG;
  const fg = theme?.foreground || DEF_FG;
  const curColor = theme?.cursor || '#bbc1ff';
  const ref = useRef<ScrollView>(null);
  const stick = useRef(true); // follow the bottom unless the user scrolled up

  const lines = useMemo(
    () => parseAnsi(normalizeGlyphs(text), {palette: theme?.palette, base: fg, bg: true}),
    [text, theme?.palette, fg],
  );

  // place the cursor: capture-pane ends rows with "\n" (trailing empty line), and
  // `up` = rows above the bottom content line.
  const rendered = useMemo(() => {
    if (!cursor || cursor.visible === false) return lines;
    let last = lines.length - 1;
    if (last > 0 && lines[last].length === 0) last--; // skip the trailing-newline blank
    const row = Math.max(0, Math.min(last, last - (cursor.up | 0)));
    const copy = lines.slice();
    copy[row] = cursorSpans(copy[row] || [], cursor.x | 0, curColor, bg);
    return copy;
  }, [lines, cursor, curColor, bg]);

  const lineHeight = Math.round(fontSize * 1.3);
  const onScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const {contentOffset, contentSize, layoutMeasurement} = e.nativeEvent;
    stick.current = contentSize.height - contentOffset.y - layoutMeasurement.height < 40;
  };
  const onContentSizeChange = () => {
    if (stick.current) ref.current?.scrollToEnd({animated: false});
  };

  // one big selectable <Text> → native cross-row selection + Copy. Rows are nested
  // <Text> joined by "\n"; spans carry fg/bg/bold.
  const body = (
    <Text selectable style={[styles.mono, {fontSize, lineHeight, color: fg}]}>
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
        scrollEventThrottle={100}
        onContentSizeChange={onContentSizeChange}>
        {body}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  pad: {padding: 6},
  mono: {fontFamily: MONO},
});
