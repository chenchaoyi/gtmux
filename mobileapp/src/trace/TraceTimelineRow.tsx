import React from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { Lang } from '../i18n';
import { ActivityIcon } from '../ui/ActivityIcon';
import {
  activityLabel,
  activityOperation,
  activityPreview,
  activitySummary,
} from '../ui/activity';
import { Chevron } from '../ui/Icons';
import { SIcon } from '../ui/SettingsIcons';
import { SenderAvatar } from '../ui/SenderAvatar';
import { ChromeText } from '../ui/ChromeText';
import { Palette } from '../ui/theme';
import { TraceEvent, TraceStep } from './model';
import { TimelineRow, tracePreview } from './timeline';

export function recordedStep(event: TraceEvent) {
  const step = event.step as TraceStep;
  return {
    kind: 'tool',
    title: event.title,
    input: step.input,
    detail: step.summary,
    operation: step.operation,
    input_kind: step.inputKind,
  };
}
export function traceEventLabel(event: TraceEvent, lang: Lang) {
  return event.kind === 'tool'
    ? activityLabel(activityOperation(recordedStep(event)), lang)
    : event.kind === 'user'
    ? event.from?.label || (lang === 'zh' ? '请求' : 'Request')
    : lang === 'zh'
    ? '进展记录'
    : 'Progress';
}
export function traceEventPreview(event: TraceEvent, lang: Lang) {
  return event.kind === 'tool'
    ? activityPreview(recordedStep(event), lang)
    : tracePreview(event.body, lang);
}

export function TraceTimelineRow({
  row,
  pal,
  lang,
  selected,
  onSelect,
  onToggle,
  onRef,
  sessionNumber,
}: {
  row: TimelineRow;
  pal: Palette;
  lang: Lang;
  selected: string;
  onSelect: (event: TraceEvent) => void;
  onToggle: (row: Extract<TimelineRow, { type: 'turn' | 'calls' }>) => void;
  onRef: (key: string, node: View | null) => void;
  sessionNumber?: number;
}) {
  const zh = lang === 'zh';
  if (row.type === 'turn') {
    const group = row.group;
    const time =
      group.time && !isNaN(new Date(group.time).getTime())
        ? new Date(group.time).toLocaleTimeString(zh ? 'zh-CN' : 'en-US', {
            hour: '2-digit',
            minute: '2-digit',
          })
        : '';
    return (
      <TouchableOpacity
        testID={'trace-turn-' + group.key}
        accessibilityRole="button"
        accessibilityState={{ expanded: row.open }}
        onPress={() => onToggle(row)}
        style={[
          s.turn,
          { backgroundColor: pal.surface, borderColor: pal.divider },
        ]}
      >
        <View style={s.heading}>
          <ChromeText
            numberOfLines={1}
            style={[s.turnTitle, { color: pal.fg }]}
          >
            {sessionNumber
              ? zh
                ? `会话 ${sessionNumber} · `
                : `Session ${sessionNumber} · `
              : ''}
            {zh ? `第 ${group.turn} 轮` : `Turn ${group.turn}`}
          </ChromeText>
          <ChromeText
            numberOfLines={1}
            style={[s.meta, s.flex, { color: pal.fg2 }]}
          >
            {group.agent}
            {time ? ' · ' + time : ''}
          </ChromeText>
          <Chevron open={row.open} size={16} color={pal.fg2} />
        </View>
        {!row.open && (
          <Text numberOfLines={2} style={[s.prompt, { color: pal.fg }]}>
            {tracePreview(
              group.prompt?.body || group.events[0]?.body || '',
              lang,
            )}
          </Text>
        )}
        <ChromeText style={[s.meta, { color: pal.fg2 }]}>
          {group.calls}
          {zh ? ' 次调用' : group.calls === 1 ? ' call' : ' calls'} ·{' '}
          {group.events.length - group.calls}
          {zh
            ? ' 条消息'
            : group.events.length - group.calls === 1
            ? ' message'
            : ' messages'}
        </ChromeText>
      </TouchableOpacity>
    );
  }
  const batch = row.type === 'calls';
  const event = batch ? row.events.at(-1)! : row.event;
  const title = batch
    ? activitySummary(row.events.map(recordedStep), lang)
    : traceEventLabel(event, lang);
  return (
    <TouchableOpacity
      ref={node => {
        if (!batch) onRef(event.key, node);
      }}
      testID={batch ? 'trace-calls-' + row.key : 'trace-record-' + event.key}
      accessibilityRole="button"
      accessibilityState={
        batch ? { expanded: row.open } : { selected: selected === event.key }
      }
      onPress={() => (batch ? onToggle(row) : onSelect(event))}
      style={[
        s.record,
        { backgroundColor: selected === event.key ? pal.rowSelected : pal.bg },
      ]}
    >
      <View style={s.rail}>
        {!row.last && (
          <View style={[s.line, { backgroundColor: pal.divider }]} />
        )}
        <View style={[s.marker, { backgroundColor: pal.bg }]}>
          {event.kind === 'tool' ? (
            <ActivityIcon
              operation={activityOperation(recordedStep(event))}
              size={20}
              color={pal.fg2}
            />
          ) : event.kind === 'user' && event.from ? (
            <SenderAvatar from={event.from} size={20} />
          ) : (
            <SIcon
              name={event.kind === 'user' ? 'person' : 'sparkle'}
              size={20}
              color={pal.fg2}
            />
          )}
        </View>
      </View>
      <View style={s.flex}>
        <View style={s.heading}>
          <ChromeText style={[s.label, { color: pal.fg }]}>{title}</ChromeText>
          <ChromeText style={[s.meta, s.flex, { color: pal.fg2 }]}>
            {batch
              ? `${row.events.length}${zh ? ' 次' : ' calls'}`
              : String(row.order).padStart(2, '0')}
          </ChromeText>
          <Chevron open={batch && row.open} size={16} color={pal.fg2} />
        </View>
        <Text numberOfLines={2} style={[s.preview, { color: pal.fg2 }]}>
          {batch && !row.open ? (zh ? '最近：' : 'Latest: ') : ''}
          {traceEventPreview(event, lang)}
        </Text>
      </View>
    </TouchableOpacity>
  );
}
const s = StyleSheet.create({
  flex: { flex: 1, minWidth: 0 },
  heading: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  turn: {
    marginHorizontal: 12,
    marginTop: 14,
    marginBottom: 6,
    padding: 12,
    gap: 5,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 12,
    minHeight: 60,
  },
  turnTitle: { fontSize: 14, fontWeight: '600', flexShrink: 1 },
  meta: { fontSize: 12, lineHeight: 18 },
  prompt: { fontSize: 14, lineHeight: 21 },
  record: {
    flexDirection: 'row',
    gap: 12,
    paddingHorizontal: 20,
    paddingVertical: 10,
    minHeight: 60,
  },
  rail: { width: 24, alignItems: 'center' },
  marker: {
    width: 24,
    height: 24,
    alignItems: 'center',
    justifyContent: 'center',
  },
  line: { position: 'absolute', top: 24, bottom: -20, left: 11.5, width: 1 },
  label: { fontSize: 13, fontWeight: '600', lineHeight: 20 },
  preview: { fontSize: 14, lineHeight: 21, marginTop: 3 },
});
