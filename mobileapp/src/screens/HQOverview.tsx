import React, { useMemo, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import type { HQWorkspace, HQOverviewItem } from '../api/hqWorkspace';
import { BRAND, Palette, StatusColor } from '../ui/theme';
import { DisclosureChevron } from '../ui/DisclosureChevron';
import { relTime } from './hqZones';
import { matterState, recentChange, matterPreview } from './hqWorkspaceModel';
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
  ];
  const toggle = (key: string) =>
    setExpanded(prev => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  return (
    <>
      <View style={[s.groups, wide && s.wide]}>
        {groups.map(group => {
          const all = [...matters.values()].filter(m => m.state === group.key);
          const items: HQOverviewItem[] = expanded.has(group.key)
            ? all.map(
                m =>
                  group.items.find(i => i.matter_id === m.id) ?? {
                    matter_id: m.id,
                  },
              )
            : group.items;
          return (
            <View key={group.key} style={s.group}>
              <View style={s.sectionHead}>
                <Text
                  accessibilityRole="header"
                  style={[s.sectionTitle, { color: pal.fg }]}
                >
                  {group.title}
                </Text>
                <Text style={[s.count, { color: pal.fg2 }]}>{all.length}</Text>
              </View>
              <View
                style={[
                  s.card,
                  { backgroundColor: pal.surface, borderColor: pal.divider },
                ]}
              >
                {items.length === 0 && (
                  <Text style={[s.empty, { color: pal.fg2 }]}>
                    {group.empty}
                  </Text>
                )}
                {items.map((item, i) => {
                  const m = matters.get(item.matter_id);
                  return m ? (
                    <TouchableOpacity
                      key={m.id}
                      testID={`hq-matter-${m.id}`}
                      accessibilityRole="button"
                      onPress={() => onMatter(m.id)}
                      style={[
                        s.matter,
                        i > 0 && {
                          borderTopWidth: StyleSheet.hairlineWidth,
                          borderTopColor: pal.divider,
                        },
                      ]}
                    >
                      <View style={s.row}>
                        <View
                          style={[
                            s.dot,
                            {
                              backgroundColor:
                                m.state === 'needs-input'
                                  ? StatusColor.waiting
                                  : BRAND,
                            },
                          ]}
                        />
                        <Text
                          style={[s.matterTitle, { color: pal.fg }]}
                          numberOfLines={2}
                        >
                          {m.title}
                        </Text>
                        <DisclosureChevron open={false} color={pal.fg2} />
                      </View>
                      <Text
                        style={[s.preview, { color: pal.fg2 }]}
                        numberOfLines={3}
                      >
                        {matterPreview(item.preview?.body ?? '') ||
                          t('Open matter for details', '查看事项详情')}
                      </Text>
                      <Text style={[s.meta, { color: pal.fg2 }]}>
                        {item.preview
                          ? t('HQ update', 'HQ 进展')
                          : matterState(m.state, zh)}
                        {item.preview?.at
                          ? ` · ${relTime(
                              item.preview.at,
                              data.overview.generated_at,
                            )}`
                          : ''}
                      </Text>
                    </TouchableOpacity>
                  ) : null;
                })}
                {all.length > group.items.length && (
                  <TouchableOpacity
                    accessibilityRole="button"
                    accessibilityState={{ expanded: expanded.has(group.key) }}
                    onPress={() => toggle(group.key)}
                    style={s.more}
                  >
                    <Text style={[s.buttonText, { color: pal.fg2 }]}>
                      {expanded.has(group.key)
                        ? t('Show less', '收起')
                        : t(
                            `View all ${all.length}`,
                            `查看全部 ${all.length} 项`,
                          )}
                    </Text>
                  </TouchableOpacity>
                )}
              </View>
            </View>
          );
        })}
      </View>
      <View style={s.sectionHead}>
        <Text
          accessibilityRole="header"
          style={[s.sectionTitle, { color: pal.fg }]}
        >
          {t('Last 24 hours', '最近 24 小时')}
        </Text>
      </View>
      <View
        style={[
          s.card,
          { backgroundColor: pal.surface, borderColor: pal.divider },
        ]}
      >
        {data.overview.recent.length === 0 && (
          <Text style={[s.empty, { color: pal.fg2 }]}>
            {t('No recent changes.', '暂无新动态。')}
          </Text>
        )}
        {data.overview.recent.map((change, i) => {
          const m = matters.get(change.matter_id);
          return m ? (
            <TouchableOpacity
              key={change.seq}
              accessibilityRole="button"
              onPress={() => onMatter(m.id)}
              style={[
                s.matter,
                i > 0 && {
                  borderTopWidth: StyleSheet.hairlineWidth,
                  borderTopColor: pal.divider,
                },
              ]}
            >
              <Text
                style={[s.matterTitle, { color: pal.fg }]}
                numberOfLines={2}
              >
                {m.title}
              </Text>
              <Text style={[s.meta, { color: pal.fg2 }]}>
                {recentChange(change.kind, change.state, zh)} ·{' '}
                {relTime(change.at, data.overview.generated_at)}
              </Text>
            </TouchableOpacity>
          ) : null;
        })}
      </View>
      {data.overview.recent_total > data.overview.recent.length && (
        <Text style={[s.footnote, { color: pal.fg2 }]}>
          {t(
            `Showing the latest ${data.overview.recent.length} of ${data.overview.recent_total} changed matters`,
            `展示 ${data.overview.recent_total} 个有变化事项中的最近 ${data.overview.recent.length} 个`,
          )}
        </Text>
      )}
    </>
  );
}
