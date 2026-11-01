import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { Text, TouchableOpacity } from 'react-native';
import type { ResourceReport } from '../api/client';
import { demoHQWorkspace } from '../ui/demoHQWorkspace';
import { DisclosureChevron } from '../ui/DisclosureChevron';
import { SIcon } from '../ui/SettingsIcons';
import { ERRORED_COLOR, StatusColor, paletteFor } from '../ui/theme';
import { HQResourceCards } from './HQResourceCards';
import type { HQResourceReadings } from './useHQResourceSummaries';

let tree: renderer.ReactTestRenderer;
const open = jest.fn();
const text = (node: renderer.ReactTestInstance) =>
  node
    .findAllByType(Text)
    .map(n => n.props.children)
    .flat()
    .join(' ');
function mount(
  machine?: ResourceReport['machine'],
  failed = false,
  zh = false,
) {
  const readings: HQResourceReadings = {
    board: {
      value: {
        exists: true,
        updated_at: 1000,
        text: '## Current focus\nVerify compatibility before merging.',
      },
    },
    knowledge: {},
    usage: { value: { resource: { machine } }, failed },
  };
  act(() => {
    tree = renderer.create(
      <HQResourceCards
        data={demoHQWorkspace(zh)}
        readings={readings}
        pal={paletteFor('dark')}
        zh={zh}
        wide={false}
        width={390}
        onOpen={open}
      />,
    );
  });
  return tree.root.findAllByProps({ testID: 'hq-resource-board' })[0];
}
afterEach(() => {
  act(() => tree?.unmount());
  open.mockClear();
});

test('Board is a static briefing while Knowledge and Usage retain their own destinations', () => {
  const board = mount();
  expect(text(board)).toContain('Verify compatibility before merging.');
  expect(text(board)).toContain('1 needs input');
  expect(board.props.onPress).toBeUndefined();
  expect(board.props.accessibilityRole).not.toBe('button');
  expect(board.findAllByType(TouchableOpacity)).toHaveLength(0);
  expect(board.findAllByType(DisclosureChevron)).toHaveLength(0);
  for (const key of ['knowledge', 'usage']) {
    const card = tree.root
      .findAllByProps({ testID: 'hq-resource-' + key })
      .find(n => typeof n.props.onPress === 'function')!;
    act(() => card.props.onPress());
    expect(open).toHaveBeenLastCalledWith(key);
  }
});

test.each([
  ['amber', ERRORED_COLOR],
  ['red', StatusColor.waiting],
] as const)(
  'machine %s warning is visible in Board rather than the token card',
  (tier, color) => {
    const board = mount({ tier, warn_key: 'disk-low', disk_free_gb: 20 });
    expect(text(board)).toContain(
      'Local resources · disk getting low · 20GB free',
    );
    const warning = board.findAllByProps({ testID: 'hq-resource-warning' })[0];
    expect(warning.findByType(SIcon).props.color).toBe(color);
    expect(board.props.accessibilityLabel).toContain('Local resources');
    const usage = tree.root.findAllByProps({ testID: 'hq-resource-usage' })[0];
    expect(text(usage)).not.toContain('disk getting low');
  },
);

test('a retained resource warning stays labelled as a previous reading after refresh failure', () => {
  const board = mount({ tier: 'red', warn_key: 'memory-critical' }, true, true);
  const warning = board.findAllByProps({ testID: 'hq-resource-warning' })[0];
  expect(text(warning)).toContain('本机资源 · 内存告急');
  expect(text(warning)).toContain('上次记录 · 刷新失败');
  expect(board.props.accessibilityLabel).toContain('上次记录');
});

test.each([undefined, { disk_free_gb: 1, warn: 'unclassified message' }])(
  'missing or unwarned machine readings do not invent a warning or healthy verdict',
  machine => {
    const board = mount(machine);
    expect(
      board.findAllByProps({ testID: 'hq-resource-warning' }),
    ).toHaveLength(0);
    expect(text(board)).not.toContain('Local resources');
    expect(text(board)).not.toContain('All clear');
  },
);
