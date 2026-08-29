import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentAvatar} from './AgentAvatar';
import {Agent} from '../api/types';

// Regression: AgentAvatar must render OUTSIDE an AgentsProvider — the pre-pairing
// Demo screen (the App Store reviewer's fallback) reuses the radar row, which
// renders AgentAvatar with no provider. Before the useAgentsOptional fix this
// threw "useAgents must be used within AgentsProvider" and red-screened the Demo.
test('renders without an AgentsProvider (Demo screen path)', () => {
  const agent = {agent: 'Claude Code'} as Agent;
  let tree: renderer.ReactTestRenderer | undefined;
  // The bug was a thrown "useAgents must be used within AgentsProvider".
  expect(() => {
    act(() => {
      tree = renderer.create(
        <AgentAvatar agent={agent} size={40} radius={10} bg="#fff" fg="#000" />,
      );
    });
  }).not.toThrow();
  expect(tree!.toJSON()).toBeTruthy();
});

// The icon URI is per AGENT, not per row: every Claude row asks for the same one. So a
// single row showing the monogram while its neighbours show the icon is never "this
// agent has no icon" — it is one request that did not land. Measured 2026-08-29 over
// 4G: 14 Claude rows, identical `icon` hints from the Mac, one monogram. The failure
// was latched per component instance and never retried, so that row stayed wrong for
// as long as it stayed mounted.
jest.mock('../state/AgentsContext', () => ({
  useAgentsOptional: () => ({
    client: {iconUri: (name: string) => ({uri: 'https://mac/api/icon?agent=' + name, headers: {}})},
  }),
}));

const withIcon = {agent: 'Claude Code', icon: '/Applications/Claude.app'} as Agent;

function images(tree: renderer.ReactTestRenderer) {
  return tree.root.findAll(n => typeof n.type !== 'string' && (n.type as {displayName?: string}).displayName === 'Image');
}
function monograms(tree: renderer.ReactTestRenderer) {
  return tree.root.findAll(n => typeof n.type !== 'string' && (n.type as {displayName?: string}).displayName === 'Text');
}
// Tolerant on purpose: a test must fail on its assertion, not on a TypeError from an
// <Image> that is no longer mounted.
function fail(tree: renderer.ReactTestRenderer) {
  const img = images(tree)[0];
  if (!img) {
    return;
  }
  act(() => {
    img.props.onError();
  });
}
function mount(a: Agent) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<AgentAvatar agent={a} size={40} radius={10} bg="#fff" fg="#000" />);
  });
  return tree;
}

test('a failed icon request is retried, not treated as a verdict', () => {
  jest.useFakeTimers();
  const tree = mount(withIcon);
  expect(images(tree)).toHaveLength(1);

  fail(tree);
  // The monogram stands in immediately — nothing flickers into a blank box…
  expect(monograms(tree).length).toBeGreaterThan(0);
  // …and the request comes back on its own.
  act(() => {
    jest.advanceTimersByTime(1000);
  });
  expect(images(tree)).toHaveLength(1);
  expect(monograms(tree)).toHaveLength(0);
  jest.useRealTimers();
});

// This one guards the RETRY, not the original bug: a self-healing request that never
// stops asking would be a worse defect than the one it replaced. It passes under the
// old latch too, which is correct — the regression tests for that are the other two.
test('an icon that never loads stops asking and settles on the monogram', () => {
  jest.useFakeTimers();
  const tree = mount(withIcon);
  for (let i = 0; i < 4; i++) {
    fail(tree);
    act(() => {
      jest.advanceTimersByTime(10000);
    });
  }
  expect(images(tree)).toHaveLength(0);
  expect(monograms(tree)).toHaveLength(1);
  // And it is not still retrying in the background.
  act(() => {
    jest.advanceTimersByTime(60000);
  });
  expect(images(tree)).toHaveLength(0);
  jest.useRealTimers();
});

test('a different agent is a fresh question, not the previous verdict', () => {
  jest.useFakeTimers();
  const tree = mount(withIcon);
  for (let i = 0; i < 4; i++) {
    fail(tree);
    act(() => {
      jest.advanceTimersByTime(10000);
    });
  }
  expect(images(tree)).toHaveLength(0);

  const other = {agent: 'Codex', icon: '/x/codex.png'} as Agent;
  act(() => {
    tree.update(<AgentAvatar agent={other} size={40} radius={10} bg="#fff" fg="#000" />);
  });
  expect(images(tree)).toHaveLength(1);
  jest.useRealTimers();
});
