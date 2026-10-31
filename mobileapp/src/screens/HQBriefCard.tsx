import React from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import type { HQMatter } from '../api/hqWorkspace';
import { DisclosureChevron } from '../ui/DisclosureChevron';
import { SIcon } from '../ui/SettingsIcons';
import { Palette, StatusColor } from '../ui/theme';
import { matterPreview, matterState } from './hqWorkspaceModel';

// A matter's current state and a saved change are different facts. The stamp
// always names the former; the byline dates the actual update, never a receipt.
export function HQBriefCard({
  matter,
  preview,
  byline,
  zh,
  pal,
  onPress,
  testID,
  wide,
}: {
  matter: HQMatter;
  preview?: string;
  byline?: string;
  zh: boolean;
  pal: Palette;
  onPress: () => void;
  testID?: string;
  wide?: boolean;
}) {
  const assessment = matterPreview(matter.assessment ?? '');
  const body = assessment || matterPreview(preview ?? '');
  const needsInput = matter.state === 'needs-input';
  return (
    <TouchableOpacity
      testID={testID}
      accessibilityRole="button"
      onPress={onPress}
      style={[
        s.card,
        wide && s.wide,
        {
          backgroundColor: pal.surface,
          borderColor: pal.divider,
          borderLeftColor: needsInput ? StatusColor.waiting : pal.divider,
        },
      ]}
    >
      <View style={s.head}>
        <SIcon
          name={
            needsInput
              ? 'info'
              : matter.state === 'completed'
              ? 'check'
              : 'history'
          }
          size={16}
          color={needsInput ? StatusColor.waiting : pal.fg2}
        />
        <Text
          style={[
            s.stamp,
            { color: needsInput ? StatusColor.waiting : pal.fg2 },
          ]}
        >
          {matterState(matter.state, zh)}
        </Text>
        <DisclosureChevron open={false} color={pal.fg2} />
      </View>
      <Text numberOfLines={2} style={[s.title, { color: pal.fg }]}>
        {matter.title}
      </Text>
      {!!body && (
        <View style={s.brief}>
          {!!assessment && (
            <Text style={[s.caption, { color: pal.fg2 }]}>
              {zh ? '当前判断' : 'Current assessment'}
            </Text>
          )}
          <Text numberOfLines={3} style={[s.body, { color: pal.fg }]}>
            {body}
          </Text>
        </View>
      )}
      {!!byline && <Text style={[s.byline, { color: pal.fg2 }]}>{byline}</Text>}
    </TouchableOpacity>
  );
}
const s = StyleSheet.create({
  card: {
    padding: 15,
    gap: 9,
    borderWidth: StyleSheet.hairlineWidth,
    borderLeftWidth: 3,
    borderRadius: 12,
    minWidth: 0,
  },
  wide: { flexGrow: 1, flexBasis: '30%', minWidth: 240, maxWidth: '49%' },
  head: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  stamp: { flex: 1, fontSize: 12, lineHeight: 18, fontWeight: '600' },
  title: { fontSize: 16, lineHeight: 23, fontWeight: '600' },
  brief: { gap: 3 },
  caption: { fontSize: 12, lineHeight: 18 },
  body: { fontSize: 14, lineHeight: 21 },
  byline: { fontSize: 12, lineHeight: 18, fontVariant: ['tabular-nums'] },
});
