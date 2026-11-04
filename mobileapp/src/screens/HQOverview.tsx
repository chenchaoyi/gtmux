import React, { useMemo, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import type { HQWorkspace, HQOverviewItem } from '../api/hqWorkspace';
import { BRAND, type Palette } from '../ui/theme';
import { relTime } from './hqZones';
import { recentChange, matterPreview } from './hqWorkspaceModel';
import { HQBriefCard } from './HQBriefCard';
import { SIcon } from '../ui/SettingsIcons';
import { HQWorkspaceStyles as s } from './hqWorkspaceStyles';

export function HQOverview({
  data,
  pal,
  zh,
  wide,
  onMatter,
}: {
  data: HQWorkspace;
  pal: Palette;
  zh: boolean;
  wide?: boolean;
  onMatter: (id: string) => void;
}) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const t = (en: string, cn: string) => (zh ? cn : en);
  const matters = useMemo(
    () => new Map(data.matters.filter(m => !m.merged_into).map(m => [m.id, m])),
    [data.matters],
  );
  const groups = [
    {
      key: 'needs-input',
      title: t('Needs input', '待你处理'),
      items:
        data.overview.human_input ??
        data.overview.needs_input.filter(i => {
          const m = matters.get(i.matter_id);
          return !!m?.user_input && !m.user_input.answer_id;
        }),
      empty: t('No decisions waiting for you.', '暂时没有需要你决定的事。'),
    },
    {
      key: 'open',
      title: t('Following', '正在跟进'),
      items: data.overview.following,
      empty: t('No ongoing matters.', '暂无正在跟进的事项。'),
    },
    {
      key: 'awaiting-hq',
      title: t('HQ follow-up', '由 HQ 跟进'),
      items: data.overview.awaiting_hq ?? [],
      empty: t('No follow-up waiting.', '暂无待核对事项。'),
    },
  ].map(group => {
    const all = [...matters.values()].filter(m =>
      group.key === 'awaiting-hq'
        ? m.state === 'needs-input' &&
          (!m.user_input || !!m.user_input.answer_id)
        : group.key === 'needs-input'
        ? m.state === 'needs-input' && !!m.user_input && !m.user_input.answer_id
        : m.state === group.key,
    );
    const preview: HQOverviewItem[] = group.items.filter(i => all.some(m => m.id === i.matter_id));
    const initial = preview.length ? preview : all.slice(0, 3).map(m => ({ matter_id: m.id }));
    const items = expanded.has(group.key)
      ? all.map(m => preview.find(i => i.matter_id === m.id) ?? { matter_id: m.id })
      : initial;
    return { ...group, all, items, previewCount: initial.length };
  });
  const toggle = (key: string) =>
    setExpanded(prev => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  const card = (item: HQOverviewItem) => {
    const matter = matters.get(item.matter_id);
    if (!matter) return null;
    const assessment = matterPreview(matter.assessment ?? '');
    const byline =
      !assessment && item.preview?.at
        ? `${t('HQ update', 'HQ 进展')} · ${relTime(
            item.preview.at,
            data.overview.generated_at,
          )}`
        : matter.updated_at
        ? `${t('Updated', '更新于')} ${relTime(
            matter.updated_at,
            data.overview.generated_at,
          )}`
        : undefined;
    return (
      <HQBriefCard
        key={matter.id}
        testID={`hq-matter-${matter.id}`}
        matter={matter}
        preview={item.preview?.body}
        pal={pal}
        zh={zh}
        wide={wide}
        byline={byline}
        onPress={() => onMatter(matter.id)}
      />
    );
  };
  const more = (group: (typeof groups)[number]) => {
    if (group.all.length <= group.previewCount) return null;
    const open = expanded.has(group.key);
    const label = open ? t('Show less', '收起') : t(
      `Show ${group.all.length - group.items.length} more`,
      `展开剩余 ${group.all.length - group.items.length} 项`,
    );
    return (
      <TouchableOpacity
        testID={`hq-overview-more-${group.key}`}
        accessibilityRole="button"
        accessibilityLabel={`${label} · ${group.title}`}
        accessibilityState={{ expanded: open }}
        onPress={() => toggle(group.key)}
        style={[styles.more, {backgroundColor: pal.surface, borderColor: pal.divider}]}
      >
        <View style={styles.moreText}>
          <Text style={[s.buttonText, {color: pal.fg, fontWeight: '600'}]}>{label}</Text>
          <Text style={[s.footnote, styles.moreCaption, {color: pal.fg2}]}>
            {t(`Showing ${group.items.length} of ${group.all.length} · ${group.title}`,
              `已显示 ${group.items.length} / ${group.all.length} 项 · ${group.title}`)}
          </Text>
        </View>
        <View style={open ? styles.up : undefined}>
          <SIcon name="chevronDown" size={22} color={BRAND} />
        </View>
      </TouchableOpacity>
    );
  };
  return (
    <>
      {wide && <View style={s.sectionHead}>
        <Text accessibilityRole="header" style={[s.sectionTitle, {color: pal.fg}]}>
          {t('Current matters', '当前事项')}
        </Text>
      </View>}
      {groups.map(group => (
        <View key={group.key}>
          <View style={s.sectionHead}>
            <Text accessibilityRole="header" style={[s.sectionTitle, {color: pal.fg}]}>{group.title}</Text>
            <Text style={[s.count, {color: pal.fg2}]}>{group.items.length} / {group.all.length}</Text>
          </View>
          <View style={[styles.cards, wide && styles.currentGrid]}>
            {group.items.length === 0 && <Text style={[s.empty, {color: pal.fg2}]}>{group.empty}</Text>}
            {group.items.map(card)}
          </View>
          {more(group)}
        </View>
      ))}
      <View style={s.sectionHead}>
        <Text
          accessibilityRole="header"
          style={[s.sectionTitle, { color: pal.fg }]}
        >
          {t('Last 24 hours', '最近 24 小时')}
        </Text>
      </View>
      <View style={[styles.cards, wide && styles.currentGrid]}>
        {data.overview.recent.length === 0 && (
          <Text style={[s.empty, { color: pal.fg2 }]}>
            {t('No recent changes.', '暂无新动态。')}
          </Text>
        )}
        {data.overview.recent.map(change => {
          const matter = matters.get(change.matter_id);
          return matter ? (
            <HQBriefCard
              key={change.seq}
              matter={matter}
              pal={pal}
              zh={zh}
              wide={wide}
              byline={`${recentChange(
                change.kind,
                change.state,
                zh,
              )} · ${relTime(change.at, data.overview.generated_at)}`}
              onPress={() => onMatter(matter.id)}
            />
          ) : null;
        })}
      </View>
      {data.overview.recent_total > data.overview.recent.length && (
        <Text style={[s.footnote, { color: pal.fg2 }]}>
          {t(
            `Latest ${data.overview.recent.length} of ${data.overview.recent_total} updated matters`,
            `最近 ${data.overview.recent.length} 项 · 共 ${data.overview.recent_total} 项有更新`,
          )}
        </Text>
      )}
    </>
  );
}
const styles = StyleSheet.create({
  cards: { gap: 10 },
  currentGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: 12 },
  more: {minHeight: 56, marginTop: 10, paddingVertical: 10, paddingHorizontal: 14, borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12},
  moreText: {flex: 1, gap: 3},
  moreCaption: {textAlign: 'left', marginTop: 0},
  up: {transform: [{rotate: '180deg'}]},
});
