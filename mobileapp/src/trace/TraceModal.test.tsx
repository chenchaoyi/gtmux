import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { FlatList, Modal, ScrollView, StyleSheet, Text } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import { TraceModal, TraceReader } from './TraceModal';
import { TraceEvent } from './model';
import { paletteFor } from '../ui/theme';
import { toAgent } from '../api/types';
import { GtmuxClient, TranscriptTurn } from '../api/client';
import { MarkdownView } from '../ui/MarkdownView';
import { ParticipantProfileModal } from '../ui/ParticipantProfileModal';
import { nativeDocumentPreview } from '../documents/nativePreview';
import * as layout from '../ui/layout';
import {KeyBus} from '../keys/bus';
import { CodeText } from '../ui/CodeText';
jest.mock('../ui/ParticipantProfileModal', () => ({
  ParticipantProfileModal: jest.fn(() => null),
}));
jest.mock('../documents/nativePreview', () => ({
  nativeDocumentPreview: { open: jest.fn().mockResolvedValue(undefined) },
}));
const agent = toAgent({
  pane_id: '%900',
  agent: 'Codex',
  session_id: 'current',
  home_id: 'home-a',
  status: 'working',
  source: 'tmux',
  task: 'Trace QA',
});
const command = 'line one\n  line two';
const turn: TranscriptTurn = {
  session_id: 'recorded',
  prompt: 'Inspect input',
  response: 'Answer',
  agent: 'Codex',
  time: '2026-10-28T10:00:00Z',
  segments: [
    {
      text: 'Answer',
      steps: [
        {
          kind: 'tool',
          title: 'exec',
          input: command,
          input_kind: 'command',
          workdir: '/project',
          details_truncated: true,
        },
      ],
    },
  ],
};
let tree: renderer.ReactTestRenderer,
  scale = 1;
const base = {
  visible: true,
  agent,
  turns: [turn],
  loading: false,
  pal: paletteFor('light'),
  lang: 'en' as const,
  onClose: jest.fn(),
};
function press(id: string) {
  const node = tree.root
    .findAllByProps({ testID: id })
    .find(n => typeof n.props.onPress === 'function');
  expect(node).toBeDefined();
  act(() => node!.props.onPress());
}
function data(): TraceEvent[] {
  return tree.root
    .findByType(FlatList)
    .props.data.filter((row: any) => row.type === 'event')
    .map((row: any) => row.event);
}
function source() {
  return tree.root.findAllByProps({ testID: 'trace-source' })[0].props.source;
}
function width(value: number) {
  const node = tree.root.findAll(
    n =>
      typeof n.props.onLayout === 'function' &&
      Array.isArray(n.props.edges) &&
      n.props.edges.includes('bottom'),
  )[0];
  act(() =>
    node.props.onLayout({
      nativeEvent: { layout: { width: value, height: 800 } },
    }),
  );
}
function mount(props: Partial<React.ComponentProps<typeof TraceReader>> = {}) {
  act(() => {
    tree = renderer.create(<TraceReader {...base} {...props} />);
  });
}

test('session name leads the heading; each expanded call opens its own fully wrapped input', () => {
  const inputs = [
    'printf ' + 'a very long command '.repeat(50),
    'second line\n  preserved indentation',
    'third',
  ];
  mount({
    turns: [
      {
        ...turn,
        segments: [
          {
            steps: inputs.map(input => ({
              kind: 'tool',
              title: 'bash',
              input,
              input_kind: 'command',
            })),
          },
        ],
      },
    ],
  });
  expect(
    tree.root.findAllByProps({ testID: 'trace-session-title' })[0].props
      .children,
  ).toBe('Trace QA');
  const batch = tree.root
    .findByType(FlatList)
    .props.data.find((row: any) => row.type === 'calls');
  press('trace-calls-' + batch.key);
  const group = tree.root
    .findAllByProps({ testID: 'trace-calls-' + batch.key })
    .find(n => typeof n.props.onPress === 'function')!;
  expect(
    group.findAllByType(Text).some(n => n.props.children === inputs[2]),
  ).toBe(false);
  const events = data().filter(e => e.kind === 'tool');
  expect(events).toHaveLength(3);
  events.forEach((event, i) => {
    press('trace-record-' + event.key);
    const input = tree.root
      .findAllByType(CodeText)
      .find(n => n.props.testID === 'trace-source')!;
    expect(input.props.source).toBe(inputs[i]);
    expect(input.props.numberOfLines).toBeUndefined();
    expect(input.props.rangeUnbounded).not.toBe(true);
    let parent = input.parent;
    while (parent) {
      if (parent.type === ScrollView)
        expect(parent.props.horizontal).not.toBe(true);
      parent = parent.parent;
    }
    press('trace-copy');
    expect(Clipboard.setString).toHaveBeenLastCalledWith(inputs[i]);
    press('trace-detail-close');
  });
});

test('bulk disclosures survive streaming and programmatic scroll without reopening calls or turns', () => {
  const pairedCalls = [
    { kind: 'tool', title: 'exec', input: 'first' },
    { kind: 'tool', title: 'wait', input: 'second' },
  ];
  const turns = [
    {
      ...turn,
      prompt: 'Older',
      time: '2026-10-29T10:00:00Z',
      segments: [{ text: 'Update', steps: pairedCalls }],
    },
    { ...turn, segments: [{ text: 'Update', steps: pairedCalls }] },
  ];
  mount({ turns });
  press('trace-fold-turns');
  expect(data().filter(event => event.kind === 'tool')).toHaveLength(4);
  press('trace-fold-calls');
  expect(data().some(event => event.kind === 'tool')).toBe(false);
  const batches = tree.root
    .findByType(FlatList)
    .props.data.filter((row: any) => row.type === 'calls');
  expect(batches).toHaveLength(2);
  press('trace-calls-' + batches[0].key);
  expect(data().filter(event => event.kind === 'tool')).toHaveLength(2);
  press('trace-fold-turns');
  expect(data()).toHaveLength(0);
  act(() =>
    tree.root.findByType(FlatList).props.onScroll({
      nativeEvent: {
        contentOffset: { y: 0 },
        contentSize: { height: 100 },
        layoutMeasurement: { height: 800 },
      },
    }),
  );
  act(() =>
    tree.update(
      <TraceReader
        {...base}
        turns={[...turns, { ...turn, prompt: 'Newly arrived' }]}
      />,
    ),
  );
  expect(data()).toHaveLength(0);
  press('trace-fold-calls');
  expect(data()).toHaveLength(0);
  press('trace-fold-turns');
  expect(data().filter(event => event.kind === 'tool')).toHaveLength(5);
});

test('context has four detail tabs, exact raw content and frozen recorded provenance for copy', () => {
  const context = {
    title: 'Environment reminder',
    text: '<system-reminder>Use the fixture\n  keep indentation</system-reminder>',
    source: 'codex/response_item.message.user',
    time: '2026-10-30T10:01:00Z',
  };
  mount({ turns: [{ ...turn, segments: [{ context }, { text: 'Answer' }] }] });
  const event = data().find(value => value.kind === 'context')!;
  press('trace-record-' + event.key);
  press('trace-tab-raw');
  expect(source()).toBe(context.text);
  for (const tab of ['overview', 'preview', 'raw', 'origin'])
    expect(
      tree.root.findAllByProps({ testID: 'trace-tab-' + tab }).length,
    ).toBeGreaterThan(0);
  press('trace-tab-overview');
  expect(
    tree.root.findAllByProps({ testID: 'trace-overview-facts' }).length,
  ).toBeGreaterThan(0);
  press('trace-tab-origin');
  press('trace-copy');
  expect(
    JSON.parse((Clipboard.setString as jest.Mock).mock.calls.at(-1)[0]),
  ).toMatchObject({
    kind: 'context',
    record_source: context.source,
    recorded_at: context.time,
  });
  act(() =>
    tree.update(
      <TraceReader
        {...base}
        turns={[
          {
            ...turn,
            segments: [
              { context: { ...context, source: 'updated-source' } },
              { text: 'Answer' },
            ],
          },
        ]}
      />,
    ),
  );
  press('trace-copy');
  expect(
    JSON.parse((Clipboard.setString as jest.Mock).mock.calls.at(-1)[0])
      .record_source,
  ).toBe(context.source);
  press('trace-update-detail');
  press('trace-copy');
  expect(
    JSON.parse((Clipboard.setString as jest.Mock).mock.calls.at(-1)[0])
      .record_source,
  ).toBe('updated-source');
  press('trace-tab-raw');
  press('trace-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(context.text);
});
beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
  scale = 1;
  jest
    .spyOn(layout, 'useTraceSplit')
    .mockImplementation(value => layout.traceSplitFits(value, scale));
});
afterEach(() => {
  act(() => tree?.unmount());
  jest.clearAllTimers();
  jest.useRealTimers();
  jest.restoreAllMocks();
});

test('select, copy and close retain the mounted list while full tool input remains exact', () => {
  mount();
  press('trace-search-toggle');
  const list = tree.root.findByType(FlatList);
  const event = data().find(e => e.kind === 'tool')!;
  press('trace-record-' + event.key);
  expect(tree.root.findAllByProps({ testID: 'trace-search' })).toHaveLength(0);
  expect(source()).toBe(command);
  expect(
    StyleSheet.flatten(
      tree.root.findAllByProps({ testID: 'trace-detail' })[0].props.style,
    ).position,
  ).toBe('absolute');
  press('trace-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(command);
  press('trace-detail-close');
  expect(
    tree.root.findAllByProps({ testID: 'trace-search' }).length,
  ).toBeGreaterThan(0);
  expect(tree.root.findAllByProps({ testID: 'trace-detail' })).toHaveLength(0);
  expect(tree.root.findByType(FlatList)).toBe(list);
});
test('details freeze through streamed input and history prepend until an explicit refresh', () => {
  mount();
  press('trace-record-' + data().find(e => e.kind === 'tool')!.key);
  const updated = {
    ...turn,
    segments: [
      {
        text: 'Answer',
        steps: [{ ...turn.segments![0].steps![0], input: 'Updated input' }],
      },
    ],
  };
  act(() =>
    tree.update(
      <TraceReader
        {...base}
        turns={[{ ...turn, session_id: 'older', prompt: 'Earlier' }, updated]}
      />,
    ),
  );
  expect(source()).toBe(command);
  press('trace-update-detail');
  expect(source()).toBe('Updated input');
  press('trace-profile');
  expect(
    (ParticipantProfileModal as jest.Mock).mock.calls.at(-1)[0].target.turn
      .session_id,
  ).toBe('recorded');
});
test('measured width enables two columns and closing detail restores full-width records', () => {
  mount();
  width(1024);
  press('trace-record-' + data().find(e => e.kind === 'tool')!.key);
  expect(
    StyleSheet.flatten(
      tree.root.findAllByProps({ testID: 'trace-list-pane' })[0].props.style,
    ).width,
  ).toBe(320);
  expect(
    StyleSheet.flatten(
      tree.root.findAllByProps({ testID: 'trace-detail' })[0].props.style,
    ).position,
  ).not.toBe('absolute');
  press('trace-detail-close');
  expect(
    StyleSheet.flatten(
      tree.root.findAllByProps({ testID: 'trace-list-pane' })[0].props.style,
    ).width,
  ).toBeUndefined();
  press('trace-record-' + data().find(e => e.kind === 'tool')!.key);
  scale = 2;
  width(1025);
  expect(
    StyleSheet.flatten(
      tree.root.findAllByProps({ testID: 'trace-detail' })[0].props.style,
    ).position,
  ).toBe('absolute');
});
test('search covers all loaded records while the native list keeps a bounded initial render', () => {
  const turns = Array.from({ length: 2501 }, (_, i) => ({
    ...turn,
    prompt: 'Request ' + i,
    response: 'Result ' + i,
    segments: [{ text: 'Result ' + i }],
  }));
  mount({ turns });
  expect(data()).toHaveLength(2);
  expect(
    tree.root.findByProps({ testID: 'trace-count' }).props.children.join(''),
  ).toContain('5002');
  expect(
    tree.root
      .findByType(FlatList)
      .props.data.filter((row: any) => row.type === 'turn'),
  ).toHaveLength(2501);
  const rows = tree.root.findAll(
    n =>
      typeof n.props.testID === 'string' &&
      n.props.testID.startsWith('trace-record-') &&
      typeof n.props.onPress === 'function',
  );
  expect(rows.length).toBeLessThan(200);
  press('trace-search-toggle');
  const search = tree.root
    .findAllByProps({ testID: 'trace-search' })
    .find(n => typeof n.props.onChangeText === 'function')!;
  act(() => search.props.onChangeText('Result 2500'));
  expect(data()).toHaveLength(1);
  expect(data()[0].body).toBe('Result 2500');
});
test('file opening uses the selected historical session and current verified home', async () => {
  const client = {
    documentRequest: jest.fn().mockReturnValue({ reference: '/tmp/report.md' }),
  } as unknown as GtmuxClient;
  mount({ client, turns: [{ ...turn, prompt: '[Report](/tmp/report.md)' }] });
  press('trace-record-' + data()[0].key);
  await act(async () => {
    await tree.root.findByType(MarkdownView).props.openFile('/tmp/report.md');
  });
  expect(client.documentRequest).toHaveBeenCalledWith(
    '/tmp/report.md',
    'recorded',
    'home-a',
    'en',
  );
  expect(nativeDocumentPreview.open).toHaveBeenCalledTimes(1);
});
test('a failed read remains distinguishable from an empty conversation and offers retry', () => {
  const retry = jest.fn();
  mount({ turns: [], failed: true, onRetry: retry });
  expect(JSON.stringify(tree.toJSON())).toContain(
    'Could not load trace records.',
  );
  const button = tree.root.findAll(n => n.props.onPress === retry)[0];
  act(() => button.props.onPress());
  expect(retry).toHaveBeenCalledTimes(1);
});

test('changing home or losing the selected record immediately retires its profile', () => {
  mount();
  press('trace-record-' + data()[0].key);
  press('trace-profile');
  expect(tree.root.findAllByType(ParticipantProfileModal)).toHaveLength(1);
  act(() =>
    tree.update(
      <TraceReader {...base} agent={{ ...agent, home_id: 'home-b' }} />,
    ),
  );
  expect(tree.root.findAllByType(ParticipantProfileModal)).toHaveLength(0);
  expect(tree.root.findAllByProps({ testID: 'trace-detail' })).toHaveLength(0);
  press('trace-record-' + data()[0].key);
  press('trace-profile');
  act(() => tree.update(<TraceReader {...base} turns={[]} />));
  expect(tree.root.findAllByType(ParticipantProfileModal)).toHaveLength(0);
});

test('earlier-session loading is an explicit parent action and disables repeat taps', () => {
  const load = jest.fn();
  mount({ earlierAvailable: true, onLoadEarlier: load });
  press('trace-load-earlier');
  expect(load).toHaveBeenCalledTimes(1);
  act(() =>
    tree.update(
      <TraceReader
        {...base}
        earlierAvailable
        loadingEarlier
        onLoadEarlier={load}
      />,
    ),
  );
  expect(
    tree.root.findAllByProps({ testID: 'trace-load-earlier' })[0].props
      .disabled,
  ).toBe(true);
});

test('accessibility escape returns from details before closing the trace', () => {
  const close = jest.fn();
  mount({ onClose: close });
  press('trace-record-' + data()[0].key);
  const root = () =>
    tree.root.findAll(
      n =>
        typeof n.props.onAccessibilityEscape === 'function' &&
        Array.isArray(n.props.edges),
    )[0];
  act(() => root().props.onAccessibilityEscape());
  expect(tree.root.findAllByProps({ testID: 'trace-detail' })).toHaveLength(0);
  expect(close).not.toHaveBeenCalled();
  act(() => root().props.onAccessibilityEscape());
  expect(close).toHaveBeenCalledTimes(1);
});

test('legacy title-only calls copy their recorded title instead of clearing the clipboard', () => {
  mount({
    turns: [
      {
        session_id: 'legacy',
        prompt: '',
        response: '',
        segments: [{ steps: [{ kind: 'tool', title: 'Wait' }] }],
      },
    ],
  });
  press('trace-record-' + data()[0].key);
  press('trace-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith('Wait');
});

test('idle entry expands the newest turn without a follow mode switch', () => {
  mount({
    agent: { ...agent, status: 'idle' },
    turns: [
      { ...turn, prompt: 'Older' },
      { ...turn, prompt: 'Newest' },
    ],
  });
  expect(tree.root.findAllByProps({ testID: 'trace-follow' })).toHaveLength(0);
  expect(data().some(event => event.body === 'Newest')).toBe(true);
  expect(data().some(event => event.body === 'Older')).toBe(false);
  expect(
    tree.root.findAllByProps({ testID: 'trace-progress' }).length,
  ).toBeGreaterThan(0);
});

test('position controls implicit following and a reading gesture prevents queued scrolling', () => {
  const scroll = jest
    .spyOn(FlatList.prototype, 'scrollToEnd')
    .mockImplementation(() => {});
  mount();
  const list = () => tree.root.findByType(FlatList);
  act(() => {
    list().props.onContentSizeChange();
    jest.advanceTimersByTime(32);
  });
  expect(scroll).toHaveBeenCalled();
  scroll.mockClear();
  act(() => {
    list().props.onContentSizeChange();
    list().props.onScrollBeginDrag();
    jest.advanceTimersByTime(32);
  });
  expect(scroll).not.toHaveBeenCalled();
  act(() =>
    list().props.onScroll({
      nativeEvent: {
        contentOffset: { y: 100 },
        contentSize: { height: 1000 },
        layoutMeasurement: { height: 400 },
      },
    }),
  );
  act(() => list().props.onScrollEndDrag());
  act(() =>
    tree.update(
      <TraceReader
        {...base}
        turns={[turn, { ...turn, prompt: 'New incoming request' }]}
      />,
    ),
  );
  act(() => {
    list().props.onContentSizeChange();
    jest.advanceTimersByTime(32);
  });
  expect(scroll).not.toHaveBeenCalled();
  expect(data().some(event => event.body === 'New incoming request')).toBe(
    false,
  );
  press('trace-latest');
  act(() => {
    list().props.onContentSizeChange();
    jest.advanceTimersByTime(32);
  });
  expect(scroll).toHaveBeenCalled();
  expect(data().some(event => event.body === 'New incoming request')).toBe(
    true,
  );
  scroll.mockClear();
  act(() => list().props.onScrollBeginDrag());
  act(() =>
    list().props.onScroll({
      nativeEvent: {
        contentOffset: { y: 598 },
        contentSize: { height: 1000 },
        layoutMeasurement: { height: 400 },
      },
    }),
  );
  act(() => list().props.onScrollEndDrag());
  act(() => {
    list().props.onContentSizeChange();
    jest.advanceTimersByTime(32);
  });
  expect(scroll).toHaveBeenCalled();
});

test('Preview uses shared syntax rendering and has no reading-explanation footer', () => {
  mount({
    turns: [
      {
        ...turn,
        segments: [
          {
            steps: [
              {
                kind: 'tool',
                title: 'exec',
                input: 'const answer = "value";',
                input_kind: 'code',
              },
            ],
          },
        ],
      },
    ],
  });
  const event = data().find(e => e.kind === 'tool')!;
  press('trace-record-' + event.key);
  press('trace-tab-preview');
  expect(
    tree.root
      .findAllByType(CodeText)
      .some(n => n.props.source === 'const answer = "value";'),
  ).toBe(true);
  expect(tree.root.findAllByProps({ testID: 'trace-limits' })).toHaveLength(0);
  expect(
    tree.root.findAllByProps({ testID: 'trace-limits-content' }),
  ).toHaveLength(0);
});

test('truncated Trace input loads its complete source and keeps Raw and Copy verbatim', async () => {
  const full = {
    kind: 'tool',
    title: 'exec',
    input: 'const result={first:1,second:2};',
    input_kind: 'code',
    input_ref: 'trace-full',
  };
  const client = {
    toolInput: jest.fn().mockResolvedValue(full),
  } as unknown as GtmuxClient;
  const target = {
    ...turn,
    segments: [
      { steps: [{ ...full, input: 'const…', details_truncated: true }] },
    ],
  };
  mount({ client, turns: [target] });
  const batch = tree.root
    .findByType(FlatList)
    .props.data.find((r: any) => r.type === 'calls');
  if (batch) press('trace-calls-' + batch.key);
  const event = data().find(e => e.kind === 'tool')!;
  await act(async () => {
    press('trace-record-' + event.key);
  });
  expect(source()).toBe(full.input);
  expect(client.toolInput).toHaveBeenCalledWith(
    'recorded',
    'home-a',
    'trace-full',
    expect.anything(),
  );
  expect(
    tree.root.findAllByProps({ testID: 'trace-detail-update' }),
  ).toHaveLength(0);
  press('trace-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(full.input);
});

test('detail navigation returns to the list while the single close exits Trace', () => {
  const close = jest.fn();
  mount({ onClose: close });
  const event = data().find(e => e.kind === 'tool')!;
  press('trace-record-' + event.key);
  const back = tree.root
    .findAllByProps({ testID: 'trace-detail-close' })
    .find(n => typeof n.props.onPress === 'function')!;
  expect(back.props.accessibilityLabel).toBe('Back to trace');
  press('trace-detail-close');
  expect(close).not.toHaveBeenCalled();
  expect(tree.root.findAllByProps({ testID: 'trace-detail' })).toHaveLength(0);
  press('trace-close');
  expect(close).toHaveBeenCalledTimes(1);
});
test('iOS dismissal retains the painted Trace tree until onDismiss, then releases it', () => {
  act(() => {
    tree = renderer.create(<TraceModal {...base} />);
  });
  expect(tree.root.findByType(Modal).props.backdropColor).toBe(base.pal.bg);
  act(() => tree.update(<TraceModal {...base} visible={false} />));
  expect(tree.root.findByType(Modal).props.children.props.children.type).toBe(TraceReader);
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(tree.root.findByType(Modal).props.children).toBe(false);
});
test('closing keeps current scope retirement and a stale dismiss cannot blank a reopened trace', () => {
  act(() => {
    tree = renderer.create(<TraceModal {...base} />);
  });
  act(() => tree.update(<TraceModal {...base} turns={[]} visible={false} />));
  expect(tree.root.findByType(Modal).props.children.props.children.props.turns).toEqual([]);
  const dismiss = tree.root.findByType(Modal).props.onDismiss;
  act(() => tree.update(<TraceModal {...base} visible />));
  act(() => dismiss());
  expect(tree.root.findAllByType(TraceReader)).toHaveLength(1);
});


test('iPad resizing and Dynamic Type preserve the selected original call with the appropriate navigation', () => {
  mount();
  width(1194);
  const event = data().find(e => e.kind === 'tool')!;
  press('trace-record-' + event.key);
  const navigation = () => tree.root.findAllByProps({testID: 'trace-detail-close'}).find(n => typeof n.props.onPress === 'function')!;
  expect(navigation().props.accessibilityLabel).toBe('Hide details');
  expect(source()).toBe(command);
  for (const w of [834, 744, 507, 320, 1366]) {
    width(w);
    expect(source()).toBe(command);
    expect(navigation().props.accessibilityLabel).toBe(w >= 820 ? 'Hide details' : 'Back to trace');
  }
  scale = 3;
  act(() => tree.update(<TraceReader {...base} />));
  expect(navigation().props.accessibilityLabel).toBe('Back to trace');
  expect(source()).toBe(command);
  press('trace-detail-close');
  expect(base.onClose).not.toHaveBeenCalled();
});


test('hardware Escape returns from call details before closing Trace and does not reach the page behind', () => {
  const page = jest.fn(), close = jest.fn();
  const stop = KeyBus.on('sheet.close', page);
  mount({onClose: close});
  press('trace-record-' + data().find(e => e.kind === 'tool')!.key);
  act(() => { KeyBus.emit('sheet.close'); });
  expect(tree.root.findAllByProps({testID:'trace-detail'})).toHaveLength(0);
  expect(close).not.toHaveBeenCalled();
  expect(page).not.toHaveBeenCalled();
  act(() => { KeyBus.emit('sheet.close'); });
  expect(close).toHaveBeenCalledTimes(1);
  expect(page).not.toHaveBeenCalled();
  stop();
});
