import React from 'react';
import {
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
  useWindowDimensions,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Agent, nativeEvidenceLabel, primary, secondary } from '../api/types';
import { Lang } from '../i18n';
import { ChromeText } from './ChromeText';
import { Chevron } from './Icons';
import { MODAL_ORIENTATIONS } from './modalOrientations';
import { Palette } from './theme';

type Props = { agent: Agent; pal: Palette; lang: Lang };

export function ExternalConversationHeader({
  agent,
  pal,
  lang,
  demo,
  onBack,
  onDetails,
  children,
}: Props & {
  demo?: boolean;
  onBack?: () => void;
  onDetails: () => void;
  children?: React.ReactNode;
}) {
  const zh = lang === 'zh';
  const title = primary(agent) || agent.agent;
  return (
    <View
      testID="external-conversation-header"
      style={[s.header, { borderBottomColor: pal.divider }]}
    >
      {onBack && (
        <TouchableOpacity
          onPress={onBack}
          accessibilityRole="button"
          accessibilityLabel={zh ? '返回' : 'Back'}
          style={s.back}
        >
          <View style={s.backIcon}>
            <Chevron size={22} color={pal.fg} />
          </View>
        </TouchableOpacity>
      )}
      <TouchableOpacity
        testID="external-conversation-details-open"
        accessibilityRole="button"
        accessibilityLabel={`${title}, ${agent.agent}, ${
          zh ? '只读' : 'Read only'
        }`}
        accessibilityHint={zh ? '查看会话详情' : 'View conversation details'}
        onPress={onDetails}
        style={s.identity}
      >
        <View style={s.titleRow}>
          <ChromeText numberOfLines={1} style={[s.title, { color: pal.fg }]}>
            {title}
          </ChromeText>
          <Chevron open size={14} color={pal.fg2} />
        </View>
        <View style={s.summary}>
          <ChromeText
            numberOfLines={1}
            style={[s.provider, { color: pal.fg2 }]}
          >
            {agent.agent}
          </ChromeText>
          <ChromeText style={[s.access, { color: pal.fg2 }]}>{` · ${
            zh ? '只读' : 'Read only'
          }${demo ? ' · DEMO' : ''}`}</ChromeText>
        </View>
      </TouchableOpacity>
      {children}
    </View>
  );
}

export function ExternalConversationDetails({
  agent,
  pal,
  lang,
  onClose,
}: Props & { onClose: () => void }) {
  const zh = lang === 'zh';
  const { height } = useWindowDimensions();
  const inset = useSafeAreaInsets();
  const evidence = nativeEvidenceLabel(agent, lang);
  return (
    <Modal
      visible
      transparent
      animationType="fade"
      supportedOrientations={MODAL_ORIENTATIONS}
      onRequestClose={onClose}
    >
      <View style={s.backdrop}>
        <Pressable
          testID="external-conversation-details-backdrop"
          accessible={false}
          style={StyleSheet.absoluteFill}
          onPress={onClose}
        />
        <View
          testID="external-conversation-details"
          accessibilityViewIsModal
          onAccessibilityEscape={onClose}
          style={[
            s.sheet,
            {
              backgroundColor: pal.surface,
              maxHeight: height - inset.top - 24,
              paddingBottom: Math.max(inset.bottom, 16),
            },
          ]}
        >
          <View style={[s.sheetHeader, { borderBottomColor: pal.divider }]}>
            <ChromeText
              accessibilityRole="header"
              style={[s.heading, { color: pal.fg }]}
            >
              {zh ? '会话详情' : 'Conversation details'}
            </ChromeText>
            <TouchableOpacity
              testID="external-conversation-details-close"
              accessibilityRole="button"
              accessibilityLabel={
                zh ? '关闭会话详情' : 'Close conversation details'
              }
              onPress={onClose}
              style={s.close}
            >
              <ChromeText style={[s.done, { color: pal.fg }]}>
                {zh ? '完成' : 'Done'}
              </ChromeText>
            </TouchableOpacity>
          </View>
          <ScrollView style={s.scroll} contentContainerStyle={s.content}>
            <Text selectable style={[s.fullTitle, { color: pal.fg }]}>
              {primary(agent) || agent.agent}
            </Text>
            <View style={s.fact}>
              <Text style={[s.label, { color: pal.fg2 }]}>
                {zh ? '来源' : 'Source'}
              </Text>
              <Text selectable style={[s.value, { color: pal.fg }]}>
                {secondary(agent, lang)}
              </Text>
            </View>
            <View style={s.fact}>
              <Text style={[s.label, { color: pal.fg2 }]}>
                {zh ? '只读' : 'Read only'}
              </Text>
              <Text style={[s.value, { color: pal.fg }]}>
                {zh
                  ? '可在这里查看对话；请在原客户端发送消息。'
                  : 'View the conversation here. Send messages from its original client.'}
              </Text>
            </View>
            {!!evidence && (
              <View style={s.fact}>
                <Text style={[s.label, { color: pal.fg2 }]}>
                  {zh ? '连接信息' : 'Connection'}
                </Text>
                <Text selectable style={[s.value, { color: pal.fg }]}>
                  {evidence}
                </Text>
              </View>
            )}
          </ScrollView>
        </View>
      </View>
    </Modal>
  );
}

const s = StyleSheet.create({
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 8,
    paddingVertical: 6,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  back: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  backIcon: { transform: [{ rotate: '180deg' }] },
  identity: {
    flex: 1,
    minWidth: 0,
    minHeight: 44,
    justifyContent: 'center',
    paddingHorizontal: 8,
    gap: 3,
  },
  titleRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  title: { flexShrink: 1, fontSize: 17, fontWeight: '600' },
  summary: { flexDirection: 'row', alignItems: 'center' },
  provider: { flexShrink: 1, fontSize: 12 },
  access: { flexShrink: 0, fontSize: 12 },
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.4)',
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  sheet: {
    width: '100%',
    maxWidth: 560,
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    overflow: 'hidden',
    flexShrink: 1,
  },
  sheetHeader: {
    paddingHorizontal: 20,
    minHeight: 56,
    flexDirection: 'row',
    alignItems: 'center',
    borderBottomWidth: StyleSheet.hairlineWidth,
    gap: 12,
  },
  heading: { flex: 1, fontSize: 17, fontWeight: '600' },
  close: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  done: { fontSize: 15, fontWeight: '600' },
  scroll: { flexGrow: 0, flexShrink: 1 },
  content: { padding: 20, gap: 20 },
  fullTitle: { fontSize: 20, lineHeight: 28, fontWeight: '600' },
  fact: { gap: 6 },
  label: { fontSize: 13, lineHeight: 18 },
  value: { fontSize: 15, lineHeight: 22 },
});
