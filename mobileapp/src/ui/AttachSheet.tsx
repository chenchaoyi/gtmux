// AttachSheet — the composer's "+" attach picker, as a branded bottom sheet
// (mirrors SnippetsPicker/HistoryModal) replacing the bare native iOS action sheet.
// Grabber + big title + one CARD per action: an icon tile, a bold title, and a dim
// one-line description.

import React from 'react';
import {Modal, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {TestIds} from '../constants/testIds';
import {Palette} from './theme';
import {PhotoLibraryIcon, CameraIcon, FileIcon, PasteIcon} from './Icons';

type Row = {
  icon: React.ComponentType<{size?: number; color?: string}>;
  title: string;
  sub: string;
  onPress: () => void;
};

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
  // Defer the chosen action until AFTER this sheet has fully dismissed. Presenting a
  // picker (photo library / camera / document) while this Modal is still animating out
  // silently fails on iOS ("attempt to present while being dismissed") — which is why
  // "+→ Photo Library" did nothing. onDismiss fires post-animation, so the picker
  // presents cleanly. A ref (not state) avoids an extra render on selection.
  const pendingRef = React.useRef<null | (() => void)>(null);
  const rows: Row[] = [
    {icon: CameraIcon, title: zh ? '拍照' : 'Camera', sub: zh ? '拍一张新照片' : 'Take a new photo', onPress: onCamera},
    {icon: PhotoLibraryIcon, title: zh ? '相册' : 'Photo Library', sub: zh ? '从相册选择照片' : 'Choose from your library', onPress: onPhoto},
    {icon: FileIcon, title: zh ? '文件' : 'File', sub: zh ? '上传文件到主机' : 'Upload a file to the host', onPress: onFile},
    {icon: PasteIcon, title: zh ? '粘贴' : 'Paste', sub: zh ? '粘贴剪贴板的内容' : 'Paste from the clipboard', onPress: onPaste},
  ];
  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={onClose}
      onDismiss={() => {
        const fn = pendingRef.current;
        pendingRef.current = null;
        fn?.();
      }}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity
          testID={TestIds.composer.attachSheet}
          accessibilityLabel={TestIds.composer.attachSheet}
          activeOpacity={1}
          style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
          <Text style={[styles.title, {color: pal.fg}]}>{zh ? '添加附件' : 'Add attachment'}</Text>
          {rows.map((r, i) => (
            <TouchableOpacity
              key={r.title}
              accessibilityLabel={`attach-${i}`}
              activeOpacity={0.6}
              onPress={() => {
                pendingRef.current = r.onPress; // run after the sheet dismisses (iOS present-race)
                onClose();
              }}
              style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <View style={[styles.tile, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
                <r.icon size={22} color={pal.fg2} />
              </View>
              <View style={styles.textCol}>
                <Text style={[styles.cardTitle, {color: pal.fg}]}>{r.title}</Text>
                <Text style={[styles.cardSub, {color: pal.fg3}]} numberOfLines={1}>
                  {r.sub}
                </Text>
              </View>
            </TouchableOpacity>
          ))}
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 18, borderTopRightRadius: 18, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 8, paddingBottom: 30},
  grabber: {width: 38, height: 5, borderRadius: 3, alignSelf: 'center', marginBottom: 14, opacity: 0.9},
  title: {fontSize: 22, fontWeight: '700', marginBottom: 14, marginLeft: 2},
  card: {flexDirection: 'row', alignItems: 'center', padding: 12, borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, marginBottom: 10},
  tile: {width: 46, height: 46, borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 14},
  textCol: {flex: 1},
  cardTitle: {fontSize: 16.5, fontWeight: '600'},
  cardSub: {fontSize: 13, marginTop: 2},
});
