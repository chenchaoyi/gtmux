// Composer — the Detail input area (MOBILE §4). Input is Phase 2 (POST /api/send,
// send-keys) behind a one-time write authorization, so until that lands the whole
// composer is rendered DISABLED + clearly labeled. It still shows the intended
// shape: agent-aware context shortcuts (waiting → 1·Yes / 2·Always / 3·No; else →
// continue / ⏎ / stop), a control-key row, a free-input field, and a mic.
//
// Color is never used for status here; disabled state is conveyed by opacity.

import React from 'react';
import {StyleSheet, Text, TextInput, TouchableOpacity, View} from 'react-native';
import {StatusName} from '../api/types';
import {Lang} from '../i18n';
import {Palette} from './theme';

function contextKeys(status: StatusName, lang: string): string[] {
  if (status === 'waiting') {
    return lang === 'zh' ? ['1 · 是', '2 · 总是', '3 · 否'] : ['1 · Yes', '2 · Always', '3 · No'];
  }
  return lang === 'zh' ? ['继续', '⏎', '停止'] : ['Continue', '⏎', 'Stop'];
}

const CONTROL_KEYS = ['⏎', 'Ctrl-C', 'Esc', 'Tab', '↑', '↓'];

export function Composer({
  status,
  pal,
  lang,
  enabled = false,
}: {
  status: StatusName;
  pal: Palette;
  lang: Lang;
  enabled?: boolean;
}) {
  const gateLabel = lang === 'zh' ? 'Phase 2 · 写入需一次性授权' : 'Phase 2 · input needs one-time authorization';
  return (
    <View
      style={[styles.wrap, {borderTopColor: pal.divider, backgroundColor: pal.bg}, !enabled && styles.disabled]}
      pointerEvents={enabled ? 'auto' : 'none'}>
      {/* gate note */}
      {!enabled && (
        <View style={styles.gate}>
          <Text style={[styles.gateText, {color: pal.fg3}]}>{gateLabel}</Text>
        </View>
      )}

      {/* agent-aware context shortcuts */}
      <View style={styles.keys}>
        {contextKeys(status, lang).map(k => (
          <View key={k} style={[styles.ctxKey, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
            <Text style={[styles.ctxText, {color: pal.fg2}]}>{k}</Text>
          </View>
        ))}
      </View>

      {/* control keys */}
      <View style={styles.keys}>
        {CONTROL_KEYS.map(k => (
          <View key={k} style={[styles.ctlKey, {borderColor: pal.divider}]}>
            <Text style={[styles.ctlText, {color: pal.fg3}]}>{k}</Text>
          </View>
        ))}
      </View>

      {/* free input + mic + send */}
      <View style={styles.inputRow}>
        <View style={[styles.mic, {borderColor: pal.divider}]}>
          <Text style={{color: pal.fg3}}>🎤</Text>
        </View>
        <TextInput
          editable={false}
          placeholder={lang === 'zh' ? '输入…' : 'Type a message…'}
          placeholderTextColor={pal.fg3}
          style={[styles.input, {backgroundColor: pal.surface, borderColor: pal.divider, color: pal.fg}]}
        />
        <TouchableOpacity disabled style={[styles.send, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.sendText, {color: pal.fg3}]}>↑</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {borderTopWidth: StyleSheet.hairlineWidth, paddingHorizontal: 12, paddingTop: 8, paddingBottom: 10},
  disabled: {opacity: 0.55},
  gate: {alignItems: 'center', paddingBottom: 6},
  gateText: {fontSize: 11, fontWeight: '600'},
  keys: {flexDirection: 'row', flexWrap: 'wrap', marginBottom: 7},
  ctxKey: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 11, paddingVertical: 6, marginRight: 7, marginBottom: 6},
  ctxText: {fontSize: 12.5, fontWeight: '600'},
  ctlKey: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 9, paddingVertical: 5, marginRight: 6, marginBottom: 6},
  ctlText: {fontSize: 12, fontFamily: 'Menlo'},
  inputRow: {flexDirection: 'row', alignItems: 'center'},
  mic: {width: 38, height: 38, borderRadius: 19, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginRight: 8},
  input: {flex: 1, height: 38, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, fontSize: 14},
  send: {width: 38, height: 38, borderRadius: 19, borderWidth: StyleSheet.hairlineWidth, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  sendText: {fontSize: 18, fontWeight: '700'},
});
