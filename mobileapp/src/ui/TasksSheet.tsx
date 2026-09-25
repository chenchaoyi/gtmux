import React, {useState} from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {Lang} from '../i18n';
import {Palette, StatusColor} from './theme';
import {
  BackgroundTask, TaskStatus, canOpen, isRunning, ordered, rowDetail,
} from '../api/backgroundTasks';

// The sheet behind the running row (chat-background-tasks): what was dispatched, in two
// groups, and a way to get to it.
//
// Tapping a row goes to that task's PANE. The reference this borrows from puts a stop
// button here, which suits a shell command; stopping an agent's work is `reap` or a
// sentence sent into the pane, and neither is a list-row button. Going there is the
// action, which is also the thing the phone was already good at.
//
// A task whose pane is gone keeps its row, dimmed and without a chevron. It happened, and
// dropping it would make the list disagree with the ledger.

const MAX_FRACTION = 0.78; // of the screen HEIGHT, resolved in points — see MOBILE.md

export function TasksSheet({
  visible,
  tasks,
  pal,
  lang,
  nowSec,
  age,
  onClose,
  onOpenPane,
}: {
  visible: boolean;
  tasks: BackgroundTask[];
  pal: Palette;
  lang: Lang;
  nowSec: number;
  age: (secs: number) => string;
  onClose: () => void;
  onOpenPane: (pane: string) => void;
}) {
  const zh = lang === 'zh';
  const {height} = useWindowDimensions();
  const [runOpen, setRunOpen] = useState(true);
  const [doneOpen, setDoneOpen] = useState(true);

  const sorted = ordered(tasks);
  const running = sorted.filter(isRunning);
  const finished = sorted.filter(t => !isRunning(t));

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      {/* The tap-outside layer is a Touchable but no accessibility element, so VoiceOver
          reaches the rows inside instead of collapsing the sheet into one element. */}
      <TouchableOpacity style={styles.backdrop} activeOpacity={1} accessible={false} onPress={onClose}>
        {/* The sheet itself only swallows taps: a View with a responder, never a Touchable. */}
        <View
          onStartShouldSetResponder={() => true}
          style={[
            styles.sheet,
            {backgroundColor: pal.surface, maxHeight: Math.round(height * MAX_FRACTION)},
          ]}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
          <View style={styles.head}>
            <TouchableOpacity
              testID="tasks-sheet-close"
              accessibilityRole="button"
              accessibilityLabel={zh ? '关闭' : 'Close'}
              onPress={onClose}
              style={[styles.close, {backgroundColor: pal.raised}]}>
              <Text style={[styles.closeMark, {color: pal.fg}]}>✕</Text>
            </TouchableOpacity>
            <Text style={[styles.title, {color: pal.fg}]}>{zh ? '后台任务' : 'Running in the background'}</Text>
            <View style={styles.close} />
          </View>

          <ScrollView contentContainerStyle={styles.body} showsVerticalScrollIndicator={false}>
            <Group
              label={zh ? '在跑' : 'Running'}
              count={0}
              open={runOpen}
              pal={pal}
              onToggle={() => setRunOpen(v => !v)}
            />
            {runOpen &&
              (running.length ? (
                running.map(t => (
                  <Row key={t.id} task={t} pal={pal} zh={zh} nowSec={nowSec} age={age} onOpenPane={onOpenPane} />
                ))
              ) : (
                <View style={[styles.empty, {backgroundColor: pal.bg}]}>
                  <Text style={[styles.emptyText, {color: pal.fg2}]}>
                    {zh ? '现在没有在跑的' : 'Nothing is running'}
                  </Text>
                </View>
              ))}

            {finished.length > 0 && (
              <>
                <Group
                  label={zh ? '干完了' : 'Finished'}
                  count={finished.length}
                  open={doneOpen}
                  pal={pal}
                  onToggle={() => setDoneOpen(v => !v)}
                />
                {doneOpen &&
                  finished.map(t => (
                    <Row key={t.id} task={t} pal={pal} zh={zh} nowSec={nowSec} age={age} onOpenPane={onOpenPane} />
                  ))}
              </>
            )}
          </ScrollView>
        </View>
      </TouchableOpacity>
    </Modal>
  );
}

function Group({
  label, count, open, pal, onToggle,
}: {
  label: string; count: number; open: boolean; pal: Palette; onToggle: () => void;
}) {
  return (
    <TouchableOpacity
      accessibilityRole="button"
      accessibilityLabel={count > 0 ? `${label} ${count}` : label}
      accessibilityState={{expanded: open}}
      onPress={onToggle}
      style={styles.group}>
      <Text style={[styles.groupLabel, {color: pal.fg2}]}>
        {label}
        {count > 0 ? ` ${count}` : ''}
      </Text>
      <Text style={[styles.groupMark, {color: pal.fg3}]}>{open ? '⌄' : '›'}</Text>
    </TouchableOpacity>
  );
}

function Row({
  task, pal, zh, nowSec, age, onOpenPane,
}: {
  task: BackgroundTask; pal: Palette; zh: boolean; nowSec: number;
  age: (s: number) => string; onOpenPane: (pane: string) => void;
}) {
  const open = canOpen(task);
  const detail = rowDetail(task, nowSec, zh, age);
  return (
    <TouchableOpacity
      testID={`task-${task.id}`}
      accessibilityRole="button"
      accessibilityLabel={`${task.goal}. ${detail}`}
      accessibilityState={{disabled: !open}}
      activeOpacity={open ? 0.6 : 1}
      disabled={!open}
      onPress={() => onOpenPane(task.pane)}
      style={[styles.card, {backgroundColor: pal.raised, opacity: open ? 1 : 0.55}]}>
      <StatusMark status={task.status} />
      <View style={styles.cardBody}>
        <Text style={[styles.goal, {color: open ? pal.fg : pal.fg2}]} numberOfLines={2}>
          {task.goal || (zh ? '（没写目标）' : '(no goal recorded)')}
        </Text>
        <Text style={[styles.detail, {color: pal.fg2}]} numberOfLines={1}>
          {detail}
        </Text>
      </View>
      {open ? <Text style={[styles.chevron, {color: pal.fg3}]}>›</Text> : null}
    </TouchableOpacity>
  );
}

// Colour, shape and glyph together (DESIGN §9's triple encoding), and nothing moves.
function StatusMark({status}: {status: TaskStatus}) {
  if (status === 'waiting') {
    return (
      <View style={styles.bars}>
        <View style={[styles.bar, {backgroundColor: StatusColor.waiting}]} />
        <View style={[styles.bar, {backgroundColor: StatusColor.waiting}]} />
      </View>
    );
  }
  if (status === 'working') {
    return <View style={[styles.ring, {borderColor: StatusColor.working}]} />;
  }
  if (status === 'idle') {
    return (
      <View style={[styles.tick, {borderColor: StatusColor.idle}]}>
        <Text style={[styles.tickMark, {color: StatusColor.idle}]}>✓</Text>
      </View>
    );
  }
  return <View style={[styles.dot, {backgroundColor: StatusColor.running}]} />;
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end'},
  sheet: {borderTopLeftRadius: 20, borderTopRightRadius: 20, paddingBottom: 28},
  grabber: {width: 36, height: 4, borderRadius: 2, alignSelf: 'center', marginTop: 8},
  head: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingTop: 10, paddingBottom: 12},
  close: {width: 32, height: 32, borderRadius: 16, alignItems: 'center', justifyContent: 'center'},
  closeMark: {fontSize: 15, fontWeight: '600'},
  title: {flex: 1, textAlign: 'center', fontSize: 17, fontWeight: '600'},
  body: {paddingHorizontal: 16, paddingBottom: 8, gap: 8},
  group: {flexDirection: 'row', alignItems: 'center', gap: 6, paddingTop: 6, paddingBottom: 2, paddingHorizontal: 2},
  groupLabel: {fontSize: 13},
  groupMark: {fontSize: 13},
  empty: {borderRadius: 12, paddingVertical: 16, alignItems: 'center'},
  emptyText: {fontSize: 14},
  card: {flexDirection: 'row', alignItems: 'center', gap: 11, borderRadius: 12, paddingHorizontal: 14, paddingVertical: 13},
  cardBody: {flex: 1, gap: 3},
  goal: {fontSize: 15, lineHeight: 20},
  detail: {fontSize: 13},
  chevron: {fontSize: 20, marginTop: -2},
  bars: {flexDirection: 'row', gap: 2},
  bar: {width: 3, height: 14, borderRadius: 1},
  ring: {width: 14, height: 14, borderRadius: 7, borderWidth: 2.4, borderRightColor: 'transparent'},
  tick: {width: 15, height: 15, borderRadius: 8, borderWidth: 1.8, alignItems: 'center', justifyContent: 'center'},
  tickMark: {fontSize: 9, fontWeight: '700', marginTop: -1},
  dot: {width: 8, height: 8, borderRadius: 4, marginHorizontal: 3},
});
