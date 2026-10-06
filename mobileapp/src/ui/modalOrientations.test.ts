// Every <Modal> in the app declares MODAL_ORIENTATIONS. Read off the source, as
// shellDrift does: the bug was a prop nobody wrote, on 20 of 20 modals, and a new sheet
// would be born the same way (see modalOrientations.ts).
// eslint-disable-next-line @typescript-eslint/no-var-requires
const fs = require('fs') as {readdirSync: (p: string, o: {recursive: boolean}) => string[]; readFileSync: (p: string, e: string) => string};
const req = require as unknown as {resolve: (m: string) => string};
const src = req.resolve('./modalOrientations').replace(/\/ui\/modalOrientations\.ts$/, '');

describe('modal orientations', () => {
  const files = fs
    .readdirSync(src, {recursive: true})
    .filter(f => f.endsWith('.tsx') && !f.includes('.test.'))
    .map(f => ({f, text: fs.readFileSync(`${src}/${f}`, 'utf8')}));

  it('finds the modals it guards', () => {
    expect(files.filter(({text}) => /<Modal\b/.test(text)).length).toBeGreaterThanOrEqual(20);
  });

  it('gives every <Modal> the app’s orientations', () => {
    const missing: string[] = [];
    for (const {f, text} of files) {
      for (const m of text.matchAll(/<Modal\b[^>]*>/g)) {
        if (!m[0].includes('supportedOrientations={MODAL_ORIENTATIONS}')) missing.push(`${f}: ${m[0].slice(0, 80)}`);
      }
    }
    expect(missing).toEqual([]);
  });
});

export {};
