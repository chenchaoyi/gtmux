import React, { useState } from 'react';
import { Text, TouchableOpacity, View, StyleSheet } from 'react-native';
import type { HQMatter } from '../api/hqWorkspace';
import type { Palette } from '../ui/theme';

// A choice prepares a message in the existing composer. It never executes work.
export function HQHumanInputCard({
  matter,
  pal,
  zh,
  onReply,
}: {
  matter: HQMatter;
  pal: Palette;
  zh: boolean;
  onReply: (body: string) => void;
}) {
  const [impact, setImpact] = useState(false);
  if (matter.state !== 'needs-input') return null;
  const q = matter.user_input;
  const label = q?.answer_id
    ? zh
      ? '答复已保存'
      : 'Reply saved'
    : !q
    ? zh
      ? '待 HQ 明确问题'
      : 'HQ needs to clarify'
    : q.kind === 'manual'
    ? zh
      ? '请你操作'
      : 'Your action needed'
    : q.kind === 'information'
    ? zh
      ? '请你补充'
      : 'A detail is missing'
    : zh
    ? '请你选择'
    : 'Your decision needed';
  return (
    <View
      testID="hq-current-human-request"
      style={[
        s.card,
        { backgroundColor: pal.surface, borderColor: pal.divider },
      ]}
    >
      <Text style={[s.label, { color: pal.fg2 }]}>{label}</Text>
      <Text style={[s.prompt, { color: pal.fg }]}>
        {q?.prompt ||
          (zh
            ? '这条旧记录没有具体问题和选项，需要 HQ 先核对是否仍需你处理。'
            : 'This older record has no specific question. HQ needs to check whether it still needs your input.')}
      </Text>
      {q && !q.answer_id && (
        <>
          <Text style={[s.body, { color: pal.fg }]}>{q.why}</Text>
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityState={{ expanded: impact }}
            onPress={() => setImpact(!impact)}
            style={s.button}
          >
            <Text style={{ color: pal.fg2 }}>
              {zh ? '范围与影响' : 'Scope and impact'} {impact ? '⌃' : '⌄'}
            </Text>
          </TouchableOpacity>
          {impact && (
            <Text style={[s.body, { color: pal.fg2 }]}>{q.impact}</Text>
          )}
          <View style={s.choices}>
            {[...(q.choices || []), zh ? '自行答复' : 'Write a reply'].map(
              (choice, i) => (
                <TouchableOpacity
                  key={i}
                  accessibilityRole="button"
                  onPress={() =>
                    onReply(
                      `${q.prompt}\n${
                        i === (q.choices || []).length ? '' : choice
                      }`,
                    )
                  }
                  style={[
                    s.button,
                    s.choice,
                    {
                      borderWidth: StyleSheet.hairlineWidth,
                      borderColor: pal.divider,
                    },
                  ]}
                >
                  <Text style={{ color: pal.fg }}>{choice}</Text>
                </TouchableOpacity>
              ),
            )}
          </View>
          <Text style={[s.label, { color: pal.fg2 }]}>
            {zh
              ? '选择后会填入输入框，由你确认发送。'
              : 'Choose to prepare a reply, then send it.'}
          </Text>
        </>
      )}
      {!!q?.answer_id && (
        <Text style={[s.body, { color: pal.fg2 }]}>
          {zh
            ? '接下来由 HQ 确认并跟进；答复已保存不代表操作已执行。'
            : 'HQ will confirm receipt and follow up. A saved reply does not mean the action has run.'}
        </Text>
      )}
    </View>
  );
}
const s = StyleSheet.create({
  card: {
    padding: 14,
    gap: 8,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 12,
  },
  label: { fontSize: 12, lineHeight: 18 },
  prompt: { fontSize: 17, lineHeight: 24, fontWeight: '600' },
  body: { fontSize: 15, lineHeight: 22 },
  choices: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  choice: { borderRadius: 8 },
  button: {
    minHeight: 44,
    paddingHorizontal: 12,
    paddingVertical: 10,
    justifyContent: 'center',
  },
});
