import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ReplyOption} from '../api/types';
import {Lang} from '../i18n';
import {Palette, StatusColor} from './theme';

// ApprovalCard renders a waiting pane's choices as a compact row of NUMBER chips
// (MOBILE B1 approval card / cardinal rule: shown ONLY for waiting). The labels are
// deliberately omitted — they're already visible in the terminal/chat above, and a
// row of numbers is faster and clearer to pick from than re-sketched label rows.
// The parser (GET /api/options) is still used, but only for the COUNT: we render
// one chip per option (1..N). Tapping a chip sends just that digit — Claude's
// numbered menus commit on the digit alone; see DetailScreen for why no Enter.
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
      <View style={styles.row}>
        {options.map(o => (
          <TouchableOpacity
            key={o.n}
            onPress={() => onSend(o.n)}
            accessibilityLabel={`reply-${o.n}`}
            style={[styles.chip, {backgroundColor: red + '14', borderColor: red}]}>
            <Text style={[styles.num, {color: red}]}>{o.n}</Text>
          </TouchableOpacity>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {borderTopWidth: 2, paddingHorizontal: 10, paddingTop: 8, paddingBottom: 10},
  head: {fontSize: 11, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase', marginBottom: 8, marginLeft: 2},
  row: {flexDirection: 'row', flexWrap: 'wrap'},
  chip: {
    width: 52,
    height: 48,
    borderRadius: 12,
    borderWidth: 1.5,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 10,
    marginBottom: 6,
  },
  num: {fontSize: 22, fontWeight: '800'},
});
