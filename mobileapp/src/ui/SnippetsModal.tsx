// SnippetsModal — manage the composer's saved snippets (MOBILE §4): add a new
// one, delete existing. A light bottom sheet; the parent persists the result.

import React, {useState} from 'react';
import {Modal, ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {addSnippet, removeSnippet} from '../state/snippets';

const ACCENT = '#06B6D4';
const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function SnippetsModal({
  visible,
  snippets,
  pal,
  lang,
  onChange,
  onClose,
}: {
  visible: boolean;
  snippets: string[];
  pal: Palette;
  lang: Lang;
  onChange: (list: string[]) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState('');
  const add = () => {
    onChange(addSnippet(snippets, draft));
    setDraft('');
  };
  const canAdd = !!draft.trim();

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity
          activeOpacity={1}
          style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={styles.headerRow}>
            <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '快捷短语' : 'Snippets'}</Text>
            <TouchableOpacity onPress={onClose} hitSlop={hit}>
              <Text style={styles.done}>{lang === 'zh' ? '完成' : 'Done'}</Text>
            </TouchableOpacity>
          </View>

          <View style={styles.addRow}>
            <TextInput
              value={draft}
              onChangeText={setDraft}
              placeholder={lang === 'zh' ? '新增短语…' : 'New snippet…'}
              placeholderTextColor={pal.fg3}
              autoCapitalize="none"
              returnKeyType="done"
              onSubmitEditing={add}
              style={[styles.input, {color: pal.fg, borderColor: pal.divider, backgroundColor: pal.surface}]}
            />
            <TouchableOpacity
              onPress={add}
              disabled={!canAdd}
              style={[styles.addBtn, {backgroundColor: canAdd ? ACCENT : pal.surface, borderColor: pal.divider}]}>
              <Text style={[styles.addText, {color: canAdd ? '#fff' : pal.fg3}]}>+</Text>
            </TouchableOpacity>
          </View>

          <ScrollView style={styles.list} keyboardShouldPersistTaps="always">
            {snippets.length === 0 && (
              <Text style={[styles.empty, {color: pal.fg3}]}>{lang === 'zh' ? '还没有短语' : 'No snippets yet'}</Text>
            )}
            {snippets.map(s => (
              <View key={s} style={[styles.item, {borderBottomColor: pal.divider}]}>
                <Text style={[styles.itemText, {color: pal.fg}]} numberOfLines={1}>
                  {s}
                </Text>
                <TouchableOpacity onPress={() => onChange(removeSnippet(snippets, s))} hitSlop={hit}>
                  <Text style={styles.del}>{lang === 'zh' ? '删除' : 'Delete'}</Text>
                </TouchableOpacity>
              </View>
            ))}
          </ScrollView>
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, justifyContent: 'flex-end', backgroundColor: 'rgba(0,0,0,0.4)'},
  sheet: {
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 16,
    paddingTop: 14,
    paddingBottom: 28,
    maxHeight: '70%',
  },
  headerRow: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12},
  title: {fontSize: 17, fontWeight: '700'},
  done: {fontSize: 15, fontWeight: '700', color: ACCENT},
  addRow: {flexDirection: 'row', alignItems: 'center', marginBottom: 8},
  input: {flex: 1, height: 40, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, fontSize: 15},
  addBtn: {width: 40, height: 40, borderRadius: 10, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  addText: {fontSize: 22, fontWeight: '500'},
  list: {marginTop: 4},
  empty: {fontSize: 13.5, textAlign: 'center', paddingVertical: 20},
  item: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  itemText: {fontSize: 15, flex: 1, marginRight: 12},
  del: {fontSize: 13.5, fontWeight: '600', color: '#EF4444'},
});
