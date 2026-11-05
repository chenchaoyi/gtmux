import { startFake, type Fake } from './server';
import { GtmuxClient } from '../../src/api/client';
// CJS assertions run unmodified under both Node and Jest. They contain no server.
const {
  contract,
  observe,
  verify,
} = require('../../../tests/contracts/http-contract.cjs');
let owner: Fake, guest: Fake;
beforeAll(async () => {
  owner = await startFake();
  guest = await startFake({ guest: true });
  for (const fixture of [owner, guest]) {
    const s = contract.seed,
      w = fixture.world;
    w.transcripts.set(s.pane, s.turns);
    w.transcripts.set('%13', []);
    w.historyPages.set(s.pane + ':before-151', s.history);
    w.transcriptMeta.set(s.pane, { etag: 'W/"contract-1"', dropped: 150 });
    w.workspace = s.workspace;
    const matter = s.workspace.matters[0];
    w.matters.set(matter.id, {
      schema: 1,
      matter,
      messages: [],
      turns: [],
      changes: [],
      events: [],
      transcript_earlier_available: false,
      earlier_available: false,
      message_earlier_available: false,
      warnings: [],
    });
    w.documents.set(s.document.reference, {
      ...s.document,
      body: Buffer.from(s.document.body),
      session: s.session,
      home: s.home,
    });
  }
});
afterAll(async () => {
  await owner?.close();
  await guest?.close();
});
test.each(contract.cases)('RN fake / Go contract: $name', async row => {
  const fixture = row.role === 'guest' ? guest : owner;
  fixture.world.workspace =
    row.capability === 'no-workspace' ? undefined : contract.seed.workspace;
  verify(await observe(fixture.url, fixture.token, row), row);
});

test('RN client consumes original ordinals, history errors, conditional emptiness and legacy workspace null', async () => {
  const c = new GtmuxClient(owner.url, owner.token);
  const result = await c.transcript(contract.seed.pane);
  expect(result.turns[0].turn_ordinal).toBe(151);
  expect(result.dropped).toBe(150);
  expect(result.turns[0].segments?.some(s => !!s.context)).toBe(true);
  expect((await c.transcript(contract.seed.pane, result.etag)).unchanged).toBe(
    true,
  );
  expect(
    (await c.traceHistory({ pane_id: contract.seed.pane }, 'before-151'))[0]
      .turn_ordinal,
  ).toBe(150);
  await expect(
    c.traceHistory({ pane_id: contract.seed.pane }, 'stale'),
  ).rejects.toMatchObject({ status: 409 });
  owner.world.workspace = undefined;
  expect(await c.hqWorkspace()).toBeNull();
  const g = new GtmuxClient(guest.url, guest.token);
  await expect(g.hqWorkspace()).rejects.toMatchObject({ status: 403 });
});
