import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {
  useTerminalKeys,
  supportsKey,
  type InputCapabilities,
  type TerminalKeyController,
  type KeyResult,
} from './terminalKeys';

const capabilities = (): InputCapabilities => ({
  version: 1,
  can_input: true,
  context: {
    boot: 'boot',
    pane_binding: 'pane',
    expires_at: Math.floor(Date.now() / 1000) + 120,
    proof: 'proof',
  },
  retry_seconds: 120,
  actions: [],
  keys: [],
  legacy_events: {},
});
let control: TerminalKeyController;
let tree: renderer.ReactTestRenderer;
afterEach(() => {
  if (tree) act(() => tree.unmount());
});

test('the interface follows advertised combinations rather than inventing support', () => {
  const key = {
    key: 'F9',
    label: 'F9',
    label_zh: 'F9',
    group: 'function',
    modifier_sets: [[], ['shift']] as 'shift'[][],
  };
  expect(supportsKey(key, ['shift'])).toBe(true);
  expect(supportsKey(key, ['control'])).toBe(false);
});

test('rapid taps send one typed event without text/Enter; unknown outcomes stop later keys and retry keeps the same ID/context', async () => {
  let resolve!: (r: KeyResult) => void;
  const send = jest.fn(
    (_payload: unknown) =>
      new Promise<KeyResult>(r => {
        resolve = r;
      }),
  );
  const load = jest.fn(async () => capabilities());
  function Harness() {
    control = useTerminalKeys('host|%1', true, load, send);
    return null;
  }
  await act(async () => {
    tree = renderer.create(<Harness />);
  });
  act(() => {
    control.fire({key: 'F2', modifiers: ['shift']});
    control.fire({key: 'F2', modifiers: ['shift']});
  });
  expect(send).toHaveBeenCalledTimes(1);
  const first = send.mock.calls[0][0];
  expect(first).toMatchObject({
    key_event: {key: 'F2', modifiers: ['shift']},
  });
  expect(first).not.toHaveProperty('enter');
  expect(first).not.toHaveProperty('text');
  await act(async () => resolve({ok: false, reason: 'network', status: 0}));
  expect(control.uncertain).toBe(true);
  act(() => {
    control.fire({key: 'ArrowLeft', modifiers: []});
  });
  expect(send).toHaveBeenCalledTimes(1);
  act(() => {
    control.retry();
  });
  expect(send.mock.calls[1][0]).toBe(first);
  await act(async () => resolve({ok: true}));
  expect(control.uncertain).toBe(false);
  expect(control.message).toBe('sent');
});

test('expired boot does not replay a previous key and a late result cannot repaint a new target', async () => {
  let finish!: (r: KeyResult) => void;
  let next = capabilities();
  const send = jest.fn(
    (_p: unknown) =>
      new Promise<KeyResult>(r => {
        finish = r;
      }),
  );
  const load = jest.fn(async () => next);
  function Harness({target}: {target: string}) {
    control = useTerminalKeys(target, true, load, send);
    return null;
  }
  await act(async () => {
    tree = renderer.create(<Harness target="host|%1" />);
  });
  act(() => {
    control.fire({key: 'F2', modifiers: []});
  });
  await act(async () => finish({ok: false, status: 0, reason: 'network'}));
  next = {
    ...capabilities(),
    context: {...next.context, boot: 'replacement'},
  };
  await act(async () => {
    await control.refresh();
  });
  act(() => {
    control.retry();
  });
  expect(send).toHaveBeenCalledTimes(1);
  act(() => {
    control.resume();
    control.fire({key: 'F3', modifiers: []});
  });
  await act(async () => {
    tree.update(<Harness target="host|%2" />);
  });
  await act(async () => finish({ok: true}));
  expect(control.message).toBe('');
  expect(control.busy).toBe(false);
});

test('malformed capability metadata is refused rather than enabling guessed combinations', () => {
  const {parseInputCapabilities} = require('../api/terminalKeys');
  const valid = capabilities();
  expect(parseInputCapabilities(valid)).not.toBeNull();
  expect(
    parseInputCapabilities({
      ...valid,
      keys: [
        {
          key: 'F2',
          label: 'F2',
          label_zh: 'F2',
          group: 'function',
          modifier_sets: 'control',
        },
      ],
    }),
  ).toBeNull();
  expect(
    parseInputCapabilities({
      ...valid,
      actions: [
        {
          id: 'question',
          revision: 'one',
          event: {key: 'F2', modifiers: ['meta']},
        },
      ],
    }),
  ).toBeNull();
});


test('catalog reuse includes its server boot and accepts new keys without a schema bump', async () => {
 let next = capabilities();
 const load = jest.fn(async (_version?: number, _boot?: string) => next);
 function Harness() {control = useTerminalKeys('host|%1', true, load);return null;}
 await act(async () => {tree = renderer.create(<Harness />);});
 await act(async () => {await control.refresh();});
 expect(load).toHaveBeenLastCalledWith(1, 'boot');
 next = {...capabilities(), context:{...capabilities().context,boot:'updated-server'},keys:[{key:'F9',label:'F9',label_zh:'F9',group:'function',modifier_sets:[[]]}]};
 await act(async () => {await control.refresh();});
 expect(control.cap?.version).toBe(1);expect(control.cap?.keys?.[0].key).toBe('F9');
 await act(async () => {await control.refresh();});
 expect(load).toHaveBeenLastCalledWith(1, 'updated-server');
});
