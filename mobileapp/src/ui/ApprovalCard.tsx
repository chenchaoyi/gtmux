import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ReplyOption} from '../api/types';
import {Lang} from '../i18n';
import {Palette, StatusColor} from './theme';

// ApprovalCard renders a waiting pane's own 1/2/3 choices as full-row buttons
// (MOBILE B1 approval card / cardinal rule: shown ONLY for waiting). Options come
// from the shared Go parser via GET /api/options; tapping one sends it. The
// composer stays below for a free-text reply.
export function ApprovalCard({
  options,
  pal,
  lang,
  onSend,
}: {
  options: ReplyOption[];
  pal: Palette;
  lang: Lang;
  onSend: (n: number) => void;
}) {
  if (!options.length) return null;
  const red = StatusColor.waiting;
  return (
    <View style={[styles.wrap, {backgroundColor: pal.bg, borderTopColor: red}]}>
      <Text style={[styles.head, {color: red}]}>
        {lang === 'zh' ? '需要你回应' : 'Needs your reply'}
      </Text>
      {options.map(o => (
        <TouchableOpacity
          key={o.n}
          onPress={() => onSend(o.n)}
          style={[styles.opt, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.num, {color: red, backgroundColor: red + '29'}]}>{o.n}</Text>
          <Text style={[styles.label, {color: pal.fg}]} numberOfLines={2}>
            {o.label}
          </Text>
        </TouchableOpacity>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {borderTopWidth: 2, paddingHorizontal: 10, paddingTop: 8, paddingBottom: 2},
  head: {fontSize: 11, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase', marginBottom: 6, marginLeft: 2},
  opt: {flexDirection: 'row', alignItems: 'center', borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 10, paddingVertical: 11, marginBottom: 7},
  num: {fontSize: 14, fontWeight: '800', width: 26, height: 26, borderRadius: 6, textAlign: 'center', lineHeight: 26, overflow: 'hidden', marginRight: 11},
  label: {flex: 1, fontSize: 15, fontWeight: '500'},
});
