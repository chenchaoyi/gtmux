// DiffModal — "what did the agent change". Fetches a unified `git diff` of the
// pane's cwd from /api/diff and renders it line-coloured (additions green,
// deletions red, hunks cyan). Read-only; full-screen over the Detail.

import React, {useEffect, useState} from 'react';
import {ActivityIndicator, Modal, ScrollView, StatusBar, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {GtmuxClient} from '../api/client';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {diffLineColor} from './diff';

export function DiffModal({
  visible,
  paneId,
  client,
  pal,
  lang,
  onClose,
}: {
  visible: boolean;
  paneId: string;
  client: GtmuxClient;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
}) {
  const [text, setText] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!visible) return;
    let alive = true;
    setLoading(true);
    setText(null);
    client
      .diff(paneId)
      .then(d => alive && (setText(d), setLoading(false)))
      .catch(() => alive && (setText(''), setLoading(false)));
    return () => {
      alive = false;
    };
  }, [visible, paneId, client]);

  const lines = (text ?? '').replace(/\n$/, '').split('\n');
  const empty = !loading && (text ?? '') === '';

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={onClose}>
      <StatusBar hidden />
      <View style={[styles.root, {backgroundColor: pal.bg}]}>
        <View style={[styles.bar, {borderBottomColor: pal.divider}]}>
          <Text style={[styles.title, {color: pal.fg}]}>{lang === 'zh' ? '改动 (git diff)' : 'Changes (git diff)'}</Text>
          <TouchableOpacity onPress={onClose} hitSlop={hit}>
            <Text style={styles.done}>{lang === 'zh' ? '完成' : 'Done'}</Text>
          </TouchableOpacity>
        </View>

        {loading ? (
          <ActivityIndicator color={pal.fg3} style={styles.center} />
        ) : empty ? (
          <Text style={[styles.empty, {color: pal.fg3}]}>
            {lang === 'zh' ? '没有改动（或这个目录不是 git 仓库）。' : 'No changes (or the directory isn’t a git repo).'}
          </Text>
        ) : (
          <ScrollView style={styles.body} contentContainerStyle={styles.bodyContent}>
            <ScrollView horizontal showsHorizontalScrollIndicator>
              <View>
                {lines.map((ln, i) => (
                  <Text key={i} style={[styles.line, {color: diffLineColor(ln)}]}>
                    {ln === '' ? ' ' : ln}
                  </Text>
                ))}
              </View>
            </ScrollView>
          </ScrollView>
        )}
      </View>
    </Modal>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  root: {flex: 1},
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingTop: 56,
    paddingBottom: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  title: {fontSize: 17, fontWeight: '700'},
  done: {fontSize: 15, fontWeight: '700', color: '#06B6D4'},
  center: {marginTop: 60},
  empty: {fontSize: 14, textAlign: 'center', marginTop: 60, paddingHorizontal: 24},
  body: {flex: 1, backgroundColor: '#0A0A0C'},
  bodyContent: {padding: 12},
  line: {fontFamily: 'Menlo', fontSize: 11, lineHeight: 15},
});
