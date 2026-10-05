import React, {useEffect, useReducer, useRef, useState} from 'react';
import {Animated, Easing, Keyboard, KeyboardAvoidingView, Modal, Platform, ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {GtmuxClient, SessionCreated, SessionCreateError} from '../api/client';
import {SizeClass} from './layout';
import {Lang} from '../i18n';
import {Palette, StatusColor} from './theme';
import {MODAL_ORIENTATIONS} from './modalOrientations';

export function normalizedSessionName(name: string): string { return name.trim().replace(/[.:]/g, '-'); }

// The form opens already where the keyboard will put it, and moves with the keyboard, never
// after it (2026-10-05). A KeyboardAvoidingView moved it in ONE FRAME once the keyboard
// began, while the Modal faded it in at the bottom: it showed at the bottom, then jumped,
// and the keyboard slid up after; on the first open after launch it sat at the bottom for
// 0.6s first (%6's frame-by-frame recordings). So on a phone the form is lifted by the
// keyboard's height from its first frame: the last height this app saw, or before any, a
// guess from the screen; the real height, when the keyboard announces it, corrects the
// lift on the keyboard's own duration and curve.
// Kept per screen height: a portrait keyboard is twice a landscape one, and the form opened
// sideways once sized itself by the portrait height it had last seen (%6, 2026-10-06).
const seenKeyboard = new Map<number, number>();
/** The keyboard height to lift the form by before the keyboard has said: the last one, or a guess. */
export function expectedKeyboard(screenHeight: number): number {
  return seenKeyboard.get(screenHeight) || Math.round(screenHeight * 0.4);
}
/** For tests: forget the last keyboard height. */
export function forgetKeyboard(): void { seenKeyboard.clear(); }
// Below this window height (a phone held sideways: 440 at most) the form is laid out short.
const SHORT_WINDOW = 500;
// When the form gives up on a keyboard and settles at the bottom. Settling and then a late
// keyboard lifting it again is the two-step motion this form exists to avoid, so it never
// settles while a keyboard may still come (%6 and HQ, review of #1353's first version,
// which settled after a flat 900 ms):
// - the focus did not take (no onFocus within FOCUS_GRACE_MS): no keyboard will come;
// - the focus took but no keyboard spoke in KEYBOARD_GRACE_MS: a hardware keyboard, most
//   likely. A software keyboard starts moving on focus; the slowest measured was the first
//   open after launch, 0.6s from open (%6), so this waits five times that.
export const FOCUS_GRACE_MS = 500;
export const KEYBOARD_GRACE_MS = 3000;
// The iOS keyboard's curve, as near as a cubic Bezier gets.
const KEYBOARD_EASING = Easing.bezier(0.38, 0.7, 0.125, 1);

export function NewSessionSheet({visible, client, macName, lang, pal, layout = 'compact', onClose, onCreated, onDismiss, onCheckSessions}: {
  layout?: SizeClass; visible: boolean; client: GtmuxClient; macName: string; lang: Lang; pal: Palette;
  onClose: () => void; onCreated: (result: SessionCreated) => void; onDismiss: () => void; onCheckSessions: () => void;
}) {
  const zh = lang === 'zh';
  const regular = layout === 'regular';
  const {height} = useWindowDimensions();
  // Lift the form with the keyboard ourselves on a phone; iPad's centred form keeps the
  // avoider (it rarely meets the keyboard, and when it does it has room).
  const follow = Platform.OS === 'ios' && !regular;
  // A phone held sideways leaves about 220 points above the keyboard, and the form needs
  // about 300: the header, Cancel and the field pushed the explanation and the label out
  // and cut the top off the field (%6, 2026-10-06). Laid out short, the field and Create
  // share a row and the explanation moves below them, where the form may scroll.
  const short = follow && height < SHORT_WINDOW;
  // The form's height depends on the keyboard's, which is known only once it says it.
  const [, keyboardSaid] = useReducer((n: number) => n + 1, 0);
  const lift = useRef(new Animated.Value(follow ? -expectedKeyboard(height) : 0)).current;
  // Whether the name field's focus took (its onFocus came): if it did, a keyboard may
  // still come, however late, and the form must not settle under it.
  const focused = useRef(false);
  // Whether a keyboard has said its height for this opening, and how to settle without one.
  const spoke = useRef(false);
  const settle = useRef<() => void>(() => {});
  useEffect(() => {
    if (!follow || !visible) return;
    focused.current = false;
    // A keyboard already on screen is where the form goes, now.
    const already = Keyboard.isVisible?.() ? Keyboard.metrics?.()?.height : undefined;
    spoke.current = !!already;
    if (already) seenKeyboard.set(height, already);
    lift.setValue(-(already ?? expectedKeyboard(height)));
    const move = (to: number, duration: number) =>
      Animated.timing(lift, {toValue: to, duration: duration || 250, easing: KEYBOARD_EASING, useNativeDriver: true}).start();
    // No software keyboard may come at all (a hardware keyboard attached, or the focus did
    // not take): then nothing corrects the guess, and the form would hang above an empty
    // band. It settles at the bottom then, but only when no keyboard can still come (see
    // FOCUS_GRACE_MS).
    // The no-focus check is scheduled by the focus itself (below), so a busy JS thread that
    // delays the focus delays the check with it.
    settle.current = () => { if (!spoke.current) move(0, 250); };
    const noKeyboard = setTimeout(() => settle.current(), KEYBOARD_GRACE_MS);
    const show = Keyboard.addListener('keyboardWillShow', e => {
      spoke.current = true;
      const was = expectedKeyboard(height);
      seenKeyboard.set(height, e.endCoordinates.height);
      if (e.endCoordinates.height !== was) keyboardSaid();
      move(-e.endCoordinates.height, e.duration);
    });
    const hide = Keyboard.addListener('keyboardWillHide', e => move(0, e.duration));
    return () => { settle.current = () => {}; clearTimeout(noKeyboard); show.remove(); hide.remove(); };
  }, [follow, visible, height, lift]);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<SessionCreateError | null>(null);
  const inFlight = useRef(false);
  const request = useRef<{id: string; name: string} | null>(null);
  const alive = useRef(true);
  const input = useRef<TextInput>(null);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  // The keyboard rises WITH the form, not after it. Focusing on the Modal's onShow waited
  // for the fade to finish: the form settled at the bottom, then the keyboard pushed it up
  // a second time, a two-step bounce the commander saw on every open (2026-10-05). Focused
  // on the frame after the form mounts, the keyboard and the fade start together, and the
  // keyboard avoider carries the form up in one motion.
  useEffect(() => {
    if (!visible) return;
    let noFocus: ReturnType<typeof setTimeout> | undefined;
    const frame = requestAnimationFrame(() => {
      input.current?.focus();
      // A focus that did not take brings no keyboard: settle. Timed from the focus call,
      // not the open, and judged by the input's own focus state as well as its onFocus
      // event: on a busy JS thread (the first open after launch) the event can queue
      // behind this timer, and settling then would drop the form under a keyboard that is
      // already coming (%6's review of #1353). isFocused() is set by focus() itself.
      noFocus = setTimeout(() => {
        if (!focused.current && !input.current?.isFocused?.()) settle.current();
      }, FOCUS_GRACE_MS);
    });
    return () => {
      cancelAnimationFrame(frame);
      if (noFocus) clearTimeout(noFocus);
    };
  }, [visible]);
  const normalized = normalizedSessionName(name);
  const uncertain = !!failure && (failure.status === 0 || failure.code === 'create_failed');
  const blocked = !!failure && ['unsupported', 'owner_only', 'unauthorized'].includes(failure.code);
  const errorText = !failure ? '' : failure.code === 'name_exists'
    ? (zh ? '这个名称已被使用，请换一个名称。' : 'That name is already in use. Choose another.')
    : failure.code === 'unsupported'
    ? (zh ? '请先更新这台 Mac 上的 gtmux，再创建会话。' : 'Update gtmux on this Mac to create sessions from the app.')
    : failure.code === 'owner_only' || failure.code === 'unauthorized'
    ? (zh ? '无法创建会话，请检查与这台 Mac 的配对。' : 'Creation is not authorized. Check your pairing with this Mac.')
    : uncertain
    ? (zh ? '未收到创建结果。重试将核对同一请求，也可先查看会话列表。' : 'Creation could not be confirmed. Retry checks the same request, or check the session list first.')
    : (zh ? '无法创建会话，请检查名称后重试。' : 'Could not create the session. Check the name and try again.');
  const create = async () => {
    if (inFlight.current || blocked) return;
    const nextName = normalizedSessionName(name);
    if (!request.current || request.current.name !== nextName) {
      request.current = {name: nextName, id: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}-${Math.random().toString(36).slice(2, 12)}`};
    }
    inFlight.current = true;
    setBusy(true); setFailure(null);
    try {
      const result = await client.createSession(nextName, request.current.id);
      if (alive.current) onCreated(result);
    } catch (e) {
      if (alive.current) setFailure(e instanceof SessionCreateError ? e : new SessionCreateError(0, 'uncertain'));
    } finally {
      inFlight.current = false;
      if (alive.current) setBusy(false);
    }
  };
  const close = () => { if (!inFlight.current) onClose(); };
  const explain = <Text style={[styles.hint, {color: pal.fg2}]}>{zh ? '在这台 Mac 上新开一个 tmux 会话，并在这里打开它的终端，你可以在里面启动 agent。' : 'Starts a new tmux session on this Mac and opens its terminal here, so you can start an agent in it.'}</Text>;
  const field = (
    <TextInput ref={input} testID="new-session-name" accessibilityLabel={zh ? '会话名称（可选）' : 'Session name (optional)'}
      value={name} onChangeText={value => { setName(value); setFailure(null); }} editable={!busy && !uncertain}
      placeholder={zh ? '自动命名' : 'Automatic name'} placeholderTextColor={pal.fg3}
      style={[styles.input, short && styles.grow, {color: pal.fg, backgroundColor: pal.raised, borderColor: pal.divLoud}]}
      selectionColor={StatusColor.working} maxLength={80} autoCapitalize="none" autoCorrect={false} returnKeyType="done" onSubmitEditing={create}
      onFocus={() => { focused.current = true; }} />
  );
  const createButton = (
    <TouchableOpacity testID="new-session-create" accessibilityRole="button" accessibilityState={{disabled: busy || blocked, busy}}
      activeOpacity={0.6} disabled={busy || blocked} onPress={create} style={[styles.create, short && styles.createShort, {backgroundColor: StatusColor.working, opacity: busy || blocked ? 0.5 : 1}]}>
      <Text style={styles.createText}>{busy ? (zh ? '正在创建…' : 'Creating…') : uncertain ? (zh ? '重试' : 'Retry') : (zh ? '创建并打开' : 'Create and open')}</Text>
    </TouchableOpacity>
  );
  // One form, two carriers: lifted with the keyboard on a phone, avoided on iPad.
  const form = (
      <SafeAreaView edges={regular ? ['top', 'bottom'] : ['bottom']} style={[styles.bounds, {maxHeight: short
        ? Math.max(120, height - 16 - expectedKeyboard(height)) // sideways: no status bar above the form
        : Math.max(180, height - 64 - (follow ? expectedKeyboard(height) : 0))}]}>
        <View accessibilityViewIsModal style={[styles.sheet, {backgroundColor: pal.surface, borderColor: pal.divLoud}]}>
          <View style={[styles.header, styles.content]}>
              <Text style={[styles.title, {color: pal.fg}]}>{zh ? '新建会话' : 'New session'}</Text>
              <TouchableOpacity onPress={close} disabled={busy} accessibilityRole="button" accessibilityState={{disabled: busy}} style={styles.cancel}>
                <Text style={{color: busy ? pal.fg3 : pal.fg2}}>{zh ? '取消' : 'Cancel'}</Text>
              </TouchableOpacity>
            </View>
          <ScrollView keyboardShouldPersistTaps="always" style={styles.body} contentContainerStyle={[styles.bodyContent, short && styles.bodyShort]}>
            <Text style={[styles.mac, {color: pal.fg2}]}>{macName}</Text>
            {!short && explain}
            <Text style={[styles.label, short && styles.labelShort, {color: pal.fg}]}>{zh ? '会话名称（可选）' : 'Session name (optional)'}</Text>
            {short ? <View testID="new-session-row" style={styles.row}>{field}{createButton}</View> : field}
            {normalized !== name.trim() && <Text style={[styles.hint, {color: pal.fg2}]}>{zh ? '创建为：' : 'Will be named: '}{normalized}</Text>}
            {!!failure && <Text accessibilityRole="alert" testID="new-session-error" style={[styles.error, {color: pal.fg}]}>{errorText}</Text>}
            {uncertain && <TouchableOpacity onPress={onCheckSessions} style={styles.check} accessibilityRole="button">
              <Text style={{color: StatusColor.working}}>{zh ? '查看会话列表' : 'Check sessions'}</Text>
            </TouchableOpacity>}
            {short && explain}
          </ScrollView>
          {!short && <View style={styles.footer}>{createButton}</View>}
        </View>
      </SafeAreaView>
  );
  return <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={visible} transparent animationType="fade" onDismiss={onDismiss} onRequestClose={close}>
    {follow ? (
      <View style={styles.overlay}>
        <Animated.View testID="new-session-lift" style={[styles.liftBox, {transform: [{translateY: lift}]}]}>
          {form}
        </Animated.View>
      </View>
    ) : (
      <KeyboardAvoidingView style={[styles.overlay, regular && styles.regular]} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        {form}
      </KeyboardAvoidingView>
    )}
  </Modal>;
}
const styles = StyleSheet.create({
  overlay: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end', alignItems: 'center', paddingHorizontal: 16, paddingTop: 16},
  regular: {justifyContent: 'center', padding: 24},
  bounds: {width: '100%', maxWidth: 480, flexShrink: 1},
  liftBox: {width: '100%', maxWidth: 480, alignItems: 'center'},
  sheet: {borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden', flexShrink: 1},
  content: {paddingHorizontal: 20, paddingTop: 12}, body: {flexGrow: 0, flexShrink: 1}, bodyContent: {paddingHorizontal: 20, paddingBottom: 4}, bodyShort: {paddingBottom: 16}, footer: {padding: 20}, header: {flexDirection: 'row', alignItems: 'center', gap: 12},
  title: {fontSize: 22, fontWeight: '700', flex: 1}, cancel: {minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center'},
  mac: {fontSize: 14, fontWeight: '600', marginTop: 4}, hint: {fontSize: 13, lineHeight: 19, marginTop: 6},
  label: {fontSize: 14, fontWeight: '600', marginTop: 24, marginBottom: 8}, labelShort: {marginTop: 10, marginBottom: 6},
  row: {flexDirection: 'row', alignItems: 'center', gap: 10}, grow: {flex: 1},
  input: {minHeight: 48, borderRadius: 10, borderWidth: 1, paddingHorizontal: 12, fontSize: 16},
  error: {fontSize: 14, lineHeight: 21, marginTop: 16}, check: {minHeight: 44, justifyContent: 'center', alignSelf: 'flex-start'},
  create: {minHeight: 48, borderRadius: 10, alignItems: 'center', justifyContent: 'center', padding: 12},
  createShort: {paddingHorizontal: 18},
  createText: {fontSize: 16, fontWeight: '600', color: '#fff'},
});
