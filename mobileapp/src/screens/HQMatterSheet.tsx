import React, { useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  Modal,
  ScrollView,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import { ApiError, isAuthError } from '../api/client';
import type { HQMatterDetail } from '../api/hqWorkspace';
import { useAgents } from '../state/AgentsContext';
import { useWorkspace } from '../state/WorkspaceContext';
import { createReadTask, startReadPolling } from '../state/readTask';
import { nativeDocumentPreview } from '../documents/nativePreview';
import { DocumentError, documentErrorMessage } from '../documents/references';
import { MarkdownView } from '../ui/MarkdownView';
import { BRAND, Palette } from '../ui/theme';
import { MODAL_ORIENTATIONS } from '../ui/modalOrientations';
import { matterState, liveParticipant } from './hqWorkspaceModel';
import { HQWorkspaceStyles as s } from './hqWorkspaceStyles';

export function HQMatterSheet({
  id,
  pal,
  zh,
  onClose,
  onTalk,
}: {
  id: string;
  pal: Palette;
  zh: boolean;
  onClose: () => void;
  onTalk: (title: string) => void;
}) {
  const { client, agents } = useAgents(),
    { select } = useWorkspace();
  const [record, setRecord] = useState<{
    client: typeof client;
    detail: HQMatterDetail;
  } | null>(null);
  const [failed, setFailed] = useState(false),
    [before, setBefore] = useState(0);
  const retry = useRef<() => void>(() => {});
  const detail = record?.client === client ? record.detail : null;
  const t = (en: string, cn: string) => (zh ? cn : en);
  useEffect(() => {
    const task = createReadTask(
      signal => client.hqMatter(id, before, signal),
      value => {
        setRecord({ client, detail: value });
        setFailed(false);
      },
      {
        onError: error => {
          setFailed(true);
          if (
            isAuthError(error) ||
            (error instanceof ApiError && error.status === 404)
          )
            setRecord(null);
        },
      },
    );
    retry.current = task.run;
    return startReadPolling(task, 10000);
  }, [client, id, before]);
  const colors = {
    text: pal.fg,
    dim: pal.fg2,
    code: pal.fg,
    codeBg: pal.raised,
    border: pal.divider,
    link: BRAND,
  };
  const openFile = (
    reference: string,
    source?: { session_id: string; home_id?: string },
  ) => {
    const showError = (error: unknown) =>
      Alert.alert(
        t('File preview', '文件预览'),
        documentErrorMessage(
          error instanceof DocumentError ? error.reason : 'failed',
          zh,
        ),
      );
    const session = source ?? agents.find(a => a.role === 'supervisor');
    if (!session?.session_id) {
      showError(new DocumentError('unsupported'));
      return;
    }
    try {
      nativeDocumentPreview
        .open(
          client.documentRequest(
            reference,
            session.session_id,
            session.home_id ?? '',
            zh ? 'zh' : 'en',
          ),
        )
        .catch(showError);
    } catch (error) {
      showError(error);
    }
  };

  return (
    <Modal
      visible
      animationType="slide"
      presentationStyle="pageSheet"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={onClose}
    >
      <View style={[s.root, { backgroundColor: pal.bg }]}>
        <View style={[s.header, { borderBottomColor: pal.divider }]}>
          <Text
            accessibilityRole="header"
            style={[s.matterTitle, { color: pal.fg }]}
          >
            {t('Matter', '事项')}
          </Text>
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityLabel={t('Close matter', '关闭事项')}
            onPress={onClose}
            style={s.icon}
          >
            <Text style={[s.buttonText, { color: pal.fg2 }]}>
              {t('Close', '关闭')}
            </Text>
          </TouchableOpacity>
        </View>
        <ScrollView key={before} contentContainerStyle={[s.content, s.reading]}>
          {failed && (
            <TouchableOpacity
              accessibilityRole="button"
              onPress={() => retry.current()}
              style={s.notice}
            >
              <Text style={{ color: pal.fg2 }}>
                {t(
                  'Couldn’t refresh this matter. Tap to retry.',
                  '事项暂时无法刷新，轻点重试。',
                )}
              </Text>
            </TouchableOpacity>
          )}
          {before > 0 && (
            <TouchableOpacity
              accessibilityRole="button"
              onPress={() => {
                setRecord(null);
                setBefore(0);
              }}
              style={s.more}
            >
              <Text style={{ color: pal.fg2 }}>
                {t('Back to latest updates', '返回最新进展')}
              </Text>
            </TouchableOpacity>
          )}
          {!detail && !failed && (
            <ActivityIndicator color={pal.fg2} style={s.loading} />
          )}
          {detail && (
            <>
              <Text style={[s.heroTitle, { color: pal.fg }]}>
                {detail.matter.title}
              </Text>
              <Text style={[s.meta, { color: pal.fg2 }]}>
                {matterState(detail.matter.state, zh)}
              </Text>
              {!!detail.matter.assessment && (
                <View style={[s.assessment, { backgroundColor: pal.surface }]}>
                  <MarkdownView
                    source={detail.matter.assessment}
                    openFile={ref => openFile(ref)}
                    colors={colors}
                    fontSize={16}
                    calmEmphasis
                  />
                </View>
              )}
              <Text
                accessibilityRole="header"
                style={[s.sectionTitle, { color: pal.fg }]}
              >
                {t('Participants', '参与者')}
              </Text>
              <View
                style={[
                  s.card,
                  { backgroundColor: pal.surface, borderColor: pal.divider },
                ]}
              >
                {detail.matter.participants.length === 0 && (
                  <Text style={[s.empty, { color: pal.fg2 }]}>
                    {t('No linked sessions.', '尚未关联会话。')}
                  </Text>
                )}
                {detail.matter.participants.map(a => {
                  const agent = liveParticipant(a, agents);
                  return (
                    <TouchableOpacity
                      key={a.id}
                      accessibilityRole="button"
                      disabled={!agent}
                      accessibilityState={{ disabled: !agent }}
                      onPress={() => {
                        if (agent) {
                          onClose();
                          select({
                            kind: agent.role === 'supervisor' ? 'hq' : 'pane',
                            agent,
                          });
                        }
                      }}
                      style={s.matter}
                    >
                      <Text style={[s.matterTitle, { color: pal.fg }]}>
                        {a.conversation.name}
                      </Text>
                      <Text style={[s.meta, { color: pal.fg2 }]}>
                        {a.conversation.agent} ·{' '}
                        {a.role === 'execution'
                          ? t('Executor', '执行')
                          : t('Reference', '参考')}
                        {a.until || a.removed_at
                          ? ` · ${t('Ended', '已结束')}`
                          : ''}
                      </Text>
                    </TouchableOpacity>
                  );
                })}
              </View>
              <View style={s.sectionHead}>
                <Text
                  accessibilityRole="header"
                  style={[s.sectionTitle, { color: pal.fg }]}
                >
                  {t('Updates', '进展与对话')}
                </Text>
                <TouchableOpacity
                  accessibilityRole="button"
                  onPress={() => onTalk(detail.matter.title)}
                  style={s.more}
                >
                  <Text style={[s.buttonText, { color: pal.fg2 }]}>
                    {t('Ask HQ', '询问 HQ')}
                  </Text>
                </TouchableOpacity>
              </View>
              {detail.message_earlier_available && detail.earlier_before && (
                <TouchableOpacity
                  accessibilityRole="button"
                  onPress={() => {
                    setRecord(null);
                    setBefore(detail.earlier_before!);
                  }}
                  style={s.more}
                >
                  <Text style={{ color: pal.fg2 }}>
                    {t('Earlier updates', '查看更早记录')}
                  </Text>
                </TouchableOpacity>
              )}
              {detail.messages.length === 0 && (
                <Text style={[s.empty, { color: pal.fg2 }]}>
                  {t('No updates recorded yet.', '暂无进展记录。')}
                </Text>
              )}
              {detail.messages.map(message => (
                <View
                  key={message.id}
                  style={[s.update, { borderBottomColor: pal.divider }]}
                >
                  <Text style={[s.meta, { color: pal.fg2 }]}>
                    {message.speaker === 'hq' ? 'HQ' : t('You', '你')}
                    {message.delivery
                      ? ` · ${deliveryLabel(message.delivery, zh)}`
                      : ''}
                  </Text>
                  <MarkdownView
                    source={message.body}
                    openFile={ref =>
                      openFile(
                        ref,
                        message.target?.session_id ? message.target : undefined,
                      )
                    }
                    colors={colors}
                    fontSize={16}
                    selectable
                    clampProse
                    calmEmphasis
                  />
                  {!!message.reply?.response && (
                    <View style={s.reply}>
                      <Text style={[s.meta, { color: pal.fg2 }]}>HQ</Text>
                      <MarkdownView
                        source={message.reply.response}
                        openFile={ref => openFile(ref, message.target)}
                        colors={colors}
                        fontSize={16}
                        selectable
                        clampProse
                        calmEmphasis
                      />
                    </View>
                  )}
                  <Text style={[s.meta, { color: pal.fg2 }]}>
                    {new Date(message.at * 1000).toLocaleString(
                      zh ? 'zh-CN' : 'en-US',
                    )}
                  </Text>
                </View>
              ))}
              {detail.warnings.length > 0 && (
                <Text style={[s.footnote, { color: pal.fg2 }]}>
                  {t(
                    'Some source records are unavailable. Saved updates remain visible.',
                    '部分来源暂时无法读取，已保存的进展仍可查看。',
                  )}
                </Text>
              )}
            </>
          )}
        </ScrollView>
      </View>
    </Modal>
  );
}
function deliveryLabel(value: string, zh: boolean) {
  const labels: Record<string, string[]> = {
    pending: ['Queued', '待发送'],
    submitting: ['Sending', '正在发送'],
    delivered: ['Delivered', '已送达'],
    'accepted-queue': ['Queued by agent', '已进入会话队列'],
    uncertain: ['Delivery unconfirmed', '送达未确认'],
    cancelled: ['Cancelled', '已取消'],
  };
  return labels[value]?.[zh ? 1 : 0] ?? value;
}
