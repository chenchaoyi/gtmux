// The native launch and RN startup view share their mark and appearance ground.
// This view exits as soon as settings restore; it never delays app readiness.
import React from 'react';
import {ScrollView, StyleSheet, Text, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {BrandLoader} from './BrandLoader';
import {LaunchMark} from './LaunchMark';
import {Lang} from '../i18n';
import {Palette} from './theme';

export function Splash({pal, lang}: {pal: Palette; lang: Lang}) {
  const insets = useSafeAreaInsets();
  const [footerHeight, setFooterHeight] = React.useState(22);
  const top = Math.max(24, insets.top + 16);
  const footerSpace = footerHeight + insets.bottom + 40;
  const loading = lang === 'zh' ? '正在准备…' : 'Getting ready…';
  return (
    <View style={[styles.root, {backgroundColor: pal.bg}]} testID="splash">
      <ScrollView
        style={{flex: 1, marginBottom: footerSpace}}
        automaticallyAdjustContentInsets={false}
        contentInsetAdjustmentBehavior="never"
        contentContainerStyle={[
          styles.canvas,
          {paddingTop: top, paddingBottom: Math.max(24, top + 48 - footerSpace)},
        ]}
        showsVerticalScrollIndicator={false}
      >
        <View style={styles.identity}>
          <LaunchMark />
          <Text accessibilityRole="header" allowFontScaling={false} style={[styles.word, {color: pal.fg}]}>
            gtmux
          </Text>
          <Text style={[styles.slogan, {color: pal.fg}]}>
            {lang === 'zh'
              ? 'tmux 里的所有 agent，\n一眼看全。'
              : 'Your agents across tmux,\nat a glance.'}
          </Text>
        </View>
      </ScrollView>
      <View
        accessible
        accessibilityRole="progressbar"
        accessibilityState={{busy: true}}
        accessibilityLabel={loading}
        style={[styles.footer, {bottom: insets.bottom + 20}]}
        onLayout={e => setFooterHeight(e.nativeEvent.layout.height)}
      >
        <View importantForAccessibility="no-hide-descendants" accessibilityElementsHidden>
          <BrandLoader size={18} neutral={pal.fg} />
        </View>
        <Text style={[styles.loading, {color: pal.fg}]}>{loading}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1},
  canvas: {flexGrow: 1, justifyContent: 'center', alignItems: 'center', paddingHorizontal: 32},
  identity: {width: '100%', maxWidth: 312, alignItems: 'center'},
  word: {fontSize: 34, lineHeight: 42, fontWeight: '700', marginTop: 24},
  slogan: {fontSize: 15, lineHeight: 22, marginTop: 12, textAlign: 'center', opacity: 0.72},
  footer: {position: 'absolute', alignSelf: 'center', maxWidth: '86%', flexDirection: 'row', alignItems: 'center', gap: 9},
  loading: {fontSize: 12, lineHeight: 18, opacity: 0.72, flexShrink: 1},
});
