import {beginChatEcho, chatEchoRecorded} from './pendingChatEcho';
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
