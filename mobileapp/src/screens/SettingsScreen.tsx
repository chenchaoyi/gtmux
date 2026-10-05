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
import {connectionHeading, routeSetting, showRouteRow, statusConnectionDetail} from './connectionGroup';
import {useRouteChoices} from './useRouteChoices';
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
  const {t, lang, pal, langPref, setLangPref, mac, servers, removeServer, pushEnabled, setPushEnabled, pushKinds, setPushKinds, returnSends, setReturnSends, defaultDetailMode, setDefaultDetailMode, themePref, setThemePref} =
    useApp();
  const {isGuest, conn, client} = useAgents();
  const {routes, loading: routesLoading, error: routesError, load: refreshRoutes} = useRouteChoices(client, conn === 'live' && !isGuest);
  useEffect(() => navigation.addListener?.('focus', () => void refreshRoutes()), [navigation, refreshRoutes]);
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

  const routeDisplay = routeSetting(routes, mac?.route, routesLoading, routesError, conn === 'live', lang === 'zh');

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
        <SettingsGroup title={connectionHeading(mac, lang === 'zh')} pal={pal}>
          {/* The group IS the connection now: the Mac's name heads it and these rows are
              its properties. "MacBook Pro · Connected · Shanghai" beside "Route ·
              Shanghai" read as siblings while they are two different questions — which
              Mac, and how I reach it (openspec/changes/phone-moves-the-route). */}
          <SettingsRow
            icon="server"
            label={lang === 'zh' ? '状态' : 'Status'}
            value={connWord}
            sub={statusConnectionDetail(mac?.route, mac?.url, isGuest, lang === 'zh')}
            pal={pal}
            divider
          />
          {/* Owners always have a route entry: loading or failure must not hide a setting. */}
          {showRouteRow(isGuest) && (
            <SettingsRow
              icon="server"
              label={lang === 'zh' ? '线路' : 'Route'}
              value={routeDisplay.value}
              sub={routeDisplay.hint}
              pal={pal}
              chevron={conn === 'live'}
              divider
              onPress={conn === 'live' ? () => navigation.navigate('Route') : undefined}
            />
          )}
          {/* Manage THIS Mac's sharing (owner-remote-admin, decision B): owner-only,
              hidden for a guest connection so no control ever 403s. */}
          {!isGuest && (
            <SettingsRow
              icon="share"
              label={lang === 'zh' ? '分享与配对' : 'Sharing & pairing'}
              sub={lang === 'zh' ? '分享链接、已配对设备' : 'Share links and paired devices'}
              pal={pal}
              chevron
              divider
              onPress={() => navigation.navigate('ManageMac')}
            />
          )}
        </SettingsGroup>

        {/* WHICH Mac is a different question from how this connection reaches it, so it
            lives in its own group rather than as a fourth row above. */}
        <SettingsGroup title={lang === 'zh' ? '我的 Mac' : 'My Macs'} pal={pal}>
          <SettingsRow
            icon="server"
            label={lang === 'zh' ? '换一台 Mac' : 'Switch Mac'}
            value={servers.length > 1 ? String(servers.length) : undefined}
            pal={pal}
            chevron
            divider
            onPress={() => navigation.navigate('Servers')}
          />
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
                  ? '导出的文件未加密，请妥善保存'
                  : 'The exported file is unencrypted; store it securely'
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

        {/* NOTIFICATIONS — owner-only: a guest doesn't receive the host's alerts. The
            heading names the SUBJECT and the row names the setting; both used to be
            t('push'), so the group read "PUSH NOTIFICATIONS / Push notifications". */}
        {!isGuest && (
        <SettingsGroup title={lang === 'zh' ? '通知' : 'Notifications'} pal={pal}>
          <SettingsRow icon="bell" label={t('push')}
            sub={lang === 'zh' ? '可在服务器列表选择通知来源' : 'Choose notification sources in Servers'}
            pal={pal} toggle={pushEnabled} onToggle={setPushEnabled} divider />
          <SettingsRow
            inset
            label={lang === 'zh' ? '等你回应' : 'Needs you'}
            sub={lang === 'zh' ? '有 agent 在等你输入' : 'An agent is waiting for your input'}
            pal={pal}
            toggle={pushEnabled && pushKinds.waiting}
            toggleDisabled={!pushEnabled}
            onToggle={v => setPushKinds({...pushKinds, waiting: v})}
            divider
          />
          <SettingsRow
            inset
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
        title={lang === 'zh' ? 'HQ 保存的内容' : 'What HQ saves'}
        lead={
          lang === 'zh'
            ? '这些内容保存在 Mac 上，供 HQ 持续使用。它们的用途和更新方式各不相同。'
            : 'These live on your Mac for HQ to use across conversations. Each has a different purpose and owner.'
        }
        items={
          lang === 'zh'
            ? [
                {label: '当前进展', body: 'HQ 汇总这台 Mac 的会话和待办，随进展更新。'},
                {label: '积累的知识', body: 'HQ 记下的事实和经验，保留每条内容的来源。'},
                {label: '你的要求', body: '你给 HQ 的长期偏好和边界，保存在 LOCAL.md；升级不会覆盖。'},
                {label: 'gtmux 的规则', body: 'HQ 默认的工作方式，保存在 AGENTS.md，随 gtmux 更新；你的要求优先。'},
              ]
            : [
                {label: 'Current work', body: 'HQ’s view of sessions and open tasks on this Mac, updated as work changes.'},
                {label: 'Saved knowledge', body: 'Facts and lessons HQ has kept, with their sources.'},
                {label: 'Your instructions', body: 'Your lasting preferences and limits, kept in LOCAL.md. Updates do not overwrite them.'},
                {label: 'gtmux instructions', body: 'How HQ works by default, kept in AGENTS.md and updated with gtmux. Your instructions take priority.'},
              ]
        }
        note={
          lang === 'zh'
            ? '手机保存的是上次获取的副本。app 无法确认手机备份是否完成。你也可以导出到「文件」；导出的档案未加密，请妥善保存。'
            : 'This phone keeps the last copy you fetched. The app cannot confirm that a phone backup ran. You can export the copy to Files; the exported archive is unencrypted, so store it securely.'
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
