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
import {Animated, Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent} from '../api/types';
import {ReplyOption} from '../api/types';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';
import {TestIds} from '../constants/testIds';
import {buildRowSheet, SheetActionKey} from './rowSheetModel';

export function RowSheet({
  agent,
  pal,
  lang,
  onClose,
  onJump,
  onDiff,
  onAct,
  loadOptions,
}: {
  agent: Agent | null;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
  onJump: (a: Agent) => void;
  onDiff: (a: Agent) => void;
  // Everything that TALKS to the session goes through one handler: answering a numbered
  // choice, carrying on, stopping, or handing it to the supervisor. The sheet decides
  // what to offer; the screen owns the sending.
  onAct: (a: Agent, act: {kind: 'option' | 'continue' | 'stop' | 'ask-hq'; n?: number}) => void;
  loadOptions?: (a: Agent) => Promise<ReplyOption[]>;
}) {
  const [options, setOptions] = React.useState<ReplyOption[]>([]);
  // Spring the sheet in rather than sliding it. The gesture that opened this was a press
  // and hold; a linear slide answers it with nothing, which is most of what "no feel"
  // meant. A real haptic tap needs a native module and is a separate, small change.
  const rise = React.useRef(new Animated.Value(0)).current;
  const blocked = !!agent && agent.status === 'waiting' && agent.source !== 'native';

  React.useEffect(() => {
    setOptions([]);
    if (!agent) return;
    rise.setValue(0);
    Animated.spring(rise, {toValue: 1, useNativeDriver: true, damping: 18, stiffness: 220, mass: 0.7}).start();
    if (!blocked || !loadOptions) return;
    let alive = true;
    loadOptions(agent)
      .then(o => alive && setOptions(o))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [agent, blocked, loadOptions, rise]);

  if (!agent) return null;

  const m = buildRowSheet(agent, lang, Math.floor(Date.now() / 1000));
  const accent = agent.error ? ERRORED_COLOR : StatusColor[agent.status] ?? pal.fg3;

  const run = (key: SheetActionKey) => {
    onClose();
    if (key === 'jump') onJump(agent);
    else if (key === 'diff') onDiff(agent);
    else if (key === 'reply') onAct(agent, {kind: 'option'});
    else if (key === 'continue') onAct(agent, {kind: 'continue'});
    else if (key === 'stop') onAct(agent, {kind: 'stop'});
    else if (key === 'ask-hq') onAct(agent, {kind: 'ask-hq'});
  };

  return (
    <Modal visible transparent animationType="fade" onRequestClose={onClose}>
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} onPress={onClose}>
        <Animated.View
          style={{transform: [{translateY: rise.interpolate({inputRange: [0, 1], outputRange: [420, 0]})}]}}>
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

            {/* The question, as buttons. A blocked session is the one case where the phone
                can finish the job instead of routing you to the Mac, so the choices come
                before every other action and read in the agent's own words. */}
            {blocked && options.length > 0 && (
              <View style={styles.options}>
                {options.map(o => (
                  <TouchableOpacity
                    key={o.n}
                    testID={`${TestIds.agent.sheetAction}-option-${o.n}`}
                    accessibilityLabel={`${TestIds.agent.sheetAction}-option-${o.n}`}
                    activeOpacity={0.6}
                    onPress={() => {
                      onClose();
                      onAct(agent, {kind: 'option', n: o.n});
                    }}
                    style={[styles.option, {borderColor: accent, backgroundColor: pal.surface}]}>
                    <Text style={[styles.optionN, {color: accent}]}>{o.n}</Text>
                    <Text style={[styles.optionText, {color: pal.fg}]} numberOfLines={3}>
                      {o.label}
                    </Text>
                  </TouchableOpacity>
                ))}
              </View>
            )}

            {/* what else you can do from here */}
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
                    {act.title}
                  </Text>
                  <Text style={[styles.cardSub, {color: pal.fg3}]} numberOfLines={2}>
                    {act.sub}
                  </Text>
                </View>
              </TouchableOpacity>
            ))}
          </ScrollView>
        </TouchableOpacity>
        </Animated.View>
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
  options: {marginBottom: 14, gap: 8},
  option: {flexDirection: 'row', alignItems: 'flex-start', gap: 10, padding: 12, borderRadius: 14, borderWidth: 1},
  optionN: {fontSize: 15, fontWeight: '800', minWidth: 14, fontVariant: ['tabular-nums']},
  optionText: {flex: 1, fontSize: 15, lineHeight: 21},
  textCol: {flex: 1},
  cardTitle: {fontSize: 16, fontWeight: '600'},
  cardSub: {fontSize: 13, marginTop: 2},
});
