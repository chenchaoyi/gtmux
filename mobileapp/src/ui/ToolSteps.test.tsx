import React from 'react';
import { ScrollView, Text } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import renderer, { act } from 'react-test-renderer';
import { ToolSteps } from './ToolSteps';
import { CodeText } from './CodeText';
import { CodeReaderModal } from './CodeReaderModal';
import { activityOperation } from './activity';
import { TranscriptStep } from '../api/client';

let tree: renderer.ReactTestRenderer;
afterEach(() => {
  if (tree) {
    act(() => tree.unmount());
  }
  jest.clearAllMocks();
});
const command =
  "printf '%s\\n' 'literal <tag>'\n" + 'long-argument-'.repeat(100);
const steps: TranscriptStep[] = [
  {
    kind: 'tool',
    title: 'Read',
    detail: 'README.md',
    input: '/example/README.md',
    input_kind: 'path',
    operation: 'read',
  },
  {
    kind: 'tool',
    title: 'exec',
    detail: 'Run focused checks',
    input: command,
    input_kind: 'command',
    workdir: '/example/repo',
    operation: 'command',
  },
  {
    kind: 'tool',
    title: 'future_tool',
    detail: '<img src=x onerror=alert(1)>',
  },
];
function props(open: boolean) {
  return {
    steps,
    open,
    onToggle: jest.fn(),
    lang: 'en' as const,
    fontSize: 18,
  };
}
function mount(open: boolean) {
  act(() => {
    tree = renderer.create(<ToolSteps {...props(open)} />);
  });
  return tree;
}
const byID = (id: string) =>
  tree.root.findAll(
    n => typeof n.type !== 'string' && n.props.testID === id,
  )[0];
test('whole group folds to a labelled summary and unmounts all command content', () => {
  mount(false);
  const button = byID('tool-steps-toggle');
  expect(button.props.accessibilityState).toEqual({ expanded: false });
  expect(button.props.accessibilityLabel).toBe('Activity, 3 calls');
  expect(
    tree.root.findAllByType(Text).some(n => n.props.children === command),
  ).toBe(false);
  act(() => button.props.onPress());
  expect(button.props.onPress).toHaveBeenCalledTimes(1);
});
test('individual input wraps without horizontal controls and copies the complete recorded input', () => {
  mount(true);
  act(() => byID('tool-step-toggle-1').props.onPress());
  expect(tree.root.findByType(CodeText).props.source).toBe(command);
  expect(
    tree.root.findAllByType(ScrollView).every(n => !n.props.horizontal),
  ).toBe(true);
  expect(byID('tool-input-scroll-control')).toBeUndefined();
  expect(byID('tool-input-info')).toBeUndefined();
  act(() => byID('tool-input-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenCalledWith(command);
  act(() => byID('tool-input-open').props.onPress());
  expect(tree.root.findByType(CodeReaderModal).props.source).toBe(command);
  expect(tree.root.findByType(CodeReaderModal).props.fontSize).toBe(17);
  act(() =>
    tree.update(
      <ToolSteps
        {...props(true)}
        steps={steps.map((step, i) =>
          i === 1 ? { ...step, input: 'later input' } : step,
        )}
      />,
    ),
  );
  expect(tree.root.findByType(CodeReaderModal).props.source).toBe(command);
  expect(tree.root.findByType(CodeReaderModal).props.fontSize).toBe(17);
  act(() => byID('code-reader-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith(command);
  act(() => byID('code-reader-close').props.onPress());
  expect(tree.root.findAllByType(CodeReaderModal)).toHaveLength(0);
  expect(tree.root.findByType(CodeText).props.source).toBe('later input');
  act(() => tree.update(<ToolSteps {...props(false)} />));
  expect(tree.root.findAllByType(ScrollView)).toHaveLength(0);
});
test('one row stays selected as calls append, with quiet provenance instead of explanation actions', () => {
  mount(true);
  act(() => byID('tool-step-toggle-1').props.onPress());
  act(() =>
    tree.update(
      <ToolSteps
        {...props(true)}
        steps={[...steps, { kind: 'tool', title: 'wait' }]}
      />,
    ),
  );
  expect(byID('tool-step-toggle-1').props.accessibilityState.expanded).toBe(
    true,
  );
  act(() => byID('tool-step-toggle-2').props.onPress());
  expect(tree.root.findByType(CodeText).props.source).toBe(steps[2].detail);
  expect(JSON.stringify(tree.toJSON())).toContain('Summary');
  expect(byID('tool-input-info')).toBeUndefined();
});
test('limited recorded input keeps a concise partial marker and no help popup', () => {
  act(() => {
    tree = renderer.create(
      <ToolSteps
        {...props(true)}
        lang="zh"
        steps={[
          {
            kind: 'tool',
            title: 'exec',
            detail: 'short…',
            details_truncated: true,
          },
        ]}
      />,
    );
  });
  act(() => byID('tool-step-toggle-0').props.onPress());
  expect(JSON.stringify(tree.toJSON())).toContain('部分');
  expect(byID('tool-input-info')).toBeUndefined();
});
test.each([
  ['Read', 'read'],
  ['mcp__fs__list_directory', 'list'],
  ['Grep', 'search'],
  ['Bash', 'command'],
  ['apply_patch', 'edit'],
  ['web_fetch', 'web'],
  ['functions.js', 'code'],
  ['wait', 'wait'],
  ['spawn_agent', 'agent'],
  ['FutureBashLikeTool', 'tool'],
])('tool %s has a semantic icon category %s', (title, operation) => {
  expect(activityOperation({ kind: 'tool', title })).toBe(operation);
});

test('an argument-free wait is a quiet non-interactive row without empty details', () => {
  act(() => {
    tree = renderer.create(
      <ToolSteps {...props(true)} steps={[{ kind: 'tool', title: 'wait' }]} />,
    );
  });
  expect(byID('tool-step-static-0')).toBeDefined();
  expect(byID('tool-step-toggle-0')).toBeUndefined();
  expect(byID('tool-step-details')).toBeUndefined();
  expect(
    tree.root
      .findAllByType(Text)
      .some(n => n.props.children === 'No input details recorded'),
  ).toBe(false);
});
test('a wait with recorded input can still be inspected and copied', () => {
  act(() => {
    tree = renderer.create(
      <ToolSteps
        {...props(true)}
        steps={[
          {
            kind: 'tool',
            title: 'wait',
            input: '{"duration_ms":1000}',
            input_kind: 'arguments',
          },
        ]}
      />,
    );
  });
  act(() => byID('tool-step-toggle-0').props.onPress());
  act(() => byID('tool-input-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('{"duration_ms":1000}');
});

test.each(['spawn_agent', 'wait'])(
  '%s exposes recorded parameters without presenting them as shell argv',
  title => {
    const input = JSON.stringify(
      title === 'wait'
        ? { cell_id: 'example-cell', yield_time_ms: 1000 }
        : {
            task_name: 'inspect-images',
            message: 'Review synthetic images',
            fork_turns: 'none',
          },
      null,
      2,
    );
    act(() => {
      tree = renderer.create(
        <ToolSteps
          {...props(true)}
          steps={[{ kind: 'tool', title, input, input_kind: 'parameters' }]}
        />,
      );
    });
    act(() => byID('tool-step-toggle-0').props.onPress());
    expect(byID('tool-input-text').props.source).toBe(input);
    expect(
      tree.root.findAllByType(ScrollView).every(n => !n.props.horizontal),
    ).toBe(true);
    expect(JSON.stringify(tree.toJSON())).toContain('Parameters');
    expect(JSON.stringify(tree.toJSON())).not.toContain('argv (JSON)');
    act(() => byID('tool-input-copy').props.onPress());
    expect(Clipboard.setString).toHaveBeenLastCalledWith(input);
    act(() => byID('tool-step-toggle-0').props.onPress());
    expect(tree.root.findAllByType(ScrollView)).toHaveLength(0);
  },
);

test.each(['en', 'zh'] as const)(
  'parameter rows use readable %s previews while the inspector keeps exact input',
  lang => {
    const input = JSON.stringify(
      { cell_id: 'fixture-cell', yield_time_ms: 1000, max_tokens: 2000 },
      null,
      2,
    );
    act(() => {
      tree = renderer.create(
        <ToolSteps
          {...props(true)}
          lang={lang}
          steps={[
            { kind: 'tool', title: 'wait', input, input_kind: 'parameters' },
          ]}
        />,
      );
    });
    const rendered = JSON.stringify(tree.toJSON());
    expect(rendered).toContain(lang === 'zh' ? '最多 1 秒' : 'Up to 1s');
    expect(rendered).not.toContain('fixture-cell');
    const heading = tree.root
      .findAllByType(Text)
      .find(n => n.props.numberOfLines === 1)!;
    expect(heading.props.children.join('')).toBe(
      lang === 'zh' ? '工具调用 · 1' : 'Tool calls · 1',
    );
    act(() => byID('tool-step-toggle-0').props.onPress());
    expect(byID('tool-input-text').props.source).toBe(input);
    act(() => byID('tool-input-copy').props.onPress());
    expect(Clipboard.setString).toHaveBeenLastCalledWith(input);
  },
);

test('opens full input on demand and copies original while formatting only the reader', async () => {
  const full = {
    kind: 'tool',
    title: 'exec',
    input: 'const value={first:1,second:2};',
    input_kind: 'code',
    input_ref: 'ref',
  };
  const client = {
    toolInput: jest.fn().mockResolvedValue(full),
  } as unknown as import('../api/client').GtmuxClient;
  await act(async () => {
    tree = renderer.create(
      <ToolSteps
        {...props(true)}
        steps={[{ ...full, input: 'const…', details_truncated: true }]}
        client={client}
        session="recorded"
        home="home"
      />,
    );
  });
  expect(client.toolInput).not.toHaveBeenCalled();
  await act(async () => {
    byID('tool-step-toggle-0').props.onPress();
  });
  expect(client.toolInput).toHaveBeenCalledWith(
    'recorded',
    'home',
    'ref',
    expect.anything(),
  );
  expect(tree.root.findByType(CodeText).props.source).toBe(full.input);
  expect(tree.root.findByType(CodeText).props.formatted).toBe(true);
  act(() => byID('tool-input-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith(full.input);
  act(() => byID('tool-input-open').props.onPress());
  expect(byID('code-reader-format').props.accessibilityState.selected).toBe(
    true,
  );
  act(() => byID('code-reader-format').props.onPress());
  expect(byID('code-reader-format').props.accessibilityState.selected).toBe(
    false,
  );
  act(() => byID('code-reader-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith(full.input);
});


test('complete-code controls keep the shared chrome scale while the recorded source retains its chosen size', () => {
  act(() => { tree = renderer.create(<CodeReaderModal source="const value={first:1};" title="Original input" hint="javascript" lang="en" fontSize={19} onClose={jest.fn()} />); });
  const format = tree.root.findAllByProps({testID:'code-reader-format'}).find(n => typeof n.props.onPress === 'function')!;
  expect(format.findByType(Text).props.maxFontSizeMultiplier).toBe(1.35);
  expect(tree.root.findAllByProps({testID:'code-reader-text'}).find(n => n.props.source)?.props.fontSize).toBe(19);
});
