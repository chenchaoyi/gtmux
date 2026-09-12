// The drift guard (change ipad-universal-app, D3). The iPad shell composes the phone's
// screens; it never copies them. What went wrong before was structural — SplitScreen was
// a copy of the radar's chrome and fell two months behind the phone — and no render test
// sees that, so this one reads the source, like detailChrome.test.ts.
// eslint-disable-next-line @typescript-eslint/no-var-requires
const fs = require('fs') as {readFileSync: (p: string, e: string) => string; readdirSync: (p: string) => string[]; existsSync: (p: string) => boolean};
// eslint-disable-next-line @typescript-eslint/no-var-requires
const path = require('path') as {join: (...p: string[]) => string};
const req = require as unknown as {resolve: (m: string) => string};
const dir = path.join(req.resolve('./RadarPanel.tsx'), '..');
const read = (f: string) => fs.readFileSync(path.join(dir, f), 'utf8');
const screens = fs.readdirSync(dir).filter((f: string) => /\.tsx$/.test(f) && !/\.test\.tsx$/.test(f));

describe('the regular shell composes, never copies', () => {
  it('has no SplitScreen', () => {
    expect(fs.existsSync(path.join(dir, 'SplitScreen.tsx'))).toBe(false);
  });

  it('renders the radar list in exactly one file', () => {
    // A second radar list outside RadarPanel is a second radar to keep in sync. (The
    // pane browser's `SectionList` is React Native's, a different component.) DemoScreen
    // is the one standing exception: it renders the demo's radar outside the navigator,
    // over fake data, and predates this rule — MOBILE §18 records that it fell behind the
    // phone twice for exactly this reason. Folding it in is a follow-up, not a licence.
    const renderers = screens.filter((f: string) => read(f).includes("from '../ui/SectionList'"));
    expect(renderers.sort()).toEqual(['DemoScreen.tsx', 'RadarPanel.tsx']);
  });

  it('has both shells render RadarPanel, with only props between them', () => {
    expect(read('RadarScreen.tsx')).toContain('<RadarPanel variant="screen"');
    expect(read('SplitShell.tsx')).toContain('<RadarPanel variant="sidebar"');
    // The shell holds no list, banner or header of its own.
    for (const token of ['SectionList', 'OfflineBanner', 'RadarSummary', 'HQDisc']) {
      expect(read('SplitShell.tsx')).not.toContain(token);
    }
  });

  it('lets only the shell read the window to choose a layout', () => {
    // Screens take a class or a prop; the one rule lives in ui/layout.ts.
    const offenders = screens.filter((f: string) => f !== 'SplitShell.tsx' && /isSplitCanvas\(|sizeClassFor\(/.test(read(f)));
    expect(offenders).toEqual([]);
    const rootFiles = ['App.tsx'].map((f: string) => fs.readFileSync(path.join(dir, '..', '..', f), 'utf8'));
    expect(rootFiles[0]).toContain('useSizeClass()');
    expect(rootFiles[0]).not.toContain('isSplitCanvas(');
  });

  it('opens things through the workspace, not a navigator', () => {
    // The screens that both shells host must not address a route to open a pane, HQ or
    // All panes: the compact wrapper (back button) is the only navigation they know.
    for (const f of ['RadarPanel.tsx', 'HQScreen.tsx', 'PaneBrowserScreen.tsx']) {
      const src = read(f);
      expect(src).toContain('useWorkspace()');
      expect(src).not.toMatch(/navigate\('(Detail|HQ|Panes)'/);
    }
  });
});

export {};
