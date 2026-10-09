import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {Agent} from '../api/types';
import {WorkspaceProvider, initialSelection, routeFor, useWorkspace} from './WorkspaceContext';

// What is open is workspace state (ipad-universal-app D4): the compact shell turns a
// selection into navigation, the regular shell keeps it for the main pane, and the caller
// of `select` never knows which.

const agent = (id: string): Agent =>
  ({pane_id: id, session: 's', window: '0', pane: '0', loc: `s:0.${id}`, agent: 'Claude Code',
    status: 'idle', task: '', latest: false, activity: false, source: 'tmux'} as Agent);

function Probe({onReady}: {onReady: (w: ReturnType<typeof useWorkspace>) => void}) {
  const w = useWorkspace();
  onReady(w);
  return <Text>{w.selection?.kind ?? 'none'}</Text>;
}

function mount(mode: 'compact' | 'regular') {
  const navigate = jest.fn();
  let ws: ReturnType<typeof useWorkspace> | undefined;
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <WorkspaceProvider mode={mode} navigate={navigate}>
        <Probe onReady={w => (ws = w)} />
      </WorkspaceProvider>,
    );
  });
  return {navigate, ws: () => ws!, text: () => tree.root.findByType(Text).props.children};
}

test('routeFor spells the phone routes once', () => {
  expect(routeFor({kind: 'pane', agent: agent('%1'), openDiff: true})[0]).toBe('Detail');
  expect(routeFor({kind: 'pane', agent: agent('%1'), openDiff: true})[1]).toMatchObject({openDiff: true});
  expect(routeFor({kind: 'hq', agent: agent('%6'), prefill: '%1 '})).toEqual(['HQ', {agent: agent('%6'), prefill: '%1 ', openResource: undefined}]);
  expect(routeFor({kind: 'panes'})[0]).toBe('Panes');
});

test('the iPad starts in a worker pane and never opens HQ as generic detail', () => {
  const hq = {...agent('%1'), role: 'supervisor'};
  const worker = agent('%7');
  expect(initialSelection([hq, worker])).toEqual({kind: 'pane', agent: worker});
  expect(initialSelection([hq])).toEqual({kind: 'hq', agent: hq});
  expect(initialSelection([])).toBeNull();
});

test('the compact shell navigates, and keeps the selection too', () => {
  const m = mount('compact');
  act(() => m.ws().select({kind: 'pane', agent: agent('%1')}));
  expect(m.navigate).toHaveBeenCalledWith('Detail', expect.objectContaining({agent: agent('%1')}));
  expect(m.text()).toBe('pane'); // a rotation into the regular shell lands on it
});

test('the regular shell only records the selection', () => {
  const m = mount('regular');
  act(() => m.ws().select({kind: 'hq', agent: agent('%6')}));
  expect(m.navigate).not.toHaveBeenCalled();
  expect(m.text()).toBe('hq');
  act(() => m.ws().select({kind: 'panes'}));
  expect(m.text()).toBe('panes');
});


test.each(['usage', 'knowledge'] as const)('HQ %s shortcut keeps the current identity and destination in compact navigation', page => {
  const hq = agent('%hq'); const openResource = {page, at: 42};
  expect(routeFor({kind: 'hq', agent: hq, openResource})).toEqual(['HQ', {agent: hq, prefill: undefined, openResource}]);
});
