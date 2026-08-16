import {deviceSub} from './ManageMacScreen';

// One row shape on every surface. The menu bar showed only a last-seen time while the app
// showed the platform — off the SAME payload, which the menu bar simply did not decode.
// These pin the phone's half of the agreement (PairedDeviceSubtitleTests pins the Mac's).
describe('deviceSub', () => {
  const now = Math.floor(Date.now() / 1000);

  it('joins what it is, where from, and when — in that order', () => {
    const s = deviceSub('iOS 26.6', '192.168.1.23', now - 120, false)!;
    expect(s.indexOf('iOS 26.6')).toBeLessThan(s.indexOf('192.168.1.23'));
    expect(s.indexOf('192.168.1.23')).toBeLessThan(s.indexOf('ago'));
  });

  it('says so when a device has never connected, instead of going blank', () => {
    expect(deviceSub(undefined, undefined, undefined, false)).toBe('never connected');
    expect(deviceSub(undefined, undefined, undefined, true)).toBe('从未连接');
  });

  it('drops the parts it does not know rather than printing empties', () => {
    expect(deviceSub('iOS 26.6', undefined, undefined, false)).toBe('iOS 26.6 · never connected');
  });
});

import {linkUsage} from './ManageMacScreen';

// A share link is a standing grant handed to someone else. Its row already says what it
// PERMITS; this line says whether anyone walked through it.
describe('linkUsage', () => {
  it('says plainly when nobody has used a link', () => {
    expect(linkUsage({}, false)).toBe('not used yet');
    expect(linkUsage({}, true)).toBe('还没有人用过');
  });

  it('names the device and the address once someone has', () => {
    const s = linkUsage({platform: 'Chrome 141 · macOS', lastIP: '10.0.0.7', lastSeen: Math.floor(Date.now() / 1000) - 60}, false);
    expect(s).toContain('Chrome 141 · macOS');
    expect(s).toContain('10.0.0.7');
    expect(s).toContain('last used');
  });
});

import {grantSummary} from './ManageMacScreen';

// A grant stores PANE IDS, and a pane can close. The header counted the stored ids while
// the rows below could only show live ones — measured on a real roster: 9 stored, 3 gone,
// so the link read "sees 9" with six rows to toggle.
describe('grantSummary', () => {
  const live = new Set(['%1', '%2', '%3']);

  it('counts what the link can actually reach', () => {
    expect(grantSummary({viewPanes: ['%1', '%2'], inputPanes: ['%1']}, live, false))
      .toBe('sees 2 · types 1');
  });

  it('names the grants that no longer exist instead of quietly inflating the count', () => {
    const s = grantSummary({viewPanes: ['%1', '%9', '%8'], inputPanes: ['%1']}, live, false);
    expect(s).toContain('sees 1');
    expect(s).toContain('2 gone');
  });

  it('says so when every grant has expired', () => {
    expect(grantSummary({viewPanes: ['%9'], inputPanes: []}, live, true)).toBe('看不到任何 pane · 1 项已失效');
  });

  it('reads as before when a link grants nothing at all', () => {
    expect(grantSummary({viewPanes: [], inputPanes: []}, live, false)).toBe('sees nothing');
  });
});

import {paneGroups} from './ManageMacScreen';
import {Agent} from '../api/types';

const pane = (id: string, session: string, task: string): Agent =>
  ({pane_id: id, session, task, source: 'tmux', status: 'idle'} as Agent);

// The picker listed panes flat, labelled only by task — and two panes in different
// sessions running the same project truncate to the same words, so the list offered two
// identical-looking rows with no way to tell which was which.
describe('paneGroups', () => {
  it('groups by session, keeping the order the radar gave', () => {
    const g = paneGroups([
      pane('%11', 'MP', 'multipilot-companion 服务端需求'),
      pane('%1', 'Diting', 'analysis'),
      pane('%12', 'MP', 'multipilot-companion feature dev'),
    ]);
    expect(g.map(x => x.session)).toEqual(['MP', 'Diting']);
    expect(g[0].panes.map(p => p.pane_id)).toEqual(['%11', '%12']);
  });

  it('keeps a session with no name rather than dropping its panes', () => {
    const g = paneGroups([pane('%9', '', 'stray')]);
    expect(g).toHaveLength(1);
    expect(g[0].panes.map(p => p.pane_id)).toEqual(['%9']);
  });

  it('is empty for no panes', () => {
    expect(paneGroups([])).toEqual([]);
  });
});
