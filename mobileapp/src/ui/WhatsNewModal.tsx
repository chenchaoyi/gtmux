// WhatsNewModal — the release notes for every version the reader crossed, shown once after
// an update and reachable again from Settings. The phone's `gtmux whatsnew`.
//
// Two layers, like the CLI: the popup shows a capped summary (newest first, grouped by
// version) and folds the rest behind "show all", which expands the SAME card rather than
// pushing a screen — the reader asked for more of what they are already looking at.
//
// The card carries the product's own vocabulary rather than generic chrome: the pane-grid
// BrandMark that is also the app icon, and versions set in Menlo, the same monospace the
// terminal and the branch chips use. That is the whole of the decoration. MOBILE §6 rules
// out the section taxonomy (NEW / ENHANCED / FIXED), accent fills and animation that
// changelog cards usually reach for — a changelog is read once and dismissed, and dressing
// it up is exactly the marketing tone the design rules forbid. Identity, not ornament.

import React, {useState} from 'react';
import {Modal, Pressable, ScrollView, StyleSheet, Text, View} from 'react-native';
import {Lang} from '../i18n';
import {ReleaseNote} from '../releaseNotes';
import {capEntries, linesOf} from '../state/whatsnew';
import {BrandMark} from './BrandMark';
import {Palette} from './theme';

export function WhatsNewModal({
  visible,
  entries,
  pal,
  lang,
  showAll = false,
  onClose,
}: {
  visible: boolean;
  /** Every version to report, NEWEST FIRST. */
  entries: ReleaseNote[];
  pal: Palette;
  lang: Lang;
  /** Start expanded (Settings opens the full list; an update opens the summary). */
  showAll?: boolean;
  onClose: () => void;
}) {
  const zh = lang === 'zh';
  const [expanded, setExpanded] = useState(showAll);
  const {shown, omitted} = expanded ? {shown: entries, omitted: 0} : capEntries(entries, lang);
  // Version headings earn their place only when there are several. With one, the header
  // chip already names it — printing it twice (as it did) is not structure, it is noise.
  const grouped = shown.length > 1;

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <View style={s.scrim}>
        {/* The backdrop is a SIBLING behind the card, never an ancestor of it. As a parent
            it claimed the touch on start and the ScrollView never got the gesture — the
            card scrolled in fits or not at all (user report, 2026-08-09). Nothing may wrap
            a ScrollView in a Pressable. */}
        <Pressable
          style={StyleSheet.absoluteFill}
          onPress={onClose}
          accessibilityLabel={zh ? '关闭' : 'Close'}
        />
        <View style={[s.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <View style={[s.head, {borderBottomColor: pal.divider}]}>
            <BrandMark size={19} neutral={pal.fg3} />
            <Text style={[s.title, {color: pal.fg}]}>{zh ? '更新内容' : "What's new"}</Text>
            <View style={s.spacer} />
            {!grouped && shown.length > 0 && (
              <Text style={[s.version, {color: pal.fg2, borderColor: pal.divider}]}>{shown[0].version}</Text>
            )}
          </View>

          <ScrollView style={s.body} contentContainerStyle={s.bodyPad} showsVerticalScrollIndicator={false}>
            {shown.map((note, gi) => (
              <View key={note.version}>
                {grouped && (
                  <View style={[s.group, gi > 0 && s.groupGap]}>
                    <Text style={[s.groupText, {color: pal.fg2}]}>{note.version}</Text>
                    <View style={[s.groupRule, {backgroundColor: pal.divider}]} />
                  </View>
                )}
                {linesOf(note, lang).map((line, i) => (
                  <View key={i} style={s.row}>
                    {/* One cell of the pane grid: the brand's own unit, at bullet size. */}
                    <View style={[s.bullet, {backgroundColor: pal.fg3}]} />
                    <Text style={[s.line, {color: pal.fg}]}>{line}</Text>
                  </View>
                ))}
              </View>
            ))}
          </ScrollView>

          {omitted > 0 && (
            <Pressable
              style={({pressed}) => [s.btn, {borderTopColor: pal.divider}, pressed && {backgroundColor: pal.rowSelected}]}
              onPress={() => setExpanded(true)}
              accessibilityRole="button">
              <Text style={[s.btnText, s.btnQuiet, {color: pal.fg2}]}>
                {zh ? `还有 ${omitted} 条 · 全部显示` : `+${omitted} more · show all`}
              </Text>
            </Pressable>
          )}

          <Pressable
            style={({pressed}) => [s.btn, {borderTopColor: pal.divider}, pressed && {backgroundColor: pal.rowSelected}]}
            onPress={onClose}
            accessibilityRole="button">
            <Text style={[s.btnText, {color: pal.fg}]}>{zh ? '知道了' : 'Got it'}</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}

const s = StyleSheet.create({
  scrim: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', alignItems: 'center', justifyContent: 'center', padding: 24},
  // maxHeight, not a fixed height: a one-bullet release gets a small card and a long
  // history scrolls, instead of either padding emptiness or running off the screen.
  card: {width: '100%', maxWidth: 460, maxHeight: '80%', borderRadius: 16, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  head: {flexDirection: 'row', alignItems: 'center', gap: 9, paddingHorizontal: 18, paddingVertical: 15, borderBottomWidth: StyleSheet.hairlineWidth},
  title: {fontSize: 18, fontWeight: '700', letterSpacing: 0.2},
  spacer: {flex: 1},
  // Menlo for a version everywhere it appears — the same monospace the terminal, the
  // branch chips and the HQ figures use. A version is a token, not prose.
  version: {fontFamily: 'Menlo', fontSize: 11, borderWidth: StyleSheet.hairlineWidth, borderRadius: 5, paddingHorizontal: 6, paddingVertical: 3, overflow: 'hidden'},
  body: {flexGrow: 0},
  bodyPad: {paddingHorizontal: 18, paddingTop: 14, paddingBottom: 16},
  group: {flexDirection: 'row', alignItems: 'center', gap: 9, marginBottom: 11},
  groupGap: {marginTop: 18},
  groupText: {fontFamily: 'Menlo', fontSize: 11},
  groupRule: {flex: 1, height: StyleSheet.hairlineWidth},
  row: {flexDirection: 'row', marginBottom: 13},
  // A pane cell, not a typographic dot: 5pt, rounded like the BrandMark's cells, and
  // nudged down to sit on the first line's optical centre.
  bullet: {width: 5, height: 5, borderRadius: 1.5, marginTop: 8, marginRight: 11},
  // A changelog line WRAPS — it is prose, not a status row, so the CJK
  // ellipsis-truncation rule (which is about list rows) does not apply here.
  line: {flex: 1, fontSize: 15, lineHeight: 22},
  btn: {alignItems: 'center', paddingVertical: 14, borderTopWidth: StyleSheet.hairlineWidth},
  btnText: {fontSize: 16, fontWeight: '600'},
  btnQuiet: {fontSize: 14, fontWeight: '500'},
});
