// RowSheet — the bottom sheet a long-press on a radar row opens.
//
// It exists to give back what the row clamps (the full task, the full error) and to let
// you act on the session without opening it. Three things it must not do, each learned
// the hard way:
//
//   - repeat the row. The first version titled itself with the agent name, which the
//     avatar already says and the row's own second line already places. The header is
//     the ANCHOR now (session · %pane) and the body leads with the task.
//   - re-animate. The spring restarted on every render because a prop was an inline
//     arrow and the radar re-renders every ~1.5s, so the sheet visibly popped up again
//     and again while it sat open. It springs once per opened row.
//   - describe an action without saying what it sends. "Carry on" is a label, not a
//     contract: every acting row names the literal keystroke or text underneath it.
//
// A long press only OPENS this. Acting takes a second, deliberate tap, and nothing here
// ends a session — the heaviest interrupts a turn, which the next "carry on" undoes.

import React from 'react';
import {Animated, Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent, ReplyOption, secondary} from '../api/types';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {ActionIcon, ActionIconName} from './ActionIcon';
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
  onAct: (a: Agent, act: {kind: 'option' | 'continue' | 'stop' | 'ask-hq'; n?: number}) => void;
  loadOptions?: (a: Agent) => Promise<ReplyOption[]>;
}) {
  const [options, setOptions] = React.useState<ReplyOption[]>([]);
  const rise = React.useRef(new Animated.Value(0)).current;

  // The identity of the row this sheet is showing. Everything below keys off THIS, not
  // off the agent object: the radar hands down a fresh object every poll, and depending
  // on it restarted the spring and re-fetched the options a few times a second.
  const paneID = agent?.pane_id ?? '';
  const blocked = !!agent && agent.status === 'waiting' && agent.source !== 'native';

  // Props are read through a ref for the same reason: a caller writing
  // `loadOptions={a => client.options(a.pane_id)}` — which is the natural way to write it
  // — changes identity on every render of the screen above.
  const latest = React.useRef({agent, loadOptions});
  latest.current = {agent, loadOptions};

  React.useEffect(() => {
    setOptions([]);
    if (!paneID) return;
    rise.setValue(0);
    Animated.spring(rise, {toValue: 1, useNativeDriver: true, damping: 18, stiffness: 220, mass: 0.7}).start();
    if (!blocked) return;
    const {agent: a, loadOptions: load} = latest.current;
    if (!a || !load) return;
    let alive = true;
    load(a)
      .then(o => alive && setOptions(o))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [paneID, blocked, rise]);

  if (!agent) return null;

  const m = buildRowSheet(agent, lang, Math.floor(Date.now() / 1000));
  const accent = agent.error ? ERRORED_COLOR : StatusColor[agent.status] ?? pal.fg3;
  const icons: Record<SheetActionKey, ActionIconName> = {
    reply: 'reply', continue: 'continue', stop: 'stop', 'ask-hq': 'ask-hq', diff: 'diff', jump: 'jump',
  };

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

            {/* The anchor: where this is and what state it is in. NOT the agent's name —
                the avatar carries that, and repeating it spends the largest type on the
                page saying what the reader already knows. */}
            <View style={styles.head}>
              <AgentAvatar agent={agent} size={38} radius={11} bg={pal.surface} fg={pal.fg2} border={pal.divider} />
              <View style={styles.headText}>
                <Text style={[styles.where, {color: pal.fg}]} numberOfLines={1}>
                  {m.anchor || secondary(agent)}
                </Text>
                <View style={styles.metaRow}>
                  {!!m.status && (
                    <Text style={[styles.status, {color: accent}]} numberOfLines={1}>
                      {m.status}
                    </Text>
                  )}
                  {!!m.where && (
                    <Text style={[styles.meta, {color: pal.fg3}]} numberOfLines={1}>
                      {m.where}
                    </Text>
                  )}
                </View>
              </View>
            </View>

            <ScrollView style={styles.body} contentContainerStyle={styles.bodyInner}>
              {/* The task, unclamped. This is the reason the sheet exists. */}
              {!!m.task && (
                <Text style={[styles.task, {color: pal.fg}]} selectable>
                  {m.task}
                </Text>
              )}
              {!!m.error && (
                <Text style={[styles.note, {color: ERRORED_COLOR}]} selectable>
                  {m.error}
                </Text>
              )}
              {!!m.background && (
                <Text style={[styles.note, {color: pal.fg3}]} selectable>
                  {m.background}
                </Text>
              )}

              {/* The question, as buttons. A blocked session is the one case where the
                  phone finishes the job instead of routing you to the Mac, so its own
                  words come before every generic action. */}
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

              {/* Actions as a grouped list: one hairline-separated row each, an icon, and
                  underneath it the literal thing that gets sent. The previous version
                  gave every action a bordered card of its own, which at four of them read
                  as four competing buttons rather than a menu. */}
              <View style={[styles.actions, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
                {m.actions.map((act, i) => (
                  <TouchableOpacity
                    key={act.key}
                    testID={`${TestIds.agent.sheetAction}-${act.key}`}
                    accessibilityLabel={`${TestIds.agent.sheetAction}-${act.key}`}
                    activeOpacity={act.disabled ? 1 : 0.55}
                    disabled={act.disabled}
                    onPress={() => run(act.key)}
                    style={[
                      styles.action,
                      i > 0 && {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider},
                      act.disabled && styles.actionOff,
                    ]}>
                    <ActionIcon name={icons[act.key]} color={act.disabled ? pal.fg3 : pal.fg2} />
                    <View style={styles.actionText}>
                      <Text style={[styles.actionTitle, {color: act.disabled ? pal.fg3 : pal.fg}]}>{act.title}</Text>
                      <Text style={[styles.actionSub, {color: pal.fg3}]} numberOfLines={2}>
                        {act.sub}
                      </Text>
                    </View>
                  </TouchableOpacity>
                ))}
              </View>
            </ScrollView>
          </TouchableOpacity>
        </Animated.View>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end'},
  sheet: {
    borderTopLeftRadius: 18, borderTopRightRadius: 18, borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 14, paddingTop: 8, paddingBottom: 30, maxHeight: '82%',
  },
  grabber: {width: 38, height: 5, borderRadius: 3, alignSelf: 'center', marginBottom: 14, opacity: 0.9},

  head: {flexDirection: 'row', alignItems: 'center', marginBottom: 12, gap: 11},
  headText: {flex: 1},
  where: {fontSize: 16, fontWeight: '700'},
  metaRow: {flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 2},
  status: {fontSize: 12.5, fontWeight: '600'},
  meta: {flex: 1, fontSize: 12.5},

  body: {flexGrow: 0},
  bodyInner: {paddingBottom: 4},
  task: {fontSize: 15, lineHeight: 21, marginBottom: 12},
  note: {fontSize: 13.5, lineHeight: 19, marginBottom: 12},

  options: {marginBottom: 14, gap: 8},
  option: {flexDirection: 'row', alignItems: 'flex-start', gap: 10, padding: 12, borderRadius: 14, borderWidth: 1},
  optionN: {fontSize: 15, fontWeight: '800', minWidth: 14, fontVariant: ['tabular-nums']},
  optionText: {flex: 1, fontSize: 15, lineHeight: 21},

  actions: {borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  action: {flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 13, paddingVertical: 11},
  actionOff: {opacity: 0.5},
  actionText: {flex: 1},
  actionTitle: {fontSize: 15, fontWeight: '600'},
  actionSub: {fontSize: 12.5, marginTop: 1, lineHeight: 17},
});
