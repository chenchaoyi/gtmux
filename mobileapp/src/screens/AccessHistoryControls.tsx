import React, { useRef, useState } from 'react';
import { StyleSheet, Text, Alert, TouchableOpacity, useWindowDimensions, View } from 'react-native';
import type {
  AccessFilter,
  AccessGroup,
  AccessOperation,
} from '../api/accessHistory';
import {
  AnchoredMenu,
  type MenuAnchor,
  type MenuItem,
} from '../ui/AnchoredMenu';
import { accessGroupFilter, accessIdentity } from './accessHistoryExploration';
import { accessName } from './accessHistoryPresentation';
import { SIcon } from '../ui/SettingsIcons';
import { Size, type Palette } from '../ui/theme';

type Kind = 'device' | 'operation' | 'outcome';
export function AccessHistoryControls({
  filter,
  groups,
  operations,
  zh,
  pal,
  onChange,
}: {
  filter: AccessFilter;
  groups: AccessGroup[];
  operations: AccessOperation[];
  zh: boolean;
  pal: Palette;
  onChange: (f: AccessFilter) => void;
}) {
  const [kind, setKind] = useState<Kind | null>(null),
    [anchor, setAnchor] = useState<MenuAnchor | null>(null);
  const refs = useRef<Partial<Record<Kind, View | null>>>({});
  const {fontScale} = useWindowDimensions();
  const names = {
    device: zh ? '来源' : 'Source',
    operation: zh ? '操作' : 'Operation',
    outcome: zh ? '结果' : 'Outcome',
  };
  const outcomes = [
    { key: '', label: zh ? '全部结果' : 'All outcomes' },
    { key: 'ok', label: zh ? '已接受' : 'Accepted' },
    { key: 'refused', label: zh ? '已拒绝' : 'Refused' },
    { key: 'failed', label: zh ? '失败' : 'Failed' },
  ];
  const all = (
    label: string,
    selected: boolean,
    onPress: () => void,
  ): MenuItem => ({ key: 'all', icon: 'history', label, selected, onPress });
  const options: Record<Kind, MenuItem[]> = {
    device: [
      all(zh ? '全部来源' : 'All sources', !accessIdentity(filter), () =>
        onChange({ ...filter, actor: undefined, device_id: undefined }),
      ),
      ...groups.map(g => ({
        key: g.key,
        icon: g.device_id ? 'phone' as const : 'globe' as const,
        label: accessName(g, zh),
        description: g.device_id || g.source_ip || (zh ? '未标识设备' : 'Unidentified client'),
        selected: accessIdentity(filter) === g.key,
        onPress: () =>
          onChange({
            ...accessGroupFilter(filter, g),
            group_by: filter.group_by,
          }),
      })),
      {
        key: 'device-id',
        icon: 'info',
        label: zh ? '按设备 ID 筛选…' : 'Filter by device ID…',
        onPress: () =>
          Alert.prompt(
            zh ? '完整设备 ID' : 'Full device ID',
            undefined,
            [
              { text: zh ? '取消' : 'Cancel', style: 'cancel' },
              {
                text: zh ? '筛选' : 'Filter',
                onPress: (value?: string) => {
                  const id = value?.trim();
                  if (id && id.length <= 128 && !/[\r\n]/.test(id))
                    onChange({ ...filter, actor: undefined, device_id: id });
                  else if (id)
                    Alert.alert(
                      zh ? '设备 ID 无效' : 'Invalid device ID',
                      zh
                        ? '设备 ID 不得超过 128 个字符，且不能包含换行。'
                        : 'Use an ID of up to 128 characters without line breaks.',
                    );
                },
              },
            ],
            'plain-text',
            filter.device_id || '',
            'default',
          ),
      },
    ],
    operation: [
      all(zh ? '全部操作' : 'All operations', !filter.operation, () =>
        onChange({ ...filter, operation: undefined }),
      ),
      ...operations.map(op => ({
        key: op.value,
        icon: 'history' as const,
        label: zh ? op.zh : op.en,
        selected: filter.operation === op.value,
        onPress: () => onChange({ ...filter, operation: op.value }),
      })),
    ],
    outcome: outcomes.map(o => ({
      key: o.key,
      icon: 'check',
      label: o.label,
      selected: (filter.outcome || '') === o.key,
      onPress: () => onChange({ ...filter, outcome: o.key || undefined }),
    })),
  };
  const chosen = groups.find(g => g.key === accessIdentity(filter));
  const labels = {
    device: chosen
      ? accessName(chosen, zh)
      : filter.device_id || filter.actor || (zh ? '全部来源' : 'All sources'),
    operation:
      operations.find(o => o.value === filter.operation)?.[zh ? 'zh' : 'en'] ||
      (zh ? '全部操作' : 'All operations'),
    outcome: outcomes.find(o => o.key === (filter.outcome || ''))!.label,
  };
  const filters: Kind[] = ['device', 'operation', 'outcome'];
  const selected: Record<Kind, boolean> = {
    device: !!accessIdentity(filter),
    operation: !!filter.operation,
    outcome: !!filter.outcome,
  };
  const selectedFilters = filters.filter(k => selected[k]);
  return (
    <View>
      <View style={[styles.segmented, {backgroundColor: pal.rowSelected, borderColor: pal.divider}]}>
        {[undefined, 'device' as const].map(mode => (
          <TouchableOpacity
            key={mode || 'logs'}
            accessibilityRole="tab"
            accessibilityState={{ selected: filter.group_by === mode }}
            onPress={() => onChange({ ...filter, group_by: mode })}
            style={[
              styles.tab,
              filter.group_by === mode && { backgroundColor: pal.surface, borderColor: pal.divider },
            ]}
          >
            <Text
              style={[styles.tabText, { color: filter.group_by === mode ? pal.fg : pal.fg2 }]}
            >
              {mode ? (zh ? '来源汇总' : 'Sources') : zh ? '日志' : 'Logs'}
            </Text>
          </TouchableOpacity>
        ))}
      </View>
      <View style={styles.filters}>
        {filters.map(k => (
          <TouchableOpacity
            key={k}
            ref={r => {
              refs.current[k] = r;
            }}
            accessibilityRole="button"
            accessibilityLabel={`${names[k]}: ${labels[k]}`}
            accessibilityState={{expanded: kind === k, selected: selected[k]}}
            onPress={() => {
              setKind(k);
              setAnchor(null);
              refs.current[k]?.measureInWindow((x, y, width, height) =>
                setAnchor({ x, y, width, height }),
              );
            }}
            style={[styles.filter, {
              backgroundColor: selected[k] ? pal.rowSelected : pal.surface,
              borderColor: selected[k] ? pal.divLoud : pal.divider,
              flexBasis: Math.ceil(95 * Math.max(1, fontScale)),
            }]}
          >
            <Text style={[styles.filterLabel, { color: pal.fg }]}>{names[k]}</Text>
            <SIcon name="chevronDown" size={14} color={pal.fg2} />
          </TouchableOpacity>
        ))}
      </View>
      {selectedFilters.length > 0 && <View style={styles.active}>
        <View style={styles.activeValues}>
          {selectedFilters.map(k => <Text key={k} style={[styles.activeFilter, {color: pal.fg2}]}>{names[k]} · {labels[k]}</Text>)}
        </View>
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={zh ? '清除访问历史筛选' : 'Clear access history filters'}
            onPress={() => onChange({ group_by: filter.group_by })}
            style={styles.clear}
          >
            <Text style={[styles.clearText, { color: pal.fg }]}>{zh ? '清除' : 'Clear'}</Text>
          </TouchableOpacity>
      </View>}
      <AnchoredMenu
        visible={kind !== null}
        anchor={anchor}
        title={kind ? names[kind] : ''}
        sections={[kind ? options[kind] : []]}
        pal={pal}
        closeLabel={zh ? '关闭筛选' : 'Close filters'}
        onClose={() => setKind(null)}
        preferredWidth={310}
      />
    </View>
  );
}
const styles = StyleSheet.create({
  segmented: {flexDirection: 'row', gap: 3, padding: 3, borderRadius: Size.radiusRow + 2, borderWidth: StyleSheet.hairlineWidth},
  tab: {
    flex: 1,
    minHeight: 44,
    paddingHorizontal: 6,
    paddingVertical: 6,
    justifyContent: 'center',
    borderRadius: Size.radiusRow - 1,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: 'transparent',
  },
  tabText: {fontSize: 14, fontWeight: '600', textAlign: 'center'},
  filters: {flexDirection: 'row', flexWrap: 'wrap', gap: 7, marginTop: 9},
  filter: {
    flexGrow: 1,
    flexShrink: 1,
    minHeight: 44,
    paddingHorizontal: 9,
    paddingVertical: 6,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 3,
    borderRadius: Size.radiusRow - 2,
    borderWidth: StyleSheet.hairlineWidth,
  },
  filterLabel: {fontSize: 13, fontWeight: '600', flexShrink: 1},
  active: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingLeft: 3},
  activeValues: {flex: 1, gap: 2, paddingVertical: 6},
  activeFilter: {fontSize: 12.5, lineHeight: 18},
  clear: {minHeight: 44, minWidth: 44, paddingHorizontal: 5, justifyContent: 'center', alignItems: 'center'},
  clearText: {fontSize: 13, fontWeight: '600'},
});
