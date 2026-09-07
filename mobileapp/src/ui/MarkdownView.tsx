// Markdown — renders the markdown.ts block/inline tree as React Native views.
// Colors are passed in (the chat surface is ALWAYS dark, so callers pass fixed
// light-on-dark colors — see the dark-surface trap). Reusable: pass a light
// palette to render on a light surface.

import React from 'react';
import {Linking, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
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
  // foldRows — each stacked table row is a COLLAPSIBLE card, closed by default.
  //
  // For a table whose cells are paragraphs rather than values. The situation board's
  // pane table is the case: one pane's `状态` cell alone runs to several screens, so
  // thirteen panes stacked open is a document nobody scrolls to the end of, and the
  // pane you came for is somewhere inside it. Closed, the section is thirteen rows you
  // can scan.
  //
  // Opt-in, because it is wrong for the other callers: a chat reply's table is small
  // and part of a sentence, and folding it would hide the answer.
  foldRows?: boolean;
  // calmEmphasis — for prose written with heavy emphasis. The situation board carries
  // roughly one **bold** span per line, at the same weight as a heading, which left its
  // 31 headings with no authority and the page reading as one wall. Under this flag bold
  // drops to 600 and headings gain size and air, so the hierarchy comes from the heading
  // rather than from everything else being loud. Chat does not use it — an agent's reply
  // emphasises a few words, and there 700 is right.
  calmEmphasis?: boolean;
}

function renderSpans(nodes: Inline[], c: MdColors, fs: number, calm = false): React.ReactNode[] {
  return nodes.map((n, i) => {
    switch (n.t) {
      case 'b':
        return (
          <Text key={i} style={calm ? styles.boldCalm : styles.bold}>
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
        // The chip (background + the spaces that fake its padding, since iOS ignores
        // padding on a nested <Text>) is right on the chat's dark surface, where a code
        // span is rare and wants to stand out.
        //
        // On a page of PROSE it is wrong twice over: a board line carries several spans,
        // so a light page fills with white highlighter marks — and when a span lands at a
        // line break, its leading padding-space keeps the background and renders as an
        // empty white rectangle floating at the end of the previous line. Under calm, the
        // monospace face alone says "this is a literal token".
        if (calm) {
          return (
            <Text key={i} style={[styles.codeCalm, {color: c.code, fontSize: fs - 0.5}]}>
              {n.s}
            </Text>
          );
        }
        return (
          <Text key={i} style={[styles.codeInline, {color: c.code, backgroundColor: c.codeBg, fontSize: fs - 1}]}>
            {' '}
            {n.s}
            {' '}
          </Text>
        );
      case 'del':
        return (
          <Text key={i} style={styles.del}>
            {n.s}
          </Text>
        );
      case 'br':
        // A real newline inside the Text: what the author asked for, and never the tag.
        return <Text key={i}>{'\n'}</Text>;
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
// Calm headings get MORE size, not less weight: the hierarchy has to be restored
// from the heading's side, since the body's emphasis is what drowned it.
const HEADING_SIZE_CALM: Record<number, number> = {1: 8, 2: 6, 3: 4, 4: 2, 5: 1, 6: 0};

function TableRow({
  cells,
  align,
  c,
  fs,
  header,
  sel,
  sc,
  ff,
  calm,
}: {
  cells: Inline[][];
  align: import('./markdown').Align[];
  c: MdColors;
  fs: number;
  header?: boolean;
  sel?: boolean;
  sc?: string;
  ff?: string;
  calm?: boolean;
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
            {renderSpans(cell, c, fs, calm)}
          </Text>
        </View>
      ))}
    </View>
  );
}

/** One table row, re-shaped for a narrow screen. */
export interface StackedRow {
  /** The row's first cell — what the row IS (the board puts the pane id here). */
  head: Inline[];
  /** The remaining cells, each paired with its column heading. Empty cells are dropped. */
  fields: {label: string; value: Inline[]}[];
}

/**
 * stackRows turns a wide table into one block per row.
 *
 * A phone is ~390pt wide. The situation board's table has seven columns, so horizontally
 * it renders about three and a half of them and cuts the fourth mid-word — which does not
 * read as "scroll me", it reads as broken. Stacked, every value is visible and labelled,
 * and nothing has to be dragged.
 *
 * Empty cells are dropped rather than rendered as blank rows: half the board's cells are
 * `—` or empty, and a label with nothing after it is noise.
 */
export function stackRows(header: Inline[][], rows: Inline[][][]): StackedRow[] {
  const labels = header.map(cells => cells.map(n => n.s).join('').trim());
  return rows.map(row => ({
    head: row[0] ?? [],
    fields: row
      .slice(1)
      .map((value, i) => ({label: labels[i + 1] ?? '', value}))
      .filter(f => {
        const text = f.value.map(n => n.s).join('').trim();
        return text !== '' && text !== '—' && text !== '-';
      }),
  }));
}

/**
 * rowSubtitle is what a CLOSED row shows beside its head, so the reader can find the
 * one they want without opening any of them.
 *
 * The first field, and only the first: in a stacked table the leading column after the
 * identity is the one that says WHICH thing this is (the board puts `loc` there, beside
 * the pane id). The rest are its contents, which is what folding is hiding.
 */
export function rowSubtitle(r: StackedRow): string {
  const f = r.fields[0];
  if (!f) return '';
  return f.value.map(n => n.s).join('').trim();
}

function StackedTable({b, c, fs, sel, sc, ff, calm, fold}: {b: Extract<Block, {t: 'table'}>; c: MdColors; fs: number; sel?: boolean; sc?: string; ff?: string; calm?: boolean; fold?: boolean}) {
  const rows = stackRows(b.header, b.rows);
  // Closed by default, and nothing seeds one open: unlike the board's pinned first
  // SECTION, no row here is the one you came for — the point is to see them all at once.
  const [open, setOpen] = React.useState<Set<number>>(new Set());
  const toggle = (i: number) =>
    setOpen(prev => {
      const next = new Set(prev);
      next.has(i) ? next.delete(i) : next.add(i);
      return next;
    });

  return (
    <View style={styles.block}>
      {rows.map((r, i) => {
        const shut = fold && !open.has(i);
        const head = (
          <Text
            selectable={sel && !fold}
            selectionColor={sc}
            numberOfLines={fold ? 1 : undefined}
            style={{flex: fold ? 1 : undefined, color: c.text, fontFamily: ff, fontSize: fs, fontWeight: '600', lineHeight: fs * 1.4, marginBottom: fold ? 0 : 3}}>
            {renderSpans(r.head, c, fs, calm)}
          </Text>
        );
        return (
          // A CLOSED row is a row, not a card. The card exists to hold a block of
          // labelled fields; with the fields hidden it is chrome around one short line,
          // and thirteen of them read as a pile of boxes rather than a list you can
          // scan. Open, the card comes back and says where the block begins and ends.
          <View
            key={i}
            style={[
              shut
                ? [styles.stackShut, {borderBottomColor: c.border}]
                : [styles.stackRow, {borderColor: c.border, backgroundColor: c.codeBg}],
            ]}>
            {fold ? (
              <TouchableOpacity
                testID={`md-stack-row-${i}`}
                activeOpacity={0.6}
                onPress={() => toggle(i)}
                style={styles.stackHead}>
                <Text style={[styles.stackChevron, {color: c.dim, fontSize: fs - 2}]}>{shut ? '▸' : '▾'}</Text>
                {head}
                {/* The first field rides along on the closed row: it is what tells one
                    row from another, and a column of bare ids tells you nothing. */}
                {rowSubtitle(r) !== '' && (
                  <Text numberOfLines={1} style={[styles.stackSub, {color: c.dim, fontSize: fs - 2}]}>
                    {rowSubtitle(r)}
                  </Text>
                )}
              </TouchableOpacity>
            ) : (
              head
            )}
            {!shut &&
              r.fields.map((f, j) => (
                <View key={j} style={styles.stackField}>
                  <Text style={[styles.stackLabel, {color: c.dim, fontSize: fs - 2, lineHeight: (fs - 1) * 1.4}]}>{f.label}</Text>
                  <Text selectable={sel} selectionColor={sc} style={{flex: 1, color: c.text, fontFamily: ff, fontSize: fs - 1, lineHeight: (fs - 1) * 1.4}}>
                    {renderSpans(f.value, c, fs - 1, calm)}
                  </Text>
                </View>
              ))}
          </View>
        );
      })}
    </View>
  );
}

function BlockView({b, c, fs, sel, sc, ff, calm, fold}: {b: Block; c: MdColors; fs: number; sel?: boolean; sc?: string; ff?: string; calm?: boolean; fold?: boolean}) {
  switch (b.t) {
    case 'h':
      return (
        <Text selectable={sel} selectionColor={sc} style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs + ((calm ? HEADING_SIZE_CALM : HEADING_SIZE)[b.level] ?? 0), fontWeight: '700', lineHeight: (fs + 6) * 1.3, marginTop: calm ? 10 : 0}]}>
          {renderSpans(b.spans, c, fs, calm)}
        </Text>
      );
    case 'p':
      return (
        <Text selectable={sel} selectionColor={sc} style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{renderSpans(b.spans, c, fs, calm)}</Text>
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
              <Text style={[styles.bullet, {color: c.dim, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{b.t === 'ol' ? `${b.start + i}. ` : '• '}</Text>
              <Text selectable={sel} selectionColor={sc} style={[styles.liText, {color: c.text, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>{renderSpans(item, c, fs, calm)}</Text>
            </View>
          ))}
        </View>
      );
    case 'quote':
      return (
        <View style={[styles.quote, {borderLeftColor: c.border}]}>
          <Text selectable={sel} selectionColor={sc} style={{color: c.dim, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45, fontStyle: 'italic'}}>{renderSpans(b.spans, c, fs, calm)}</Text>
        </View>
      );
    case 'table':
      // Wide tables stack on a phone (see stackRows). A table narrow enough to fit still
      // renders as a table — a two-column key/value grid reads better as one.
      if (calm && b.header.length > 3) {
        return <StackedTable b={b} c={c} fs={fs} sel={sel} sc={sc} ff={ff} calm={calm} fold={fold} />;
      }
      return (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.block}>
          <View>
            <TableRow cells={b.header} align={b.align} c={c} fs={fs} header sel={sel} sc={sc} ff={ff} calm={calm} />
            {b.rows.map((row, i) => (
              <TableRow key={i} cells={row} align={b.align} c={c} fs={fs} sel={sel} sc={sc} ff={ff} calm={calm} />
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

export function MarkdownView({source, colors, fontSize = 14, selectable, selectionColor, fontFamily, calmEmphasis, foldRows}: Props) {
  const blocks = React.useMemo(() => parseBlocks(source), [source]);
  return (
    <View>
      {blocks.map((b, i) => (
        <BlockView key={i} b={b} c={colors} fs={fontSize} sel={selectable} sc={selectionColor} ff={fontFamily} calm={calmEmphasis} fold={foldRows} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  block: {marginBottom: 8},
  bold: {fontWeight: '700'},
  // Emphasis at the SAME weight as a heading erases the hierarchy. The situation
  // board carries ~1 bold span per line (634 across 654 lines when measured), so
  // there it is 600: still emphasis, no longer a shout.
  boldCalm: {fontWeight: '600'},
  italic: {fontStyle: 'italic'},
  del: {textDecorationLine: 'line-through'},
  codeInline: {fontFamily: 'Menlo', borderRadius: 3},
  codeCalm: {fontFamily: 'Menlo'},
  codeBlock: {borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, marginBottom: 8, maxWidth: '100%'},
  codeBlockContent: {padding: 10},
  codeBlockText: {fontFamily: 'Menlo'},
  li: {flexDirection: 'row', alignItems: 'flex-start'},
  bullet: {fontVariant: ['tabular-nums']},
  liText: {flex: 1},
  quote: {borderLeftWidth: 3, paddingLeft: 10, marginBottom: 8},
  hr: {height: StyleSheet.hairlineWidth, marginVertical: 10},
  // A CARD per row, not a rule beside it. One row of the situation board's table can run
  // a dozen lines, and a 2pt left rule with a 9pt gap gave the eye nothing to find the end
  // of a ship by: two entries read as one continuous wall. Same treatment the radar's rows
  // and the long-press sheet use, so a list looks like a list wherever it appears.
  stackRow: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    paddingHorizontal: 11,
    paddingVertical: 9,
    marginBottom: 10,
  },
  stackShut: {borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 2},
  stackHead: {flexDirection: 'row', alignItems: 'center', gap: 6, paddingVertical: 6},
  stackChevron: {width: 10},
  stackSub: {flexShrink: 1, maxWidth: '55%'},
  stackField: {flexDirection: 'row', alignItems: 'flex-start', gap: 9, marginTop: 3},
  stackLabel: {minWidth: 58, fontVariant: ['tabular-nums']},
  tr: {flexDirection: 'row'},
  td: {borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 8, paddingVertical: 5, minWidth: 92, justifyContent: 'center'},
});
