import React, { useMemo } from 'react';
import {
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
  useWindowDimensions,
} from 'react-native';
import type { HQWorkspace } from '../api/hqWorkspace';
import type { HQResourceRequest } from '../state/WorkspaceContext';
import { DisclosureChevron } from '../ui/DisclosureChevron';
import { SIcon, IconName } from '../ui/SettingsIcons';
import { ERRORED_COLOR, StatusColor, type Palette } from '../ui/theme';
import { parseBoardSections } from './boardSections';
import { usageDoorValue } from './hqHeaderModel';
import { knowledgeValue } from './knowledgeModel';
import {
  matterPreview,
  workspaceCounts,
  workspaceSummary,
} from './hqWorkspaceModel';
import { boardAge } from './hqZones';
import { machineWarn } from './usageModel';
import type { HQResourceReadings } from './useHQResourceSummaries';

export function HQResourceCards({
  data,
  readings,
  pal,
  zh,
  wide,
  width,
  onOpen,
}: {
  data: HQWorkspace;
  readings: HQResourceReadings;
  pal: Palette;
  zh: boolean;
  wide: boolean;
  width: number;
  onOpen: (page: Exclude<HQResourceRequest['page'], 'board'>) => void;
}) {
  const { fontScale } = useWindowDimensions();
  const stacked = fontScale > 1.3 || width < 350;
  const t = (en: string, cn: string) => (zh ? cn : en);
  const board = readings.board.value;
  const boardBrief = useMemo(() => {
    const first = parseBoardSections(board?.text ?? '')[0];
    return first
      ? matterPreview(
          first.body || first.title || first.children[0]?.title || '',
        )
      : '';
  }, [board?.text]);
  const counts = workspaceCounts(data);
  const knowledge = readings.knowledge.value;
  const usage = readings.usage.value;
  const usageText = usageDoorValue(
    (usage?.limits?.windows ?? [])
      .filter(w => !w.stale)
      .map(w => ({ label: w.label, pct: w.pct_used, agent: w.agent })),
    zh,
    usage?.history,
  );
  const machine = usage?.resource?.machine;
  const warning =
    machine?.tier === 'amber' || machine?.tier === 'red'
      ? machineWarn(machine, zh) || t('Resource warning', '本机资源警示')
      : '';
  const warningColor =
    machine?.tier === 'red' ? StatusColor.waiting : ERRORED_COLOR;
  const warningText = warning
    ? `${t('Local resources', '本机资源')} · ${warning}`
    : '';
  const previous = t(
    'Previous reading · couldn’t refresh',
    '上次记录 · 刷新失败',
  );
  const unavailable = (failed?: boolean) =>
    failed ? t('Unavailable', '暂时无法加载') : t('Loading…', '正在加载…');
  const resources: Array<{
    key: HQResourceRequest['page'];
    icon: IconName;
    title: string;
    value: string;
    note?: string;
    failed?: boolean;
    loaded: boolean;
  }> = [
    {
      key: 'board',
      icon: 'layout',
      title: t('Board', '态势板'),
      value: workspaceSummary(counts, zh, stacked ? '\n' : ' · '),
      note:
        boardBrief ||
        (board
          ? board.exists
            ? boardAge(board.updated_at, data.overview.generated_at, zh)
            : t('No board yet', '尚无态势板记录')
          : unavailable(readings.board.failed)),
      failed: readings.board.failed,
      loaded: !!board,
    },
    {
      key: 'knowledge',
      icon: 'knowledge',
      title: t('Knowledge', '知识库'),
      value: knowledge
        ? knowledgeValue(knowledge, zh) || t('No entries yet', '暂无条目')
        : unavailable(readings.knowledge.failed),
      note:
        knowledge && (knowledge.candidates?.pending ?? 0) > 0
          ? t(
              `${knowledge.candidates.pending} candidates`,
              `${knowledge.candidates.pending} 条待整理`,
            )
          : undefined,
      failed: readings.knowledge.failed,
      loaded: !!knowledge,
    },
    {
      key: 'usage',
      icon: 'usage',
      title: t('Usage', '用量'),
      value:
        usageText ||
        (usage === undefined
          ? unavailable(readings.usage.failed)
          : t('No usage recorded', '暂无用量记录')),
      failed: readings.usage.failed,
      loaded: usage !== undefined,
    },
  ];
  return (
    <View testID="hq-resource-summaries" style={[s.grid, wide && s.wide]}>
      {resources.map(resource => {
        const page = resource.key;
        const label = `${resource.title}. ${resource.value}${
          resource.note ? '. ' + resource.note : ''
        }${resource.failed && resource.loaded ? '. ' + previous : ''}${
          resource.key === 'board' && warningText
            ? '. ' +
              warningText +
              (readings.usage.failed ? '. ' + previous : '')
            : ''
        }`;
        const style = [
          s.card,
          { backgroundColor: pal.surface, borderColor: pal.divider },
          wide
            ? s.column
            : resource.key === 'board' || stacked
            ? s.full
            : s.half,
        ];
        const content = (
          <>
            <View style={s.heading}>
              <SIcon name={resource.icon} size={20} color={pal.fg2} />
              <Text style={[s.label, { color: pal.fg2 }]}>
                {resource.title}
              </Text>
              {resource.key !== 'board' && (
                <DisclosureChevron open={false} color={pal.fg2} />
              )}
            </View>
            <Text style={[s.value, { color: pal.fg }]}>
              {resource.value.replace(
                /\s+·\s+/g,
                resource.key === 'usage' ? '\n' : ' · ',
              )}
            </Text>
            {!!resource.note && (
              <Text numberOfLines={2} style={[s.note, { color: pal.fg2 }]}>
                {resource.note}
              </Text>
            )}
            {resource.failed && resource.loaded && (
              <Text style={[s.note, { color: pal.fg2 }]}>{previous}</Text>
            )}
            {resource.key === 'board' && !!warningText && (
              <View
                testID="hq-resource-warning"
                style={[s.warning, { borderTopColor: pal.divider }]}
              >
                <SIcon name="server" size={20} color={warningColor} />
                <View style={s.warningContent}>
                  <Text style={[s.note, { color: pal.fg }]}>{warningText}</Text>
                  {readings.usage.failed && (
                    <Text style={[s.note, { color: pal.fg2 }]}>{previous}</Text>
                  )}
                </View>
              </View>
            )}
          </>
        );
        // Matters are already below: the board is a briefing, not another door.
        return page === 'board' ? (
          <View
            key={resource.key}
            testID="hq-resource-board"
            accessible
            accessibilityLabel={label}
            style={style}
          >
            {content}
          </View>
        ) : (
          <TouchableOpacity
            key={resource.key}
            testID={`hq-resource-${resource.key}`}
            accessibilityRole="button"
            accessibilityLabel={label}
            onPress={() => onOpen(page)}
            style={style}
          >
            {content}
          </TouchableOpacity>
        );
      })}
    </View>
  );
}
const s = StyleSheet.create({
  grid: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, marginBottom: 6 },
  wide: { flexWrap: 'nowrap' },
  card: {
    borderRadius: 14,
    borderWidth: StyleSheet.hairlineWidth,
    padding: 14,
    minHeight: 100,
    gap: 9,
  },
  full: { width: '100%' },
  half: { flexGrow: 1, flexBasis: '46%', minWidth: 0 },
  column: { flex: 1, minWidth: 0 },
  heading: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  label: { flex: 1, fontSize: 13, lineHeight: 18, fontWeight: '500' },
  value: {
    fontSize: 16,
    lineHeight: 23,
    fontWeight: '600',
    fontVariant: ['tabular-nums'],
  },
  note: { fontSize: 13, lineHeight: 19 },
  warning: {
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingTop: 9,
    flexDirection: 'row',
    gap: 8,
  },
  warningContent: { flex: 1, gap: 3 },
});
