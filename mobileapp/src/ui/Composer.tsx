// Composer — the Detail input area (MOBILE §4), Moshi-style. Types into the pane
// via POST /api/send (a WRITE, gated by the bearer token).
//
// Layout (MOBILE §4):
//   • Resting (keyboard down): a single horizontally-scrollable row of key pills
//     (agent-aware context shortcuts + control keys + arrows + snippets), sitting
//     ABOVE the home-indicator safe area so the edge pills stay tappable.
//   • Typing (keyboard up): tap ⌨ to reveal a compact input field; on iOS the key
//     row docks directly above the keyboard via an InputAccessoryView (replacing
//     iOS's sparse prev/next/done assistant bar), exactly like Moshi.
//
// Color is never used for status here.

import React, {useEffect, useRef, useState} from 'react';
import {
  ActivityIndicator,
  Image,
  InputAccessoryView,
  KeyboardAvoidingView,
  Modal,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import Clipboard from '@react-native-clipboard/clipboard';
import {launchCamera, launchImageLibrary} from 'react-native-image-picker';
import {pick} from '@react-native-documents/picker';
import {SendPayload} from '../api/client';
import {Lang} from '../i18n';
import {TestIds} from '../constants/testIds';
import {Palette, StatusColor} from './theme';
import {ImageMarkup} from './ImageMarkup';
import {SnippetsModal} from './SnippetsModal';
import {SnippetsPicker} from './SnippetsPicker';
import {AttachSheet} from './AttachSheet';
import {HistoryModal} from './HistoryModal';
import {KeyboardIcon, KeyboardDismissIcon, HistoryIcon, ExpandIcon, FileIcon} from './Icons';
import {loadSnippets, saveSnippets} from '../state/snippets';
import {loadHistory, saveHistory, pushHistory} from '../state/history';

// iOS docks the key row on the keyboard via this accessory id (so it replaces the
// default assistant bar instead of stacking another sparse row above it).
const ACCESSORY_ID = 'gtmux-composer-keys';
const ACCENT = '#06B6D4';

// A staged attachment — picked (and, for images, edited) but not yet uploaded. It
// uploads on SEND. `isImage` drives thumbnail vs file-chip rendering.
type Attachment = {id: string; uri: string; name: string; type: string; isImage: boolean};

// A waiting prompt's reply is NOT offered here: the ApprovalCard (shown above the
// composer whenever waiting, from GET /api/options) already renders the agent's
// ACTUAL choices as number chips 1..N. The old hardcoded "1·Yes / 2·Always / 3·No"
// triad here was redundant with that card AND wrong for a plan/question with a
// different option set — so it's removed; the card is the single, accurate reply UI.

// Permanent control-key pills. Tab accepts the agent's highlighted/recommended
// choice or completes its ghost suggestion; ⏎ then SUBMITS that line — a bare
// Enter to the pane, NOT the same as Send (↑), which only submits text typed into
// the composer field. Tab→⏎ sit adjacent so "accept the suggestion, then send it"
// is two taps in the resting row, with no detour through the input box. Ctrl-C
// interrupts, Esc cancels. Directional nav stays removed.
const CONTROL_KEYS: {label: string; key: string}[] = [
  {label: 'Tab', key: 'Tab'},
  {label: '⏎', key: 'Enter'},
  {label: 'Ctrl-C', key: 'C-c'},
  {label: 'Esc', key: 'Escape'},
];

export function Composer({
  pal,
  lang,
  enabled = true,
  returnSends = false,
  onSend,
  onUpload,
}: {
  pal: Palette;
  lang: Lang;
  enabled?: boolean;
  returnSends?: boolean; // D7: when off (default) Return = newline; send via ↑ only
  onSend?: (p: SendPayload) => void;
  onUpload?: (
    uri: string,
    name: string,
    type: string,
    onProgress?: (fraction: number) => void,
  ) => Promise<string | null>;
}) {
  const insets = useSafeAreaInsets();
  const [text, setText] = useState('');
  const [inputH, setInputH] = useState(0); // measured content height, for 1→6 line auto-grow
  // Staged attachments (iMessage-style): picked/edited but NOT uploaded yet. They
  // upload on SEND, together with the typed text — never on pick. `sending` guards
  // the upload; `progress` is the per-attachment fraction; `sendError` surfaces a
  // failure so nothing is lost and the user can retry.
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [sending, setSending] = useState(false);
  const [progress, setProgress] = useState<Record<string, number>>({});
  const [sendError, setSendError] = useState<string | null>(null);
  const attachId = useRef(0);
  const [markupUri, setMarkupUri] = useState<string | null>(null);
  const [snippets, setSnippets] = useState<string[]>([]);
  const [snippetsOpen, setSnippetsOpen] = useState(false); // the picker sheet
  const [manageSnippets, setManageSnippets] = useState(false);
  const [fullCompose, setFullCompose] = useState(false); // B3 ②: full-screen editor
  const [attachOpen, setAttachOpen] = useState(false); // the branded attach bottom sheet
  const [history, setHistory] = useState<string[]>([]);
  const [historyOpen, setHistoryOpen] = useState(false);
  // Moshi-style: the composer rests as a single key row; the text field +
  // keyboard appear only when you tap the ⌨ key (and collapse again with ▾). This
  // keeps the terminal full-height and stops an accidental tap from popping the
  // keyboard. Any action that needs the field (snippets/history/attach) opens it.
  const [composing, setComposing] = useState(false);
  const openCompose = () => setComposing(true);

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
  // Send = upload every staged attachment (with progress), THEN send one message
  // combining the uploaded paths + the typed text. On an upload failure nothing is
  // cleared — the text + attachments stay staged and `sendError` invites a retry.
  const sendText = async () => {
    if (sending) return;
    const body = text.trim();
    if (!body && attachments.length === 0) return;
    const now = Date.now();
    if (now - lastSubmit.current < 600) return;
    lastSubmit.current = now;

    let paths: string[] = [];
    if (attachments.length > 0) {
      if (!onUpload) return;
      setSending(true);
      setSendError(null);
      setProgress({});
      const uploaded: string[] = [];
      for (const att of attachments) {
        const path = await onUpload(att.uri, att.name, att.type, f =>
          setProgress(p => ({...p, [att.id]: f})),
        );
        if (!path) {
          setSending(false);
          setSendError(lang === 'zh' ? '上传失败，点发送重试' : 'Upload failed — tap send to retry');
          return; // keep text + attachments staged
        }
        uploaded.push(path);
      }
      paths = uploaded;
      setSending(false);
    }

    const parts: string[] = [];
    if (body) parts.push(body);
    if (paths.length) parts.push(paths.join('\n'));
    send({text: parts.join('\n'), enter: true});
    if (body) {
      setHistory(h => {
        const next = pushHistory(h, body);
        saveHistory(next);
        return next;
      });
    }
    setText('');
    setAttachments([]);
    setProgress({});
    setSendError(null);
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
    if (s) {
      setText(t => (t ? t + s : s));
      openCompose();
    }
  };

  // Stage a picked/edited item WITHOUT uploading (upload happens on send). Reveal the
  // input so the thumbnail + field appear together, ready for a caption.
  const addAttachment = (uri: string, name: string, type: string, isImage: boolean) => {
    setAttachments(a => [...a, {id: String(attachId.current++), uri, name, type, isImage}]);
    setSendError(null);
    openCompose();
  };
  const removeAttachment = (id: string) => {
    setAttachments(a => a.filter(x => x.id !== id));
    setProgress(p => {
      const next = {...p};
      delete next[id];
      return next;
    });
  };

  // Attach → a branded bottom sheet (AttachSheet). Photos (camera/library/paste)
  // route through the markup EDITOR first (annotate → stage a thumbnail); files stage
  // directly. Nothing uploads until send.
  const pickPhoto = async () => {
    try {
      const r = await launchImageLibrary({mediaType: 'photo', quality: 0.8});
      const a = r.assets?.[0];
      if (a?.uri) setMarkupUri(a.uri); // edit first → onDone stages the thumbnail
    } catch {
      // cancelled or unsupported — ignore.
    }
  };
  const takePhoto = async () => {
    try {
      const r = await launchCamera({mediaType: 'photo', quality: 0.8, saveToPhotos: false});
      const a = r.assets?.[0];
      if (a?.uri) setMarkupUri(a.uri); // edit first → onDone stages the thumbnail
    } catch {
      // cancelled or unsupported — ignore.
    }
  };
  const pickFile = async () => {
    try {
      const [f]: any = await pick();
      if (f?.uri) addAttachment(f.uri, f.name ?? 'file', f.type ?? 'application/octet-stream', false);
    } catch {
      // cancelled or unsupported — ignore.
    }
  };

  // A pill in the key row. `glyph` keys are square-ish single symbols; `text` keys
  // size to their label. All are filled (surface) with a hairline border.
  const Key = ({
    children,
    onPress,
    glyph,
    fg,
    activeBg,
    testID,
    icon,
  }: {
    children: React.ReactNode;
    onPress: () => void;
    glyph?: boolean;
    fg?: string;
    activeBg?: boolean;
    testID?: string;
    icon?: boolean; // render children directly (an SVG), not wrapped in <Text>
  }) => (
    <TouchableOpacity
      testID={testID}
      accessibilityLabel={testID}
      onPress={onPress}
      activeOpacity={0.7}
      style={[
        styles.key,
        icon && styles.keyIcon, // tighter padding so the (bigger) glyph isn't dwarfed
        {
          backgroundColor: activeBg ? ACCENT : pal.surface,
          borderColor: activeBg ? ACCENT : pal.divider,
        },
      ]}>
      {icon ? (
        children
      ) : (
        <Text style={[glyph ? styles.keyGlyph : styles.keyText, {color: activeBg ? '#fff' : fg || pal.fg2}]} numberOfLines={1}>
          {children}
        </Text>
      )}
    </TouchableOpacity>
  );

  // The key row (context shortcuts + control keys + arrows + snippets). Always
  // visible — when composing it sits just above the input field, so the special
  // keys AND the ▾ dismiss stay reachable while the keyboard is up.
  // Resting row — decluttered + grouped: ⌨ | Tab ⏎ Ctrl-C Esc | 快捷短语▾ 历史.
  // Waiting responses live in the ApprovalCard (real /api/options choices), never
  // as hardcoded keys here. Snippets are a picker (not a flat list); attach/
  // compose/paste live in the input row + attach sheet; directional keypads were
  // removed.
  const renderKeys = () => (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      keyboardShouldPersistTaps="always"
      contentContainerStyle={styles.keys}>
      <Key onPress={() => setComposing(c => !c)} icon activeBg={composing} testID={TestIds.composer.keyboard}>
        {composing ? (
          <KeyboardDismissIcon size={28} color="#fff" />
        ) : (
          <KeyboardIcon size={28} color={pal.fg2} />
        )}
      </Key>
      <View style={[styles.sep, {backgroundColor: pal.divider}]} />
      {CONTROL_KEYS.map(k => (
        <Key key={k.label} onPress={() => send({key: k.key})}>
          {k.label}
        </Key>
      ))}
      <View style={[styles.sep, {backgroundColor: pal.divider}]} />
      <Key onPress={() => setSnippetsOpen(true)} testID={TestIds.composer.snippets}>
        {lang === 'zh' ? '快捷短语 ▾' : 'Snippets ▾'}
      </Key>
      <Key onPress={() => setHistoryOpen(true)} icon testID={TestIds.composer.history}>
        <HistoryIcon size={28} color={pal.fg2} />
      </Key>
    </ScrollView>
  );

  // attach + free input + send — shown while composing (tap ⌨ to reveal). The
  // TextInput auto-focuses on mount so the keyboard rises exactly then; on iOS the
  // key row rides up docked to the keyboard (inputAccessoryViewID). Staged
  // attachments show as a thumbnail strip ABOVE the input (the field wraps below),
  // with a per-thumbnail upload % overlay during send and a × to remove before it.
  const canSend = text.trim().length > 0 || attachments.length > 0;
  const renderInputRow = () => (
    <View>
      {attachments.length > 0 && (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          keyboardShouldPersistTaps="always"
          contentContainerStyle={styles.thumbStrip}>
          {attachments.map(att => (
            <View key={att.id} style={[styles.thumbWrap, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
              {att.isImage ? (
                <Image source={{uri: att.uri}} style={styles.thumbImg} resizeMode="cover" />
              ) : (
                <View style={styles.thumbFile}>
                  <FileIcon size={20} color={pal.fg2} />
                  <Text numberOfLines={1} style={[styles.thumbName, {color: pal.fg2}]}>
                    {att.name}
                  </Text>
                </View>
              )}
              {sending && progress[att.id] != null ? (
                <View style={styles.thumbOverlay}>
                  <Text style={styles.thumbPct}>{Math.round((progress[att.id] || 0) * 100)}%</Text>
                </View>
              ) : (
                !sending && (
                  <TouchableOpacity
                    onPress={() => removeAttachment(att.id)}
                    accessibilityLabel={`attach-remove-${att.id}`}
                    hitSlop={{top: 8, bottom: 8, left: 8, right: 8}}
                    style={styles.thumbRemove}>
                    <Text style={styles.thumbRemoveX}>×</Text>
                  </TouchableOpacity>
                )
              )}
            </View>
          ))}
        </ScrollView>
      )}
      {sendError && <Text style={[styles.sendError, {color: StatusColor.waiting}]}>{sendError}</Text>}
      <View style={styles.inputRow}>
        <TouchableOpacity
          testID={TestIds.composer.attach}
          accessibilityLabel={TestIds.composer.attach}
          onPress={() => setAttachOpen(true)}
          disabled={!enabled || sending || !onUpload}
          style={[styles.attach, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.attachText, {color: pal.fg2}]}>+</Text>
        </TouchableOpacity>
        <TextInput
        testID={TestIds.composer.input}
        value={text}
        onChangeText={setText}
        editable={enabled}
        autoFocus
        placeholder={lang === 'zh' ? '输入…' : 'Type a message…'}
        placeholderTextColor={pal.fg3}
        autoCapitalize="none"
        autoCorrect={false}
        keyboardAppearance={pal.bg === '#ffffff' ? 'light' : 'dark'}
        inputAccessoryViewID={Platform.OS === 'ios' ? ACCESSORY_ID : undefined}
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
      {/* expand to the full-screen editor for long messages */}
      <TouchableOpacity
        onPress={() => setFullCompose(true)}
        testID={TestIds.composer.expand}
        accessibilityLabel={TestIds.composer.expand}
        style={[styles.expand, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
        <ExpandIcon size={20} color={pal.fg2} />
      </TouchableOpacity>
      <TouchableOpacity
        testID={TestIds.composer.send}
        accessibilityLabel={TestIds.composer.send}
        onPress={sendText}
        disabled={!enabled || sending || !canSend}
        style={[styles.send, {backgroundColor: canSend ? ACCENT : pal.surface, borderColor: canSend ? ACCENT : pal.divider}]}>
        {sending ? (
          <ActivityIndicator size="small" color="#fff" />
        ) : (
          <Text style={[styles.sendText, {color: canSend ? '#fff' : pal.fg3}]}>↑</Text>
        )}
      </TouchableOpacity>
      </View>
    </View>
  );

  return (
    <View
      style={[
        styles.wrap,
        {
          borderTopColor: pal.divider,
          backgroundColor: pal.bg,
          // sit above the home indicator when resting; the keyboard covers it
          // while composing, so collapse the inset then.
          paddingBottom: composing ? 8 : Math.max(8, insets.bottom),
        },
        !enabled && styles.disabled,
      ]}>
      {renderKeys()}
      {composing && renderInputRow()}

      {/* An empty accessory replaces (and thus suppresses) iOS's sparse
          prev/next/done assistant bar above the keyboard — the key row above the
          input field already covers those actions. */}
      {Platform.OS === 'ios' && (
        <InputAccessoryView nativeID={ACCESSORY_ID}>
          <View style={styles.accessory} />
        </InputAccessoryView>
      )}

      {/* recall a previously-sent message → load it into the input for editing */}
      <HistoryModal
        visible={historyOpen}
        history={history}
        pal={pal}
        lang={lang}
        onPick={h => {
          setText(t => (t ? t + h : h));
          setHistoryOpen(false);
          openCompose();
        }}
        onDelete={i => {
          setHistory(h => {
            const next = h.filter((_, idx) => idx !== i);
            saveHistory(next);
            return next;
          });
        }}
        onClear={() => {
          setHistory([]);
          saveHistory([]);
        }}
        onClose={() => setHistoryOpen(false)}
      />

      {/* attach → photo / camera / file / paste — branded bottom sheet (icon + label) */}
      <AttachSheet
        visible={attachOpen}
        pal={pal}
        lang={lang}
        onClose={() => setAttachOpen(false)}
        onPhoto={pickPhoto}
        onCamera={takePhoto}
        onFile={pickFile}
        onPaste={paste}
      />

      {/* pick a snippet to send (or jump to manage) — branded bottom sheet */}
      <SnippetsPicker
        visible={snippetsOpen}
        snippets={snippets}
        pal={pal}
        lang={lang}
        onPick={s => {
          send({text: s, enter: true});
          setSnippetsOpen(false);
        }}
        onManage={() => {
          setSnippetsOpen(false);
          setManageSnippets(true);
        }}
        onClose={() => setSnippetsOpen(false)}
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
              <Text style={[styles.fcAction, {color: text ? ACCENT : pal.fg3, fontWeight: '700'}]}>
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
            placeholder={lang === 'zh' ? '输入…' : 'Type a message…'}
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
          addAttachment(fileUri, 'markup.png', 'image/png', true);
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {borderTopWidth: StyleSheet.hairlineWidth, paddingHorizontal: 10, paddingTop: 8},
  disabled: {opacity: 0.55},
  keys: {flexDirection: 'row', alignItems: 'center', paddingRight: 8, paddingVertical: 2},
  // one unified pill: ≥44pt touch target, filled, pill-rounded (Moshi-like).
  key: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 11,
    paddingHorizontal: 13,
    height: 40,
    minWidth: 40,
    marginRight: 8,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  keyIcon: {paddingHorizontal: 6}, // icon keys (⌨/history): tight box around the glyph
  keyText: {fontSize: 14, fontWeight: '600'},
  keyGlyph: {fontSize: 17, fontWeight: '600'},
  sep: {width: StyleSheet.hairlineWidth, height: 24, marginHorizontal: 6},
  accessory: {height: 0}, // empty: only there to suppress iOS's default assistant bar
  inputRow: {flexDirection: 'row', alignItems: 'flex-end', marginTop: 8},
  attach: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 8},
  attachText: {fontSize: 24, fontWeight: '400', lineHeight: 26},
  input: {flex: 1, minHeight: 40, borderWidth: StyleSheet.hairlineWidth, borderRadius: 11, paddingHorizontal: 12, paddingTop: 10, paddingBottom: 10, fontSize: 15},
  // Full-screen-compose button: its own style (not `attach`) so its gaps stay EVEN —
  // owns the 8pt gap to its left (the input) and lets `send`'s marginLeft own the gap
  // to its right, so + · input · ⤢ · ↑ are all 8pt apart (was 8 · 0 · 16).
  expand: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  fcWrap: {flex: 1},
  fcBar: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 16, paddingTop: 56, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  fcTitle: {fontSize: 13, fontWeight: '600'},
  fcAction: {fontSize: 15},
  fcInput: {flex: 1, fontSize: 16, lineHeight: 22, padding: 16, fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace'},
  send: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  sendText: {fontSize: 19, fontWeight: '700'},
  // Staged-attachment thumbnail strip (above the input, so the field wraps below).
  thumbStrip: {flexDirection: 'row', alignItems: 'center', paddingTop: 8, paddingRight: 8},
  thumbWrap: {width: 60, height: 60, borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, marginRight: 8, overflow: 'hidden'},
  thumbImg: {width: '100%', height: '100%'},
  thumbFile: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 4},
  thumbName: {fontSize: 8, marginTop: 3, maxWidth: 54},
  // dim overlay with the live upload %, shown per thumbnail during send.
  thumbOverlay: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.5)', alignItems: 'center', justifyContent: 'center'},
  thumbPct: {color: '#fff', fontSize: 13, fontWeight: '800'},
  // the × remove chip, top-right of a thumbnail (hidden while uploading).
  thumbRemove: {position: 'absolute', top: 2, right: 2, width: 20, height: 20, borderRadius: 10, backgroundColor: 'rgba(0,0,0,0.6)', alignItems: 'center', justifyContent: 'center'},
  thumbRemoveX: {color: '#fff', fontSize: 15, fontWeight: '700', lineHeight: 17},
  sendError: {fontSize: 12, fontWeight: '600', marginTop: 8, marginLeft: 2},
});
