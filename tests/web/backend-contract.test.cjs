const { test, before, after } = require('node:test');
const { start } = require('./fixture.cjs');
const { contract, observe, verify } = require('../contracts/http-contract.cjs');
let fixture;
before(async () => {
  fixture = await start();
  const { state } = fixture,
    s = contract.seed;
  state.requireAuth = 'contract-owner';
  state.turns[s.pane] = s.turns;
  state.turns['%13'] = [];
  state.historyPages[s.pane + ':before-151'] = s.history;
  state.transcriptMeta[s.pane] = { etag: 'W/"contract-1"', dropped: 150 };
  state.workspace = s.workspace;
  const matter = s.workspace.matters[0];
  state.matters[matter.id] = {
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
  };
  state.documents = {
    [s.document.reference]: { ...s.document, session: s.session, home: s.home },
  };
});
after(async () => fixture?.close());
for (const row of contract.cases)
  test('Web fake / Go contract: ' + row.name, async () => {
    fixture.state.share = {
      all: row.role !== 'guest',
      view_panes: [contract.seed.pane],
      input: false,
      panes: [],
    };
    fixture.state.workspace =
      row.capability === 'no-workspace' ? undefined : contract.seed.workspace;
    verify(await observe(fixture.url, 'contract-owner', row), row);
  });
