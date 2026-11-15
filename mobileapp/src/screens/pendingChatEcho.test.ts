import {beginChatEcho, chatEchoRecorded, noteChatEchoReceipt} from './pendingChatEcho';
import cases from './promptIdentityCases.json';
import {promptIdentity} from './promptIdentity';
const turn = (prompt: string, time?: string) => ({prompt, time, response: ''});
test('confirms a prompt even when the same poll includes later turns', () => {
  const echo = beginChatEcho('a', 'please fix it', [turn('older', 't1')]);
  expect(chatEchoRecorded(echo, [turn('older', 't1'), turn('please fix it', 't2'), turn('continue', 't3')])).toBe(true);
});
test('a previous identical prompt cannot confirm a repeated send', () => {
  const old = turn('continue', '2026-10-17T12:00:00Z');
  const echo = beginChatEcho('b', 'continue', [old]);
  expect(chatEchoRecorded(echo, [old, turn('other', 't2')])).toBe(false);
  expect(chatEchoRecorded(echo, [turn('continue', '2026-10-17T12:01:00Z')])).toBe(true);
});
test('missing timestamps require a new occurrence; response text is not confirmation', () => {
  const echo = beginChatEcho('c', 'again\r\nplease', [turn('again\nplease')]);
  expect(chatEchoRecorded(echo, [turn('again\nplease')])).toBe(false);
  expect(chatEchoRecorded(echo, [turn('again\nplease'), turn(' again\nplease ')])).toBe(true);
  expect(chatEchoRecorded(echo, [{prompt: 'other', response: echo.text}])).toBe(false);
});

test.each(cases)('attachment prompt identity $same: $sent -> $recorded', ({sent, recorded, same}) => {
  expect(promptIdentity(sent) === promptIdentity(recorded)).toBe(same);
  const echo = beginChatEcho('image', sent, []);
  expect(chatEchoRecorded(echo, [turn(recorded, 'new')])).toBe(same);
  const repeated = beginChatEcho('again', sent, [turn(recorded, 'old')]);
  expect(chatEchoRecorded(repeated, [turn(recorded, 'old')])).toBe(false);
  expect(chatEchoRecorded(repeated, [turn(recorded, 'new')])).toBe(same);
  expect(chatEchoRecorded(echo, [{prompt: 'another request', response: recorded}])).toBe(false);
});


test('CAR-004 successful receipt lifetime is owned by its stable echo identity', () => {
  const echo = beginChatEcho('one', 'repeat', [turn('repeat', 'old')]);
  const observed = echo.observed;
  noteChatEchoReceipt(echo, 'queued', 1000);
  expect(echo.receiptExpiresAt).toBeUndefined();
  noteChatEchoReceipt(echo, 'accepted', 2000);
  expect(echo.receiptExpiresAt).toBe(4500);
  noteChatEchoReceipt(echo, 'submitted', 8000);
  expect(echo.receiptExpiresAt).toBe(4500);
  expect(echo.observed).toBe(observed);
  expect(echo.id).toBe('one'); expect(echo.text).toBe('repeat');
  expect(chatEchoRecorded(echo, [turn('repeat', 'old')])).toBe(false);
  expect(chatEchoRecorded(echo, [turn('repeat', 'new')])).toBe(true);
  const next = beginChatEcho('two', 'repeat', []);
  noteChatEchoReceipt(next, undefined, 8000);
  expect(next.receiptExpiresAt).toBe(10500);
  expect(echo.receiptExpiresAt).toBe(4500);
});
