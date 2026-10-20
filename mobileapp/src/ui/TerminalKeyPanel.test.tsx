import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {TerminalKeyPanel} from './TerminalKeyPanel';
import {useTerminalKeys, type InputCapabilities} from './terminalKeys';
import {paletteFor} from './theme';
import catalog from '../api/terminalKeysCatalog.json';
import type {SendPayload} from '../api/client';

test('compact keycaps send canonical keys, clear one-shot modifiers and keep function keys reachable', async () => {
  const send = jest.fn(async (_payload: SendPayload) => ({ok: true as const}));
  const load = async (): Promise<InputCapabilities> => ({
    version: 1,
    can_input: true,
    context: {
      boot: 'b',
      pane_binding: 'p',
      proof: 's',
      expires_at: Date.now() / 1000 + 120,
    },
    retry_seconds: 120,
    actions: [],
    keys: catalog.keys as InputCapabilities['keys'],
  });
  function Harness() {
    const control = useTerminalKeys('fixture|%54', true, load, send);
    return (
      <TerminalKeyPanel
        visible
        enabled
        pane="%54"
        pal={paletteFor('light')}
        lang="en"
        onClose={() => {}}
        control={control}
      />
    );
  }
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<Harness />);
  });
  const button = (id: string) =>
    tree.root.find(
      n => n.props.testID === id && typeof n.props.onPress === 'function',
    );
  try {
    expect(
      button('terminal-key-PageDown').findByType(Text).props.children,
    ).toBe('PgDn');
    expect(button('terminal-key-Backspace').props.accessibilityLabel).toBe(
      'Backspace',
    );
    act(() => button('terminal-modifier-control').props.onPress());
    expect(
      button('terminal-modifier-control').props.accessibilityState.selected,
    ).toBe(true);
    await act(async () => button('terminal-key-PageDown').props.onPress());
    expect(send.mock.calls[0][0]).toMatchObject({
      key_event: {key: 'PageDown', modifiers: ['control']},
    });
    expect(send.mock.calls[0][0]).not.toHaveProperty('enter');
    expect(
      button('terminal-modifier-control').props.accessibilityState.selected,
    ).toBe(false);
    act(() => button('terminal-category-function').props.onPress());
    await act(async () => button('terminal-key-F2').props.onPress());
    expect(send.mock.calls[1][0]).toMatchObject({
      key_event: {key: 'F2', modifiers: []},
    });
  } finally {
    act(() => tree.unmount());
  }
});
