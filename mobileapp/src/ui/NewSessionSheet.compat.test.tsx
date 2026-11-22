import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NewSessionSheet} from './NewSessionSheet';
import {GtmuxClient} from '../api/client';
import {paletteFor} from './theme';

const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => { for (const tree of mounted.splice(0)) act(() => tree.unmount()); });

function mount() {
  const createSession = jest.fn().mockResolvedValue({session: 'Review #12', pane_id: '%8', window: '0', pane: '0', loc: 'Review #12:0.0'});
  const noop = () => {};
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<NewSessionSheet visible client={{createSession} as unknown as GtmuxClient}
    macName="Fixture Mac" lang="en" pal={paletteFor('light')} onCreated={noop} onClose={noop} onDismiss={noop} onCheckSessions={noop} />); });
  mounted.push(tree);
  const input = () => tree.root.findByProps({testID: 'new-session-name'});
  const submit = () => tree.root.findByProps({testID: 'new-session-create'});
  const text = () => tree.root.findAllByType(Text).map(node => String(node.props.children)).join(' ');
  return {createSession, input, submit, text};
}

test('MCE-005 shows the trimmed name and sends the same value', async () => {
  const m = mount();
  act(() => m.input().props.onChangeText('  Review #12  '));
  expect(m.text()).toContain('Will be named Review #12');
  await act(async () => { await m.submit().props.onPress(); });
  expect(m.createSession).toHaveBeenCalledWith('Review #12', expect.any(String));
});

test.each(['QA#{version}', 'QA##', 'QA#H', 'QA#'])('MCE-005 blocks tmux formatting in %s', async name => {
  const m = mount();
  act(() => m.input().props.onChangeText(name));
  expect(m.text()).toContain('Change the # sequence');
  expect(m.submit().props.accessibilityState.disabled).toBe(true);
  await act(async () => {
    await m.submit().props.onPress();
    await m.input().props.onSubmitEditing();
  });
  expect(m.createSession).not.toHaveBeenCalled();
});
