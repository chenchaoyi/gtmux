// WhatsNewModal — the release notes for every version the reader crossed, shown once after
// an update and reachable again from Settings. The phone's `gtmux whatsnew`.
//
// Two layers, like the CLI: the popup shows a capped summary (newest first, grouped by
// version) and folds the rest behind "show all", which expands the SAME card rather than
// pushing a screen — the reader asked for more of what they are already looking at.
//
// Deliberately plain, per MOBILE §6: a title, the version, the bullets, one button. No
// section taxonomy (NEW / ENHANCED / FIXED), no accent colour, no animation — a changelog
// is read once and dismissed, and dressing it up is the marketing tone the design rules
// rule out.

import React, {useState} from 'react';
import {Modal, Pressable, ScrollView, StyleSheet, Text, View} from 'react-native';
import {Lang} from '../i18n';
import {ReleaseNote} from '../releaseNotes';
import {capEntries, linesOf} from '../state/whatsnew';
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
  const {shown, omitted} = expanded
    ? {shown: entries, omitted: 0}
    : capEntries(entries, lang);
  // Only worth naming versions when there is more than one — a single-version update is
  // just "what's new", and a heading over the only group is noise.
  const grouped = shown.length > 1;

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      {/* Tap the scrim to dismiss as well as the button: a changelog must never be
          something the reader has to hunt for a way out of. */}
      <Pressable style={s.scrim} onPress={onClose} accessibilityLabel={zh ? '关闭' : 'Close'}>
        {/* Stop taps inside the card from closing it (a tap on a bullet is not a dismiss). */}
        <Pressable style={[s.card, {backgroundColor: pal.surface}]} onPress={() => {}}>
          <View style={s.head}>
            <Text style={[s.title, {color: pal.fg}]}>{zh ? '更新内容' : "What's new"}</Text>
            {shown.length > 0 && (
              <Text style={[s.version, {color: pal.fg2, borderColor: pal.divider}]}>
                {shown[0].version}
              </Text>
            )}
          </View>

          <ScrollView style={s.body} contentContainerStyle={s.bodyPad} showsVerticalScrollIndicator={false}>
            {shown.map(note => (
              <View key={note.version}>
                {grouped && (
                  <Text style={[s.group, {color: pal.fg3, borderBottomColor: pal.divider}]}>
                    {note.version}
                  </Text>
                )}
                {linesOf(note, lang).map((line, i) => (
                  <View key={i} style={s.row}>
                    <Text style={[s.bullet, {color: pal.fg3}]}>·</Text>
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
              <Text style={[s.btnText, {color: pal.fg2}]}>
                {zh ? `还有 ${omitted} 条 —— 全部显示` : `+${omitted} more — show all`}
              </Text>
            </Pressable>
          )}

          <Pressable
            style={({pressed}) => [s.btn, {borderTopColor: pal.divider}, pressed && {backgroundColor: pal.rowSelected}]}
            onPress={onClose}
            accessibilityRole="button">
            <Text style={[s.btnText, {color: pal.fg}]}>{zh ? '知道了' : 'Got it'}</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const s = StyleSheet.create({
  scrim: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', alignItems: 'center', justifyContent: 'center', padding: 24},
  // maxHeight, not a fixed height: a one-bullet release gets a small card and a long
  // history scrolls, instead of either padding emptiness or running off the screen.
  card: {width: '100%', maxWidth: 460, maxHeight: '80%', borderRadius: 14, overflow: 'hidden'},
  head: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 20, paddingTop: 20, paddingBottom: 4},
  title: {fontSize: 20, fontWeight: '700'},
  version: {fontSize: 12, fontVariant: ['tabular-nums'], borderWidth: StyleSheet.hairlineWidth, borderRadius: 6, paddingHorizontal: 7, paddingVertical: 3, overflow: 'hidden'},
  body: {flexGrow: 0},
  bodyPad: {paddingHorizontal: 20, paddingTop: 12, paddingBottom: 18},
  group: {fontSize: 12, fontVariant: ['tabular-nums'], letterSpacing: 0.5, marginTop: 6, marginBottom: 10, paddingBottom: 6, borderBottomWidth: StyleSheet.hairlineWidth},
  row: {flexDirection: 'row', marginBottom: 12},
  bullet: {width: 14, fontSize: 15, lineHeight: 21},
  // A changelog line WRAPS — it is prose, not a status row, so the CJK
  // ellipsis-truncation rule (which is about list rows) does not apply here.
  line: {flex: 1, fontSize: 15, lineHeight: 21},
  btn: {alignItems: 'center', paddingVertical: 14, borderTopWidth: StyleSheet.hairlineWidth},
  btnText: {fontSize: 16, fontWeight: '600'},
});
