// The radar's title is the switch into the Servers page: "● <name> ⌄" (servers-reachability).
// A grey ⇄ square between the name and the dot went unnoticed (2026-10-05). Read off the
// source, as shellDrift does: rendering the radar needs its whole context for a question
// about the order of three things in one row.
// eslint-disable-next-line @typescript-eslint/no-var-requires
const fs = require('fs') as {readFileSync: (p: string, e: string) => string};
const req = require as unknown as {resolve: (m: string) => string};
const src = fs.readFileSync(req.resolve('./RadarPanel.tsx'), 'utf8');
const run = src.slice(src.indexOf('<View style={styles.titleRun}>'), src.indexOf('<View style={[styles.headerRight'));

describe('the radar title', () => {
  it('leads with the connection dot and ends in a brand chevron, in one run', () => {
    expect(run).not.toBe('');
    const dot = run.indexOf('<ConnDot');
    const chip = run.indexOf('testID={TestIds.radar.serverChip}');
    const name = run.indexOf('styles.brand');
    const chevron = run.indexOf('<SIcon name="chevronDown"');
    expect(dot).toBeGreaterThanOrEqual(0);
    expect([dot < chip, chip < name, name < chevron]).toEqual([true, true, true]);
    expect(run).toContain('color={BRAND}');
  });

  it('has no ⇄ square any more', () => {
    // Rendered text, not a comment that remembers it.
    expect(src).not.toMatch(/>\s*⇄\s*</);
    expect(src).not.toContain('switchGlyph');
  });

  it('reaches over the dot: the title target extends left past the dot and its gap', () => {
    const m = /const titleHit = \{top: \d+, bottom: \d+, left: (\d+), right: \d+\}/.exec(src);
    const dot = /connDot: \{width: (\d+), height: \d+, borderRadius: [\d.]+, marginRight: (\d+)\}/.exec(src);
    expect(m && dot).toBeTruthy();
    expect(Number(m![1])).toBeGreaterThanOrEqual(Number(dot![1]) + Number(dot![2]));
  });
});

// A module, so its names do not collide with the other source-reading tests.
export {};
