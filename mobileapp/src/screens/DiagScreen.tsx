// DiagScreen — the phone's diagnostic record, readable.
//
// Settings used to give this three rows: a summary, "Copy", and "Share". You could hand
// the record to someone else, and you could not look at it yourself, which is backwards:
// the person most likely to recognise "the pairing was refused because the code had
// already been used" is the one who just tried to pair. So the record is a page, each
// entry is a sentence, and copying it is one action at the bottom rather than a row of
// its own.
//
// Nothing here leaves the phone on its own: the buffer is local, and Copy or Share is
// what sends it anywhere.

import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {Alert, Platform, ScrollView, Share, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {SafeAreaView} from 'react-native-safe-area-context';
import {APP_VERSION as appVersion} from '../version';
import {useApp} from '../state/AppContext';
import {ContentColumn} from '../ui/ContentColumn';
import {Entry, diagBuffer} from '../diag';
import {DiagSection, countProblems, diagLines, groupByDay} from '../diag/lines';

export function DiagScreen({navigation}: any) {
  const {t, lang, pal} = useApp();
  const zh = lang === 'zh';
  const [problemsOnly, setProblemsOnly] = useState(false);
  const [copied, setCopied] = useState(false);

  // Read once per visit (and again after a clear). The buffer is in memory, so this is a
  // map over at most 500 entries, not a fetch.
  const [entries, setEntries] = useState<Entry[]>([]);
  const [stats, setStats] = useState(() => diagBuffer.stats());
  const reload = useCallback(() => {
    setEntries(diagBuffer.entries());
    setStats(diagBuffer.stats());
  }, []);
  useEffect(reload, [reload]);

  const problems = useMemo(() => countProblems(entries), [entries]);
  const sections: DiagSection[] = useMemo(() => {
    const lines = diagLines(entries, zh).filter(l => !problemsOnly || l.problem);
    return groupByDay(lines, new Date(), zh);
  }, [entries, problemsOnly, zh]);

  const text = useCallback(
    () =>
      diagBuffer.text({
        app: appVersion,
        platform: `${Platform.OS} ${Platform.Version}`,
        at: new Date().toISOString(),
      }),
    [],
  );

  const copyAll = () => {
    Clipboard.setString(text());
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const shareAll = () => {
    Share.share({message: text()}).catch(() => {});
  };

  const confirmClear = () =>
    Alert.alert(
      zh ? '清空诊断记录？' : 'Clear the record?',
      zh
        ? '这台设备上的记录会被删掉。出问题时会重新开始记。'
        : 'What this phone recorded is deleted. It starts recording again the next time something happens.',
      [
        {text: t('cancel'), style: 'cancel'},
        {
          text: zh ? '清空' : 'Clear',
          style: 'destructive',
          onPress: () => {
            diagBuffer.clear().then(reload);
          },
        },
      ],
    );

  const kb = Math.max(1, Math.round(stats.bytes / 1024));
  const summary =
    stats.count === 0
      ? zh
        ? '还没有记录，也没有上传过任何东西：这里记下的只留在这台手机上。'
        : 'Nothing recorded yet, and nothing is uploaded: what lands here stays on this phone.'
      : zh
      ? `${stats.count} 条 · ${kb} KB，只存在这台手机上。拷贝或分享出去，它就去了你发的地方；除此之外不会上传。`
      : `${stats.count} entries · ${kb} KB, kept on this phone. Copy or share it and it goes where you send it; nothing is uploaded.`;

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹ </Text>
        </TouchableOpacity>
        <Text style={[styles.title, {color: pal.fg}]}>{zh ? '诊断记录' : 'Diagnostic record'}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        <ContentColumn>
          <Text style={[styles.summary, {color: pal.fg3}]}>{summary}</Text>

          {stats.count > 0 && (
            <View style={[styles.filter, {borderColor: pal.divider}]}>
              {[
                {key: false, label: zh ? '全部' : 'Everything'},
                {key: true, label: zh ? '只看问题' : 'Problems only', n: problems},
              ].map(o => {
                const on = problemsOnly === o.key;
                return (
                  <TouchableOpacity
                    key={String(o.key)}
                    accessibilityRole="button"
                    accessibilityState={{selected: on}}
                    activeOpacity={0.7}
                    style={[styles.chip, on && {backgroundColor: pal.surface}]}
                    onPress={() => setProblemsOnly(o.key)}>
                    <Text style={[styles.chipText, {color: on ? pal.fg : pal.fg3}]}>
                      {o.label}
                      {o.n ? ` · ${o.n}` : ''}
                    </Text>
                  </TouchableOpacity>
                );
              })}
            </View>
          )}

          {sections.length === 0 ? (
            <EmptyRecord zh={zh} pal={pal} problemsOnly={problemsOnly} onShowAll={() => setProblemsOnly(false)} />
          ) : (
            sections.map(s => (
              <View key={s.day} style={styles.section}>
                <Text style={[styles.dayTitle, {color: pal.fg3}]}>{s.title.toUpperCase()}</Text>
                <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
                  {s.lines.map((l, i) => (
                    <View
                      key={l.id}
                      style={[
                        styles.line,
                        i < s.lines.length - 1 && {
                          borderBottomColor: pal.divider,
                          borderBottomWidth: StyleSheet.hairlineWidth,
                        },
                      ]}>
                      <Text style={[styles.clock, {color: pal.fg3}]}>{l.clock}</Text>
                      {/* The problem marker is amber, the modifier colour: red is
                          "an agent is waiting for you" everywhere else in the app. */}
                      <View style={[styles.dot, {backgroundColor: l.problem ? '#F59E0B' : pal.divider}]} />
                      <View style={styles.lineText}>
                        <Text style={[styles.lineTitle, {color: pal.fg}]}>{l.title}</Text>
                        {!!l.detail && <Text style={[styles.lineDetail, {color: pal.fg3}]}>{l.detail}</Text>}
                      </View>
                    </View>
                  ))}
                </View>
              </View>
            ))
          )}

          {stats.count > 0 && (
            <View style={styles.actions}>
              <Action
                label={copied ? (zh ? '已拷贝' : 'Copied') : zh ? `拷贝全部 ${stats.count} 条` : `Copy all ${stats.count} entries`}
                pal={pal}
                onPress={copyAll}
              />
              <Action label={zh ? '分享' : 'Share'} pal={pal} onPress={shareAll} />
              <Action label={zh ? '清空记录' : 'Clear the record'} pal={pal} danger onPress={confirmClear} />
            </View>
          )}
        </ContentColumn>
      </ScrollView>
    </SafeAreaView>
  );
}

// What shows up here, for someone looking at an empty page. It says what the record is
// for BEFORE anything has gone wrong, which is the only time this page is calm.
function EmptyRecord({
  zh, pal, problemsOnly, onShowAll,
}: {
  zh: boolean;
  pal: any;
  problemsOnly: boolean;
  onShowAll: () => void;
}) {
  if (problemsOnly) {
    return (
      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider, padding: 16}]}>
        <Text style={[styles.lineTitle, {color: pal.fg}]}>{zh ? '没有出过问题' : 'Nothing has gone wrong'}</Text>
        <TouchableOpacity onPress={onShowAll} hitSlop={hit}>
          <Text style={[styles.link, {color: '#06B6D4'}]}>{zh ? '看全部' : 'Show everything'}</Text>
        </TouchableOpacity>
      </View>
    );
  }
  const items = zh
    ? [
        '发给 Mac 的请求失败了，记下是哪个接口、回了什么',
        '配对没成功，记下被拒的原因',
        '实时连接断了，以及断了多久',
      ]
    : [
        'A request to the Mac that failed, with the route and what came back',
        'A pairing that did not go through, and the reason it was refused',
        'The live connection dropping, and how long it was gone',
      ];
  return (
    <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider, padding: 16}]}>
      <Text style={[styles.emptyTitle, {color: pal.fg}]}>{zh ? '这里会出现什么' : 'What shows up here'}</Text>
      {items.map(s => (
        <View key={s} style={styles.bullet}>
          <Text style={[styles.bulletDot, {color: pal.fg3}]}>·</Text>
          <Text style={[styles.bulletText, {color: pal.fg2}]}>{s}</Text>
        </View>
      ))}
      <Text style={[styles.lineDetail, {color: pal.fg3, marginTop: 10}]}>
        {zh ? 'token 和配对码在写下之前就被替换掉了。' : 'Tokens and pairing codes are replaced before anything is written down.'}
      </Text>
    </View>
  );
}

function Action({label, pal, onPress, danger}: {label: string; pal: any; onPress: () => void; danger?: boolean}) {
  return (
    <TouchableOpacity
      accessibilityRole="button"
      activeOpacity={0.7}
      onPress={onPress}
      style={[styles.actionBtn, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
      <Text style={[styles.actionText, {color: danger ? '#EF4444' : pal.fg}]}>{label}</Text>
    </TouchableOpacity>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 10},
  back: {fontSize: 28, fontWeight: '300'},
  title: {fontSize: 20, fontWeight: '700'},
  body: {paddingBottom: 28},
  summary: {fontSize: 12.5, lineHeight: 18, paddingHorizontal: 16, paddingBottom: 14},
  filter: {flexDirection: 'row', alignSelf: 'flex-start', marginHorizontal: 16, marginBottom: 18, padding: 3, borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  chip: {paddingHorizontal: 12, paddingVertical: 6, borderRadius: 7},
  chipText: {fontSize: 13.5},
  section: {marginBottom: 20},
  dayTitle: {fontSize: 11.5, fontWeight: '700', letterSpacing: 0.6, marginBottom: 8, marginLeft: 16},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden', marginHorizontal: 16},
  line: {flexDirection: 'row', alignItems: 'flex-start', paddingHorizontal: 14, paddingVertical: 11},
  clock: {fontSize: 12, width: 40, marginTop: 1, fontVariant: ['tabular-nums']},
  dot: {width: 6, height: 6, borderRadius: 3, marginTop: 6, marginRight: 10},
  lineText: {flex: 1, minWidth: 0},
  lineTitle: {fontSize: 15, lineHeight: 20},
  lineDetail: {fontSize: 12.5, lineHeight: 17, marginTop: 2},
  emptyTitle: {fontSize: 15, fontWeight: '600', marginBottom: 8},
  bullet: {flexDirection: 'row', marginTop: 6},
  bulletDot: {fontSize: 15, width: 12},
  bulletText: {flex: 1, fontSize: 13.5, lineHeight: 19},
  link: {fontSize: 14, marginTop: 8},
  actions: {marginHorizontal: 16, marginTop: 4, gap: 10},
  actionBtn: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, paddingVertical: 14, alignItems: 'center'},
  actionText: {fontSize: 16},
});
