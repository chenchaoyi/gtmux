// SnippetsPicker — pick a saved quick reply (常用语) to send (or jump to manage). A
// proper branded bottom sheet (mirrors HistoryModal), replacing the bare iOS action
// sheet. User-facing copy says "Quick replies" / 「常用语」 (2026-08 rename — "Snippets"
// is developer jargon); the file/testID/internal names deliberately keep `snippets`.

import React from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {TestIds} from '../constants/testIds';

export function SnippetsPicker({
  visible,
  snippets,
  pal,
  lang,
  onPick,
  onManage,
  onClose,
}: {
  visible: boolean;
  snippets: string[];
  pal: Palette;
  lang: Lang;
  onPick: (text: string) => void;
  onManage: () => void;
  onClose: () => void;
}) {
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      {/* accessible={false} on both touchables: they're tap-catchers (dismiss /
          swallow), and a Touchable is an AX element by DEFAULT — without the
          opt-out the whole sheet merged into ONE unreadable element (the same
          PickerSheet a11y bug). The title/rows below stay individual elements. */}
      <TouchableOpacity accessible={false} style={styles.backdrop} activeOpacity={1} onPress={onClose} testID={TestIds.composer.snippetSheet}>
        <TouchableOpacity accessible={false} activeOpacity={1} style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={styles.head}>
            <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '常用语' : 'Quick replies'}</Text>
            <TouchableOpacity onPress={onManage} hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}>
              <Text style={[styles.manage, {color: '#06B6D4'}]}>{lang === 'zh' ? '管理' : 'Manage'}</Text>
            </TouchableOpacity>
          </View>
          {snippets.length === 0 ? (
            <Text style={[styles.empty, {color: pal.fg3}]}>
              {lang === 'zh' ? '还没有常用语，点「管理」添加常用指令' : 'No quick replies yet — tap Manage to add common commands'}
            </Text>
          ) : (
            <ScrollView style={styles.list} keyboardShouldPersistTaps="always">
              {snippets.map((s, i) => (
                <TouchableOpacity
                  key={`${i}-${s}`}
                  onPress={() => onPick(s)}
                  style={[styles.row, {borderBottomColor: pal.divider}]}>
                  <Text style={[styles.rowText, {color: pal.fg}]} numberOfLines={2}>
                    {s}
                  </Text>
                  <Text style={[styles.send, {color: pal.fg3}]}>↗</Text>
                </TouchableOpacity>
              ))}
            </ScrollView>
          )}
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 16, borderTopRightRadius: 16, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 14, paddingBottom: 28, maxHeight: '70%'},
  head: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8},
  title: {fontSize: 16, fontWeight: '700'},
  manage: {fontSize: 13, fontWeight: '600'},
  empty: {fontSize: 13, textAlign: 'center', paddingVertical: 28, lineHeight: 19, paddingHorizontal: 12},
  list: {marginTop: 2},
  row: {flexDirection: 'row', alignItems: 'center', paddingVertical: 13, borderBottomWidth: StyleSheet.hairlineWidth},
  rowText: {flex: 1, fontSize: 14.5, lineHeight: 19},
  send: {fontSize: 16, marginLeft: 10},
});
