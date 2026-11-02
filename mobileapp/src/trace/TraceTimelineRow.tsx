import React from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { Lang } from '../i18n';
import Svg, { Path } from 'react-native-svg';
import { ActivityIcon } from '../ui/ActivityIcon';
import { activityOperation } from '../ui/activity';
import { Chevron } from '../ui/Icons';
import { SenderAvatar } from '../ui/SenderAvatar';
import { ChromeText } from '../ui/ChromeText';
import { Palette } from '../ui/theme';
import { TraceEvent } from './model';
import { TimelineRow, tracePreview } from './timeline';

export {
  recordedStep,
  traceEventLabel,
  traceEventPreview,
} from './presentation';
import {
  recordedStep,
  traceEventLabel,
  traceEventPreview,
  traceRecordPaths,
} from './presentation';
export function TraceTimelineRow({
  row,
  pal,
  lang,
  selected,
  onSelect,
  onToggle,
  onRef,
  sessionNumber,
  foldingDisabled = false,
}: {
  row: TimelineRow;
  pal: Palette;
  lang: Lang;
  selected: string;
  onSelect: (event: TraceEvent) => void;
  onToggle: (row: Extract<TimelineRow, { type: 'turn' | 'calls' }>) => void;
  onRef: (key: string, node: View | null) => void;
  sessionNumber?: number;
  foldingDisabled?: boolean;
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
        ref={node => onRef('turn:' + group.key, node)}
        testID={'trace-turn-' + group.key}
        accessibilityRole="button"
        accessibilityState={{ expanded: row.open, disabled: foldingDisabled }}
        disabled={foldingDisabled}
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
          {
            group.events.filter(
              event => event.kind === 'user' || event.kind === 'assistant',
            ).length
          }
          {zh
            ? ' 条消息'
            : group.events.filter(
                event => event.kind === 'user' || event.kind === 'assistant',
              ).length === 1
            ? ' message'
            : ' messages'}
          {group.events.some(event => event.kind === 'context') &&
            ` · ${
              group.events.filter(event => event.kind === 'context').length
            }${
              zh
                ? ' 条上下文'
                : group.events.filter(event => event.kind === 'context')
                    .length === 1
                ? ' context record'
                : ' context records'
            }`}
        </ChromeText>
      </TouchableOpacity>
    );
  }
  const batch = row.type === 'calls';
  const event = batch ? row.events.at(-1)! : row.event;
  const title = batch
    ? zh
      ? `${row.events.length} 个工具调用`
      : `${row.events.length} tool ${
          row.events.length === 1 ? 'call' : 'calls'
        }`
    : traceEventLabel(event, lang);
  return (
    <TouchableOpacity
      ref={node => {
        if (!batch) onRef(event.key, node);
      }}
      testID={batch ? 'trace-calls-' + row.key : 'trace-record-' + event.key}
      accessibilityRole="button"
      accessibilityHint={
        !batch && row.parentCall
          ? zh
            ? `本组第 ${row.parentCall.index} 个调用，共 ${row.parentCall.total} 个`
            : `Call ${row.parentCall.index} of ${row.parentCall.total} in this group`
          : undefined
      }
      accessibilityState={
        batch ? { expanded: row.open } : { selected: selected === event.key }
      }
      onPress={() => (batch ? onToggle(row) : onSelect(event))}
      style={[
        s.record,
        !batch && row.parentCall && s.nestedRecord,
        {
          backgroundColor:
            !batch && selected === event.key ? pal.rowSelected : pal.bg,
        },
      ]}
    >
      {!batch && row.parentCall && (
        <>
          <View
            testID="trace-call-parent-rail"
            style={[
              s.parentRail,
              row.last && s.parentRailEnd,
              { backgroundColor: pal.divider },
            ]}
          />
          <View style={[s.branch, { backgroundColor: pal.divider }]} />
        </>
      )}
      <View style={s.rail}>
        {!row.last &&
          (batch ||
            !row.parentCall ||
            row.parentCall.index < row.parentCall.total) && (
            <View style={[s.line, { backgroundColor: pal.divider }]} />
          )}
        <View style={[s.marker, { backgroundColor: pal.bg }]}>
          {event.kind === 'context' ? (
            <RecordIcon kind="context" color={pal.fg2} />
          ) : event.kind === 'tool' ? (
            <ActivityIcon
              operation={activityOperation(recordedStep(event))}
              size={20}
              color={pal.fg2}
            />
          ) : event.kind === 'user' && event.from ? (
            <SenderAvatar from={event.from} size={20} />
          ) : (
            <RecordIcon
              kind={event.kind === 'user' ? 'user' : 'assistant'}
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
              ? [...new Set(row.events.map(value => value.title))].join(', ')
              : String(row.order).padStart(2, '0')}
          </ChromeText>
          <Chevron open={batch && row.open} size={16} color={pal.fg2} />
        </View>
        {!batch && (
          <Text numberOfLines={2} style={[s.preview, { color: pal.fg2 }]}>
            {traceEventPreview(event, lang)}
          </Text>
        )}
      </View>
    </TouchableOpacity>
  );
}
function RecordIcon({
  kind,
  color,
}: {
  kind: keyof typeof traceRecordPaths;
  color: string;
}) {
  return (
    <Svg
      width={20}
      height={20}
      viewBox="0 0 24 24"
      fill="none"
      accessible={false}
    >
      <Path
        d={traceRecordPaths[kind]}
        stroke={color}
        strokeWidth={1.8}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
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
  nestedRecord: { paddingLeft: 48 },
  parentRail: { position: 'absolute', left: 31.5, top: 0, bottom: 0, width: 1 },
  parentRailEnd: { bottom: undefined, height: 22 },
  branch: { position: 'absolute', left: 31.5, top: 22, width: 28, height: 1 },
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
