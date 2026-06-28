// Composer — the Detail input area (MOBILE §4), Termius-style. Types into the
// pane via POST /api/send (a WRITE, gated by the bearer token). A single compact,
// horizontally-scrollable key toolbar (agent-aware context shortcuts + control
// keys) sits above a free-input row; DetailScreen wraps it in a
// KeyboardAvoidingView so it floats above the keyboard instead of being covered.
//
// Color is never used for status here.

import React, {useEffect, useRef, useState} from 'react';
import {
  ActionSheetIOS,
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Linking,
  Modal,
  Platform,
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
import {Lang, makeT} from '../i18n';
import {mergeVoiceText, useVoiceInput} from '../voice/useVoiceInput';
import {TestIds} from '../constants/testIds';
import {Palette} from './theme';
import {ImageMarkup} from './ImageMarkup';
import {MoveKey} from './MoveKey';
import {SnippetsModal} from './SnippetsModal';
import {HistoryModal} from './HistoryModal';
import {loadSnippets, saveSnippets} from '../state/snippets';
import {loadHistory, saveHistory, pushHistory} from '../state/history';

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
  returnSends = false,
  onSend,
  onUpload,
  onOpenKeys,
}: {
  status: StatusName;
  pal: Palette;
  lang: Lang;
  enabled?: boolean;
  returnSends?: boolean; // D7: when off (default) Return = newline; send via ↑ only
  onSend?: (p: SendPayload) => void;
  onUpload?: (uri: string, name: string, type: string) => Promise<string | null>;
  onOpenKeys?: () => void;
}) {
  const [text, setText] = useState('');
  const [inputH, setInputH] = useState(0); // measured content height, for 1→6 line auto-grow
  const [uploading, setUploading] = useState(false);
  const [markupUri, setMarkupUri] = useState<string | null>(null);
  const [snippets, setSnippets] = useState<string[]>([]);
  const [manageSnippets, setManageSnippets] = useState(false);
  const [fullCompose, setFullCompose] = useState(false); // B3 ②: full-screen editor
  const [history, setHistory] = useState<string[]>([]);
  const [historyOpen, setHistoryOpen] = useState(false);
  const t = makeT(lang);

  // Voice dictation: capture whatever's typed as the base when listening starts,
  // then stream the transcript after it (partials replace the live region).
  const voiceBase = useRef('');
  const voice = useVoiceInput(lang, recognized => {
    setText(mergeVoiceText(voiceBase.current, recognized));
  });
  const toggleVoice = () => {
    if (!voice.listening) voiceBase.current = text;
    voice.toggle();
  };
  // Surface a denied/failed mic once, with a jump to Settings to grant access.
  useEffect(() => {
    if (!voice.error) return;
    Alert.alert(t('voiceDeniedTitle'), t('voiceDeniedBody'), [
      {text: t('cancel'), style: 'cancel'},
      {text: t('openSettings'), onPress: () => Linking.openSettings()},
    ]);
  }, [voice.error]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    loadSnippets().then(setSnippets);
    loadHistory().then(setHistory);
  }, []);
  const updateSnippets = (list: string[]) => {
    setSnippets(list);
    saveSnippets(list);
  };
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
    setHistory(h => {
      const next = pushHistory(h, text);
      saveHistory(next);
      return next;
    });
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
    if (s) setText(prev => (prev ? prev + s : s));
  };

  // Upload a picked file to the Mac and drop its path into the input, so the
  // agent can read it (e.g. "look at /…/screenshot.png").
  const doUpload = async (uri: string, name: string, type: string) => {
    if (!onUpload || uploading) return;
    setUploading(true);
    const path = await onUpload(uri, name, type);
    setUploading(false);
    if (path) setText(prev => (prev ? prev + ' ' + path : path));
  };

  // Attach → photo library / camera / file (iOS action sheet).
  const attach = () => {
    const labels =
      lang === 'zh'
        ? ['相册', '拍照', '文件', '取消']
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
        <TouchableOpacity
          onPress={() => setFullCompose(true)}
          style={[styles.ctlKey, {borderColor: pal.divider}]}>
          <Text style={[styles.ctlText, {color: pal.fg2}]}>⤢</Text>
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
        {/* input history: recall a previously-sent message (mockup §10 "↑ 历史") */}
        <View style={[styles.sep, {backgroundColor: pal.divider}]} />
        <TouchableOpacity
          onPress={() => setHistoryOpen(true)}
          style={[styles.ctlKey, {borderColor: pal.divider}]}>
          <Text style={[styles.ctlText, {color: pal.fg2}]}>{lang === 'zh' ? '↑ 历史' : '↑ History'}</Text>
        </TouchableOpacity>
        {/* saved snippets: one-tap habitual sends + a manage button */}
        <View style={[styles.sep, {backgroundColor: pal.divider}]} />
        {snippets.map(s => (
          <TouchableOpacity
            key={s}
            onPress={() => send({text: s, enter: true})}
            style={[styles.ctxKey, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
            <Text style={[styles.ctxText, {color: pal.fg2}]} numberOfLines={1}>
              {s}
            </Text>
          </TouchableOpacity>
        ))}
        <TouchableOpacity
          onPress={() => setManageSnippets(true)}
          style={[styles.ctlKey, {borderColor: pal.divider}]}>
          <Text style={[styles.ctlText, {color: pal.fg3}]}>✎</Text>
        </TouchableOpacity>
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
        {voice.supported && (
          <TouchableOpacity
            testID={TestIds.composer.mic}
            accessibilityLabel={TestIds.composer.mic}
            onPress={toggleVoice}
            disabled={!enabled}
            style={[
              styles.mic,
              {
                backgroundColor: voice.listening ? '#06B6D4' : pal.surface,
                borderColor: voice.listening ? '#06B6D4' : pal.divider,
              },
            ]}>
            <Text style={[styles.micText, {color: voice.listening ? '#fff' : pal.fg2}]}>
              {voice.listening ? '■' : '🎤'}
            </Text>
          </TouchableOpacity>
        )}
        <TextInput
          testID={TestIds.composer.input}
          value={text}
          onChangeText={setText}
          editable={enabled}
          placeholder={voice.listening ? t('voiceListening') : lang === 'zh' ? '输入…' : 'Type a message…'}
          placeholderTextColor={pal.fg3}
          autoCapitalize="none"
          autoCorrect={false}
          // D7 core fix: multiline so Return inserts a newline; sending is the ↑
          // button only (unless the user opted into "Return sends"). Auto-grows
          // 1→6 lines, then scrolls inside.
          multiline
          textAlignVertical="top"
          onContentSizeChange={e => setInputH(e.nativeEvent.contentSize.height)}
          returnKeyType={returnSends ? 'send' : 'default'}
          onSubmitEditing={returnSends ? sendText : undefined}
          blurOnSubmit={false}
          style={[
            styles.input,
            {
              backgroundColor: pal.surface,
              borderColor: pal.divider,
              color: pal.fg,
              height: Math.min(132, Math.max(40, inputH + 16)),
            },
          ]}
        />
        <TouchableOpacity
          testID={TestIds.composer.send}
          accessibilityLabel={TestIds.composer.send}
          onPress={sendText}
          disabled={!enabled || !text}
          style={[styles.send, {backgroundColor: text ? '#06B6D4' : pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.sendText, {color: text ? '#fff' : pal.fg3}]}>↑</Text>
        </TouchableOpacity>
      </View>

      {/* recall a previously-sent message → load it into the input for editing */}
      <HistoryModal
        visible={historyOpen}
        history={history}
        pal={pal}
        lang={lang}
        onPick={h => {
          setText(prev => (prev ? prev + h : h));
          setHistoryOpen(false);
        }}
        onClear={() => {
          setHistory([]);
          saveHistory([]);
        }}
        onClose={() => setHistoryOpen(false)}
      />

      {/* manage saved snippets */}
      <SnippetsModal
        visible={manageSnippets}
        snippets={snippets}
        pal={pal}
        lang={lang}
        onChange={updateSnippets}
        onClose={() => setManageSnippets(false)}
      />

      {/* B3 ②: full-screen compose — a big monospace editor for long replies.
          Return = newline here too; ⌘⏎ (hardware kbd) or the Send button sends. */}
      <Modal visible={fullCompose} animationType="slide" onRequestClose={() => setFullCompose(false)}>
        <KeyboardAvoidingView
          behavior={Platform.OS === 'ios' ? 'padding' : undefined}
          style={[styles.fcWrap, {backgroundColor: pal.bg}]}>
          <View style={[styles.fcBar, {borderBottomColor: pal.divider}]}>
            <TouchableOpacity onPress={() => setFullCompose(false)} hitSlop={{top: 10, bottom: 10, left: 10, right: 10}}>
              <Text style={[styles.fcAction, {color: pal.fg2}]}>{lang === 'zh' ? '收起' : 'Done'}</Text>
            </TouchableOpacity>
            <Text style={[styles.fcTitle, {color: pal.fg3}]}>{lang === 'zh' ? '撰写' : 'Compose'}</Text>
            <TouchableOpacity
              disabled={!enabled || !text}
              onPress={() => { sendText(); setFullCompose(false); }}>
              <Text style={[styles.fcAction, {color: text ? '#06B6D4' : pal.fg3, fontWeight: '700'}]}>
                {lang === 'zh' ? '发送' : 'Send'}
              </Text>
            </TouchableOpacity>
          </View>
          <TextInput
            value={text}
            onChangeText={setText}
            editable={enabled}
            multiline
            autoFocus
            textAlignVertical="top"
            placeholder={lang === 'zh' ? '输入…（回车换行，⌘⏎ 发送）' : 'Type… (Return = newline, ⌘⏎ to send)'}
            placeholderTextColor={pal.fg3}
            keyboardAppearance={pal.bg === '#ffffff' ? 'light' : 'dark'}
            onKeyPress={e => {
              // hardware keyboard ⌘⏎ sends (soft keyboard has no modifiers).
              const ne: any = e.nativeEvent;
              if (ne.key === 'Enter' && (ne.metaKey || ne.ctrlKey)) { sendText(); setFullCompose(false); }
            }}
            style={[styles.fcInput, {color: pal.fg}]}
          />
        </KeyboardAvoidingView>
      </Modal>

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
  inputRow: {flexDirection: 'row', alignItems: 'flex-end', marginTop: 8},
  attach: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 8},
  attachText: {fontSize: 24, fontWeight: '400', lineHeight: 26},
  mic: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 8},
  micText: {fontSize: 17, lineHeight: 20},
  input: {flex: 1, minHeight: 40, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, paddingTop: 10, paddingBottom: 10, fontSize: 15},
  fcWrap: {flex: 1},
  fcBar: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 16, paddingTop: 56, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  fcTitle: {fontSize: 13, fontWeight: '600'},
  fcAction: {fontSize: 15},
  fcInput: {flex: 1, fontSize: 16, lineHeight: 22, padding: 16, fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace'},
  send: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  sendText: {fontSize: 19, fontWeight: '700'},
});
