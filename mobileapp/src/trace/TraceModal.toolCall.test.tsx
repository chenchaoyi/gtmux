import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { FlatList } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import { TraceReader } from './TraceModal';
import { ToolSteps } from '../ui/ToolSteps';
import { CodeBlock } from '../ui/CodeBlock';
import { paletteFor } from '../ui/theme';
import { toAgent } from '../api/types';
import type {
  GtmuxClient,
  TranscriptStep,
  TranscriptTurn,
} from '../api/client';
const step: TranscriptStep = {
  kind: 'tool',
  title: 'github.fetch_commit_workflow_runs',
  operation: 'tool',
  input_ref: 'native-ref',
  call: {
    name: 'github.fetch_commit_workflow_runs',
    provider: 'GitHub',
    status: 'completed',
    duration_ms: 42,
    has_arguments: true,
    has_result: true,
    ref: 'native-ref',
  },
};
const full: TranscriptStep = {
  ...step,
  call: {
    ...step.call,
    loaded: true,
    arguments: '{"repo_full_name":"example/project","commit_sha":"abc123"}',
    result:
      '{"structuredContent":{"workflow_runs":[{"conclusion":"success"}]}}',
  },
};
const turns: TranscriptTurn[] = [
  {
    session_id: 'recorded',
    prompt: 'Inspect runs',
    response: 'Done',
    segments: [{ steps: [step] }],
  },
];
const agent = toAgent({
  pane_id: '%900',
  agent: 'Codex',
  session_id: 'recorded',
  home_id: 'home-a',
  status: 'idle',
  source: 'tmux',
  task: 'Trace QA',
});
let tree: renderer.ReactTestRenderer;
function press(id: string) {
  const n = tree.root
    .findAllByProps({ testID: id })
    .find(v => typeof v.props.onPress === 'function');
  expect(n).toBeDefined();
  act(() => n!.props.onPress());
}
function text() {
  const visit = (value: any): string =>
    typeof value === 'string'
      ? value
      : Array.isArray(value)
      ? value.map(visit).join(' ')
      : value && value.children
      ? visit(value.children)
      : '';
  return visit(tree.toJSON());
}
afterEach(() => {
  if (tree) act(() => tree.unmount());
  jest.clearAllMocks();
});
async function open(client: GtmuxClient) {
  await act(async () => {
    tree = renderer.create(
      <TraceReader
        visible
        agent={agent}
        client={client}
        turns={turns}
        loading={false}
        pal={paletteFor('light')}
        lang="en"
        onClose={() => {}}
      />,
    );
  });
  press('trace-fold-turns');
  const data = tree.root.findByType(FlatList).props.data;
  const batch = data.find((r: any) => r.type === 'calls');
  if (batch) press('trace-calls-' + batch.key);
  const row = tree.root
    .findByType(FlatList)
    .props.data.find((r: any) => r.type === 'event' && r.event.kind === 'tool');
  await act(async () => press('trace-record-' + row.event.key));
}
test('Trace loads recorded query details and both Preview and Raw are useful', async () => {
  const client = {
    toolInput: jest.fn().mockResolvedValue(full),
  } as unknown as GtmuxClient;
  await open(client);
  expect(client.toolInput).toHaveBeenCalledWith(
    'recorded',
    'home-a',
    'native-ref',
    expect.anything(),
  );
  expect(text()).toContain('GitHub · Fetch commit workflow runs');
  expect(text()).not.toContain('Only a call summary');
  press('trace-tab-preview');
  expect(text()).toContain('example/project');
  expect(
    tree.root
      .findAllByType(CodeBlock)
      .some(n => n.props.source.includes('success')),
  ).toBe(true);
  press('trace-tab-raw');
  expect(
    tree.root
      .findAllByType(CodeBlock)
      .some(
        n =>
          n.props.source.includes('commit_sha') &&
          n.props.source.includes('structuredContent'),
      ),
  ).toBe(true);
  press('trace-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(
    expect.stringContaining('abc123'),
  );
});
test('Chat reuses the same query details, failure retries and late reads retire on home change', async () => {
  let resolve!: (v: TranscriptStep) => void;
  const client = {
    toolInput: jest
      .fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockImplementationOnce(
        () =>
          new Promise(r => {
            resolve = r;
          }),
      ),
  } as unknown as GtmuxClient;
  await act(async () => {
    tree = renderer.create(
      <ToolSteps
        steps={[step]}
        client={client}
        session="recorded"
        home="home-a"
        open
        onToggle={() => {}}
        lang="en"
        fontSize={15}
      />,
    );
  });
  await act(async () => press('tool-step-toggle-0'));
  expect(text()).toContain('Could not load');
  await act(async () => press('tool-input-retry'));
  await act(async () =>
    tree.update(
      <ToolSteps
        steps={[step]}
        client={client}
        session="recorded"
        home="home-b"
        open
        onToggle={() => {}}
        lang="en"
        fontSize={15}
      />,
    ),
  );
  await act(async () => resolve(full));
  expect(text()).not.toContain('example/project');
});
