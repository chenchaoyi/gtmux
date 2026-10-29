// DiffModal — "what did the agent change". Fetches a unified `git diff` of the
// pane's cwd from /api/diff and renders it line-coloured (additions green,
// deletions red, hunks cyan). Read-only; full-screen over the Detail.

import React, {useEffect, useMemo, useState} from 'react';
import {ActivityIndicator, FlatList, Modal, PixelRatio, ScrollView, StatusBar, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {GtmuxClient} from '../api/client';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {diffLineColor, diffRows} from './diff';
import {MODAL_ORIENTATIONS} from './modalOrientations';

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
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const {width, fontScale} = useWindowDimensions();
  const scale = fontScale;
  const rowHeight = PixelRatio.roundToNearestPixel(16 * scale);

  useEffect(() => {
    if (!visible) { setText(null); return; }
    let alive = true;
    setLoading(true);
    setFailed(false);
    setText(null);
    client
      .diff(paneId)
      .then(d => alive && (setText(d), setLoading(false)))
      .catch(() => { if (alive) { setFailed(true); setLoading(false); } });
    return () => {
      alive = false;
    };
  }, [visible, paneId, client, attempt]);

  const rows = useMemo(() => diffRows(text ?? ''), [text]);
  const contentWidth = Math.max(width, rows.columns * 7 * scale + 24);
  const empty = !loading && (text ?? '') === '';

  return (
    <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={visible} animationType="slide" onRequestClose={onClose}>
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
        ) : failed ? (
          <View style={styles.center}>
            <Text accessibilityRole="alert" style={[styles.empty, {color: pal.fg3}]}>{lang === 'zh' ? '无法读取改动，请检查连接后重试。' : 'Could not load changes. Check the connection and retry.'}</Text>
            <TouchableOpacity accessibilityRole="button" onPress={() => setAttempt(value => value + 1)} style={styles.retry}>
              <Text style={styles.done}>{lang === 'zh' ? '重试' : 'Retry'}</Text>
            </TouchableOpacity>
          </View>
        ) : empty ? (
          <Text style={[styles.empty, {color: pal.fg3}]}>
            {lang === 'zh' ? '没有改动（或这个目录不是 git 仓库）。' : 'No changes (or the directory isn’t a git repo).'}
          </Text>
        ) : (
          <View style={styles.body}>
            {rows.shortened && <Text style={styles.notice}>{lang === 'zh' ? '超长行已缩短显示；完整内容请在本机查看。' : 'Very long lines are shortened. View the complete content on your Mac.'}</Text>}
            <ScrollView horizontal contentContainerStyle={styles.hContent} showsHorizontalScrollIndicator>
              <FlatList
                key={rowHeight}
                testID="diff-lines"
                style={{width: contentWidth}}
                contentContainerStyle={styles.bodyContent}
                data={rows.lines}
                initialNumToRender={40}
                maxToRenderPerBatch={24}
                windowSize={5}
                getItemLayout={(_, index) => ({length: rowHeight, offset: rowHeight * index, index})}
                keyExtractor={(_, index) => String(index)}
                renderItem={({item}) => <Text numberOfLines={1} style={[styles.line, {height: rowHeight, lineHeight: rowHeight, color: diffLineColor(item)}]}>{item || ' '}</Text>}
              />
            </ScrollView>
          </View>
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
  retry: {alignSelf: 'center', minHeight: 44, justifyContent: 'center', paddingHorizontal: 20},
  notice: {color: '#EBEBF5', padding: 12, fontSize: 12},
  empty: {fontSize: 14, textAlign: 'center', marginTop: 60, paddingHorizontal: 24},
  body: {flex: 1, backgroundColor: '#0A0A0C'},
  // horizontal content must at least fill the screen so a narrow diff doesn't
  // leave a void to its right (the dark body bg still covers, but edges snap).
  hContent: {flexGrow: 1, minWidth: '100%'},
  bodyContent: {paddingHorizontal: 12, paddingBottom: 40},
  line: {fontFamily: 'Menlo', fontSize: 11, lineHeight: 15},
});
