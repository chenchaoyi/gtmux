import {startFake, Fake, GUEST_VIEW, GUEST_INPUT} from './server';
import {GtmuxClient} from '../../src/api/client';

/**
 * A guest's reach, one negative test per gated endpoint.
 *
 * The list is not invented: it is every handler in the real serve that consults
 * callerScope (server.go, hq.go, events.go, attach.go, share.go, push.go), read off the
 * call sites of its guest filters. The table lives in the PR that added this file.
 *
 * Why negative tests specifically: a fake that is MORE permissive than the real serve is
 * the worst kind, because a suite written against it proves the opposite of the rule and
 * keeps passing when the real filter breaks. That already happened here once — the radar
 * returned the whole fleet to a guest.
 *
 * The two allowlists are deliberately different in the fixture (view has %13, input does
 * not), so a test cannot pass by collapsing them.
 */
let owner: Fake;
let guest: Fake;
let asOwner: GtmuxClient;
let asGuest: GtmuxClient;

beforeEach(async () => {
  owner = await startFake();
  guest = await startFake({guest: true});
  asOwner = new GtmuxClient(owner.url, owner.token);
  asGuest = new GtmuxClient(guest.url, guest.token);
});
afterEach(async () => {
  await owner.close();
  await guest.close();
});

const status = async (f: Fake, path: string, init?: RequestInit): Promise<number> => {
  const r = await fetch(`${f.url}${path}`, {
    ...init,
    headers: {Authorization: `Bearer ${f.token}`, 'Content-Type': 'application/json', ...(init?.headers ?? {})},
  });
  return r.status;
};

describe('what a guest may SEE', () => {
  test('the radar carries only the view allowlist', async () => {
    expect((await asGuest.agents()).map(a => a.pane_id)).toEqual(GUEST_VIEW);
    expect((await asOwner.agents()).length).toBeGreaterThan(GUEST_VIEW.length);
  });

  test('a pane outside it cannot be read, listed, or transcribed', async () => {
    // A pane id STARTS with '%', so it has to be encoded in a query string or it reads as
    // a percent-escape: a hand-written `?id=%11` arrives as the control character 0x11 and
    // the server answers 404, which would have looked like a passing negative test for the
    // wrong reason.
    const id = encodeURIComponent('%11');
    for (const path of [`/api/pane?id=${id}`, `/api/options?id=${id}`, `/api/transcript?id=${id}`]) {
      expect({path, code: await status(guest, path)}).toEqual({path, code: 403});
      expect({path, code: await status(owner, path)}).toEqual({path, code: 200});
    }
  });

  test('the pane browser is filtered the same way', async () => {
    const r = await fetch(`${guest.url}/api/panes`, {headers: {Authorization: `Bearer ${guest.token}`}});
    const rows = (await r.json()) as Array<{pane_id: string}>;
    expect(rows.map(x => x.pane_id)).toEqual(GUEST_VIEW);
  });
});

describe('what a guest may DO', () => {
  test('it types only where the link granted the keyboard, which is a shorter list', async () => {
    // The two lists differ on purpose: %13 may be watched and not typed into.
    expect(GUEST_VIEW).toContain('%13');
    expect(GUEST_INPUT).not.toContain('%13');
    expect(await status(guest, '/api/send', {method: 'POST', body: JSON.stringify({id: '%13', text: 'hi'})})).toBe(403);
    expect(await status(guest, '/api/send', {method: 'POST', body: JSON.stringify({id: '%12', text: 'hi'})})).toBe(200);
  });

  test('focus obeys the view list', async () => {
    expect(await status(guest, '/api/focus', {method: 'POST', body: JSON.stringify({id: '%11'})})).toBe(403);
  });
});

describe('what a guest may never reach at all', () => {
  // Every owner-only surface: the whole fleet's assessment, the machine, and everything
  // the supervisor knows.
  const OWNER_ONLY = [
    '/api/digest',
    '/api/usage',
    '/api/awake',
    '/api/hq/board',
    '/api/hq/events',
    '/api/hq/knowledge',
    '/api/hq/knowledge/entry?id=pitfalls/kb-entry-date-is-utc-kb',
  ];

  test.each(OWNER_ONLY)('%s refuses a guest and answers the owner', async path => {
    expect(await status(guest, path)).toBe(403);
    expect(await status(owner, path)).toBe(200);
  });

  test('and cannot write knowledge either', async () => {
    expect(
      await status(guest, '/api/hq/knowledge/act', {
        method: 'POST',
        body: JSON.stringify({op: 'retire', id: 'pitfalls/kb-entry-date-is-utc-kb', why: 'x'}),
      }),
    ).toBe(403);
  });
});
