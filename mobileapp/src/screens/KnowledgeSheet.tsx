// KnowledgeSheet — reading and closing out HQ's knowledge base from the phone
// (hq-knowledge-on-phone).
//
// The base is HQ's long-term memory: on this machine 330 entries across 7 topics. A phone
// cannot be a browser for that and should not try. It answers three questions in order,
// and the order is the design:
//
//   1. WHAT DOES IT OWE ME. The promotion queue leads. HQ judged these entries
//      charter-level and wrote an export brief for each; carrying one somewhere durable is
//      the ONLY step of the whole knowledge lifecycle that blocks on a person, and `land`
//      is a fact only that person holds. Oldest first, because the oldest is the one
//      rotting, and marked overdue at the same floor `gtmux doctor` uses.
//   2. WHAT DID IT JUST LEARN. A wrong entry is not inert — it is echoed into every
//      dispatch, so it repeats. Spot-checking recent writes is cheap and is reading, which
//      is what a phone is for.
//   3. WHERE IS EVERYTHING. Topics with counts, empties dropped.
//
// Two actions, both one short line of text: mark a promotion landed, retire a wrong entry.
// Writing an entry is deliberately absent — prose typed on a phone is how a knowledge base
// fills with entries nobody wants to read.
//
// Placement rules live in knowledgeModel.ts and are tested there; this file draws.

import React from 'react';
import {
  ActivityIndicator,
  Keyboard,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {KnowledgeAct, KnowledgeEntry, KnowledgeIndex} from '../api/client';
import {MarkdownView, MdColors} from '../ui/MarkdownView';
import {ERRORED_COLOR, Palette, StatusColor} from '../ui/theme';
import {relTime} from './hqZones';
import {
  KnowledgeView,
  buildKnowledgeView,
  entriesOfTopic,
  landPrompt,
  provenanceOf,
  retirePrompt,
  splitTitleKey,
} from './knowledgeModel';

/**
 * EntryTitle draws HQ's "key + prose" title as what it is: an identifier and a sentence.
 * A title with no key renders as one run, unchanged.
 */
function EntryTitle({title, id, pal, style, keyStyle, lines, wrapStyle}: {
  title: string;
  id: string;
  pal: Palette;
  style: object;
  keyStyle: object;
  lines?: number;
  wrapStyle?: object;
}) {
  const {key, rest} = splitTitleKey(title, id);
  // The key goes on its OWN line. Inline it reads as the first words of the sentence and
  // pushes the prose into an odd wrap ("…-model-and-agent 派活前先 / 选 agent 和 model"),
  // which costs more than the line it saves.
  if (!key) {
    return (
      <Text style={[style, wrapStyle]} numberOfLines={lines}>
        {title}
      </Text>
    );
  }
  return (
    <View style={wrapStyle}>
      <Text style={[keyStyle, {color: pal.fg3}]} numberOfLines={1}>
        {key}
      </Text>
      <Text style={style} numberOfLines={lines}>
        {rest}
      </Text>
    </View>
  );
}

type Pane = {kind: 'index'} | {kind: 'topic'; name: string} | {kind: 'entry'; id: string};
type Pending = {kind: 'land' | 'retire'; id: string} | null;

export interface KnowledgeSheetProps {
  visible: boolean;
  index: KnowledgeIndex;
  nowSecs: number;
  pal: Palette;
  zh: boolean;
  onClose: () => void;
  loadEntry: (id: string) => Promise<KnowledgeEntry | null>;
  act: (a: KnowledgeAct) => Promise<{ok: true} | {ok: false; error: string}>;
}

const mdColors = (pal: Palette): MdColors => ({
  text: pal.fg, dim: pal.fg3, code: pal.fg, codeBg: pal.surface, border: pal.divider, link: pal.fg2,
});

export function KnowledgeSheet({visible, index, nowSecs, pal, zh, onClose, loadEntry, act}: KnowledgeSheetProps) {
  const t = (en: string, cn: string) => (zh ? cn : en);
  const [pane, setPane] = React.useState<Pane>({kind: 'index'});
  const [entry, setEntry] = React.useState<KnowledgeEntry | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [pending, setPending] = React.useState<Pending>(null);
  const [draft, setDraft] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [done, setDone] = React.useState<string | null>(null);

  // The act bar is the one thing here you type into, and it sits at the bottom. A
  // KeyboardAvoidingView is the usual answer and it is WRONG inside a pageSheet: it
  // measures against the screen while the sheet is inset from it, so on the simulator it
  // lifted the bar by less than the keyboard and left exactly the input behind. The
  // keyboard's own reported height is in screen coordinates and the sheet is anchored to
  // the screen's bottom edge, so using it directly is both simpler and correct.
  const [kbHeight, setKbHeight] = React.useState(0);
  React.useEffect(() => {
    const show = Keyboard.addListener('keyboardWillShow', e => setKbHeight(e.endCoordinates.height));
    const hide = Keyboard.addListener('keyboardWillHide', () => setKbHeight(0));
    return () => {
      show.remove();
      hide.remove();
    };
  }, []);

  const view: KnowledgeView = React.useMemo(() => buildKnowledgeView(index, nowSecs), [index, nowSecs]);

  // Opening the sheet always starts at the top. Resuming where a previous visit left off
  // would put a reader inside one entry with no memory of why.
  React.useEffect(() => {
    if (!visible) return;
    setPane({kind: 'index'});
    setPending(null);
    setError(null);
    setDone(null);
  }, [visible]);

  const openEntry = React.useCallback(
    async (id: string) => {
      setPane({kind: 'entry', id});
      setEntry(null);
      setLoading(true);
      const e = await loadEntry(id);
      setEntry(e);
      setLoading(false);
    },
    [loadEntry],
  );

  const submit = React.useCallback(async () => {
    if (!pending || draft.trim() === '') return;
    const a: KnowledgeAct =
      pending.kind === 'land'
        ? {op: 'land', id: pending.id, ref: draft.trim()}
        : {op: 'retire', id: pending.id, why: draft.trim()};
    const r = await act(a);
    if (!r.ok) {
      // The server's own words. "has no pending promotion to land" tells the reader what
      // to do; a generic failure throws that away.
      setError(r.error);
      return;
    }
    setPending(null);
    setDraft('');
    setError(null);
    setDone(pending.kind === 'land' ? (zh ? '已标记落地' : 'marked landed') : zh ? '已退休' : 'retired');
    // A retired entry is gone from the live set, so there is nothing left to look at.
    setPane({kind: 'index'});
  }, [act, draft, pending, zh]);

  const ageOf = (secs?: number) => (secs ? relTime(secs, nowSecs) + (zh ? '前' : ' ago') : '');

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <View style={[styles.root, {backgroundColor: pal.bg}]}>
        <View style={[styles.head, {borderBottomColor: pal.divider}]}>
          {pane.kind !== 'index' ? (
            <TouchableOpacity testID="knowledge-back" onPress={() => setPane({kind: 'index'})} hitSlop={hit}>
              <Text style={[styles.back, {color: pal.fg2}]}>‹</Text>
            </TouchableOpacity>
          ) : null}
          <View style={styles.headText}>
            <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
              {pane.kind === 'topic' ? pane.name : t('knowledge', '知识库')}
            </Text>
            <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
              {pane.kind === 'index'
                ? zh
                  ? `${view.entries.length} 条 · ${view.topics.length} 个主题`
                  : `${view.entries.length} entries · ${view.topics.length} topics`
                : pane.kind === 'topic'
                  ? `${entriesOfTopic(view, pane.name).length}`
                  : (entry?.topic ?? '')}
            </Text>
          </View>
          <TouchableOpacity testID="knowledge-close" onPress={onClose} hitSlop={hit}>
            <Text style={[styles.close, {color: pal.fg2}]}>✕</Text>
          </TouchableOpacity>
        </View>

        {done ? (
          <View style={[styles.toast, {backgroundColor: pal.surface, borderBottomColor: pal.divider}]}>
            <Text style={[styles.toastText, {color: StatusColor.idle}]}>✓ {done}</Text>
          </View>
        ) : null}

        <ScrollView contentContainerStyle={styles.body}>
          {pane.kind === 'index' && (
            <IndexPane view={view} pal={pal} zh={zh} t={t} ageOf={ageOf} onOpen={openEntry} onTopic={n => setPane({kind: 'topic', name: n})} />
          )}
          {pane.kind === 'topic' && (
            <EntryList entries={entriesOfTopic(view, pane.name)} pal={pal} zh={zh} ageOf={ageOf} onOpen={openEntry} />
          )}
          {pane.kind === 'entry' && (
            <EntryPane
              entry={entry}
              loading={loading}
              pal={pal}
              zh={zh}
              t={t}
              ageOf={ageOf}
              onAct={(kind, id) => {
                setPending({kind, id});
                setDraft('');
                setError(null);
              }}
            />
          )}
        </ScrollView>

        {pending ? (
          <ActBar
            bottom={kbHeight}
            kind={pending.kind}
            pal={pal}
            zh={zh}
            t={t}
            draft={draft}
            error={error}
            onDraft={setDraft}
            onCancel={() => {
              setPending(null);
              setError(null);
            }}
            onSubmit={submit}
          />
        ) : null}
      </View>
    </Modal>
  );
}

/** The index: what is owed, what is new, where everything is. */
function IndexPane({
  view, pal, zh, t, ageOf, onOpen, onTopic,
}: {
  view: KnowledgeView;
  pal: Palette;
  zh: boolean;
  t: (en: string, cn: string) => string;
  ageOf: (s?: number) => string;
  onOpen: (id: string) => void;
  onTopic: (name: string) => void;
}) {
  if (view.empty) {
    return (
      <View style={styles.emptyBox}>
        <Text style={[styles.empty, {color: pal.fg3}]}>
          {t(
            'Nothing recorded yet. HQ writes here when it distills a lesson worth keeping.',
            '还没有记录。参谋长提炼出值得留下的经验时会写进这里。',
          )}
        </Text>
      </View>
    );
  }
  return (
    <>
      {view.promotions.length > 0 && (
        <View testID="knowledge-promotions">
          <SectionLabel pal={pal} text={t('waiting on you', '待你带走')} count={view.promotions.length} accent />
          {view.promotions.map(p => (
            <TouchableOpacity
              key={p.entry.id}
              testID={`knowledge-promotion-${p.entry.id}`}
              activeOpacity={0.6}
              onPress={() => onOpen(p.entry.id)}
              style={[styles.card, {borderColor: p.overdue ? ERRORED_COLOR : pal.divider, backgroundColor: pal.surface}]}>
              <View style={styles.cardHead}>
                <EntryTitle
                  title={p.entry.title}
                  id={p.entry.id}
                  pal={pal}
                  wrapStyle={styles.grow}
                  style={[styles.cardTitle, {color: pal.fg}]}
                  keyStyle={styles.titleKey}
                  lines={2}
                />
                <Text style={[styles.cardAge, {color: p.overdue ? ERRORED_COLOR : pal.fg3}]}>
                  {p.overdue ? (zh ? '逾期 ' : 'overdue · ') : ''}
                  {ageOf(p.entry.promoted_at)}
                </Text>
              </View>
              {!!p.entry.promote_why && (
                <Text style={[styles.cardWhy, {color: pal.fg2}]} numberOfLines={2}>
                  {p.entry.promote_why}
                </Text>
              )}
              {!!p.entry.promote_target && (
                <Text style={[styles.cardTarget, {color: pal.fg3}]} numberOfLines={1}>
                  → {p.entry.promote_target}
                </Text>
              )}
            </TouchableOpacity>
          ))}
        </View>
      )}

      <SectionLabel pal={pal} text={t('newest', '最近')} />
      <EntryList entries={view.recent} pal={pal} zh={zh} ageOf={ageOf} onOpen={onOpen} />

      <SectionLabel pal={pal} text={t('topics', '主题')} />
      {view.topics.map(tp => (
        <TouchableOpacity
          key={tp.name}
          testID={`knowledge-topic-${tp.name}`}
          activeOpacity={0.6}
          onPress={() => onTopic(tp.name)}
          style={[styles.row, {borderBottomColor: pal.divider}]}>
          <Text style={[styles.rowTitle, {color: pal.fg}]} numberOfLines={1}>
            {tp.name}
          </Text>
          <Text style={[styles.rowMeta, {color: pal.fg3}]}>{tp.count}</Text>
          <Text style={[styles.chev, {color: pal.fg3}]}>›</Text>
        </TouchableOpacity>
      ))}

      {view.candidates.pending > 0 && (
        <Text style={[styles.foot, {color: pal.fg3}]}>
          {zh
            ? `${view.candidates.pending} 条候选等参谋长提炼`
            : `${view.candidates.pending} captured candidates waiting for HQ to distill`}
        </Text>
      )}
    </>
  );
}

function EntryList({
  entries, pal, zh, ageOf, onOpen,
}: {
  entries: KnowledgeEntry[];
  pal: Palette;
  zh: boolean;
  ageOf: (s?: number) => string;
  onOpen: (id: string) => void;
}) {
  return (
    <>
      {entries.map(e => (
        <TouchableOpacity
          key={e.id}
          testID={`knowledge-entry-${e.id}`}
          activeOpacity={0.6}
          onPress={() => onOpen(e.id)}
          style={[styles.row, {borderBottomColor: pal.divider}]}>
          <View style={styles.rowText}>
            <EntryTitle
              title={e.title}
              id={e.id}
              pal={pal}
              style={[styles.rowTitle, {color: pal.fg}]}
              keyStyle={styles.titleKey}
              lines={2}
            />
            <Text style={[styles.rowMeta, {color: pal.fg3}]} numberOfLines={1}>
              {e.topic} · {ageOf(e.at)}
              {e.landed_ref ? (zh ? ` · 已落地 ${e.landed_ref}` : ` · landed ${e.landed_ref}`) : ''}
            </Text>
          </View>
          <Text style={[styles.chev, {color: pal.fg3}]}>›</Text>
        </TouchableOpacity>
      ))}
    </>
  );
}

/** One entry: its body, where it came from, and the two things you can do to it. */
function EntryPane({
  entry, loading, pal, zh, t, ageOf, onAct,
}: {
  entry: KnowledgeEntry | null;
  loading: boolean;
  pal: Palette;
  zh: boolean;
  t: (en: string, cn: string) => string;
  ageOf: (s?: number) => string;
  onAct: (kind: 'land' | 'retire', id: string) => void;
}) {
  if (loading) return <ActivityIndicator style={styles.spinner} color={pal.fg3} />;
  if (!entry) {
    return (
      <View style={styles.emptyBox}>
        <Text style={[styles.empty, {color: pal.fg3}]}>{t('This entry is gone.', '这条已经不在了。')}</Text>
      </View>
    );
  }
  const prov = provenanceOf(entry, zh);
  const pendingPromotion = !!entry.promoted_at && !entry.landed_at;
  return (
    <View style={styles.entry}>
      <EntryTitle
        title={entry.title}
        id={entry.id}
        pal={pal}
        style={[styles.entryTitle, {color: pal.fg}]}
        keyStyle={styles.titleKeyBig}
      />
      <Text style={[styles.entryMeta, {color: pal.fg3}]}>
        {entry.topic} · {ageOf(entry.at)}
        {prov ? ` · ${prov}` : ''}
      </Text>

      {pendingPromotion && (
        <View style={[styles.promoBox, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
          <Text style={[styles.promoLabel, {color: ERRORED_COLOR}]}>
            ⚑ {t('promoted, waiting to be carried', '已提升，待带走')} · {ageOf(entry.promoted_at)}
          </Text>
          {!!entry.promote_why && <Text style={[styles.promoText, {color: pal.fg2}]}>{entry.promote_why}</Text>}
          {!!entry.promote_target && (
            <Text style={[styles.promoText, {color: pal.fg3}]}>→ {entry.promote_target}</Text>
          )}
        </View>
      )}
      {!!entry.landed_ref && (
        <Text style={[styles.landed, {color: StatusColor.idle}]}>
          ✓ {t('landed', '已落地')} {entry.landed_ref}
        </Text>
      )}

      {!!entry.body && (
        <View style={styles.entryBody}>
          <MarkdownView source={entry.body} colors={mdColors(pal)} fontSize={13.5} selectable calmEmphasis />
        </View>
      )}

      <View style={[styles.actions, {borderTopColor: pal.divider}]}>
        {pendingPromotion && (
          <TouchableOpacity
            testID="knowledge-act-land"
            activeOpacity={0.6}
            onPress={() => onAct('land', entry.id)}
            style={[styles.action, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
            <Text style={[styles.actionText, {color: pal.fg}]}>{t('mark it landed…', '标记为已落地…')}</Text>
          </TouchableOpacity>
        )}
        <TouchableOpacity
          testID="knowledge-act-retire"
          activeOpacity={0.6}
          onPress={() => onAct('retire', entry.id)}
          style={[styles.action, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
          <Text style={[styles.actionText, {color: ERRORED_COLOR}]}>{t('retire it…', '退休这一条…')}</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}

/** The one line of text an action needs, asked for where the action was tapped. */
function ActBar({
  kind, pal, zh, t, draft, error, bottom, onDraft, onCancel, onSubmit,
}: {
  kind: 'land' | 'retire';
  /** Keyboard height, so the bar clears it. */
  bottom: number;
  pal: Palette;
  zh: boolean;
  t: (en: string, cn: string) => string;
  draft: string;
  error: string | null;
  onDraft: (s: string) => void;
  onCancel: () => void;
  onSubmit: () => void;
}) {
  const p = kind === 'land' ? landPrompt(zh) : retirePrompt(zh);
  const ready = draft.trim() !== '';
  return (
    <View
      testID="knowledge-act-bar"
      style={[styles.actBar, {borderTopColor: pal.divider, backgroundColor: pal.bg, paddingBottom: bottom > 0 ? bottom + 10 : 20}]}>
      {/* The buttons ride the TITLE row, not below the field. Measured on the simulator:
          with them under the input, the keyboard covered both — the sheet lifts the field
          into view and leaves its actions behind it. The field sits last, nearest the
          thumb, and Return submits so the buttons are the second way, not the only one. */}
      <View style={styles.actHead}>
        <Text style={[styles.actTitle, {color: pal.fg}]} numberOfLines={1}>
          {p.title}
        </Text>
        <TouchableOpacity testID="knowledge-act-cancel" onPress={onCancel} hitSlop={hit} style={styles.actBtn}>
          <Text style={[styles.actBtnText, {color: pal.fg3}]}>{t('cancel', '取消')}</Text>
        </TouchableOpacity>
        <TouchableOpacity
          testID="knowledge-act-submit"
          onPress={onSubmit}
          disabled={!ready}
          hitSlop={hit}
          style={[styles.actBtn, !ready && styles.actBtnOff]}>
          <Text style={[styles.actBtnText, {color: StatusColor.working}]}>{t('confirm', '确认')}</Text>
        </TouchableOpacity>
      </View>
      <Text style={[styles.actHint, {color: pal.fg3}]}>{p.hint}</Text>
      {!!error && <Text style={[styles.actError, {color: ERRORED_COLOR}]}>{error}</Text>}
      <TextInput
        testID="knowledge-act-input"
        value={draft}
        onChangeText={onDraft}
        placeholder={p.placeholder}
        placeholderTextColor={pal.fg3}
        autoFocus
        returnKeyType="done"
        onSubmitEditing={() => ready && onSubmit()}
        style={[styles.actInput, {color: pal.fg, borderColor: pal.divider, backgroundColor: pal.surface}]}
      />
    </View>
  );
}

function SectionLabel({pal, text, count, accent}: {pal: Palette; text: string; count?: number; accent?: boolean}) {
  return (
    <View style={styles.secRow}>
      <Text style={[styles.sec, {color: accent ? ERRORED_COLOR : pal.fg3}]}>{text}</Text>
      {count != null && <Text style={[styles.secCount, {color: accent ? ERRORED_COLOR : pal.fg3}]}>{count}</Text>}
    </View>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  root: {flex: 1},
  head: {flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 11, borderBottomWidth: StyleSheet.hairlineWidth},
  headText: {flex: 1},
  back: {fontSize: 28, fontWeight: '300', marginTop: -4},
  title: {fontSize: 17, fontWeight: '700'},
  sub: {fontSize: 12, marginTop: 1},
  close: {fontSize: 17, fontWeight: '600'},

  toast: {paddingHorizontal: 14, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  toastText: {fontSize: 13, fontWeight: '600'},

  body: {paddingBottom: 40},
  secRow: {flexDirection: 'row', alignItems: 'center', gap: 7, paddingHorizontal: 14, paddingTop: 18, paddingBottom: 7},
  sec: {fontSize: 10.5, fontWeight: '700', letterSpacing: 0.5, textTransform: 'uppercase'},
  secCount: {fontSize: 10.5, fontWeight: '700'},

  card: {marginHorizontal: 14, marginBottom: 9, borderWidth: 1, borderRadius: 12, padding: 12, gap: 5},
  cardHead: {flexDirection: 'row', alignItems: 'flex-start', gap: 10},
  grow: {flex: 1},
  cardTitle: {fontSize: 14.5, fontWeight: '600', lineHeight: 20},
  cardAge: {fontSize: 11, fontWeight: '600'},
  titleKey: {fontFamily: 'Menlo', fontSize: 10.5, lineHeight: 15, marginBottom: 1},
  titleKeyBig: {fontFamily: 'Menlo', fontSize: 12, lineHeight: 17, marginBottom: 2},
  cardWhy: {fontSize: 12.5, lineHeight: 18},
  cardTarget: {fontSize: 12},

  row: {flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 11, borderBottomWidth: StyleSheet.hairlineWidth},
  rowText: {flex: 1},
  rowTitle: {fontSize: 14, lineHeight: 19},
  rowMeta: {fontSize: 11.5, marginTop: 2},
  chev: {fontSize: 16},

  foot: {fontSize: 12, paddingHorizontal: 14, paddingTop: 16},
  emptyBox: {padding: 24},
  empty: {fontSize: 13.5, lineHeight: 20, textAlign: 'center'},
  spinner: {marginTop: 28},

  entry: {paddingHorizontal: 14, paddingTop: 14},
  entryTitle: {fontSize: 17, fontWeight: '700', lineHeight: 23},
  entryMeta: {fontSize: 11.5, marginTop: 5},
  entryBody: {marginTop: 14},
  promoBox: {marginTop: 12, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, padding: 11, gap: 4},
  promoLabel: {fontSize: 12, fontWeight: '700'},
  promoText: {fontSize: 12.5, lineHeight: 18},
  landed: {fontSize: 12.5, fontWeight: '600', marginTop: 10},

  actions: {flexDirection: 'row', gap: 10, marginTop: 20, paddingTop: 14, borderTopWidth: StyleSheet.hairlineWidth},
  action: {flex: 1, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingVertical: 11, alignItems: 'center'},
  actionText: {fontSize: 13.5, fontWeight: '600'},

  actBar: {paddingHorizontal: 14, paddingTop: 12, borderTopWidth: StyleSheet.hairlineWidth, gap: 7},
  actTitle: {flex: 1, fontSize: 14, fontWeight: '700'},
  actHint: {fontSize: 12, lineHeight: 17},
  actInput: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 9, paddingHorizontal: 11, paddingVertical: 10, fontSize: 14.5},
  actError: {fontSize: 12.5, lineHeight: 17},
  actHead: {flexDirection: 'row', alignItems: 'center', gap: 4},
  actBtn: {paddingHorizontal: 14, paddingVertical: 8},
  actBtnOff: {opacity: 0.4},
  actBtnText: {fontSize: 14.5, fontWeight: '600'},
});
