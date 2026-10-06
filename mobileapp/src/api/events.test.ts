// The live stream says HOW it failed: a Mac that answered and refused this phone, and a
// Mac nothing heard from, call for different things from the reader (AgentsContext).
import {subscribe} from './events';

type Source = {listeners: Record<string, (e: unknown) => void>};
const mockSources: Source[] = [];
jest.mock('react-native-sse', () =>
  jest.fn().mockImplementation(() => {
    const src = {
      listeners: {} as Record<string, (e: unknown) => void>,
      addEventListener(name: string, fn: (e: unknown) => void) {
        src.listeners[name] = fn;
      },
      close: jest.fn(),
    };
    mockSources.push(src);
    return src;
  }),
);
jest.mock('./client', () => ({clientTag: () => 'test'}));
jest.mock('../diag', () => ({Diag: {info: jest.fn(), warn: jest.fn()}}));

test('a refused stream reports the status the Mac answered with, and only that', () => {
  const onError = jest.fn();
  subscribe('https://mac.example', 't', {onAgents: jest.fn(), onAlert: jest.fn(), onError});
  const error = mockSources[mockSources.length - 1].listeners.error;
  error({type: 'error', xhrStatus: 401, message: '{"error":"unauthorized"}'});
  expect(onError).toHaveBeenLastCalledWith(401);
  // No answer at all: XMLHttpRequest reports status 0, which is not a status.
  error({type: 'error', xhrStatus: 0});
  expect(onError).toHaveBeenLastCalledWith(undefined);
  error({type: 'timeout'});
  expect(onError).toHaveBeenLastCalledWith(undefined);
});

// Server mode changed on the Mac (or an off request was accepted): the stream says so
// with `awake`, which carries no state; the handler re-reads GET /api/awake.
test('an awake event asks for a server-mode re-read, and is harmless unhandled', () => {
  const onServerMode = jest.fn();
  subscribe('https://mac.example', 't', {onAgents: jest.fn(), onAlert: jest.fn(), onServerMode});
  mockSources[mockSources.length - 1].listeners.awake({data: '{}'});
  expect(onServerMode).toHaveBeenCalledTimes(1);
  subscribe('https://mac.example', 't', {onAgents: jest.fn(), onAlert: jest.fn()});
  expect(() => mockSources[mockSources.length - 1].listeners.awake({data: '{}'})).not.toThrow();
});
