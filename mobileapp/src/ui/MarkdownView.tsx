// Markdown — renders the markdown.ts block/inline tree as React Native views.
// Colors are passed in (the chat surface is ALWAYS dark, so callers pass fixed
// light-on-dark colors — see the dark-surface trap). Reusable: pass a light
// palette to render on a light surface.

import React from 'react';
import {Linking, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Block, Inline, parseBlocks} from './markdown';
import {DisclosureChevron} from './DisclosureChevron';
import {Lang} from '../i18n';

export interface MdColors {
  text: string; // body text
  dim: string; // blockquote / hr / muted
  code: string; // inline + block code text
  codeBg: string; // code background
  border: string; // code block / blockquote border
  link: string; // links
}

interface Props {
  lang?: Lang;
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
  // clampProse — a paragraph past `PROSE_CLAMP_CHARS` renders as its first few lines with
  // a tap to open it. For a surface whose author is a machine: HQ writes the situation
  // board, and one cell there was measured at ~1,180 characters — a single semicolon-joined
  // investigation log — which arrives as a wall nobody can read (user report, 2026-09-08).
  // The reader cannot fix the writing, but it must not hand the whole wall over at once.
  clampProse?: boolean;
  // calmEmphasis — for prose written with heavy emphasis. The situation board carries
  // roughly one **bold** span per line, at the same weight as a heading, which left its
  // 31 headings with no authority and the page reading as one wall. Under this flag bold
  // drops to 600 and headings gain size and air, so the hierarchy comes from the heading
  // rather than from everything else being loud. Chat does not use it — an agent's reply
  // emphasises a few words, and there 700 is right.
  calmEmphasis?: boolean;
}

// Inline code's size in calm prose, as a share of the text around it (see 'code' below).
const CODE_IN_PROSE = 0.85;

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
        //
        // At 85% of the prose: Menlo's letters are wider and taller than the system face's,
        // so at nearly the same size a token looked set in a larger, different font
        // (the user's markup of a knowledge entry, 2026-10-07). 85% is where their
        // lowercase heights meet, the ratio GitHub uses for inline code.
        if (calm) {
          return (
            <Text key={i} style={[styles.codeCalm, {color: c.code, fontSize: Math.round(fs * CODE_IN_PROSE * 2) / 2}]}>
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
/**
 * How long a paragraph may run before it is folded. Roughly a phone screen of Chinese
 * text: enough that ordinary prose is never touched, short enough that a machine-written
 * ledger is.
 */
export const PROSE_CLAMP_CHARS = 220;

/** Lines shown while a long paragraph is folded. */
const PROSE_CLAMP_LINES = 4;

function Paragraph({
  b, c, fs, sel, sc, ff, calm, clamp, lang = 'en',
}: {
  b: Extract<Block, {t: 'p'}>;
  c: MdColors;
  fs: number;
  sel?: boolean;
  sc?: string;
  ff?: string;
  calm?: boolean;
  clamp?: boolean;
  lang?: Lang;
}) {
  const long = clamp === true && plainLength(b.spans) > PROSE_CLAMP_CHARS;
  const [open, setOpen] = React.useState(false);
  const body = (
    <Text
      selectable={sel}
      selectionColor={sc}
      numberOfLines={long && !open ? PROSE_CLAMP_LINES : undefined}
      style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs, lineHeight: fs * 1.45}]}>
      {renderSpans(b.spans, c, fs, calm)}
    </Text>
  );
  if (!long) return body;
  return (
    <View>
      {body}
      <TouchableOpacity
        testID="md-prose-toggle"
        accessibilityRole="button"
        accessibilityState={{expanded: open}}
        accessibilityLabel={lang === 'zh' ? (open ? '收起全文' : '展开全文') : (open ? 'Show less' : 'Show full text')}
        onPress={() => setOpen(o => !o)}
        activeOpacity={0.6}
        style={[styles.proseToggle, {borderColor: c.border}]}>
        <Text style={[styles.proseToggleText, {color: c.link}]}>
          {lang === 'zh' ? (open ? '收起' : '展开全文') : (open ? 'Show less' : 'Show full text')}
        </Text>
        <DisclosureChevron open={open} color={c.link} prose />
      </TouchableOpacity>
    </View>
  );
}

/** The visible length of a run of spans — what a reader actually faces. */
export function plainLength(spans: Inline[]): number {
  return spans.reduce((n, x) => n + x.s.length, 0);
}

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
 * Generic first-field context for a closed row. The board reader prefers an explicit
 * location column when present, regardless of column order; unknown tables use this
 * fallback instead of inferring a location from free text.
 */
export function rowSubtitle(r: StackedRow): string {
  const f = r.fields[0];
  if (!f) return '';
  return f.value.map(n => n.s).join('').trim();
}

/** Known board columns only; unknown headings retain the author's words. */
function fieldKind(label: string): string {
  const aliases: Record<string, string> = {loc: 'location', location: 'location', '位置': 'location',
    doing: 'task', task: 'task', '在做什么': 'task', '任务': 'task',
    '谁派的': 'origin', 'dispatched by': 'origin', '来源': 'origin',
    priority: 'priority', '优先级': 'priority', status: 'status', '状态': 'status',
    '等你定': 'decision', 'your call': 'decision', '教训': 'lessons', lessons: 'lessons'};
  return aliases[label.trim().toLowerCase()] ?? '';
}

function fieldLabel(label: string, lang: Lang): string {
  const labels: Record<string, [string, string]> = {location: ['Location', '位置'], task: ['Task', '任务'],
    origin: ['Requested by', '来源'], priority: ['Priority', '优先级'], status: ['Status', '状态'],
    decision: ['Your decision', '待你确认'], lessons: ['Lessons', '经验']};
  const pair = labels[fieldKind(label)];
  return pair ? pair[lang === 'zh' ? 1 : 0] : label;
}

function StackedTable({b, c, fs, sel, sc, ff, calm, fold, lang = 'en'}: {b: Extract<Block, {t: 'table'}>; c: MdColors; fs: number; sel?: boolean; sc?: string; ff?: string; calm?: boolean; fold?: boolean; lang?: Lang}) {
  const rows = stackRows(b.header, b.rows);
  // Keys identify rows, not positions: inserting a new pane during a poll must not
  // move the reader's open state onto a different pane.
  const seen = new Map<string, number>();
  const keys = rows.map(r => {
    const id = r.head.map(n => n.s).join('');
    const occurrence = seen.get(id) ?? 0;
    seen.set(id, occurrence + 1);
    return JSON.stringify([id, occurrence]);
  });
  const [open, setOpen] = React.useState<Set<string>>(new Set());
  const toggle = (key: string) => setOpen(prev => {
    const next = new Set(prev);
    next.has(key) ? next.delete(key) : next.add(key);
    return next;
  });

  return (
    <View style={styles.block}>
      {rows.map((r, i) => {
        const shut = fold && !open.has(keys[i]);
        const identity = r.head.map(n => n.s).join('').trim();
        const task = r.fields.find(f => fieldKind(f.label) === 'task');
        const summary = task?.value.map(n => n.s).join('').trim();
        const location = r.fields.find(f => fieldKind(f.label) === 'location');
        const subtitle = location ? location.value.map(n => n.s).join('').trim() : rowSubtitle(r);
        return (
          <View key={keys[i]} style={shut ? [styles.stackShut, {borderBottomColor: c.border}]
            : [styles.stackRow, {borderColor: c.border, backgroundColor: c.codeBg}]}>
            {fold ? (
              <TouchableOpacity testID={`md-stack-row-${i}`} accessibilityRole="button"
                accessibilityState={{expanded: !shut}} accessibilityLabel={[summary, identity, subtitle].filter(Boolean).join(', ')}
                activeOpacity={0.6} onPress={() => toggle(keys[i])} style={styles.stackHead}>
                <View style={styles.stackIdentity}>
                  <Text numberOfLines={2} style={{color: c.text, fontSize: fs, fontWeight: '600', lineHeight: fs * 1.4}}>
                    {summary || renderSpans(r.head, c, fs, calm)}
                  </Text>
                  {(summary || subtitle) && (
                    <Text numberOfLines={2} style={[styles.stackSub, {color: c.dim, fontSize: fs - 1, lineHeight: fs * 1.4}]}>
                      {[summary ? identity : '', subtitle !== summary ? subtitle : ''].filter(Boolean).join(' · ')}
                    </Text>
                  )}
                </View>
                <DisclosureChevron open={!shut} color={c.dim} />
              </TouchableOpacity>
            ) : (
              <Text selectable={sel} selectionColor={sc} style={{color: c.text, fontFamily: ff, fontSize: fs, fontWeight: '600', lineHeight: fs * 1.4, marginBottom: 3}}>
                {renderSpans(r.head, c, fs, calm)}
              </Text>
            )}
            {!shut && r.fields.map((f, j) => (
              <View key={j} style={fold ? styles.stackFieldVertical : styles.stackField}>
                <Text style={[fold ? styles.stackLabelVertical : styles.stackLabel, {color: c.dim, fontSize: fs - 1, lineHeight: fs * 1.4}]}>
                  {fold ? fieldLabel(f.label, lang) : f.label}
                </Text>
                {fold ? (
                  <Paragraph b={{t: 'p', spans: f.value}} c={c} fs={fs} sel={sel} sc={sc} ff={ff} calm={calm} clamp lang={lang} />
                ) : (
                  <Text selectable={sel} selectionColor={sc} style={{flex: 1, color: c.text, fontFamily: ff, fontSize: fs - 1, lineHeight: (fs - 1) * 1.4}}>
                    {renderSpans(f.value, c, fs - 1, calm)}
                  </Text>
                )}
              </View>
            ))}
          </View>
        );
      })}
    </View>
  );
}

function BlockView({b, c, fs, sel, sc, ff, calm, fold, clampProse, lang}: {b: Block; c: MdColors; fs: number; sel?: boolean; sc?: string; ff?: string; calm?: boolean; fold?: boolean; clampProse?: boolean; lang?: Lang}) {
  switch (b.t) {
    case 'h':
      return (
        <Text selectable={sel} selectionColor={sc} style={[styles.block, {color: c.text, fontFamily: ff, fontSize: fs + ((calm ? HEADING_SIZE_CALM : HEADING_SIZE)[b.level] ?? 0), fontWeight: '700', lineHeight: (fs + 6) * 1.3, marginTop: calm ? 10 : 0}]}>
          {renderSpans(b.spans, c, fs, calm)}
        </Text>
      );
    case 'p':
      return <Paragraph b={b} c={c} fs={fs} sel={sel} sc={sc} ff={ff} calm={calm} clamp={clampProse} lang={lang} />;
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
        return <StackedTable b={b} c={c} fs={fs} sel={sel} sc={sc} ff={ff} calm={calm} fold={fold} lang={lang} />;
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

export function MarkdownView({source, colors, fontSize = 14, selectable, selectionColor, fontFamily, calmEmphasis, foldRows, clampProse, lang}: Props) {
  const blocks = React.useMemo(() => parseBlocks(source), [source]);
  return (
    <View>
      {blocks.map((b, i) => (
        <BlockView key={i} b={b} c={colors} fs={fontSize} sel={selectable} sc={selectionColor} ff={fontFamily} calm={calmEmphasis} fold={foldRows} clampProse={clampProse} lang={lang} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  proseToggle: {alignSelf: 'flex-start', flexDirection: 'row', alignItems: 'center', gap: 6, minHeight: 44, paddingHorizontal: 10, borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, marginBottom: 8},
  proseToggleText: {fontSize: 12.5, fontWeight: '600'},
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
  stackHead: {flexDirection: 'row', alignItems: 'center', gap: 10, minHeight: 48, paddingVertical: 8},
  stackIdentity: {flex: 1, minWidth: 0},
  stackSub: {marginTop: 3},
  stackFieldVertical: {marginTop: 10},
  stackLabelVertical: {fontWeight: '500', marginBottom: 3},
  stackField: {flexDirection: 'row', alignItems: 'flex-start', gap: 9, marginTop: 3},
  stackLabel: {minWidth: 58, fontVariant: ['tabular-nums']},
  tr: {flexDirection: 'row'},
  td: {borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 8, paddingVertical: 5, minWidth: 92, justifyContent: 'center'},
});
