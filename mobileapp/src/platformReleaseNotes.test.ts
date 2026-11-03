import {notesForPlatform} from './platformReleaseNotes';
import {RELEASE_NOTES} from './releaseNotes';
import {APP_VERSION} from './version';
import {cmpVersion} from './state/whatsnew';

test('iOS keeps its archive; Android never inherits iPhone-only historical releases', () => {
  expect(notesForPlatform('ios')).toBe(RELEASE_NOTES);
  const android=notesForPlatform('android');
  expect(cmpVersion(android[0].version,APP_VERSION)).toBeLessThanOrEqual(0);
  expect(android.every(row=>row.en.length>0&&row.zh.length>0)).toBe(true);
  expect(new Set(android.map(row=>row.version)).size).toBe(android.length);
  for(let i=1;i<android.length;i++)expect(cmpVersion(android[i-1].version,android[i].version)).toBeGreaterThan(0);
  expect(android.flatMap(row=>row.en).join('\n')).not.toMatch(/Live Activit|iPhone|iPad|APNs/);
});
