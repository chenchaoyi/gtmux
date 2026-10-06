// ServerDetailsSheet — what a paired Mac actually is (GET /api/host): its own name and
// host name, its system, chip, memory and uptime, and the gtmux and tmux it runs, next to
// what this phone knows (the name the user gave it, its address, its access). Opened from
// a server row's More menu. A share link is never asked (the Mac refuses it), an older
// gtmux says it is too old, and an unreachable Mac still shows what the phone knows.
//
// Laid out like its own row, then the facts (2026-10-07, redesigned from the user's
// markup). The head is the name the user gave it, under it the row's own status line,
// and Done, so the name is not repeated as a "Name" row. Each group is an inset card one
// step above the sheet (pal.raised, as RowSheet's controls are), not a band from edge to
// edge. A label never gives up its width to a value: it used to, and a long chip string
// squeezed "Chip" to one letter a line. A short value sits beside its label; a long one
// (address, host name, chip) goes under it at full width. Values are the text you read,
// so they are pal.fg, not the faint pal.fg3 a settings value uses.

import React, {useEffect, useRef, useState} from 'react';
import {AccessibilityInfo, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {PairedMac} from '../pairing/qr';
import {HostAnswer} from '../api/types';
import {SheetShell} from '../ui/SettingsRow';
import {SIcon} from '../ui/SettingsIcons';
import {BRAND} from '../ui/theme';
import {Lang} from '../i18n';
import {chipLabel, hostName, hostSystem, loadHost, memoryLabel, sinceLabel} from '../state/hostInfo';

type T = (k: string) => string;
type Fact = {label: string; value: string; long?: boolean; right?: React.ReactNode};
/** The row's status line, as ServersScreen draws it: words, dot colour, filled or hollow. */
export type DetailsStatus = {text: string; tone: string; st: {filled: boolean}};

export function ServerDetailsSheet({mac, status, pal, lang, t, onClose}: {
  mac: PairedMac | null;
  status?: DetailsStatus;
  pal: any;
  lang: Lang;
  t: T;
  onClose: () => void;
}) {
  const guest = mac?.scope === 'guest';
  const [answer, setAnswer] = useState<HostAnswer | 'loading' | null>(null);
  useEffect(() => {
    if (!mac) return;
    if (guest) {
      setAnswer({ok: false, why: 'guest'});
      return;
    }
    let live = true;
    setAnswer('loading');
    loadHost(mac.url, mac.token, 0).then(a => live && setAnswer(a));
    return () => {
      live = false;
    };
  }, [mac, guest]);

  // Copying is silent, so the button turns into a check for a moment.
  const [copied, setCopied] = useState(false);
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (copiedTimer.current) clearTimeout(copiedTimer.current);
  }, []);
  const copyAddress = () => {
    if (!mac) return;
    Clipboard.setString(mac.url);
    setCopied(true);
    AccessibilityInfo.announceForAccessibility(t('copied'));
    if (copiedTimer.current) clearTimeout(copiedTimer.current);
    copiedTimer.current = setTimeout(() => setCopied(false), 1500);
  };

  const info = answer && answer !== 'loading' && answer.ok ? answer.info : null;
  const note =
    answer === 'loading' ? t('hostLoading')
      : answer && !answer.ok
        ? answer.why === 'guest' ? t('hostGuestNote')
          : answer.why === 'auth' ? t('hostAuthNote')
            : answer.why === 'old' ? t('hostOldNote') : t('hostUnreachable')
        : '';

  const copy = (
    <TouchableOpacity
      testID="server-details-copy"
      onPress={copyAddress}
      style={styles.copy}
      accessibilityRole="button"
      accessibilityLabel={copied ? t('copied') : t('copyAddress')}>
      <SIcon name={copied ? 'check' : 'copy'} size={20} color={BRAND} />
    </TouchableOpacity>
  );

  return (
    <SheetShell visible={!!mac} pal={pal} onClose={onClose}>
      {mac && (
        <ScrollView style={styles.scroll} testID="server-details">
          <View style={styles.head}>
            <View style={styles.headText}>
              <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1} accessibilityRole="header">
                {mac.name}
              </Text>
              {!!status && (
                <View style={styles.statusLine}>
                  <View style={[styles.dot, status.st.filled ? {backgroundColor: status.tone} : [styles.dotHollow, {borderColor: status.tone}]]} />
                  <Text style={[styles.status, {color: pal.fg2}]} numberOfLines={2}>{status.text}</Text>
                </View>
              )}
            </View>
            <TouchableOpacity onPress={onClose} style={styles.done} accessibilityRole="button" testID="server-details-done">
              <Text style={[styles.doneText, {color: pal.fg}]}>{t('sheetDone')}</Text>
            </TouchableOpacity>
          </View>
          <Group title={t('hostOnPhone')} pal={pal} facts={[
            {label: t('hostAddress'), value: mac.url, long: true, right: copy},
            {label: t('hostAccess'), value: guest ? t('hostAccessGuest') : t('hostAccessOwner')},
          ]} />
          {info && (
            <Group title={t('hostThisMac')} pal={pal} facts={[
              {label: t('hostComputerName'), value: hostName(info)},
              {label: t('hostHostname'), value: info.hostname, long: true},
              {label: t('hostSystem'), value: hostSystem(info, true)},
              {label: t('hostChip'), value: chipLabel(info), long: true},
              {label: t('hostCores'), value: info.cores ? String(info.cores) : ''},
              {label: t('hostMemory'), value: memoryLabel(info.memory_bytes)},
              {label: t('hostUptime'), value: sinceLabel(info.boot_time, lang)},
            ]} />
          )}
          {info && (
            <Group title={t('hostGtmux')} pal={pal} facts={[
              {label: t('hostGtmuxVersion'), value: info.gtmux_version},
              {label: t('hostServeUp'), value: sinceLabel(info.serve_started, lang)},
              {label: t('hostTmux'), value: info.tmux ?? ''},
            ]} />
          )}
          {!!note && (
            <View style={styles.noteWrap}>
              <Text testID="server-details-note" style={[styles.note, {color: pal.fg2}]}>{note}</Text>
            </View>
          )}
        </ScrollView>
      )}
    </SheetShell>
  );
}

/** One titled card of facts; an empty value leaves its row out. */
function Group({title, facts, pal}: {title: string; facts: Fact[]; pal: any}) {
  const shown = facts.filter(f => !!f.value);
  if (!shown.length) return null;
  return (
    <View style={styles.group}>
      <Text style={[styles.groupTitle, {color: pal.fg2}]}>{title.toUpperCase()}</Text>
      <View style={[styles.card, {backgroundColor: pal.raised}]}>
        {shown.map((f, i) => (
          <View key={f.label}>
            {i > 0 && <View style={[styles.sep, {backgroundColor: pal.divider}]} />}
            {f.long ? (
              <View style={styles.longRow} testID={`server-details-fact-${f.label}`}>
                <View style={styles.longText}>
                  <Text style={[styles.longLabel, {color: pal.fg2}]}>{f.label}</Text>
                  <Text style={[styles.value, {color: pal.fg}]} selectable>{f.value}</Text>
                </View>
                {f.right}
              </View>
            ) : (
              <View style={styles.row} testID={`server-details-fact-${f.label}`}>
                <Text style={[styles.label, {color: pal.fg2}]}>{f.label}</Text>
                <Text style={[styles.value, styles.valueRight, {color: pal.fg}]} selectable>{f.value}</Text>
              </View>
            )}
          </View>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  // At most 620pt tall; the sheet itself is bounded by the window, so in landscape the
  // scroll shrinks to fit and its content scrolls (F14).
  scroll: {maxHeight: 620, flexShrink: 1},
  head: {flexDirection: 'row', alignItems: 'flex-start', gap: 12, paddingLeft: 20, paddingRight: 10, paddingTop: 4, paddingBottom: 6},
  headText: {flex: 1, minWidth: 0, gap: 4},
  title: {fontSize: 22, lineHeight: 28, fontWeight: '700'},
  statusLine: {flexDirection: 'row', alignItems: 'center', gap: 7},
  dot: {width: 8, height: 8, borderRadius: 4},
  dotHollow: {borderWidth: 1.5},
  status: {fontSize: 15, lineHeight: 20, flexShrink: 1},
  done: {minHeight: 44, justifyContent: 'center', paddingHorizontal: 10, marginTop: -8},
  doneText: {fontSize: 17, fontWeight: '600'},
  group: {marginTop: 16, marginHorizontal: 16},
  groupTitle: {fontSize: 13, letterSpacing: 0.4, marginLeft: 16, marginBottom: 6},
  card: {borderRadius: 12, overflow: 'hidden'},
  sep: {height: StyleSheet.hairlineWidth, marginLeft: 16},
  row: {minHeight: 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 16, paddingHorizontal: 16, paddingVertical: 11},
  label: {fontSize: 16, lineHeight: 21, flexShrink: 0},
  value: {fontSize: 16, lineHeight: 21},
  valueRight: {flexShrink: 1, textAlign: 'right'},
  longRow: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingLeft: 16, paddingRight: 4, paddingVertical: 8},
  longText: {flex: 1, minWidth: 0, gap: 2},
  longLabel: {fontSize: 13, lineHeight: 17},
  copy: {width: 44, height: 44, alignItems: 'center', justifyContent: 'center'},
  noteWrap: {paddingHorizontal: 20, paddingTop: 14, paddingBottom: 16},
  note: {fontSize: 13, lineHeight: 19},
});
