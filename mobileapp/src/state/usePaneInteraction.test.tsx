import React from 'react';
import renderer, {act} from 'react-test-renderer';
import type {GtmuxClient} from '../api/client';
import {usePaneInteraction} from './usePaneInteraction';

let result: ReturnType<typeof usePaneInteraction>;
let tree: renderer.ReactTestRenderer;
const answeredAt = {current: 0};
const view = (text: string) => ({text, options: [{n: 1, label: text}], awaiting_reply: true, truncated: false});
const flush = async () => {for (let i = 0; i < 5; i++) await act(async () => {await Promise.resolve();});};
function Probe({client, pane, active = true}: {client: GtmuxClient; pane: string; active?: boolean}) {result = usePaneInteraction(client, pane, active, answeredAt);return null;}
afterEach(() => {act(() => tree?.unmount()); answeredAt.current = 0;});
test('late reads from another pane cannot replace the current prompt; failures clear choices', async () => {
  let old!: (v: unknown) => void;
  const read = jest.fn().mockImplementationOnce(() => new Promise(r => {old = r;})).mockResolvedValue(view('new prompt'));
  const client = {interaction: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<Probe client={client} pane="%1" />);});
  await flush();
  await act(async () => tree.update(<Probe client={client} pane="%2" />)); await flush();
  expect(result.snapshot?.text).toBe('new prompt');
  await act(async () => old(view('old prompt'))); await flush();
  expect(result.snapshot?.text).toBe('new prompt');
  read.mockRejectedValue(new Error('offline')); act(() => result.refresh()); await flush();
  expect(result.snapshot).toBeNull(); expect(result.failed).toBe(true);
});
test('inactive panes do not poll; answering immediately invalidates old choices', async () => {
  const read = jest.fn().mockResolvedValue(view('permission'));
  const client = {interaction: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<Probe client={client} pane="%1" active={false} />);});await flush();
  expect(read).not.toHaveBeenCalled();
  await act(async () => tree.update(<Probe client={client} pane="%1" />));await flush();
  expect(result.snapshot?.options).toHaveLength(1);
  act(() => result.answered());expect(result.snapshot?.options).toHaveLength(0);
  act(() => result.refresh());await flush();expect(result.snapshot?.options).toHaveLength(0);
});
