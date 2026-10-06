// ServerDetailsSheet — what a paired Mac actually is (GET /api/host): its own name and
// host name, its system, chip, memory and uptime, and the gtmux and tmux it runs, next to
// what this phone knows (the name the user gave it, its address, its access). Opened from
// a server row's More menu. A share link is never asked (the Mac refuses it), an older
// gtmux says it is too old, and an unreachable Mac still shows what the phone knows.

import React, {useEffect, useState} from 'react';
import {ScrollView, StyleSheet, Text, View} from 'react-native';
import {PairedMac} from '../pairing/qr';
import {HostAnswer} from '../api/types';
import {SettingsGroup, SettingsRow, SheetShell} from '../ui/SettingsRow';
import {Lang} from '../i18n';
import {hostName, hostSystem, loadHost, memoryLabel, sinceLabel} from '../state/hostInfo';

type T = (k: string) => string;

export function ServerDetailsSheet({mac, pal, lang, t, onClose}: {
  mac: PairedMac | null;
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

  const row = (label: string, value: string, divider = true) =>
    value ? <SettingsRow key={label} label={label} value={value} pal={pal} divider={divider} /> : null;
  const info = answer && answer !== 'loading' && answer.ok ? answer.info : null;
  const note =
    answer === 'loading' ? t('hostLoading')
      : answer && !answer.ok
        ? answer.why === 'guest' ? t('hostGuestNote')
          : answer.why === 'auth' ? t('hostAuthNote')
            : answer.why === 'old' ? t('hostOldNote') : t('hostUnreachable')
        : '';

  return (
    <SheetShell visible={!!mac} pal={pal} onClose={onClose}>
      {mac && (
        <ScrollView style={styles.scroll} testID="server-details">
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>{mac.name}</Text>
          <SettingsGroup title={t('hostOnPhone')} pal={pal}>
            {row(t('hostName'), mac.name)}
            {row(t('hostAddress'), mac.url)}
            {row(t('hostAccess'), guest ? t('hostAccessGuest') : t('hostAccessOwner'), false)}
          </SettingsGroup>
          {info && (
            <SettingsGroup title={t('hostThisMac')} pal={pal}>
              {row(t('hostComputerName'), hostName(info))}
              {row(t('hostHostname'), info.hostname)}
              {row(t('hostSystem'), hostSystem(info, true))}
              {row(t('hostChip'), info.cpu ? `${info.cpu} · ${info.arch}` : info.arch)}
              {row(t('hostCores'), info.cores ? String(info.cores) : '')}
              {row(t('hostMemory'), memoryLabel(info.memory_bytes))}
              {row(t('hostUptime'), sinceLabel(info.boot_time, lang), false)}
            </SettingsGroup>
          )}
          {info && (
            <SettingsGroup title={t('hostGtmux')} pal={pal}>
              {row(t('hostGtmuxVersion'), info.gtmux_version)}
              {row(t('hostServeUp'), sinceLabel(info.serve_started, lang))}
              {row(t('hostTmux'), info.tmux ?? '', false)}
            </SettingsGroup>
          )}
          {!!note && (
            <View style={styles.noteWrap}>
              <Text testID="server-details-note" style={[styles.note, {color: pal.fg3}]}>{note}</Text>
            </View>
          )}
        </ScrollView>
      )}
    </SheetShell>
  );
}

const styles = StyleSheet.create({
  // At most 620pt tall; the sheet itself is bounded by the window, so in landscape the
  // scroll shrinks to fit and its content scrolls (F14).
  scroll: {maxHeight: 620, flexShrink: 1},
  title: {fontSize: 20, fontWeight: '700', paddingHorizontal: 20, paddingTop: 4, paddingBottom: 10},
  noteWrap: {paddingHorizontal: 20, paddingBottom: 16},
  note: {fontSize: 13, lineHeight: 19},
});
