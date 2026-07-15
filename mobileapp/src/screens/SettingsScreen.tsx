// SettingsScreen — Moshi-style grouped preferences. Each multi-option setting is
// one row showing its current value + a chevron that opens a PickerSheet (instead
// of a long inline radio list); booleans are inline toggles; sections are labelled
// cards with leading outline icons. Removing the Mac clears the Keychain and the
// app falls back to Pairing automatically.

import React, {useState} from 'react';
import {Alert, Share, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {APP_VERSION as appVersion} from '../version';
import {LangPref} from '../i18n';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {SettingsGroup, SettingsRow, PickerSheet} from '../ui/SettingsRow';
import {ContentColumn} from '../ui/ContentColumn';

type PickerKind = 'lang' | 'theme' | 'mode' | null;

export function SettingsScreen({navigation}: any) {
  const {t, lang, pal, langPref, setLangPref, mac, removeServer, pushEnabled, setPushEnabled, pushKinds, setPushKinds, returnSends, setReturnSends, defaultDetailMode, setDefaultDetailMode, themePref, setThemePref} =
    useApp();
  const {client, isGuest} = useAgents();
  const [picker, setPicker] = useState<PickerKind>(null);

  const langs: {key: LangPref; label: string}[] = [
    {key: 'system', label: t('system')},
    {key: 'en', label: 'English'},
    {key: 'zh', label: '中文'},
  ];
  const themes: {key: 'system' | 'light' | 'dark'; label: string}[] = [
    {key: 'system', label: t('system')},
    {key: 'light', label: lang === 'zh' ? '浅色' : 'Light'},
    {key: 'dark', label: lang === 'zh' ? '深色' : 'Dark'},
  ];
  const detailModes: {key: 'chat' | 'terminal'; label: string; sub?: string}[] = [
    {key: 'terminal', label: lang === 'zh' ? '终端' : 'Terminal', sub: lang === 'zh' ? '完整 TUI' : 'Full TUI'},
    {key: 'chat', label: lang === 'zh' ? '对话' : 'Chat', sub: lang === 'zh' ? '当前屏幕概览 + 审批卡' : 'Glance + approval card'},
  ];

  const labelOf = <T extends string>(arr: {key: T; label: string}[], k: T) => arr.find(o => o.key === k)?.label ?? '';

  // Handoff: mint a one-time code on the paired Mac and share a browser link.
  const openOnComputer = async () => {
    try {
      const code = client && (await client.enrollMint());
      if (!code || !mac) return Alert.alert(t('openOnComputer'), t('openOnComputerFail'));
      await Share.share({message: `${mac.url.replace(/\/+$/, '')}/#c=${code}`});
    } catch {
      Alert.alert(t('openOnComputer'), t('openOnComputerFail'));
    }
  };

  const confirmRemove = () =>
    mac &&
    Alert.alert(mac.name, t('removeServerQ'), [
      {text: t('cancel'), style: 'cancel'},
      {text: t('removeMac'), style: 'destructive', onPress: () => removeServer(mac.url)},
    ]);

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹ </Text>
        </TouchableOpacity>
        <Text style={[styles.title, {color: pal.fg}]}>{t('settings')}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        <ContentColumn>
        {/* CONNECTION */}
        <SettingsGroup title={lang === 'zh' ? '连接' : 'Connection'} pal={pal}>
          <SettingsRow icon="server" label={mac?.name || '—'} sub={mac?.url} pal={pal} chevron divider onPress={() => navigation.navigate('Servers')} />
          {/* Handing the Mac off to a computer browser mints an OWNER device code —
              never available to a guest. */}
          {!isGuest && (
            <SettingsRow icon="share" label={t('openOnComputer')} sub={t('openOnComputerSub')} pal={pal} chevron divider onPress={openOnComputer} />
          )}
          <SettingsRow icon="trash" label={t('removeMac')} danger pal={pal} onPress={confirmRemove} />
        </SettingsGroup>

        {/* TERMINAL */}
        <SettingsGroup title={lang === 'zh' ? '终端' : 'Terminal'} pal={pal}>
          <SettingsRow icon="palette" label={lang === 'zh' ? '外观' : 'Appearance'} value={labelOf(themes, themePref)} pal={pal} chevron divider onPress={() => setPicker('theme')} />
          <SettingsRow icon="layout" label={lang === 'zh' ? '默认模式' : 'Default mode'} value={labelOf(detailModes, defaultDetailMode)} pal={pal} chevron divider onPress={() => setPicker('mode')} />
          <SettingsRow icon="return" label={lang === 'zh' ? '回车直接发送' : 'Return sends'} sub={lang === 'zh' ? '关闭时回车为换行，用 ↑ 发送' : 'Off: Return = newline; send with ↑'} pal={pal} toggle={returnSends} onToggle={setReturnSends} />
        </SettingsGroup>

        {/* NOTIFICATIONS — owner-only: a guest doesn't receive the host's alerts. */}
        {!isGuest && (
        <SettingsGroup title={t('push')} pal={pal}>
          <SettingsRow icon="bell" label={t('push')} pal={pal} toggle={pushEnabled} onToggle={setPushEnabled} divider />
          <SettingsRow
            label={lang === 'zh' ? '等你回应' : 'Needs you'}
            sub={lang === 'zh' ? '有 agent 在等你输入' : 'An agent is waiting for your input'}
            pal={pal}
            toggle={pushEnabled && pushKinds.waiting}
            toggleDisabled={!pushEnabled}
            onToggle={v => setPushKinds({...pushKinds, waiting: v})}
            divider
          />
          <SettingsRow
            label={lang === 'zh' ? '已完成' : 'Finished'}
            sub={lang === 'zh' ? 'agent 完成了一轮' : 'An agent finished a turn'}
            pal={pal}
            toggle={pushEnabled && pushKinds.done}
            toggleDisabled={!pushEnabled}
            onToggle={v => setPushKinds({...pushKinds, done: v})}
          />
        </SettingsGroup>
        )}

        {/* GENERAL */}
        <SettingsGroup title={lang === 'zh' ? '通用' : 'General'} pal={pal}>
          <SettingsRow icon="globe" label={t('language')} value={labelOf(langs, langPref)} pal={pal} chevron onPress={() => setPicker('lang')} />
        </SettingsGroup>

        {/* ABOUT */}
        <SettingsGroup title={lang === 'zh' ? '关于' : 'About'} pal={pal}>
          <SettingsRow icon="info" label={t('version')} value={appVersion} pal={pal} />
        </SettingsGroup>
        </ContentColumn>
      </ScrollView>

      <PickerSheet visible={picker === 'lang'} title={t('language')} options={langs} selected={langPref} pal={pal} onSelect={setLangPref} onClose={() => setPicker(null)} />
      <PickerSheet visible={picker === 'theme'} title={lang === 'zh' ? '外观' : 'Appearance'} options={themes} selected={themePref} pal={pal} onSelect={setThemePref} onClose={() => setPicker(null)} />
      <PickerSheet visible={picker === 'mode'} title={lang === 'zh' ? '默认模式' : 'Default mode'} options={detailModes} selected={defaultDetailMode} pal={pal} onSelect={setDefaultDetailMode} onClose={() => setPicker(null)} />
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 10},
  back: {fontSize: 28, fontWeight: '300'},
  title: {fontSize: 20, fontWeight: '700'},
  body: {paddingVertical: 16},
  action: {fontSize: 20, fontWeight: '300'},
});
