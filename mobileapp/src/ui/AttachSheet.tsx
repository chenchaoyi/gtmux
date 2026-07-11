// AttachSheet — the composer's "+" attach picker, as a branded bottom sheet
// (mirrors SnippetsPicker/HistoryModal) with an icon + label per row, replacing the
// bare native iOS action sheet (a plain stack of gray text pills).

import React from 'react';
import {Modal, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {PhotoLibraryIcon, CameraIcon, FileIcon, PasteIcon} from './Icons';

type Row = {icon: React.ComponentType<{size?: number; color?: string}>; label: string; onPress: () => void};

export function AttachSheet({
  visible,
  pal,
  lang,
  onClose,
  onPhoto,
  onCamera,
  onFile,
  onPaste,
}: {
  visible: boolean;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
  onPhoto: () => void;
  onCamera: () => void;
  onFile: () => void;
  onPaste: () => void;
}) {
  const zh = lang === 'zh';
  const rows: Row[] = [
    {icon: PhotoLibraryIcon, label: zh ? '相册' : 'Photo Library', onPress: onPhoto},
    {icon: CameraIcon, label: zh ? '拍照' : 'Take Photo', onPress: onCamera},
    {icon: FileIcon, label: zh ? '文件' : 'File', onPress: onFile},
    {icon: PasteIcon, label: zh ? '粘贴' : 'Paste', onPress: onPaste},
  ];
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity activeOpacity={1} style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <Text style={[styles.title, {color: pal.fg}]}>{zh ? '添加' : 'Attach'}</Text>
          {rows.map((r, i) => (
            <TouchableOpacity
              key={r.label}
              accessibilityLabel={`attach-${i}`}
              onPress={() => {
                onClose();
                r.onPress();
              }}
              style={[styles.row, {borderTopColor: pal.divider}, i === 0 && styles.firstRow]}>
              <View style={[styles.iconWrap, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
                <r.icon size={20} color={pal.fg2} />
              </View>
              <Text style={[styles.rowText, {color: pal.fg}]}>{r.label}</Text>
            </TouchableOpacity>
          ))}
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 16, borderTopRightRadius: 16, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 14, paddingBottom: 28},
  title: {fontSize: 16, fontWeight: '700', marginBottom: 6},
  row: {flexDirection: 'row', alignItems: 'center', paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth},
  firstRow: {borderTopWidth: 0},
  iconWrap: {width: 38, height: 38, borderRadius: 10, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 14},
  rowText: {fontSize: 16, fontWeight: '500'},
});
