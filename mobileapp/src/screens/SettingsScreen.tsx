// SettingsScreen — Moshi-style grouped preferences. Each multi-option setting is
// one row showing its current value + a chevron that opens a PickerSheet (instead
// of a long inline radio list); booleans are inline toggles; sections are labelled
// cards with leading outline icons. Removing the Mac clears the Keychain and the
// app falls back to Pairing automatically.

import React, {useEffect, useState} from 'react';
import {Alert, ScrollView, Share, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {APP_VERSION as appVersion} from '../version';
import {LangPref} from '../i18n';
import {useApp} from '../state/AppContext';
import {MemoryCopy, describeCopy, fetchCopy, readCopy} from '../state/hqMemory';
import {useAgents} from '../state/AgentsContext';
import {SettingsGroup, SettingsRow, PickerSheet, InfoSheet} from '../ui/SettingsRow';
import {ContentColumn} from '../ui/ContentColumn';
import {WhatsNewModal} from '../ui/WhatsNewModal';
import {RELEASE_NOTES} from '../releaseNotes';
import {diagBuffer} from '../diag';
import {countProblems, describeRecord} from '../diag/lines';

type PickerKind = 'lang' | 'theme' | 'mode' | null;

export function SettingsScreen({navigation}: any) {
  const {t, lang, pal, langPref, setLangPref, mac, removeServer, pushEnabled, setPushEnabled, pushKinds, setPushKinds, returnSends, setReturnSends, defaultDetailMode, setDefaultDetailMode, themePref, setThemePref} =
    useApp();
  const {isGuest, conn} = useAgents();
  // The phone's copy of HQ's memory. Read on mount so the row states a fact rather than
  // a spinner, and re-read after every act.
  const [memCopy, setMemCopy] = useState<MemoryCopy | null>(null);
  const [memBusy, setMemBusy] = useState(false);
  useEffect(() => {
    readCopy().then(setMemCopy);
  }, []);
  const fetchMemory = async () => {
    if (!mac) return;
    setMemBusy(true);
    const {copy, error} = await fetchCopy(mac.url, mac.token);
    setMemBusy(false);
    if (error) {
      // Loudly. A silent failure on a backup screen would leave you believing you have
      // a copy you do not have, which is worse than having none.
      Alert.alert(
        lang === 'zh' ? '没能取回' : 'Could not fetch it',
        error,
      );
      return;
    }
    setMemCopy(copy ?? null);
  };
  const shareMemory = () => {
    if (!memCopy) return;
    Share.share({url: memCopy.url}).catch(() => {});
  };

  // The diagnostics buffer (src/diag) is a PAGE now, not three rows here. This screen
  // keeps only what the row has to say: how much is in there, and whether any of it is a
  // problem, which is the one thing that should pull the eye down to it.
  const [diagStats, setDiagStats] = useState(diagBuffer.stats());
  const [diagProblems, setDiagProblems] = useState(0);
  const refreshDiag = () => {
    setDiagStats(diagBuffer.stats());
    setDiagProblems(countProblems(diagBuffer.entries()));
  };
  // Re-read on every focus: the record grows while you are elsewhere in the app, and
  // coming back from the page itself may have cleared it.
  useEffect(() => {
    refreshDiag();
    return navigation.addListener?.('focus', refreshDiag);
  }, [navigation]);

  const [picker, setPicker] = useState<PickerKind>(null);
  const [whatsNew, setWhatsNew] = useState(false);
  const [hqInfo, setHqInfo] = useState(false);

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

  // What the connection row says under the Mac's name: the state first, then where it
  // is reached, because "connected" and "offline" are the two words that decide whether
  // anything else on this screen matters.
  const connWord =
    conn === 'live'
      ? lang === 'zh'
        ? '已连接'
        : 'Connected'
      : conn === 'connecting'
      ? lang === 'zh'
        ? '连接中'
        : 'Connecting'
      : conn === 'unauthorized'
      ? lang === 'zh'
        ? '没有权限'
        : 'Not authorized'
      : lang === 'zh'
      ? '离线'
      : 'Offline';
  const connSub = mac?.url ? `${connWord} · ${mac.url.replace(/^https?:\/\//, '')}` : connWord;

  // The row's right-hand value is a VALUE: how big and how old, or a word when there is
  // no copy. The sentence explaining what to do stays in the subtitle, where a sentence
  // belongs — in the value slot it ran into the label on both languages.
  const copyValue = memCopy
    ? describeCopy(memCopy, Math.floor(Date.now() / 1000), lang === 'zh')
    : lang === 'zh'
    ? '还没有'
    : 'None yet';

  const confirmRemove = () =>
    mac &&
    Alert.alert(mac.name, t('removeServerQ'), [
      {text: t('cancel'), style: 'cancel'},
      {text: t('removeMac'), style: 'destructive', onPress: () => removeServer(mac.url)},
    ]);

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      {/* The title rides in the same column as the rows: on iPad the page is centred,
          and a header outside the column sits alone at the far left of the screen. */}
      <ContentColumn>
        <View style={styles.header}>
          <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
            <Text style={[styles.back, {color: pal.fg2}]}>‹ </Text>
          </TouchableOpacity>
          <Text style={[styles.title, {color: pal.fg}]}>{t('settings')}</Text>
        </View>
      </ContentColumn>

      <ScrollView contentContainerStyle={styles.body}>
        <ContentColumn>
        {/* CONNECTION */}
        <SettingsGroup title={lang === 'zh' ? '连接' : 'Connection'} pal={pal}>
          {/* The row says whether this phone is actually talking to that Mac. The
              address was the old subtitle, and the address is not the question anyone
              opens Settings with when the radar has stopped moving. */}
          <SettingsRow
            icon="server"
            label={mac?.name || '—'}
            sub={connSub}
            pal={pal}
            chevron
            divider
            onPress={() => navigation.navigate('Servers')}
          />
          {/* Manage THIS Mac's sharing (owner-remote-admin, decision B): owner-only,
              hidden for a guest connection so no control ever 403s. */}
          {!isGuest && (
            <SettingsRow
              icon="share"
              label={lang === 'zh' ? '分享与设备' : 'Sharing & devices'}
              sub={lang === 'zh' ? '分享链接、权限、已配对设备' : 'Share links, scopes, paired devices'}
              pal={pal}
              chevron
              divider
              onPress={() => navigation.navigate('ManageMac')}
            />
          )}
        </SettingsGroup>

        {/* THE SUPERVISOR'S MEMORY — owner only. It is the board, the knowledge base and
            the operator's LOCAL.md in one file; a guest link is for watching a pane. */}
        {!isGuest && mac && (
          <SettingsGroup
            title={lang === 'zh' ? 'HQ 档案' : 'HQ records'}
            pal={pal}
            onInfo={() => setHqInfo(true)}
            infoLabel={lang === 'zh' ? 'HQ 档案是什么' : 'What HQ records are'}>
            <SettingsRow
              icon="server"
              label={lang === 'zh' ? '这台设备上的副本' : 'Copy on this device'}
              sub={
                memBusy
                  ? lang === 'zh'
                    ? '正在取…'
                    : 'Fetching…'
                  : memCopy
                  ? lang === 'zh'
                    ? '点一下取一份新的'
                    : 'Tap to fetch a fresh one'
                  : lang === 'zh'
                  ? '点一下从 Mac 取一份'
                  : 'Tap to fetch one from the Mac'
              }
              value={copyValue}
              pal={pal}
              divider
              onPress={memBusy ? undefined : fetchMemory}
            />
            {/* Said out loud, because iOS will not let the app verify it: the file is
                included in the iPhone's backup, and there is no API for whether that
                backup ran. Sharing it somewhere yourself is the version you can check. */}
            <SettingsRow
              icon="share"
              label={lang === 'zh' ? '导出这份副本' : 'Export the copy'}
              sub={
                lang === 'zh'
                  ? '自己存一份进「文件」或 iCloud 云盘'
                  : 'Save it to Files or iCloud Drive yourself'
              }
              pal={pal}
              chevron
              onPress={memCopy ? shareMemory : undefined}
            />
          </SettingsGroup>
        )}

        {/* TERMINAL */}
        <SettingsGroup title={lang === 'zh' ? '终端' : 'Terminal'} pal={pal}>
          <SettingsRow icon="palette" label={lang === 'zh' ? '外观' : 'Appearance'} value={labelOf(themes, themePref)} pal={pal} chevron divider onPress={() => setPicker('theme')} />
          <SettingsRow icon="layout" label={lang === 'zh' ? '默认模式' : 'Default mode'} value={labelOf(detailModes, defaultDetailMode)} pal={pal} chevron divider onPress={() => setPicker('mode')} />
          <SettingsRow icon="return" label={lang === 'zh' ? '回车直接发送' : 'Return sends'} sub={lang === 'zh' ? '关闭时回车是换行，用 ↑ 发送' : 'Off: Return makes a newline, send with ↑'} pal={pal} toggle={returnSends} onToggle={setReturnSends} />
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

        {/* ABOUT AND DIAGNOSTICS — the end of the page: what this app is, what changed
            in it, what it recorded, and the one irreversible thing on the screen.
            "Remove this Mac" used to be the third row of the FIRST group, one thumb-width
            from the row you tap to switch Macs. */}
        <SettingsGroup title={lang === 'zh' ? '关于与诊断' : 'About & diagnostics'} pal={pal}>
          <SettingsRow icon="info" label={t('version')} value={appVersion} pal={pal} divider />
          {/* The same notes the update popup showed, on demand — `gtmux whatsnew` has
              always been readable again after the fact, and a changelog you can only
              see once is a changelog you cannot go back to. */}
          <SettingsRow
            icon="sparkle"
            label={lang === 'zh' ? '更新内容' : "What's new"}
            pal={pal}
            chevron
            divider
            onPress={() => setWhatsNew(true)}
          />
          <SettingsRow
            icon="phone"
            label={lang === 'zh' ? '诊断记录' : 'Diagnostic record'}
            value={describeRecord(diagStats, diagProblems, lang === 'zh')}
            pal={pal}
            chevron
            divider
            onPress={() => navigation.navigate('Diag')}
          />
          <SettingsRow icon="trash" label={t('removeMac')} danger pal={pal} onPress={confirmRemove} />
        </SettingsGroup>

        </ContentColumn>
      </ScrollView>

      {/* Settings opens the FULL history expanded — someone who navigated here asked for
          it, so folding it behind another tap would only be in the way. */}
      <WhatsNewModal
        visible={whatsNew}
        entries={RELEASE_NOTES}
        pal={pal}
        lang={lang}

        onClose={() => setWhatsNew(false)}
      />

      <InfoSheet
        visible={hqInfo}
        pal={pal}
        onClose={() => setHqInfo(false)}
        doneLabel={lang === 'zh' ? '好' : 'Done'}
        title={lang === 'zh' ? 'HQ 的档案里有什么' : 'What HQ keeps'}
        lead={
          lang === 'zh'
            ? 'Mac 上 HQ 的目录里有四样东西，合起来叫它的档案。换一轮对话，它能带走的就这些。'
            : "Four things live in HQ's folder on the Mac. Together they are its records, and they are all it carries from one conversation to the next."
        }
        items={
          lang === 'zh'
            ? [
                {label: '态势板', body: '它现在怎么看眼下的局面。干活时它自己改，上下文清掉之后再读回来。'},
                {label: '知识库', body: '它归档下来的经验，按主题分，每条都记着从哪来的、后来有没有再撞上。'},
                {label: '你的规矩', body: '你说过一次、不想再说第二次的事。gtmux 只生成一次，之后从不覆盖。'},
                {label: '守则', body: 'gtmux 随版本发的那份章程，更新时会跟着升级。和你的规矩冲突时听你的。'},
              ]
            : [
                {label: 'The board', body: 'How HQ reads the situation right now. It rewrites it as it works, and reads it back after its context is cleared.'},
                {label: 'The knowledge base', body: 'Lessons it has filed, by topic, each with where it came from and whether it has been hit again.'},
                {label: 'Your standing rules', body: 'What you told it once and do not want to repeat. gtmux writes this file once and never over it.'},
                {label: 'The charter', body: 'The playbook gtmux ships and upgrades with each release. Your rules sit above it when the two disagree.'},
              ]
        }
        note={
          lang === 'zh'
            ? '这台手机上的副本，是你上次要的时候这四样东西的快照。它跟着这台手机的备份走；iOS 不会告诉 app 备份到底跑没跑，所以想要一份自己能核对的，就自己导出来。'
            : 'The copy on this phone is a snapshot of all four, taken the last time you asked for one. It rides this phone\'s backup. iOS will not tell an app whether that backup ran, so export it yourself if you want a copy you can check.'
        }
      />

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
