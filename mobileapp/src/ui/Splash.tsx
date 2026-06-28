// Splash — the RN launch view (MOBILE §16 / D8). The native LaunchScreen shows
// instantly; this matches it while AppContext restores the paired Mac + settings,
// so the first frame is branded, not a bare spinner. Same pane-grid motif + the
// product slogan; a spinner sits near the bottom.

import React from 'react';
import {ActivityIndicator, StyleSheet, Text, View} from 'react-native';
import {BrandMark} from './BrandMark';
import {Lang} from '../i18n';
import {Palette} from './theme';

export function Splash({pal, lang}: {pal: Palette; lang: Lang}) {
  return (
    <View style={[styles.root, {backgroundColor: pal.bg}]}>
      <View style={styles.center}>
        <BrandMark size={68} />
        <Text style={[styles.word, {color: pal.fg}]}>gtmux</Text>
        <Text style={[styles.slogan, {color: pal.fg3}]}>
          {lang === 'zh'
            ? 'tmux × agent · 一眼看全 tmux 里的所有 agent'
            : 'tmux × agent · Your agents across tmux, at a glance.'}
        </Text>
      </View>
      <ActivityIndicator style={styles.spin} color={pal.fg3} />
    </View>
  );
}

const styles = StyleSheet.create({
  root: {flex: 1, alignItems: 'center', justifyContent: 'center'},
  center: {alignItems: 'center', paddingHorizontal: 40},
  word: {fontSize: 30, fontWeight: '800', marginTop: 18, letterSpacing: 0.5},
  slogan: {fontSize: 13.5, marginTop: 8, textAlign: 'center', lineHeight: 19},
  spin: {position: 'absolute', bottom: 64},
});
