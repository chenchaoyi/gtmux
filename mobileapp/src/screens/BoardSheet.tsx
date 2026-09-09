// BoardSheet — the supervisor's situation board, read as an OUTLINE (MOBILE §17.3).
//
// The board is an archive: 48,000 characters on the machine this was built for, pinned
// handoff at the top, newest progress at the bottom. Rendering it as one scroll meant
// dragging through everything to reach anything, so it was split into collapsible
// sections. That split stopped at `##` — and the board's `##` level is TWO rows.
//
// Two rows is not an outline. Closed, it is a screen of void; open, it is a
// 26,000-character wall that took a visible pause to lay out. Both complaints landed
// together (2026-09-06: 「这个 UI 太差劲了」 and 「展开交接记录…卡了一阵才展开」), and both
// are the same defect: the outline was not reaching the level where this board's entries
// actually live, which is `###`.
//
// Reaching it fixes the lag by not doing the work. Measured on the real board:
//
//     whole section, as it was     1317 text nodes    78 ms
//     its 26 headings                 80 text nodes    3 ms
//     one entry, once opened          54 text nodes    2 ms
//
// So there is no loading state here, deliberately. A spinner is what you show when the
// work is unavoidable; 3 ms of work is not worth announcing, and announcing it would be
// covering for a render nobody asked for.

import React, {useEffect, useRef, useState} from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {MarkdownView, MdColors} from '../ui/MarkdownView';
import {Palette, StatusColor} from '../ui/theme';
import {BoardSection, findAsk, parseBoardSections, sectionCount} from './boardSections';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

export function boardMdColors(pal: Palette): MdColors {
  return {text: pal.fg, dim: pal.fg3, code: pal.fg, codeBg: pal.surface, border: pal.divider, link: pal.fg2};
}

export function BoardSheet({
  visible,
  sections,
  age,
  pal,
  zh,
  onClose,
}: {
  visible: boolean;
  sections: BoardSection[];
  /** Already-formatted age of the board, e.g. "50m ago" — the sheet does no clock work. */
  age: string;
  pal: Palette;
  zh: boolean;
  onClose: () => void;
}) {
  const t = (en: string, cn: string) => (zh ? cn : en);
  const [open, setOpen] = useState<Set<string>>(new Set());
  // The one section gtmux owns and both surfaces lift.
  const ask = React.useMemo(() => findAsk(sections), [sections]);

  // Seeded ONCE, not on every parse. The board is polled, so `sections` is a new array
  // every few minutes even when nothing in it changed — re-seeding on that snapped shut
  // whatever the reader had opened, mid-read, for no reason visible to them.
  const seeded = useRef(false);
  useEffect(() => {
    if (seeded.current) return;
    const first = sections.find(x => x.title !== '');
    if (!first) return;
    seeded.current = true;
    setOpen(new Set([first.key]));
  }, [sections]);
  const toggle = (k: string) =>
    setOpen(prev => {
      const next = new Set(prev);
      next.has(k) ? next.delete(k) : next.add(k);
      return next;
    });

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <View style={[styles.root, {backgroundColor: pal.bg}]}>
        <View style={[styles.head, {borderBottomColor: pal.divider}]}>
          <View style={styles.mid}>
            <Text style={[styles.title, {color: pal.fg}]}>{t('Situation board', '态势板')}</Text>
            {/* The age and nothing else. This line used to name the board again, right
                under a title that already says it, so the sheet read «Situation board»
                over «situation board · 50m ago». */}
            <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
              {age} · {t('read-only', '只读')}
            </Text>
          </View>
          {/* A labelled button, not a bare ✕: the way out of a full-screen reader has to
              be unmissable, and a word cannot be mistaken for decoration. */}
          <TouchableOpacity
            testID="hq-board-close"
            accessibilityLabel="hq-board-close"
            onPress={onClose}
            hitSlop={hit}
            style={[styles.close, {borderColor: pal.divider}]}>
            <Text style={[styles.closeText, {color: pal.fg}]}>{t('Done', '完成')}</Text>
          </TouchableOpacity>
        </View>

        <ScrollView contentContainerStyle={styles.pad}>
          {/* The commander's own section, lifted. Everything else here is HQ's posture;
              this is the part with a claim on HIS attention, and it was a `###` inside ①
              — visible only after expanding the section above it (2026-09-09). Absent or
              empty, nothing is drawn: a band that is always there stops being read. */}
          {ask && ask.body.trim() !== '' && (
            <View testID="hq-board-ask" style={[styles.ask, {borderColor: 'rgba(239,68,68,0.3)'}]}>
              <Text style={[styles.askHead, {color: StatusColor.waiting}]} numberOfLines={1}>
                {ask.title.replace(/^#+\s*/, '').toUpperCase()}
              </Text>
              <MarkdownView
                source={ask.body}
                colors={boardMdColors(pal)}
                fontSize={13.5}
                selectable
                calmEmphasis
                foldRows
                clampProse
              />
            </View>
          )}
          {sections.map((sec, i) => {
            const secOpen = open.has(sec.key);
            // What the section is ABOUT: the rows or bullets of its own content when it
            // has any, else the entries nested under it. Never how many lines it was
            // typed on — that read "154" beside a twelve-row table.
            //
            // Own content wins because the heading is a promise. 「① 现状 — 在跑的 pane」
            // leads with a 13-row table of panes AND carries four sub-headings; counting
            // the sub-headings turned a 13 into a 4 and hid the number the title just
            // asked about. 「② 交接记录」 has no body of its own, so there its entries are
            // the only thing there is to count.
            const n = sectionCount(sec.body) ?? (sec.children.length > 0 ? sec.children.length : null);
            return (
              <View key={sec.key} style={[styles.sec, {borderColor: pal.divider}]}>
                {sec.title !== '' && (
                  <TouchableOpacity
                    testID={`hq-board-section-${i}`}
                    activeOpacity={0.6}
                    onPress={() => toggle(sec.key)}
                    style={styles.secHead}>
                    <Text style={[styles.chevron, {color: pal.fg3}]}>{secOpen ? '▾' : '▸'}</Text>
                    <Text style={[styles.secTitle, {color: pal.fg}]} numberOfLines={secOpen ? undefined : 2}>
                      {sec.title}
                    </Text>
                    {n == null ? null : (
                      <View style={[styles.countBox, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
                        <Text style={[styles.count, {color: pal.fg3}]}>{n}</Text>
                      </View>
                    )}
                  </TouchableOpacity>
                )}
                {(secOpen || sec.title === '') && (
                  <View style={sec.title === '' ? undefined : styles.secBody}>
                    {sec.body !== '' && (
                      <MarkdownView
                        source={sec.body}
                        colors={boardMdColors(pal)}
                        fontSize={13.5}
                        selectable
                        calmEmphasis
                        foldRows
                        clampProse
                      />
                    )}
                    {sec.children.map((kid, k) => {
                      const kidOpen = open.has(kid.key);
                      return (
                        <View key={kid.key} style={[styles.kid, {borderTopColor: pal.divider}]}>
                          <TouchableOpacity
                            testID={`hq-board-entry-${i}-${k}`}
                            activeOpacity={0.6}
                            onPress={() => toggle(kid.key)}
                            style={styles.kidHead}>
                            <Text style={[styles.chevron, {color: pal.fg3}]}>{kidOpen ? '▾' : '▸'}</Text>
                            <Text
                              style={[styles.kidTitle, {color: pal.fg2}]}
                              numberOfLines={kidOpen ? undefined : 2}>
                              {kid.title}
                            </Text>
                          </TouchableOpacity>
                          {kidOpen && (
                            <View style={styles.kidBody}>
                              <MarkdownView
                                source={kid.body}
                                colors={boardMdColors(pal)}
                                fontSize={13.5}
                                selectable
                                calmEmphasis
                                foldRows
                              />
                            </View>
                          )}
                        </View>
                      );
                    })}
                  </View>
                )}
              </View>
            );
          })}
        </ScrollView>
      </View>
    </Modal>
  );
}

export {parseBoardSections};

const styles = StyleSheet.create({
  ask: {borderWidth: 1, borderRadius: 11, backgroundColor: 'rgba(239,68,68,0.07)',
    paddingHorizontal: 11, paddingTop: 9, paddingBottom: 4, marginBottom: 12, gap: 4},
  askHead: {fontSize: 10.5, fontWeight: '700', letterSpacing: 0.6},
  root: {flex: 1},
  head: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 14,
    paddingVertical: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  mid: {flex: 1},
  title: {fontSize: 17, fontWeight: '700'},
  sub: {fontSize: 11.5, marginTop: 2},
  close: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 6},
  closeText: {fontSize: 13, fontWeight: '600'},
  pad: {paddingHorizontal: 14, paddingVertical: 10},
  sec: {borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 2},
  secHead: {flexDirection: 'row', alignItems: 'flex-start', paddingVertical: 11, gap: 8},
  chevron: {fontSize: 12, width: 12, marginTop: 1},
  secTitle: {flex: 1, fontSize: 14, fontWeight: '700', lineHeight: 19},
  count: {fontSize: 10.5, fontVariant: ['tabular-nums']},
  countBox: {
    minWidth: 24,
    alignItems: 'center',
    paddingHorizontal: 5,
    paddingVertical: 1,
    borderRadius: 8,
    borderWidth: StyleSheet.hairlineWidth,
    marginTop: 1,
  },
  secBody: {paddingBottom: 10},
  // The second level, indented so the outline reads as one — an entry belongs to the
  // section above it, and a rule between entries is what says "another one".
  kid: {borderTopWidth: StyleSheet.hairlineWidth, marginLeft: 20},
  kidHead: {flexDirection: 'row', alignItems: 'flex-start', paddingVertical: 9, gap: 8},
  kidTitle: {flex: 1, fontSize: 13, fontWeight: '600', lineHeight: 18},
  kidBody: {paddingBottom: 8},
});
