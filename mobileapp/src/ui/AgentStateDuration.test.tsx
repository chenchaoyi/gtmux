import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {AgentRow} from './AgentRow';
import {paletteFor} from './theme';
import {toAgent} from '../api/types';
import {buildRowSheet} from './rowSheetModel';

test('CSR-004 real decoded rows hide explicit uncertain duration and retain legacy clocks', () => {
  const now = 1_800_000_000;
  jest.spyOn(Date, 'now').mockReturnValue(now * 1000);
  const trees: renderer.ReactTestRenderer[] = [];
  const render = (fields: Record<string, unknown>) => {
    const agent = toAgent({pane_id: '%17', agent: 'Codex', status: 'working',
      task: 'Synthetic work', activity_at: now, ...fields});
    let tree: renderer.ReactTestRenderer | undefined;
    act(() => { tree = renderer.create(
      <AgentRow agent={agent} pal={paletteFor('light')} lang="en" onPress={() => {}} />,
    ); });
    trees.push(tree!);
    return tree!.root.findAllByType(Text).flatMap(n => ([] as unknown[])
      .concat(n.props.children as unknown[])).filter(c => typeof c === 'string');
  };
  try {
    const unknown = toAgent({pane_id: '%17', agent: 'Codex', status: 'working',
      since: now - 17 * 3600, activity_at: now, since_uncertain: true});
    expect(buildRowSheet(unknown, 'en', now).status).not.toContain('17h');
    expect(render({since_uncertain: true, since: now - 17 * 3600})).not.toContain('17h');
    expect(render({since_uncertain: true, since: 0})).not.toContain('0s');
    expect(render({since_uncertain: true, status: 'idle', since: now - 60})).not.toContain('1m');
    expect(render({since: now - 60})).toContain('1m');
    expect(render({since: 0})).toContain('0s');
  } finally {
    act(() => trees.forEach(tree => tree.unmount()));
    jest.restoreAllMocks();
  }
});
