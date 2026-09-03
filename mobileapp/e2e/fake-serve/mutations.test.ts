import {startFake, Fake, GUEST_VIEW} from './server';

// The mutating half of the app, exercised against the fake rather than a live Mac
// (hq asked for exactly this list: send incl. keys and draft protection, focus,
// knowledge land/retire). These drive the CLIENT — the same code the app runs — and
// assert on what the server RECEIVED, which is a stronger claim than a screenshot: not
// "the screen changed" but "the app sent C-c to %12".
//
// Error paths are first class here. A suite that only ever drives the happy path teaches
// itself that failure does not happen, and every one of these failures is a real state:
// a pane that went away, a key that is not allow-listed, a half-typed draft in the box, a
// promotion that was already landed.
import {GtmuxClient} from '../../src/api/client';

let fake: Fake;
let client: GtmuxClient;

beforeEach(async () => {
  fake = await startFake();
  client = new GtmuxClient(fake.url, fake.token);
});
afterEach(async () => {
  await fake.close();
});

describe('send', () => {
  test('text lands as text, and a control key lands as a key', async () => {
    await client.send('%12', {text: '继续', enter: true});
    await client.send('%12', {key: 'C-c'});
    expect(fake.world.writesTo('/api/send')).toEqual([
      {id: '%12', text: '继续', enter: true},
      {id: '%12', key: 'C-c'},
    ]);
  });

  test('a key outside the allow-list is refused by the server, not by hope', async () => {
    // The client reports a failed send as null — it keeps no reason. See the note in
    // this file's tail about what that costs the reader.
    expect(await client.send('%12', {key: 'C-x' as 'C-c'})).toBeNull();
  });

  test('a send into a pane holding an unsubmitted draft is refused', async () => {
    // A paste APPENDS, so delivering here would submit someone's half-written line along
    // with the payload. The core refuses.
    fake.world.drafts.set('%12', 'half a thought');
    expect(await client.send('%12', {text: 'continue', enter: true})).toBeNull();
    // The attempt still reached the server, which is what makes this a refusal rather
    // than the app deciding on its own not to try.
    expect(fake.world.writesTo('/api/send')).toEqual([{id: '%12', text: 'continue', enter: true}]);
  });

  test('a pane that went away fails', async () => {
    expect(await client.send('%999', {text: 'hello'})).toBeNull();
  });
});

// hq's requirement, verbatim: the three refusals must be READABLE after crossing the
// client boundary. They are the reasons a reader could act on — someone is typing in that
// pane, the pane is gone, that key will never be run — and they used to arrive as one
// undifferentiated null.
describe('a refusal keeps the server’s words', () => {
  test('the three real refusals survive the boundary', async () => {
    fake.world.drafts.set('%12', 'half a thought');
    const draft = await client.sendResult('%12', {text: 'continue', enter: true});
    const gone = await client.sendResult('%999', {text: 'hello'});
    const key = await client.sendResult('%12', {key: 'C-x' as 'C-c'});

    expect(draft).toMatchObject({ok: false, status: 400});
    expect(gone).toMatchObject({ok: false, status: 400});
    expect(key).toMatchObject({ok: false, status: 400});
    expect((draft as {reason: string}).reason).toContain('refused-draft');
    expect((gone as {reason: string}).reason).toContain('no such pane');
    expect((key as {reason: string}).reason).toContain('key not allowed');
  });

  test('a success carries the pane, not a reason', async () => {
    const ok = await client.sendResult('%12', {text: 'hi'});
    expect(ok.ok).toBe(true);
  });

  test('the old shape is unchanged, so no caller behaves differently', async () => {
    // Additive by construction: every call site reads null as "did not land", and still
    // does. What the reader is TOLD is a separate question, left open on purpose.
    expect(await client.send('%999', {text: 'hello'})).toBeNull();
    expect(await client.send('%12', {text: 'hi'})).not.toBeNull();
  });
});

describe('knowledge, remotely', () => {
  const PENDING = 'best-practices/spawn-must-decide-model';
  const LANDED = 'workflows/green-means-merge';

  test('landing a pending promotion clears it from the queue', async () => {
    const before = await client.hqKnowledge();
    expect(before.promotions.pending).toBe(2);

    const r = await client.hqKnowledgeAct({op: 'land', id: PENDING, ref: 'AGENTS.md'});
    expect(r.ok).toBe(true);

    const after = await client.hqKnowledge();
    expect(after.promotions.pending).toBe(1);
    expect(after.entries.find(e => e.id === PENDING)?.landed_ref).toBe('AGENTS.md');
  });

  test('landing something already landed is refused, with a message that says why', async () => {
    const r = await client.hqKnowledgeAct({op: 'land', id: LANDED, ref: 'again.md'});
    expect(r.ok).toBe(false);
    expect(String((r as {ok: false; error: string}).error)).toContain('no pending promotion');
  });

  test('retiring removes the entry from the live set', async () => {
    const r = await client.hqKnowledgeAct({op: 'retire', id: PENDING, why: 'superseded'});
    expect(r.ok).toBe(true);
    const after = await client.hqKnowledge();
    expect(after.entries.some(e => e.id === PENDING)).toBe(false);
  });

  test('an unknown id changes nothing', async () => {
    const r = await client.hqKnowledgeAct({op: 'retire', id: 'nope/nope', why: 'x'});
    expect(r.ok).toBe(false);
    const after = await client.hqKnowledge();
    expect(after.entries.length).toBe(4);
  });

  test('a guest is refused the whole surface', async () => {
    const guest = await startFake({guest: true});
    try {
      const c = new GtmuxClient(guest.url, guest.token);
      // The client degrades a refusal to an empty base, which is what the UI renders —
      // the point is that nothing of the supervisor's assessment reaches a guest.
      const idx = await c.hqKnowledge();
      expect(idx.entries).toEqual([]);
      expect((await c.hqKnowledgeAct({op: 'retire', id: 'x', why: 'y'})).ok).toBe(false);
    } finally {
      await guest.close();
    }
  });
});

describe('a guest sees only its own link', () => {
  test('the radar is filtered to the allowlist, not merely styled differently', async () => {
    // The real serve filters here (filterAgentsForGuest). A fake that returned the whole
    // fleet would let a suite prove the opposite and keep passing if that filter broke.
    const guest = await startFake({guest: true});
    try {
      const c = new GtmuxClient(guest.url, guest.token);
      const rows = await c.agents();
      // The list itself, not a copy of it: guest-scope.test.ts deliberately makes view and
      // input differ, and a hard-coded ['%12'] here went red the moment it did.
      expect(rows.map(r => r.pane_id)).toEqual(GUEST_VIEW);
      // And a pane outside it cannot be read even by asking directly (the client throws
      // on a non-2xx here, which is the shape the caller already handles).
      await expect(c.pane('%11')).rejects.toThrow(/403/);
    } finally {
      await guest.close();
    }
  });
});

describe('reply options', () => {
  test('a waiting pane offers its own words, an idle one offers nothing', async () => {
    expect((await client.options('%11')).map(o => o.n)).toEqual([1, 2, 3]);
    expect(await client.options('%13')).toEqual([]);
  });
});

// Still open, and deliberately: the client now KEEPS these reasons, but nothing shows them
// to the reader yet. What a failed send should say, and where, is a question for the
// commander — see the PR that added sendResult.
