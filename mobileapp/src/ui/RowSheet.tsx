// RowSheet — the bottom sheet a long-press on a radar row opens.
//
// It replaces a system alert that showed the agent's name and the task, both of which
// were already on the row being pressed. What belongs here is what the row CANNOT say:
// the full task and the full error (the row clamps both to one line), which pane this
// is, and the one action that otherwise lives two screens away.
//
// What never belongs here: anything destructive. A long-press is a browsing gesture and
// a misfire must not cost a session.

import React from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {Agent} from '../api/types';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';
import {TestIds} from '../constants/testIds';
import {buildRowSheet, focusCommand, SheetActionKey} from './rowSheetModel';

export function RowSheet({
  agent,
  pal,
  lang,
  onClose,
  onOpen,
  onJump,
  onDiff,
}: {
  agent: Agent | null;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
  onOpen: (a: Agent) => void;
  onJump: (a: Agent) => void;
  onDiff: (a: Agent) => void;
}) {
  const zh = lang === 'zh';
  const [copied, setCopied] = React.useState(false);
  React.useEffect(() => {
    if (agent) setCopied(false);
  }, [agent]);
  if (!agent) return null;

  const m = buildRowSheet(agent, lang, Math.floor(Date.now() / 1000));
  const accent = agent.error ? ERRORED_COLOR : StatusColor[agent.status] ?? pal.fg3;

  const run = (key: SheetActionKey) => {
    if (key === 'copy') {
      Clipboard.setString(focusCommand(agent.pane_id));
      setCopied(true); // stay open: copying is not leaving
      return;
    }
    onClose();
    if (key === 'jump') onJump(agent);
    if (key === 'diff') onDiff(agent);
    if (key === 'open') onOpen(agent);
  };

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity
          testID={TestIds.agent.sheet}
          accessibilityLabel={TestIds.agent.sheet}
          activeOpacity={1}
          style={[styles.sheet, {backgroundColor: pal.bg, borderColor: pal.divider}]}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />

          {/* header: who this is, and what state it is in */}
          <View style={styles.head}>
            <AgentAvatar
              agent={agent}
              size={40}
              radius={11}
              bg={pal.surface}
              fg={pal.fg2}
              border={pal.divider}
            />
            <View style={styles.headText}>
              <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
                {m.title}
              </Text>
              {!!m.status && (
                <Text style={[styles.status, {color: accent}]} numberOfLines={1}>
                  {m.status}
                </Text>
              )}
            </View>
          </View>

          <ScrollView style={styles.body} contentContainerStyle={styles.bodyInner}>
            {/* identity — the anchor, always first */}
            <View style={[styles.block, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              {m.identity.map(line => (
                <Text key={line} style={[styles.ident, {color: pal.fg2}]}>
                  {line}
                </Text>
              ))}
            </View>

            {/* what it is doing — unclamped, which is the whole reason this exists */}
            {!!m.task && (
              <Text style={[styles.task, {color: pal.fg}]} selectable>
                {m.task}
              </Text>
            )}
            {!!m.error && (
              <Text style={[styles.note, {color: ERRORED_COLOR}]} selectable>
                ⚠ {m.error}
              </Text>
            )}
            {!!m.background && (
              <Text style={[styles.note, {color: ERRORED_COLOR}]} selectable>
                ⧗ {m.background}
              </Text>
            )}

            {/* what you can do from here */}
            {m.actions.map(act => (
              <TouchableOpacity
                key={act.key}
                testID={`${TestIds.agent.sheetAction}-${act.key}`}
                accessibilityLabel={`${TestIds.agent.sheetAction}-${act.key}`}
                activeOpacity={act.disabled ? 1 : 0.6}
                disabled={act.disabled}
                onPress={() => run(act.key)}
                style={[
                  styles.card,
                  {backgroundColor: pal.surface, borderColor: pal.divider},
                  act.disabled && styles.cardOff,
                ]}>
                <View style={styles.textCol}>
                  <Text style={[styles.cardTitle, {color: act.disabled ? pal.fg3 : pal.fg}]}>
                    {act.key === 'copy' && copied ? (zh ? '已复制' : 'Copied') : act.title}
                  </Text>
                  <Text style={[styles.cardSub, {color: pal.fg3}]} numberOfLines={2}>
                    {act.sub}
                  </Text>
                </View>
              </TouchableOpacity>
            ))}
          </ScrollView>
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 18, borderTopRightRadius: 18, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 8, paddingBottom: 30, maxHeight: '82%'},
  grabber: {width: 38, height: 5, borderRadius: 3, alignSelf: 'center', marginBottom: 14, opacity: 0.9},
  head: {flexDirection: 'row', alignItems: 'center', marginBottom: 12},
  headText: {flex: 1, marginLeft: 12},
  title: {fontSize: 20, fontWeight: '700'},
  status: {fontSize: 13.5, fontWeight: '600', marginTop: 2},
  body: {flexGrow: 0},
  bodyInner: {paddingBottom: 4},
  block: {padding: 12, borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, marginBottom: 12},
  ident: {fontSize: 13.5, lineHeight: 20, fontVariant: ['tabular-nums']},
  task: {fontSize: 15.5, lineHeight: 22, marginBottom: 12},
  note: {fontSize: 14, lineHeight: 20, marginBottom: 12},
  card: {padding: 12, borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, marginBottom: 10},
  cardOff: {opacity: 0.55},
  textCol: {flex: 1},
  cardTitle: {fontSize: 16, fontWeight: '600'},
  cardSub: {fontSize: 13, marginTop: 2},
});
