// SettingsScreen — language (en/zh/system), the paired Mac (+ remove), push
// status, and app version. Removing the Mac clears the Keychain; the app then
// falls back to Pairing automatically (the navigator unmounts).

import React from 'react';
import {ScrollView, StyleSheet, Switch, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {APP_VERSION as appVersion} from '../version';
import {LangPref} from '../i18n';
import {useApp} from '../state/AppContext';

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
  const {t, pal, langPref, setLangPref, mac, unpair, pushEnabled, setPushEnabled} = useApp();

  const langs: {key: LangPref; label: string}[] = [
    {key: 'system', label: t('system')},
    {key: 'en', label: 'English'},
    {key: 'zh', label: '中文'},
  ];

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
            onPress={unpair}>
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
