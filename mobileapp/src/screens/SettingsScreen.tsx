// SettingsScreen — language (en/zh/system), the paired Mac (+ remove), push
// status, and app version. Removing the Mac clears the Keychain; the app then
// falls back to Pairing automatically (the navigator unmounts).

import React from 'react';
import {Alert, Share, ScrollView, StyleSheet, Switch, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {APP_VERSION as appVersion} from '../version';
import {LangPref} from '../i18n';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';

function Section({title, pal, children}: any) {
  return (
    <View style={styles.section}>
      <Text style={[styles.sectionTitle, {color: pal.fg3}]}>{title.toUpperCase()}</Text>
      <View style={[styles.card, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
        {children}
      </View>
    </View>
  );
}

export function SettingsScreen({navigation}: any) {
  const {t, lang, pal, langPref, setLangPref, mac, removeServer, pushEnabled, setPushEnabled, xtermEnabled, setXtermEnabled, fontPref, setFontPref, returnSends, setReturnSends, defaultDetailMode, setDefaultDetailMode} =
    useApp();

  const detailModes: {key: 'chat' | 'terminal'; label: string}[] = [
    {key: 'terminal', label: lang === 'zh' ? '终端' : 'Terminal'},
    {key: 'chat', label: lang === 'zh' ? '对话' : 'Chat'},
  ];
  const {client} = useAgents();

  const langs: {key: LangPref; label: string}[] = [
    {key: 'system', label: t('system')},
    {key: 'en', label: 'English'},
    {key: 'zh', label: '中文'},
  ];
  const fonts: {key: string; label: string}[] = [
    {key: 'auto', label: t('fontAuto')},
    {key: 'system', label: t('fontSystem')},
    {key: 'Hack', label: 'Hack'},
    {key: 'JetBrains Mono', label: 'JetBrains Mono'},
    {key: 'Fira Code', label: 'Fira Code'},
    {key: 'IBM Plex Mono', label: 'IBM Plex Mono'},
  ];

  // Handoff: mint a one-time code on the paired Mac and share a browser link so you
  // can continue watching on a computer (the browser pairs via /#c=<code>).
  const openOnComputer = async () => {
    try {
      const code = client && (await client.enrollMint());
      if (!code || !mac) {
        Alert.alert(t('openOnComputer'), t('openOnComputerFail'));
        return;
      }
      const url = `${mac.url.replace(/\/+$/, '')}/#c=${code}`;
      // one link only (message, not message+url) — and `message` is the portable
      // field for a future Android build.
      await Share.share({message: url});
    } catch {
      Alert.alert(t('openOnComputer'), t('openOnComputerFail'));
    }
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹ </Text>
        </TouchableOpacity>
        <Text style={[styles.title, {color: pal.fg}]}>{t('settings')}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.body}>
        <Section title={t('language')} pal={pal}>
          {langs.map((l, i) => (
            <TouchableOpacity
              key={l.key}
              style={[styles.rowItem, i < langs.length - 1 && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth}]}
              onPress={() => setLangPref(l.key)}>
              <Text style={[styles.rowLabel, {color: pal.fg}]}>{l.label}</Text>
              {langPref === l.key && <Text style={styles.check}>✓</Text>}
            </TouchableOpacity>
          ))}
        </Section>

        <Section title={t('terminalFont')} pal={pal}>
          {fonts.map((f, i) => (
            <TouchableOpacity
              key={f.key}
              style={[styles.rowItem, i < fonts.length - 1 && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth}]}
              onPress={() => setFontPref(f.key)}>
              <Text style={[styles.rowLabel, {color: pal.fg}]}>{f.label}</Text>
              {fontPref === f.key && <Text style={styles.check}>✓</Text>}
            </TouchableOpacity>
          ))}
        </Section>

        <Section title={t('pairedMac')} pal={pal}>
          <View style={styles.rowItem}>
            <View style={styles.flex}>
              <Text style={[styles.rowLabel, {color: pal.fg}]} numberOfLines={1}>
                {mac?.name || '—'}
              </Text>
              <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
                {mac?.url || ''}
              </Text>
            </View>
          </View>
          <TouchableOpacity
            style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}
            onPress={openOnComputer}>
            <View style={styles.flex}>
              <Text style={[styles.rowLabel, {color: pal.fg}]}>{t('openOnComputer')}</Text>
              <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>{t('openOnComputerSub')}</Text>
            </View>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>↗</Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}
            onPress={() => navigation.navigate('Servers')}>
            <Text style={[styles.rowLabel, {color: pal.fg}]}>{t('switchServer')}</Text>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>›</Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}
            onPress={() =>
              mac &&
              Alert.alert(mac.name, t('removeServerQ'), [
                {text: t('cancel'), style: 'cancel'},
                {text: t('removeMac'), style: 'destructive', onPress: () => removeServer(mac.url)},
              ])
            }>
            <Text style={[styles.rowLabel, styles.danger]}>{t('removeMac')}</Text>
          </TouchableOpacity>
        </Section>

        <Section title={t('push')} pal={pal}>
          <View style={styles.rowItem}>
            <Text style={[styles.rowLabel, {color: pal.fg}]}>{t('push')}</Text>
            <Switch value={pushEnabled} onValueChange={setPushEnabled} />
          </View>
          <View style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>{t('pushHint')}</Text>
          </View>
        </Section>

        <Section title={lang === 'zh' ? '详情页默认模式' : 'Detail default mode'} pal={pal}>
          {detailModes.map((m, i) => (
            <TouchableOpacity
              key={m.key}
              style={[styles.rowItem, i < detailModes.length - 1 && {borderBottomColor: pal.divider, borderBottomWidth: StyleSheet.hairlineWidth}]}
              onPress={() => setDefaultDetailMode(m.key)}>
              <Text style={[styles.rowLabel, {color: pal.fg}]}>{m.label}</Text>
              {defaultDetailMode === m.key && <Text style={styles.check}>✓</Text>}
            </TouchableOpacity>
          ))}
          <View style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>
              {lang === 'zh'
                ? '打开窗格时默认进哪个模式。对话=当前屏幕概览 + 审批卡；终端=完整 TUI。每个窗格的手动切换会被单独记住。'
                : 'Which mode a pane opens in. Chat = current-screen glance + approval card; Terminal = full TUI. Each pane remembers its own manual switch.'}
            </Text>
          </View>
        </Section>

        <Section title={lang === 'zh' ? '终端' : 'Terminal'} pal={pal}>
          <View style={styles.rowItem}>
            <Text style={[styles.rowLabel, {color: pal.fg}]}>
              {lang === 'zh' ? 'xterm 渲染（实验）' : 'xterm renderer (beta)'}
            </Text>
            <Switch value={xtermEnabled} onValueChange={setXtermEnabled} />
          </View>
          <View style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>
              {lang === 'zh'
                ? '用真正的终端内核（xterm.js）渲染窗格，全屏 TUI、真彩、中文宽度更准；关则用经典渲染。'
                : 'Render the pane with a real terminal core (xterm.js) — better for full-screen TUIs, true color, CJK widths. Off uses the classic renderer.'}
            </Text>
          </View>
          <View style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
            <Text style={[styles.rowLabel, {color: pal.fg}]}>
              {lang === 'zh' ? '回车直接发送' : 'Return sends'}
            </Text>
            <Switch value={returnSends} onValueChange={setReturnSends} />
          </View>
          <View style={[styles.rowItem, {borderTopColor: pal.divider, borderTopWidth: StyleSheet.hairlineWidth}]}>
            <Text style={[styles.rowSub, {color: pal.fg3}]}>
              {lang === 'zh'
                ? '开启后回车直接发送消息；关闭（默认）时回车为换行，用 ↑ 按钮发送。'
                : 'On: Return sends the message. Off (default): Return inserts a newline; send with the ↑ button.'}
            </Text>
          </View>
        </Section>

        <View style={styles.versionWrap}>
          <Text style={[styles.version, {color: pal.fg3}]}>
            {t('version')} {appVersion}
          </Text>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 10},
  back: {fontSize: 28, fontWeight: '300'},
  title: {fontSize: 20, fontWeight: '700'},
  body: {padding: 16},
  section: {marginBottom: 24},
  sectionTitle: {fontSize: 11, fontWeight: '700', letterSpacing: 0.6, marginBottom: 8, marginLeft: 4},
  card: {borderRadius: 12, borderWidth: StyleSheet.hairlineWidth, overflow: 'hidden'},
  rowItem: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 14, paddingVertical: 13},
  rowLabel: {fontSize: 15},
  rowSub: {fontSize: 12.5, marginTop: 2},
  flex: {flex: 1, minWidth: 0},
  check: {color: '#06B6D4', fontSize: 16, fontWeight: '700'},
  danger: {color: '#EF4444'},
  versionWrap: {alignItems: 'center', marginTop: 8},
  version: {fontSize: 12},
});
