// Composer — the Detail input area (MOBILE §4), Termius-style. Types into the
// pane via POST /api/send (a WRITE, gated by the bearer token). A single compact,
// horizontally-scrollable key toolbar (agent-aware context shortcuts + control
// keys) sits above a free-input row; DetailScreen wraps it in a
// KeyboardAvoidingView so it floats above the keyboard instead of being covered.
//
// Color is never used for status here.

import React, {useState} from 'react';
import {ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, View} from 'react-native';
import {StatusName} from '../api/types';
import {SendPayload} from '../api/client';
import {Lang} from '../i18n';
import {Palette} from './theme';

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

const CONTROL_KEYS: {label: string; key: string}[] = [
  {label: '⏎', key: 'Enter'},
  {label: 'Ctrl-C', key: 'C-c'},
  {label: 'Esc', key: 'Escape'},
  {label: 'Tab', key: 'Tab'},
  {label: '↑', key: 'Up'},
  {label: '↓', key: 'Down'},
  {label: '←', key: 'Left'},
  {label: '→', key: 'Right'},
];

export function Composer({
  status,
  pal,
  lang,
  enabled = true,
  onSend,
}: {
  status: StatusName;
  pal: Palette;
  lang: Lang;
  enabled?: boolean;
  onSend?: (p: SendPayload) => void;
}) {
  const [text, setText] = useState('');
  const send = (p: SendPayload) => {
    if (enabled && onSend) onSend(p);
  };
  const sendText = () => {
    if (text) {
      send({text, enter: true});
      setText('');
    }
  };

  return (
    <View style={[styles.wrap, {borderTopColor: pal.divider, backgroundColor: pal.bg}, !enabled && styles.disabled]}>
      {/* one compact, scrollable key toolbar (context shortcuts + control keys) */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        keyboardShouldPersistTaps="always"
        contentContainerStyle={styles.keys}>
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

      {/* free input + send */}
      <View style={styles.inputRow}>
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
  input: {flex: 1, height: 40, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, fontSize: 15},
  send: {width: 40, height: 40, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  sendText: {fontSize: 19, fontWeight: '700'},
});
