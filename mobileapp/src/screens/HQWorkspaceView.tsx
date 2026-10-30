// Shared matter data, with a compact landing page and the existing HQ console.
import React, { useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { Agent } from '../api/types';
import { isAuthError } from '../api/client';
import type { HQWorkspace } from '../api/hqWorkspace';
import { useAgents } from '../state/AgentsContext';
import { useApp } from '../state/AppContext';
import type { HQResourceRequest } from '../state/WorkspaceContext';
import { createReadTask, startReadPolling } from '../state/readTask';
import { HQAvatar } from '../ui/HQAvatar';
import type { SizeClass } from '../ui/layout';
import { HQOverview } from './HQOverview';
import { HQMatterSheet } from './HQMatterSheet';
import { HQWorkspaceStyles as s } from './hqWorkspaceStyles';

type Props = {
  agent: Agent;
  prefill?: string;
  onBack?: () => void;
  layout?: SizeClass;
  showInspector?: boolean;
  openResource?: HQResourceRequest;
};
export function HQWorkspaceView(
  props: Props & { renderConsole: (props: Props) => React.ReactNode },
) {
  const { client, isGuest } = useAgents();
  const { pal, lang } = useApp();
  const zh = lang === 'zh',
    t = (en: string, cn: string) => (zh ? cn : en);
  const [page, setPage] = useState<'overview' | 'console'>(() =>
    props.prefill || props.openResource ? 'console' : 'overview',
  );
  const [prefill, setPrefill] = useState(props.prefill);
  const [resource, setResource] = useState(props.openResource);
  const [record, setRecord] = useState<{
    client: typeof client;
    value: HQWorkspace | null;
  } | null>(null);
  const [failed, setFailed] = useState(false),
    [refreshing, setRefreshing] = useState(false);
  const [contentWidth, setContentWidth] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const retry = useRef<() => void>(() => {});
  const supported = typeof client.hqWorkspace === 'function' && !isGuest;
  const data = record?.client === client ? record.value : undefined;
  useEffect(() => {
    if (props.prefill || props.openResource) {
      setPage('console');
      setPrefill(props.prefill);
      setResource(props.openResource);
    }
  }, [props.prefill, props.openResource]);
  useEffect(() => {
    setSelected(null);
    setFailed(false);
  }, [client]);
  useEffect(() => {
    if (!supported || page !== 'overview') return;
    const task = createReadTask(
      signal => client.hqWorkspace(signal),
      value => {
        setRecord({ client, value });
        setFailed(false);
      },
      {
        onError: error => {
          setFailed(true);
          if (isAuthError(error)) {
            setRecord(null);
            setSelected(null);
          }
        },
        onSettled: () => setRefreshing(false),
      },
    );
    retry.current = task.run;
    return startReadPolling(task, 10000);
  }, [client, supported, page]);
  if (!supported || data === null || page === 'console')
    return props.renderConsole({
      ...props,
      prefill,
      openResource: resource,
      onBack:
        supported && data !== null
          ? () => {
              setPage('overview');
              setPrefill(undefined);
              setResource(undefined);
            }
          : props.onBack,
    });
  const talk = (text?: string, door?: HQResourceRequest['page']) => {
    setPrefill(text);
    setResource(door ? { page: door, at: Date.now() } : undefined);
    setPage('console');
  };
  return (
    <SafeAreaView
      onLayout={event => setContentWidth(event.nativeEvent.layout.width)}
      style={[s.root, { backgroundColor: pal.bg }]}
      edges={['top', 'bottom']}
    >
      <View style={[s.header, { borderBottomColor: pal.divider }]}>
        {props.onBack && (
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={t('Back', '返回')}
            onPress={props.onBack}
            style={s.icon}
          >
            <Text style={[s.back, { color: pal.fg2 }]}>‹</Text>
          </TouchableOpacity>
        )}
        <HQAvatar size={32} ink={pal.fg2} />
        <View style={s.flex}>
          <Text accessibilityRole="header" style={[s.title, { color: pal.fg }]}>
            HQ
          </Text>
          <Text style={[s.caption, { color: pal.fg2 }]}>
            {t('Overview', '总览')}
          </Text>
        </View>
        <TouchableOpacity
          testID="hq-overview-talk"
          accessibilityRole="button"
          onPress={() => talk()}
          style={[
            s.talk,
            { backgroundColor: pal.surface, borderColor: pal.divider },
          ]}
        >
          <Text style={[s.buttonText, { color: pal.fg }]}>
            {t('Talk to HQ', '与 HQ 对话')}
          </Text>
        </TouchableOpacity>
      </View>
      <ScrollView
        testID="hq-overview"
        contentContainerStyle={s.content}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={() => {
              setRefreshing(true);
              retry.current();
            }}
            tintColor={pal.fg2}
          />
        }
      >
        {failed && (
          <TouchableOpacity
            accessibilityRole="button"
            onPress={() => retry.current()}
            style={s.notice}
          >
            <Text style={{ color: pal.fg2 }}>
              {t(
                data
                  ? 'Couldn’t refresh. Showing the last update. Tap to retry.'
                  : 'Couldn’t load HQ. Tap to retry.',
                data
                  ? '刷新失败，保留上次内容。轻点重试。'
                  : 'HQ 暂时无法加载，轻点重试。',
              )}
            </Text>
          </TouchableOpacity>
        )}
        {!data && !failed && (
          <ActivityIndicator
            accessibilityLabel={t('Loading HQ', '正在加载 HQ')}
            style={s.loading}
            color={pal.fg2}
          />
        )}
        {data && (
          <HQOverview
            data={data}
            pal={pal}
            zh={zh}
            wide={contentWidth >= 700}
            onMatter={setSelected}
          />
        )}
        <Text
          accessibilityRole="header"
          style={[s.sectionTitle, { color: pal.fg }]}
        >
          {t('Resources', '资料')}
        </Text>
        <View
          style={[
            s.resources,
            { backgroundColor: pal.surface, borderColor: pal.divider },
          ]}
        >
          {(['board', 'knowledge', 'usage'] as const).map((door, i) => (
            <TouchableOpacity
              key={door}
              accessibilityRole="button"
              style={[
                s.resource,
                i > 0 && {
                  borderLeftWidth: StyleSheet.hairlineWidth,
                  borderLeftColor: pal.divider,
                },
              ]}
              onPress={() => talk(undefined, door)}
            >
              <Text style={[s.buttonText, { color: pal.fg }]}>
                {
                  [
                    t('Board', '态势板'),
                    t('Knowledge', '知识库'),
                    t('Usage', '用量'),
                  ][i]
                }
              </Text>
            </TouchableOpacity>
          ))}
        </View>
        {data && (
          <Text style={[s.footnote, { color: pal.fg2 }]}>
            {t('Updated', '更新于')}{' '}
            {new Date(data.overview.generated_at * 1000).toLocaleTimeString(
              zh ? 'zh-CN' : 'en-US',
              { hour: '2-digit', minute: '2-digit' },
            )}
          </Text>
        )}
      </ScrollView>
      {selected && (
        <HQMatterSheet
          key={`${client.base}|${selected}`}
          id={selected}
          pal={pal}
          zh={zh}
          onClose={() => setSelected(null)}
          onTalk={title => {
            setSelected(null);
            talk(t(`About “${title}”: `, `关于「${title}」：`));
          }}
        />
      )}
    </SafeAreaView>
  );
}
