// Composer — the Detail input area (MOBILE §4), Termius-style. Types into the
// pane via POST /api/send (a WRITE, gated by the bearer token). A single compact,
// horizontally-scrollable key toolbar (agent-aware context shortcuts + control
// keys) sits above a free-input row; DetailScreen wraps it in a
// KeyboardAvoidingView so it floats above the keyboard instead of being covered.
//
// Color is never used for status here.

import React, {useRef, useState} from 'react';
import {
  ActionSheetIOS,
  ActivityIndicator,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {launchCamera, launchImageLibrary} from 'react-native-image-picker';
import {pick} from '@react-native-documents/picker';
import {StatusName} from '../api/types';
import {SendPayload} from '../api/client';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {ImageMarkup} from './ImageMarkup';
import {MoveKey} from './MoveKey';

function contextKeys(status: StatusName, lang: string): {label: string; payload: SendPayload}[] {
  if (status === 'waiting') {
    return [
      {label: lang === 'zh' ? '1 · 是' : '1 · Yes', payload: {text: '1', enter: true}},
      {label: lang === 'zh' ? '2 · 总是' : '2 · Always', payload: {text: '2', enter: true}},
      {label: lang === 'zh' ? '3 · 否' : '3 · No', payload: {text: '3', enter: true}},
    ];
  }
  return [
    {label: lang === 'zh' ? '继续' : 'Continue', payload: {key: 'Enter'}},
    {label: lang === 'zh' ? '停止' : 'Stop', payload: {key: 'C-c'}},
  ];
}

// Quick control keys in the toolbar; directional nav lives in the floating keypad
// summoned by the keypad button (onOpenKeys).
const CONTROL_KEYS: {label: string; key: string}[] = [
  {label: '⏎', key: 'Enter'},
  {label: 'Ctrl-C', key: 'C-c'},
  {label: 'Esc', key: 'Escape'},
  {label: 'Tab', key: 'Tab'},
];

export function Composer({
  status,
  pal,
  lang,
  enabled = true,
  onSend,
  onUpload,
  onOpenKeys,
}: {
  status: StatusName;
  pal: Palette;
  lang: Lang;
  enabled?: boolean;
  onSend?: (p: SendPayload) => void;
  onUpload?: (uri: string, name: string, type: string) => Promise<string | null>;
  onOpenKeys?: () => void;
}) {
  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [markupUri, setMarkupUri] = useState<string | null>(null);
  // Guard against a double submit: iOS fires onSubmitEditing twice with
  // blurOnSubmit=false (notably after voice dictation), which sent the message
  // twice. Drop a second submit within a short window.
  const lastSubmit = useRef(0);
  const send = (p: SendPayload) => {
    if (enabled && onSend) onSend(p);
  };
  const sendText = () => {
    if (!text) return;
    const now = Date.now();
    if (now - lastSubmit.current < 600) return;
    lastSubmit.current = now;
    send({text, enter: true});
    setText('');
  };

  // Paste is image-aware: if the clipboard holds an image, open the markup editor
  // (annotate → upload → reference by path); otherwise paste the text string.
  const paste = async () => {
    try {
      if (await Clipboard.hasImage()) {
        const raw = await Clipboard.getImagePNG();
        if (raw) {
          setMarkupUri(raw.startsWith('data:') ? raw : `data:image/png;base64,${raw}`);
          return;
        }
      }
    } catch {
      // fall through to text paste
    }
    const s = await Clipboard.getString();
    if (s) setText(t => (t ? t + s : s));
  };

  // Upload a picked file to the Mac and drop its path into the input, so the
  // agent can read it (e.g. "look at /…/screenshot.png").
  const doUpload = async (uri: string, name: string, type: string) => {
    if (!onUpload || uploading) return;
    setUploading(true);
    const path = await onUpload(uri, name, type);
    setUploading(false);
    if (path) setText(t => (t ? t + ' ' + path : path));
  };

  // Attach → photo library / camera / file (iOS action sheet).
  const attach = () => {
    const labels =
      lang === 'zh'
        ? ['照片图库', '拍照', '文件', '取消']
        : ['Photo Library', 'Take Photo', 'File', 'Cancel'];
    ActionSheetIOS.showActionSheetWithOptions(
      {options: labels, cancelButtonIndex: 3},
      async idx => {
        try {
          if (idx === 0) {
            const r = await launchImageLibrary({mediaType: 'photo', quality: 0.8});
            const a = r.assets?.[0];
            if (a?.uri) await doUpload(a.uri, a.fileName ?? 'photo.jpg', a.type ?? 'image/jpeg');
          } else if (idx === 1) {
            const r = await launchCamera({mediaType: 'photo', quality: 0.8, saveToPhotos: false});
            const a = r.assets?.[0];
            if (a?.uri) await doUpload(a.uri, a.fileName ?? 'photo.jpg', a.type ?? 'image/jpeg');
          } else if (idx === 2) {
            const [f]: any = await pick();
            if (f?.uri) await doUpload(f.uri, f.name ?? 'file', f.type ?? 'application/octet-stream');
          }
        } catch {
          // cancelled or unsupported — ignore.
        }
      },
    );
  };

  return (
    <View style={[styles.wrap, {borderTopColor: pal.divider, backgroundColor: pal.bg}, !enabled && styles.disabled]}>
      {/* one compact, scrollable key toolbar (context shortcuts + control keys) */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        keyboardShouldPersistTaps="always"
        contentContainerStyle={styles.keys}>
        <MoveKey pal={pal} enabled={enabled} onKey={k => send({key: k})} />
        {onOpenKeys && (
          <TouchableOpacity
            onPress={onOpenKeys}
            style={[styles.ctlKey, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
            <Text style={[styles.ctlText, {color: pal.fg2}]}>⌨</Text>
          </TouchableOpacity>
        )}
        <TouchableOpacity
          onPress={paste}
          style={[styles.ctlKey, {borderColor: pal.divider}]}>
          <Text style={[styles.ctlText, {color: pal.fg2}]}>{lang === 'zh' ? '粘贴' : 'Paste'}</Text>
        </TouchableOpacity>
        <View style={[styles.sep, {backgroundColor: pal.divider}]} />
        {contextKeys(status, lang).map(k => (
          <TouchableOpacity
            key={k.label}
            onPress={() => send(k.payload)}
            style={[styles.ctxKey, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
            <Text style={[styles.ctxText, {color: pal.fg2}]}>{k.label}</Text>
          </TouchableOpacity>
        ))}
        <View style={[styles.sep, {backgroundColor: pal.divider}]} />
        {CONTROL_KEYS.map(k => (
          <TouchableOpacity
            key={k.label}
            onPress={() => send({key: k.key})}
            style={[styles.ctlKey, {borderColor: pal.divider}]}>
            <Text style={[styles.ctlText, {color: pal.fg2}]}>{k.label}</Text>
          </TouchableOpacity>
        ))}
      </ScrollView>

      {/* attach + free input + send */}
      <View style={styles.inputRow}>
        <TouchableOpacity
          onPress={attach}
          disabled={!enabled || uploading || !onUpload}
          style={[styles.attach, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          {uploading ? (
            <ActivityIndicator size="small" color={pal.fg3} />
          ) : (
            <Text style={[styles.attachText, {color: pal.fg2}]}>+</Text>
          )}
        </TouchableOpacity>
        <TextInput
          value={text}
          onChangeText={setText}
          editable={enabled}
          placeholder={lang === 'zh' ? '输入…' : 'Type a message…'}
          placeholderTextColor={pal.fg3}
          autoCapitalize="none"
          autoCorrect={false}
          returnKeyType="send"
          onSubmitEditing={sendText}
          blurOnSubmit={false}
          style={[styles.input, {backgroundColor: pal.surface, borderColor: pal.divider, color: pal.fg}]}
        />
        <TouchableOpacity
          onPress={sendText}
          disabled={!enabled || !text}
          style={[styles.send, {backgroundColor: text ? '#06B6D4' : pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.sendText, {color: text ? '#fff' : pal.fg3}]}>↑</Text>
        </TouchableOpacity>
      </View>

      {/* clipboard-image → annotate → upload → reference by path */}
      <ImageMarkup
        visible={!!markupUri}
        uri={markupUri}
        lang={lang}
        onCancel={() => setMarkupUri(null)}
        onDone={fileUri => {
          setMarkupUri(null);
          doUpload(fileUri, 'markup.png', 'image/png');
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {borderTopWidth: StyleSheet.hairlineWidth, paddingHorizontal: 10, paddingTop: 8, paddingBottom: 8},
  disabled: {opacity: 0.55},
  keys: {flexDirection: 'row', alignItems: 'center', paddingRight: 8},
  ctxKey: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 8, marginRight: 7},
  ctxText: {fontSize: 13, fontWeight: '600'},
  sep: {width: StyleSheet.hairlineWidth, height: 22, marginHorizontal: 6},
  ctlKey: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 11, paddingVertical: 8, marginRight: 7},
  ctlText: {fontSize: 13, fontFamily: 'Menlo'},
  inputRow: {flexDirection: 'row', alignItems: 'center', marginTop: 8},
  attach: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 8},
  attachText: {fontSize: 24, fontWeight: '400', lineHeight: 26},
  input: {flex: 1, height: 40, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, fontSize: 15},
  send: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  sendText: {fontSize: 19, fontWeight: '700'},
});
