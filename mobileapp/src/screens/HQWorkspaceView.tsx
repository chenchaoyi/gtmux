// Shared matter data, with a compact landing page and the existing HQ console.
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  RefreshControl,
  ScrollView,
  Text,
  TouchableOpacity,
  View,
  useWindowDimensions,
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
import { ChromeText } from '../ui/ChromeText';
import type { SizeClass } from '../ui/layout';
import { HQOverview } from './HQOverview';
import { HQMatterSheet } from './HQMatterSheet';
import { HQResourceCards } from './HQResourceCards';
import { useHQResourceSummaries } from './useHQResourceSummaries';
import { HQWorkspaceStyles as s } from './hqWorkspaceStyles';

type Props = {
  agent: Agent;
  prefill?: string;
  onBack?: () => void;
  layout?: SizeClass;
  showInspector?: boolean;
  openResource?: HQResourceRequest;
  onResourceExit?: (destination: 'overview' | 'console') => void;
};
export function HQWorkspaceView(
  props: Props & { renderConsole: (props: Props) => React.ReactNode },
) {
  const { client, isGuest } = useAgents();
  const { pal, lang } = useApp();
  const { fontScale } = useWindowDimensions();
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
  const retireWorkspace = useRef<() => void>(() => {});
  const supported = typeof client.hqWorkspace === 'function' && !isGuest;
  const data = record?.client === client ? record.value : undefined;
  const clearPrivate = useCallback(() => {
    retireWorkspace.current();
    setRefreshing(false);
    setRecord(null);
    setSelected(null);
    setFailed(true);
  }, []);
  const resources = useHQResourceSummaries(
    client,
    supported && page === 'overview' && !!data,
    clearPrivate,
  );
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
    retireWorkspace.current = task.invalidate;
    return startReadPolling(task, 10000);
  }, [client, supported, page]);
  if (!supported || data === null || page === 'console')
    return props.renderConsole({
      ...props,
      prefill,
      openResource: resource,
      onResourceExit:
        resource && supported && data !== null
          ? destination => {
              setResource(undefined);
              if (destination === 'overview') {
                setPage('overview');
                setPrefill(undefined);
              }
            }
          : undefined,
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
            <ChromeText style={[s.back, { color: pal.fg2 }]}>‹</ChromeText>
          </TouchableOpacity>
        )}
        <HQAvatar size={32} ink={pal.fg2} />
        <View style={s.flex}>
          <ChromeText
            accessibilityRole="header"
            style={[s.title, { color: pal.fg }]}
          >
            HQ
          </ChromeText>
          <ChromeText style={[s.caption, { color: pal.fg2 }]}>
            {t('Overview', '总览')}
          </ChromeText>
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
          <ChromeText style={[s.buttonText, { color: pal.fg }]}>
            {t('Talk to HQ', '与 HQ 对话')}
          </ChromeText>
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
              resources.refresh.current();
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
          <HQResourceCards
            data={data}
            readings={resources.readings}
            pal={pal}
            zh={zh}
            wide={contentWidth / fontScale >= 700}
            width={contentWidth}
            onOpen={door => talk(undefined, door)}
          />
        )}
        {data && (
          <HQOverview
            data={data}
            pal={pal}
            zh={zh}
            wide={contentWidth / fontScale >= 700}
            onMatter={setSelected}
          />
        )}
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
