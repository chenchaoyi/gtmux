import React, { useMemo, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import type { HQWorkspace, HQOverviewItem } from '../api/hqWorkspace';
import type { Palette } from '../ui/theme';
import { relTime } from './hqZones';
import { recentChange, matterPreview } from './hqWorkspaceModel';
import { HQBriefCard } from './HQBriefCard';
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
      items: data.overview.needs_input,
      empty: t('No decisions waiting for you.', '暂时没有需要你决定的事。'),
    },
    {
      key: 'open',
      title: t('Following', '正在跟进'),
      items: data.overview.following,
      empty: t('No ongoing matters.', '暂无正在跟进的事项。'),
    },
  ].map(group => {
    const all = [...matters.values()].filter(m => m.state === group.key);
    const items: HQOverviewItem[] = expanded.has(group.key)
      ? all.map(
          m =>
            group.items.find(i => i.matter_id === m.id) ?? { matter_id: m.id },
        )
      : group.items;
    return { ...group, all, items, previewCount: group.items.length };
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
  const more = (group: (typeof groups)[number]) =>
    group.all.length > group.previewCount ? (
      <TouchableOpacity
        key={group.key}
        accessibilityRole="button"
        accessibilityLabel={`${
          expanded.has(group.key)
            ? t('Show less', '收起')
            : t('View all', '查看全部')
        } · ${group.title}`}
        accessibilityState={{ expanded: expanded.has(group.key) }}
        onPress={() => toggle(group.key)}
        style={s.more}
      >
        <Text style={[s.buttonText, { color: pal.fg2 }]}>
          {expanded.has(group.key)
            ? t('Show less', '收起')
            : wide
            ? t(
                `View all ${group.all.length} · ${group.title}`,
                `查看全部 ${group.all.length} 项 · ${group.title}`,
              )
            : t(
                `View all ${group.all.length}`,
                `查看全部 ${group.all.length} 项`,
              )}
        </Text>
      </TouchableOpacity>
    ) : null;
  return (
    <>
      {wide ? (
        <>
          <View style={s.sectionHead}>
            <Text
              accessibilityRole="header"
              style={[s.sectionTitle, { color: pal.fg }]}
            >
              {t('Current matters', '当前事项')}
            </Text>
            <Text style={[s.count, { color: pal.fg2 }]}>
              {groups.reduce((n, group) => n + group.all.length, 0)}
            </Text>
          </View>
          <View style={styles.currentGrid}>
            {groups.flatMap(group => group.items.map(card))}
          </View>
          {groups.every(group => group.all.length === 0) && (
            <Text style={[s.empty, { color: pal.fg2 }]}>
              {t('No ongoing matters.', '暂无正在跟进的事项。')}
            </Text>
          )}
          <View style={styles.moreRow}>{groups.map(more)}</View>
        </>
      ) : (
        groups.map(group => (
          <View key={group.key}>
            <View style={s.sectionHead}>
              <Text
                accessibilityRole="header"
                style={[s.sectionTitle, { color: pal.fg }]}
              >
                {group.title}
              </Text>
              <Text style={[s.count, { color: pal.fg2 }]}>
                {group.all.length}
              </Text>
            </View>
            <View style={styles.cards}>
              {group.items.length === 0 && (
                <Text style={[s.empty, { color: pal.fg2 }]}>{group.empty}</Text>
              )}
              {group.items.map(card)}
              {more(group)}
            </View>
          </View>
        ))
      )}
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
  moreRow: { flexDirection: 'row', flexWrap: 'wrap' },
});
