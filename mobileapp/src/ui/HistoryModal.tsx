// HistoryModal — pick a previously-sent message (MOBILE §10 toolbar "↑ 历史").
// A light bottom sheet listing recent inputs (newest first); tap to load it back
// into the composer for editing/resending. The parent owns the list + clearing.

import React from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function HistoryModal({
  visible,
  history,
  pal,
  lang,
  onPick,
  onClear,
  onClose,
}: {
  visible: boolean;
  history: string[];
  pal: Palette;
  lang: Lang;
  onPick: (text: string) => void;
  onClear: () => void;
  onClose: () => void;
}) {
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity activeOpacity={1} style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={styles.head}>
            <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '历史' : 'History'}</Text>
            {history.length > 0 && (
              <TouchableOpacity onPress={onClear} hitSlop={hit}>
                <Text style={[styles.clear, {color: pal.fg3}]}>{lang === 'zh' ? '清空' : 'Clear'}</Text>
              </TouchableOpacity>
            )}
          </View>
          {history.length === 0 ? (
            <Text style={[styles.empty, {color: pal.fg3}]}>
              {lang === 'zh' ? '还没有发送过消息' : 'No messages sent yet'}
            </Text>
          ) : (
            <ScrollView style={styles.list} keyboardShouldPersistTaps="always">
              {history.map((h, i) => (
                <TouchableOpacity
                  key={`${i}-${h}`}
                  onPress={() => onPick(h)}
                  style={[styles.row, {borderBottomColor: pal.divider}]}>
                  <Text style={[styles.rowText, {color: pal.fg}]} numberOfLines={2}>
                    {h}
                  </Text>
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
  clear: {fontSize: 13, fontWeight: '600'},
  empty: {fontSize: 13, textAlign: 'center', paddingVertical: 28},
  list: {marginTop: 2},
  row: {paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  rowText: {fontSize: 14, lineHeight: 19},
});
