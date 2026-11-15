import React from 'react';
import {AccessibilityInfo, Linking, Modal, ScrollView, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {GtmuxClient, type KnowledgeAct, type KnowledgeEntry} from '../api/client';
import {AnchoredMenu} from '../ui/AnchoredMenu';
import {MarkdownView} from '../ui/MarkdownView';
import {paletteFor} from '../ui/theme';
import {KnowledgeSheet, type KnowledgeSheetProps} from './KnowledgeSheet';

jest.setTimeout(30_000);
const NOW = 1_800_000_000;
const LEAD = 'Resident memory is not a disk estimate.\n\n';
const DETAIL = 'Check the actual resource before acting.\n\n';
const HISTORY = '## History (retained context)\nAn older observation must remain available.';
const entry = (overrides: Partial<KnowledgeEntry> = {}): KnowledgeEntry => ({id: 'pitfalls/one', topic: 'pitfalls', title: 'Resource guidance', at: NOW - 60, revision: 'R0', body: LEAD + DETAIL + HISTORY, ...overrides});
let tree: renderer.ReactTestRenderer;
let props: KnowledgeSheetProps;
let records: jest.Mock;
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const words = () => tree.root.findAllByType(Text).map(content).join('|');
const button = (id: string) => tree.root.findAll(n => n.props.testID === id && n.type === TouchableOpacity)[0];
const flush = async () => {await act(async () => {for (let i = 0; i < 18; i++) await Promise.resolve();});};
const press = async (id: string) => {await act(async () => {button(id).props.onPress();}); await flush();};
const sources = () => tree.root.findAllByType(MarkdownView).map(n => n.props.source as string);
const mount = async (extra: Partial<KnowledgeSheetProps> = {}) => {
  const first = entry({audience: 'everyone', promoted_at: NOW - 30, issue_url: 'https://example.invalid/issues/new?title=Synthetic'});
  records = jest.fn(async (_a: KnowledgeAct) => ({ok: true as const}));
  props = {visible: true, index: {entries: [first], topics: [{name: 'pitfalls', count: 1}], promotions: {pending: 1}, candidates: {pending: 0}}, nowSecs: NOW,
    pal: paletteFor('light'), zh: false, onClose: jest.fn(), readScope: {}, openAt: {id: first.id, at: 1},
    loadEntry: async id => props.index.entries.find(e => e.id === id) ?? null, act: records, ...extra};
  await act(async () => {tree = renderer.create(<KnowledgeSheet {...props} />);}); await flush();
};
const update = async (extra: Partial<KnowledgeSheetProps>) => {
  props = {...props, ...extra}; await act(async () => {tree.update(<KnowledgeSheet {...props} />);}); await flush();
};
const openMenu = async () => {
  await press('knowledge-entry-more');
  const menu = tree.root.findByType(AnchoredMenu);
  expect(menu.props.visible).toBe(true);
  const box = menu.findAll(n => n.props.testID === 'knowledge-act' && typeof n.props.onLayout === 'function')[0];
  await act(async () => {box.props.onLayout({nativeEvent: {layout: {height: 220}}}); jest.advanceTimersByTime(150);});
  return menu;
};
const secondary = async (kind: string, beforeDismiss?: () => void) => {
  const menu = await openMenu(); await press(`knowledge-act-${kind}`);
  beforeDismiss?.(); await act(async () => {menu.findByType(Modal).props.onDismiss();}); await flush();
};
beforeEach(() => {jest.useFakeTimers(); jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);});
afterEach(async () => {await act(async () => {tree?.unmount(); for (let i = 0; i < 18; i++) await Promise.resolve();}); tree = undefined as any; jest.useRealTimers(); jest.restoreAllMocks();});

for (const layout of ['compact', 'regular'] as const) for (const theme of ['light', 'dark']) {
  test(`reading hierarchy keeps original guidance first in ${layout} ${theme}`, async () => {
    await mount({layout, pal: paletteFor(theme)});
    expect(sources()).toEqual([LEAD, DETAIL]);
    expect(button('knowledge-history-details').props.accessibilityState.expanded).toBe(false);
    expect(button('knowledge-record-details').props.accessibilityState.expanded).toBe(false);
    expect(tree.root.findAllByProps({testID: 'knowledge-axes'})).toHaveLength(0);
    const primary = button('knowledge-act-feedback'); expect(primary).toBeDefined();
    for (let parent = primary.parent; parent; parent = parent.parent) expect(parent.type).not.toBe(ScrollView);
    await press('knowledge-history-details'); expect(sources()).toEqual([LEAD, DETAIL, HISTORY]);
    await press('knowledge-body-details'); expect(sources()).toEqual([LEAD, HISTORY]);
    await press('knowledge-record-details'); expect(words()).toContain('pitfalls/one'); expect(words()).toContain('does not publish');
  });
}

test('another entry resets disclosures and resolves its actual language before reading', async () => {
  await mount(); await press('knowledge-history-details'); await press('knowledge-record-details');
  const next = entry({id: 'pitfalls/two', lang: 'zh', body: '原文建议。\n\n## 历史记录\n原文历史。', alt_lang: 'en', alt_title: 'Translated title', alt_body: 'Translated guidance.\n\n## History\nTranslated history.'});
  await update({index: {...props.index, entries: [...props.index.entries, next]}, openAt: {id: next.id, at: 2}});
  expect(sources()).toEqual(['Translated guidance.\n\n']);
  expect(button('knowledge-history-details').props.accessibilityState.expanded).toBe(false);
  expect(button('knowledge-record-details').props.accessibilityState.expanded).toBe(false);
  await update({zh: true}); expect(sources()).toEqual(['原文建议。\n\n']);
});

test('feedback opens only its existing draft while secondary landing retains the viewed revision', async () => {
  await mount(); const viewed = props.index.entries[0];
  const open = jest.spyOn(Linking, 'openURL').mockResolvedValue(true);
  await press('knowledge-act-feedback'); expect(open).toHaveBeenCalledWith(viewed.issue_url); expect(records).not.toHaveBeenCalled();
  await secondary('land', () => {viewed.revision = 'R1'; expect(records).not.toHaveBeenCalled();});
  await act(async () => {tree.root.findAllByProps({testID: 'knowledge-act-input'})[0].props.onChangeText('https://example.invalid/issues/1');});
  await press('knowledge-act-submit');
  expect(records).toHaveBeenCalledWith({op: 'land', id: viewed.id, revision: 'R0', ref: 'https://example.invalid/issues/1'});
});

test('a delayed secondary action cannot reopen a form after the source scope changes', async () => {
  await mount(); const menu = await openMenu(); await press('knowledge-act-retire');
  const dismiss = menu.findByType(Modal).props.onDismiss;
  await update({readScope: {}});
  await act(async () => {dismiss();}); await flush();
  expect(records).not.toHaveBeenCalled(); expect(tree.root.findAllByProps({testID: 'knowledge-act-bar'})).toHaveLength(0);
});

test('a captured submit callback cannot mutate after the user changes source scope', async () => {
  const first = entry({audience: 'hq', promoted_at: NOW - 30});
  await mount({index: {entries: [first], topics: [], promotions: {pending: 1}, candidates: {pending: 0}}});
  await press('knowledge-act-carry'); const submit = button('knowledge-act-submit').props.onPress;
  await update({readScope: {}}); await act(async () => {await submit();});
  expect(records).not.toHaveBeenCalled();
});

test('late entry A cannot replace entry B and wrong identities remain unavailable', async () => {
  const a = entry({body: 'Original A'}), b = entry({id: 'pitfalls/two', body: 'Current B'});
  let resolveA!: (e: KnowledgeEntry) => void;
  const loader = jest.fn((id: string) => id === a.id ? new Promise<KnowledgeEntry>(resolve => {resolveA = resolve;}) : Promise.resolve(b));
  await mount({index: {entries: [a, b], topics: [], promotions: {pending: 0}, candidates: {pending: 0}}, loadEntry: loader});
  expect(words()).toContain('Loading entry');
  await update({openAt: {id: b.id, at: 2}}); expect(sources()).toEqual(['Current B']);
  await act(async () => {resolveA(a);}); await flush(); expect(sources()).toEqual(['Current B']);
  await update({openAt: {id: 'pitfalls/wrong', at: 3}}); expect(sources()).toEqual([]); expect(button('knowledge-entry-retry')).toBeDefined();
});

test('whole-read timeout has immediate retry feedback and ignores the timed-out response', async () => {
  let resolveOld!: (e: KnowledgeEntry) => void, resolveRetry!: (e: KnowledgeEntry) => void;
  const loader = jest.fn().mockImplementationOnce(() => new Promise(resolve => {resolveOld = resolve;})).mockImplementationOnce(() => new Promise(resolve => {resolveRetry = resolve;}));
  await mount({loadEntry: loader});
  await act(async () => {jest.advanceTimersByTime(15_000);}); await flush(); expect(words()).toContain('Loading timed out');
  await press('knowledge-entry-retry'); expect(words()).toContain('Loading entry');
  await act(async () => {resolveOld(entry({body: 'Late body'}));}); await flush(); expect(sources()).toEqual([]);
  await act(async () => {resolveRetry(entry({body: 'Fresh body'}));}); await flush(); expect(sources()).toEqual(['Fresh body']);
});

test('failed post-action refresh keeps readable content and never repeats the action', async () => {
  const first = entry({audience: 'hq', promoted_at: NOW - 30});
  const loader = jest.fn().mockResolvedValueOnce(first).mockRejectedValueOnce(new Error('synthetic offline')).mockResolvedValueOnce(null).mockResolvedValueOnce({...first, body: 'Fresh content'});
  await mount({loadEntry: loader}); await press('knowledge-act-carry'); await press('knowledge-act-submit');
  expect(sources()).toEqual([LEAD, DETAIL]); expect(words()).toContain('Content could not be refreshed'); expect(records).toHaveBeenCalledTimes(1);
  await press('knowledge-entry-retry'); expect(sources()).toEqual([LEAD, DETAIL]); expect(words()).toContain('Content could not be refreshed'); expect(records).toHaveBeenCalledTimes(1);
  await press('knowledge-entry-retry'); expect(sources()).toEqual(['Fresh content']); expect(records).toHaveBeenCalledTimes(1);
});

test('the real client forwards cancellation and the whole JSON-body read has a deadline', async () => {
  let finishBody!: (e: KnowledgeEntry) => void;
  const originalFetch = globalThis.fetch;
  const fetcher = jest.fn(async () => ({ok: true, status: 200, headers: {get: () => null}, json: () => new Promise<KnowledgeEntry>(resolve => {finishBody = resolve;})}));
  (globalThis as any).fetch = fetcher;
  try {
    const client = new GtmuxClient('https://fixture.invalid', 'synthetic-only-token');
    await mount({readScope: client, loadEntry: (id, signal) => client.hqKnowledgeEntry(id, signal)});
    expect(fetcher).toHaveBeenCalledTimes(1);
    const request = (fetcher.mock.calls[0] as unknown as [string, RequestInit])[1];
    expect(request.signal?.aborted).toBe(false);
    await act(async () => {jest.advanceTimersByTime(15_000);}); await flush();
    expect(request.signal?.aborted).toBe(true); expect(words()).toContain('Loading timed out');
    await act(async () => {finishBody(entry({body: 'Late client body'}));}); await flush();
    expect(sources()).toEqual([]);
  } finally {(globalThis as any).fetch = originalFetch;}
});

test('a completed old action cannot replace the newly selected entry', async () => {
  const a = entry({audience: 'hq', promoted_at: NOW - 30});
  const b = entry({id: 'pitfalls/two', body: 'Current entry B'});
  let complete!: (r: {ok: true}) => void;
  const change = jest.fn(() => new Promise<{ok: true}>(resolve => {complete = resolve;}));
  const loader = jest.fn(async (id: string) => id === a.id ? a : b);
  await mount({loadEntry: loader, act: change, index: {entries: [a, b], topics: [], promotions: {pending: 1}, candidates: {pending: 0}}});
  await press('knowledge-act-carry'); await press('knowledge-act-submit'); expect(change).toHaveBeenCalledTimes(1);
  await update({openAt: {id: b.id, at: 2}}); expect(sources()).toEqual(['Current entry B']);
  await act(async () => {complete({ok: true});}); await flush();
  expect(sources()).toEqual(['Current entry B']); expect(loader.mock.calls.map(c => c[0])).toEqual([a.id, b.id]);
});

test('an entry without body content is distinct from a failed record read', async () => {
  await mount({loadEntry: async () => entry({body: ''})});
  expect(words()).toContain('No content to display'); expect(sources()).toEqual([]);
  expect(tree.root.findAllByProps({testID: 'knowledge-entry-retry'})).toHaveLength(0);
});
