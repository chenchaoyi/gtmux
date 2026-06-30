// Markdown — renders the markdown.ts block/inline tree as React Native views.
// Colors are passed in (the chat surface is ALWAYS dark, so callers pass fixed
// light-on-dark colors — see the dark-surface trap). Reusable: pass a light
// palette to render on a light surface.

import React from 'react';
import {Linking, ScrollView, StyleSheet, Text, View} from 'react-native';
import {Block, Inline, parseBlocks} from './markdown';

export interface MdColors {
  text: string; // body text
  dim: string; // blockquote / hr / muted
  code: string; // inline + block code text
  codeBg: string; // code background
  border: string; // code block / blockquote border
  link: string; // links
}

interface Props {
  source: string;
  colors: MdColors;
  fontSize?: number;
  selectable?: boolean; // long-press to select + Copy (per block; chat uses this)
  // iOS won't paint the DEFAULT selection tint on nested/colored <Text selectable>
  // (the same quirk NativeTerm works around). Passing an explicit selectionColor
  // forces the highlight to render, so the chat shows a visible selection band.
  selectionColor?: string;
  // Optional font family for PROSE blocks (headings/paragraphs/lists/quotes/tables);
  // chat passes the terminal's font so the two surfaces match. Code stays monospace.
  fontFamily?: string;
}

function renderSpans(nodes: Inline[], c: MdColors, fs: number): React.ReactNode[] {
  return nodes.map((n, i) => {
    switch (n.t) {
      case 'b':
        return (
          <Text key={i} style={styles.bold}>
            {n.s}
          </Text>
        );
      case 'i':
        return (
          <Text key={i} style={styles.italic}>
            {n.s}
          </Text>
        );
      case 'code':
        return (
          <Text key={i} style={[styles.codeInline, {color: c.code, backgroundColor: c.codeBg, fontSize: fs - 1}]}>
            {' '}
            {n.s}
            {' '}
          </Text>
        );
      case 'link':
        return (
          <Text key={i} style={{color: c.link, textDecorationLine: 'underline'}} onPress={() => Linking.openURL(n.href).catch(() => {})}>
            {n.s}
          </Text>
        );
      default:
        return <Text key={i}>{n.s}</Text>;
    }
  });
}

const HEADING_SIZE: Record<number, number> = {1: 6, 2: 4, 3: 2, 4: 1, 5: 0, 6: 0};

function TableRow({
  cells,
  align,
  c,
  fs,
  header,
  sel,
  sc,
  ff,
}: {
  cells: Inline[][];
  align: import('./markdown').Align[];
  c: MdColors;
  fs: number;
  header?: boolean;
  sel?: boolean;
  sc?: string;
  ff?: string;
}) {
  return (
    <View style={styles.tr}>
      {cells.map((cell, i) => (
        <View key={i} style={[styles.td, {borderColor: c.border, backgroundColor: header ? c.codeBg : 'transparent'}]}>
          <Text
            selectable={sel}
            selectionColor={sc}
            style={{
              color: c.text,
              fontFamily: ff,
              fontSize: fs - 0.5,
              lineHeight: (fs - 0.5) * 1.4,
              fontWeight: header ? '700' : '400',
              textAlign: align[i] ?? 'left',
            }}>
            {renderSpans(cell, c, fs)}
          </Text>
        </View>
      ))}
    </View>
  );
}

function BlockView({b, c, fs, sel, sc, ff}: {b: Block; c: MdColors; fs: number; sel?: boolean; sc?: string; ff?: string}) {
  switch (b.t) {
    case 'h':
      return (
        <Text selectable={sel} selectionColor={sc} style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs + (HEADING_SIZE[b.level] ?? 0), fontWeight: '700', lineHeight: (fs + 6) * 1.3}]}>
          {renderSpans(b.spans, c, fs)}
        </Text>
      );
    case 'p':
      return (
        <Text selectable={sel} selectionColor={sc} style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{renderSpans(b.spans, c, fs)}</Text>
      );
    case 'code':
      return (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          style={[styles.codeBlock, {backgroundColor: c.codeBg, borderColor: c.border}]}
          contentContainerStyle={styles.codeBlockContent}>
          <Text selectable={sel} selectionColor={sc} style={[styles.codeBlockText, {color: c.code, fontSize: fs - 1, lineHeight: (fs - 1) * 1.4}]}>{b.text}</Text>
        </ScrollView>
      );
    case 'ul':
    case 'ol':
      return (
        <View style={styles.block}>
          {b.items.map((item, i) => (
            <View key={i} style={styles.li}>
              <Text style={[styles.bullet, {color: c.dim, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{b.t === 'ol' ? `${i + 1}. ` : '• '}</Text>
              <Text selectable={sel} selectionColor={sc} style={[styles.liText, {color: c.text, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{renderSpans(item, c, fs)}</Text>
            </View>
          ))}
        </View>
      );
    case 'quote':
      return (
        <View style={[styles.quote, {borderLeftColor: c.border}]}>
          <Text selectable={sel} selectionColor={sc} style={{color: c.dim, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45, fontStyle: 'italic'}}>{renderSpans(b.spans, c, fs)}</Text>
        </View>
      );
    case 'table':
      return (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.block}>
          <View>
            <TableRow cells={b.header} align={b.align} c={c} fs={fs} header sel={sel} sc={sc} ff={ff} />
            {b.rows.map((row, i) => (
              <TableRow key={i} cells={row} align={b.align} c={c} fs={fs} sel={sel} sc={sc} ff={ff} />
            ))}
          </View>
        </ScrollView>
      );
    case 'hr':
      return <View style={[styles.hr, {backgroundColor: c.border}]} />;
    default:
      return null;
  }
}

export function MarkdownView({source, colors, fontSize = 14, selectable, selectionColor, fontFamily}: Props) {
  const blocks = React.useMemo(() => parseBlocks(source), [source]);
  return (
    <View>
      {blocks.map((b, i) => (
        <BlockView key={i} b={b} c={colors} fs={fontSize} sel={selectable} sc={selectionColor} ff={fontFamily} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  block: {marginBottom: 8},
  bold: {fontWeight: '700'},
  italic: {fontStyle: 'italic'},
  codeInline: {fontFamily: 'Menlo', borderRadius: 3},
  codeBlock: {borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, marginBottom: 8, maxWidth: '100%'},
  codeBlockContent: {padding: 10},
  codeBlockText: {fontFamily: 'Menlo'},
  li: {flexDirection: 'row', alignItems: 'flex-start'},
  bullet: {fontVariant: ['tabular-nums']},
  liText: {flex: 1},
  quote: {borderLeftWidth: 3, paddingLeft: 10, marginBottom: 8},
  hr: {height: StyleSheet.hairlineWidth, marginVertical: 10},
  tr: {flexDirection: 'row'},
  td: {borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 8, paddingVertical: 5, minWidth: 92, justifyContent: 'center'},
});
