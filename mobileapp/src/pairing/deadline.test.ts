import {ApiError} from '../api/client';
import {checkServer, withDeadline} from './deadline';

// Pairing waited on requests bounded only by iOS's own idle timeout, about a minute each.
// On 2026-09-22 a VPN on the phone swallowed them and the scan spun until it read as
// "forever", with nothing to say what was wrong. These pin the bound, and pin that a step
// which never answers is reported as unreachable rather than as a rejected token.

const never = <T,>() => new Promise<T>(() => {});

describe('withDeadline', () => {
  it('answers with the fallback once the time is up', async () => {
    const t0 = Date.now();
    await expect(withDeadline(never<boolean>(), 30, false)).resolves.toBe(false);
    expect(Date.now() - t0).toBeLessThan(1000);
  });
  it('passes a value through', async () => {
    await expect(withDeadline(Promise.resolve(7), 1000, 0)).resolves.toBe(7);
  });
  it('passes a rejection through', async () => {
    await expect(withDeadline(Promise.reject(new Error('x')), 1000, 0)).rejects.toThrow('x');
  });
});

describe('checkServer', () => {
  const client = (health: Promise<boolean>, agents: Promise<unknown>) => ({health: () => health, agents: () => agents});

  it('a server that never answers its health check is unreachable', async () => {
    expect(await checkServer(client(never(), Promise.resolve([])), 30)).toBe('unreachable');
  });
  it('a server that answers health but never answers the authed call is unreachable, not a rejected token', async () => {
    expect(await checkServer(client(Promise.resolve(true), never()), 30)).toBe('unreachable');
  });
  it('a token the server refuses is rejected', async () => {
    expect(await checkServer(client(Promise.resolve(true), Promise.reject(new ApiError(401, 'agents'))), 30)).toBe('rejected');
    expect(await checkServer(client(Promise.resolve(true), Promise.reject(new ApiError(403, 'agents'))), 30)).toBe('rejected');
  });
  it('an authed call that fails for any other reason is unreachable, not a rejected token', async () => {
    // A dropped connection, and the edge answering for a Mac that is not behind it.
    expect(await checkServer(client(Promise.resolve(true), Promise.reject(new TypeError('Network request failed'))), 30)).toBe('unreachable');
    expect(await checkServer(client(Promise.resolve(true), Promise.reject(new ApiError(502, 'agents'))), 30)).toBe('unreachable');
  });
  it('a server that says it is not healthy is unreachable', async () => {
    expect(await checkServer(client(Promise.resolve(false), Promise.resolve([])), 30)).toBe('unreachable');
  });
  it('a reachable server that takes the token is ok', async () => {
    expect(await checkServer(client(Promise.resolve(true), Promise.resolve([])), 30)).toBe('ok');
  });
});
