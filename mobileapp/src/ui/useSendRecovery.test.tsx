import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {useSendRecovery} from './useSendRecovery';

let current!: ReturnType<typeof useSendRecovery>;
function Probe({scope}: {scope: object}) {current = useSendRecovery(scope); return null;}
let tree: renderer.ReactTestRenderer;
afterEach(() => {act(() => tree?.unmount());});
const payload = {text: 'held', send_id: 'same-id', enter: true};
test('SFR-005 retry is synchronous, keeps its record visible and cannot replay a settled action', () => {
  const scope = {};
  act(() => {tree = renderer.create(<Probe scope={scope} />);});
  act(() => {current.record(payload, 'old refusal', 400, 'input-state-unavailable');});
  const failure = current.failure!;
  const send = jest.fn(p => current.begin(p));
  act(() => {current.retry(failure, send); current.retry(failure, send); current.edit(failure);});
  expect(send).toHaveBeenCalledTimes(1); expect(send).toHaveBeenCalledWith(payload);
  expect(current.failure).toMatchObject({payload, retrying: true, reason: 'old refusal'});
  expect(current.restore).toBeNull();
  act(() => {current.record(payload, 'new refusal', 400, 'input-awaiting-confirmation');});
  expect(current.failure).toMatchObject({retrying: false, reason: 'new refusal'});
  act(() => {current.retry(failure, send);}); expect(send).toHaveBeenCalledTimes(1);
  act(() => {current.accept(payload); current.retry(failure, send);});
  expect(current.failure).toBeNull(); expect(send).toHaveBeenCalledTimes(1);
});
test('SFR-005 edits once and captured actions cannot cross a changed client or ABA scope', () => {
  const first = {};
  act(() => {tree = renderer.create(<Probe scope={first} />);});
  // Effects settle before reading the first held record.
  act(() => {current.record(payload, 'refused');});
  const failure = current.failure!; const old = current;
  act(() => {current.edit(failure); current.edit(failure);});
  expect(current.failure).toBeNull(); expect(current.restore?.text).toBe('held');
  const at = current.restore?.at;
  act(() => {current.edit(failure);}); expect(current.restore?.at).toBe(at);
  act(() => {tree.update(<Probe scope={{}} />);});
  expect(current.failure).toBeNull(); expect(current.restore).toBeNull();
  const dispatch = jest.fn();
  act(() => {old.retry(failure, dispatch); old.edit(failure); expect(old.begin(payload)).toBe(false); expect(old.record(payload, 'late')).toBe(false);});
  expect(dispatch).not.toHaveBeenCalled();
  act(() => {tree.update(<Probe scope={{}} />);});
  act(() => {old.retry(failure, dispatch);}); expect(dispatch).not.toHaveBeenCalled();
  const last = current; act(() => tree.unmount());
  expect(last.begin(payload)).toBe(false); expect(last.record(payload, 'after unmount')).toBe(false);
});
