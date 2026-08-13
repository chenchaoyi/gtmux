import {makeDemoClient} from './demoClient';
import {demoPanes, demoPaneText, sampleAgents} from './demoData';

// The Demo tour is the ONLY view of the app App Review gets. Anything the real radar
// offers and the tour does not is a feature a reviewer cannot see — the All-panes
// browser was exactly that: `demoClient` had no `panes()` at all, so the surface was
// unreachable in the tour while being described in the listing.
describe('demo all-panes browser', () => {
  it('the demo client answers panes()', async () => {
    const client = makeDemoClient('en');
    const rows = await client.panes();
    expect(rows.length).toBeGreaterThan(0);
  });

  it('every agent on the radar is also a pane in the browser', async () => {
    // The browser is the SUPERSET of the radar. Deriving the panes from sampleAgents is
    // what keeps that true; a hand-written second list would drift the first time either
    // changed.
    const agents = sampleAgents().filter(a => a.source !== 'native');
    const ids = demoPanes().map(p => p.pane_id);
    for (const a of agents) expect(ids).toContain(a.pane_id);
  });

  it('carries plain panes too, and marks the tiers apart', () => {
    const rows = demoPanes();
    const plain = rows.filter(r => r.tier === 'plain');
    expect(plain.length).toBeGreaterThan(0);
    // A plain pane names no agent — waiting/working/idle are agent concepts.
    for (const r of plain) expect(r.agent).toBeUndefined();
    for (const r of rows.filter(x => x.tier === 'agent')) expect(r.agent).toBeTruthy();
  });

  it('groups by session, contiguously, the way the browser lists them', () => {
    const seen = new Set<string>();
    let prev = '';
    for (const r of demoPanes()) {
      if (r.session !== prev) {
        expect(seen.has(r.session)).toBe(false); // a session must not reappear later
        seen.add(r.session);
        prev = r.session;
      }
    }
    expect(seen.size).toBeGreaterThan(1);
  });

  it('every pane the browser lists opens onto a real screen', () => {
    // Tapping a row in the browser opens Detail, which reads demoPaneText. A pane with
    // no canned screen falls through to "(no live screen for this demo session)" — a
    // dead end in the one tour a reviewer gets.
    for (const r of demoPanes()) {
      expect(demoPaneText(r.pane_id)).not.toMatch(/no live screen/);
    }
  });
});
