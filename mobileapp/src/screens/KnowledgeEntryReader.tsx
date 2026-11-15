import React from 'react';
import {Linking, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import type {KnowledgeEntry} from '../api/client';
import {AnchoredMenu, type MenuAnchor, type MenuItem} from '../ui/AnchoredMenu';
import {BrandLoader} from '../ui/BrandLoader';
import {Chevron} from '../ui/Icons';
import {MarkdownView, type MdColors} from '../ui/MarkdownView';
import {SIcon} from '../ui/SettingsIcons';
import {BRAND, type Palette} from '../ui/theme';
import {READING_WIDTH} from '../ui/layout';
import {actsFor, axesLine, provenanceOf, resolveEntry, splitTitleKey, type EntryAct} from './knowledgeModel';
import {knowledgeUsage} from './knowledgePresentation';
import {knowledgeReading} from './knowledgeReading';

type RecordAction = Exclude<EntryAct, {kind: 'feedback'}>['kind'];
export interface KnowledgeEntryReaderProps {
  entry: KnowledgeEntry | null;
  loading: boolean;
  error?: 'failed' | 'timeout' | null;
  pal: Palette;
  zh: boolean;
  scope?: object;
  actionPending?: boolean;
  ageOf: (at?: number) => string;
  onRetry: () => void;
  onAct: (kind: RecordAction, id: string, revision?: string) => void;
}

function Disclosure({id, title, open, toggle, pal, children}: {
  id: string; title: string; open: boolean; toggle: () => void; pal: Palette; children: React.ReactNode;
}) {
  return <View style={[styles.section, {borderTopColor: pal.divider}]}>
    <TouchableOpacity testID={id} accessibilityRole="button" accessibilityState={{expanded: open}}
      onPress={toggle} style={styles.sectionHead}>
      <Text style={[styles.sectionTitle, {color: pal.fg}]}>{title}</Text>
      <Chevron size={16} color={pal.fg2} open={open} />
    </TouchableOpacity>
    {open && children}
  </View>;
}

export function KnowledgeEntryReader({entry, loading, error, pal, zh, scope, actionPending, ageOf, onRetry, onAct}: KnowledgeEntryReaderProps) {
  const insets = useSafeAreaInsets();
  const [bodyOpen, setBodyOpen] = React.useState(true);
  const [historyOpen, setHistoryOpen] = React.useState(false);
  const [recordOpen, setRecordOpen] = React.useState(false);
  const [menuOpen, setMenuOpen] = React.useState(false);
  const [anchor, setAnchor] = React.useState<MenuAnchor | null>(null);
  const more = React.useRef<View>(null);
  const mounted = React.useRef(true);
  React.useEffect(() => {mounted.current = true; return () => {mounted.current = false;};}, []);
  const current = React.useRef({entry: '', scope});
  current.current = {entry: JSON.stringify([entry?.id, entry?.revision]), scope};
  React.useEffect(() => {
    setBodyOpen(true); setHistoryOpen(false); setRecordOpen(false); setMenuOpen(false);
  }, [entry?.id, scope]);
  React.useEffect(() => setMenuOpen(false), [entry?.revision]);
  const t = (en: string, cn: string) => zh ? cn : en;
  const retry = <TouchableOpacity testID="knowledge-entry-retry" accessibilityRole="button" disabled={loading}
    onPress={onRetry} style={[styles.retry, {borderColor: pal.divider}]}>
    <Text style={{color: pal.fg, fontSize: 14, fontWeight: '600'}}>{loading ? t('Loading…', '加载中…') : t('Try again', '重新加载')}</Text>
  </TouchableOpacity>;
  if (!entry) return <View style={styles.state}>
    {loading ? <BrandLoader label={t('Loading entry…', '正在加载条目…')} neutral={pal.fg3} labelColor={pal.fg2} /> : <>
      <Text style={[styles.stateText, {color: pal.fg}]}>{error === 'timeout' ? t('Loading timed out', '加载超时') : t('Could not load this entry', '暂时无法加载条目')}</Text>
      {retry}
    </>}
  </View>;

  const resolved = resolveEntry(entry, zh);
  const parts = knowledgeReading(resolved.body);
  const use = knowledgeUsage(entry, zh);
  const actions = actsFor(entry);
  const primary = actions.find(a => a.kind === 'feedback' || a.kind === 'carry' || a.kind === 'land');
  const secondary = actions.filter(a => a !== primary);
  const key = current.current.entry;
  const entryID = entry.id;
  const revision = entry.revision;
  const run = (action: EntryAct) => {
    if (!mounted.current || current.current.entry !== key || current.current.scope !== scope) return;
    if (action.kind === 'feedback') void Linking.openURL(action.url);
    else onAct(action.kind, entryID, revision);
  };
  const label = (action: EntryAct) => action.kind === 'feedback' ? t('Prepare feedback', '准备反馈')
    : action.kind === 'carry' ? t('Write instructions', '写入指令')
    : action.kind === 'land' ? t('Record result', '记录结果')
    : action.kind === 'withdraw' ? t('Cancel plan', '取消安排') : t('No longer applicable', '不再适用');
  const items: MenuItem[] = secondary.map(action => ({key: action.kind, label: label(action),
    icon: action.kind === 'land' ? 'check' : action.kind === 'withdraw' ? 'return' : 'trash',
    danger: action.kind === 'retire', onPress: () => run(action)}));
  const colors: MdColors = {text: pal.fg, dim: pal.fg2, code: pal.fg, codeBg: pal.surface, border: pal.divider, link: pal.fg};
  const markdown = (source: string) => <MarkdownView source={source} colors={colors} fontSize={16} selectable calmEmphasis />;
  return <View style={styles.reader}>
    <ScrollView testID="knowledge-entry-reading" keyboardShouldPersistTaps="handled" contentContainerStyle={styles.scroll}>
      <Text accessibilityRole="header" style={[styles.title, {color: pal.fg}]}>{splitTitleKey(resolved.title, entry.id).rest}</Text>
      <View style={styles.metadata}>
        <View testID="knowledge-use-summary" style={styles.status}><Text style={[styles.meta, {color: pal.fg2}]}>{[ageOf(entry.at), use.state || use.title].filter(Boolean).join(' · ')}</Text></View>
        <TouchableOpacity ref={more} testID="knowledge-entry-more" accessibilityRole="button" accessibilityLabel={t('More entry actions', '更多条目操作')}
          accessibilityState={{expanded: menuOpen}} style={styles.more} onPress={() => {
            setAnchor(null); more.current?.measureInWindow((x, y, width, height) => setAnchor({x, y, width, height})); setMenuOpen(true);
          }}>
          <Text style={{fontSize: 13, color: pal.fg2}}>{t('More', '更多')}</Text><SIcon name="chevronDown" size={14} color={pal.fg2} />
        </TouchableOpacity>
      </View>
      {(loading || error) && <View style={[styles.refresh, {borderColor: pal.divider}]}>
        <Text style={[styles.meta, {color: pal.fg2}]}>{loading ? t('Refreshing…', '正在刷新…') : t('Content could not be refreshed', '内容暂未刷新')}</Text>{retry}
      </View>}
      {!resolved.body?.trim() && <Text style={[styles.emptyBody, {color: pal.fg2}]}>{t('No content to display', '暂无可显示的正文')}</Text>}
      {!!parts.lead && <View testID="knowledge-core-guidance" style={styles.guidance}>
        <View style={styles.guidanceHead}><View style={[styles.accent, {backgroundColor: BRAND}]} /><Text accessibilityRole="header" style={[styles.sectionTitle, {color: pal.fg}]}>{t('Key guidance', '核心建议')}</Text></View>
        {markdown(parts.lead)}
      </View>}
      {!!parts.body && <Disclosure id="knowledge-body-details" title={t('Details', '正文说明')} open={bodyOpen} toggle={() => setBodyOpen(o => !o)} pal={pal}>{markdown(parts.body)}</Disclosure>}
      {!!parts.history && <Disclosure id="knowledge-history-details" title={t('History', '历史记录')} open={historyOpen} toggle={() => setHistoryOpen(o => !o)} pal={pal}>{markdown(parts.history)}</Disclosure>}
      <Disclosure id="knowledge-record-details" title={t('Record details', '资料详情')} open={recordOpen} toggle={() => setRecordOpen(o => !o)} pal={pal}>
        <View style={styles.records}>
          <Text selectable style={[styles.meta, {color: pal.fg2}]}>{entry.id}</Text>
          {!!resolved.tag && <Text style={[styles.meta, {color: pal.fg2}]}>{t('Original language: ', '原文语言：')}{resolved.tag === 'zh' ? t('Chinese', '中文') : resolved.tag === 'en' ? t('English', '英文') : resolved.tag}</Text>}
          {!!axesLine(entry, zh) && <Text testID="knowledge-axes" style={[styles.meta, {color: pal.fg2}]}>{axesLine(entry, zh)}</Text>}
          {!!provenanceOf(entry, zh) && <Text style={[styles.meta, {color: pal.fg2}]}>{provenanceOf(entry, zh)}</Text>}
          <Text style={[styles.meta, {color: pal.fg2}]}>{use.line}</Text>
          <Text style={[styles.meta, {color: pal.fg2}]}>{use.explanation}</Text>
          {!!entry.audience_repo && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{entry.audience_repo}</Text>}
          {!!entry.promote_why && <Text style={[styles.meta, {color: pal.fg2}]}>{entry.promote_why}</Text>}
          {!!entry.landed_ref && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{t('Recorded result: ', '应用记录：')}{entry.landed_ref}</Text>}
          {!!entry.promote_target && <Text style={[styles.meta, {color: pal.fg2}]}>{t('Planned destination: ', '计划位置：')}{entry.promote_target}</Text>}
        </View>
      </Disclosure>
    </ScrollView>
    {!!primary && !actionPending && <View style={[styles.footer, {backgroundColor: pal.bg, borderTopColor: pal.divider, paddingBottom: Math.max(16, insets.bottom)}]}>
      <TouchableOpacity testID={`knowledge-act-${primary.kind}`} accessibilityRole="button" onPress={() => run(primary)} style={[styles.primary, {borderColor: BRAND}]}>
        <SIcon name="pencil" color={pal.fg} size={19} /><Text style={[styles.primaryText, {color: pal.fg}]}>{label(primary)}</Text>
      </TouchableOpacity>
    </View>}
    <AnchoredMenu testID="knowledge-act" visible={menuOpen} anchor={anchor} title={t('Entry actions', '条目操作')}
      sections={[items.filter(item => !item.danger), items.filter(item => item.danger)]} iconPlacement="leading" pal={pal}
      closeLabel={t('Close actions', '关闭操作菜单')} onClose={() => setMenuOpen(false)} />
  </View>;
}

const styles = StyleSheet.create({
  reader: {flex: 1, minHeight: 0, minWidth: 0},
  scroll: {paddingHorizontal: 20, paddingTop: 20, paddingBottom: 28, width: '100%', maxWidth: READING_WIDTH, alignSelf: 'center'},
  title: {fontSize: 21, lineHeight: 29, fontWeight: '700'},
  metadata: {flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 2},
  status: {flex: 1},
  more: {minHeight: 44, paddingHorizontal: 4, flexDirection: 'row', alignItems: 'center', gap: 4},
  meta: {fontSize: 13, lineHeight: 20},
  guidance: {marginTop: 28, gap: 14, marginBottom: 8},
  guidanceHead: {flexDirection: 'row', alignItems: 'center', gap: 10},
  accent: {width: 3, height: 20, borderRadius: 2},
  section: {marginTop: 24, borderTopWidth: StyleSheet.hairlineWidth},
  sectionHead: {minHeight: 56, flexDirection: 'row', alignItems: 'center', gap: 16, paddingVertical: 12},
  sectionTitle: {flex: 1, fontSize: 18, lineHeight: 25, fontWeight: '600'},
  records: {gap: 10},
  emptyBody: {fontSize: 16, lineHeight: 24, marginTop: 24},
  refresh: {marginTop: 18, paddingTop: 8, borderTopWidth: StyleSheet.hairlineWidth, gap: 4},
  footer: {paddingHorizontal: 20, paddingTop: 12, borderTopWidth: StyleSheet.hairlineWidth},
  primary: {minHeight: 48, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingVertical: 12, paddingHorizontal: 16, flexDirection: 'row', gap: 10, alignItems: 'center', justifyContent: 'center', width: '100%', maxWidth: READING_WIDTH, alignSelf: 'center'},
  primaryText: {fontSize: 16, lineHeight: 22, fontWeight: '600', flexShrink: 1},
  retry: {minHeight: 44, borderWidth: StyleSheet.hairlineWidth, borderRadius: 9, paddingHorizontal: 16, paddingVertical: 10, alignSelf: 'flex-start', justifyContent: 'center'},
  state: {flex: 1, alignItems: 'center', justifyContent: 'center', padding: 24, gap: 18},
  stateText: {fontSize: 16, lineHeight: 24, textAlign: 'center'},
});
