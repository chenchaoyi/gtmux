import {execFileSync, spawn} from 'child_process';
import {killMatching, matchingPids} from './reclaim';

// The e2e harness leaked a WebDriverAgent that ran for 25 hours and 50 minutes, because
// the only cleanup was a process-GROUP kill in a teardown that an interrupted run never
// reaches — and by then the WDA is re-parented to launchd, where no group kill can find
// it. The fix reclaims by PREDICATE, at setup. This pins the mechanism.
//
// It matches on a marker of the test's own making, never on the real WebDriverAgent
// pattern: a unit test that killed whatever WDA happened to be running on the machine
// would be a side effect no test may have.
const MARKER = `gtmux-reclaim-probe-${process.pid}`;

const alive = (pid: number): boolean => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};

const settle = (ms: number) => new Promise(r => setTimeout(r, ms));

describe('reclaim kills by predicate, not by process group', () => {
  test('an ORPHANED match still dies', async () => {
    // Detached + unref'd is the shape that defeated the old teardown: once its parent is
    // gone the child belongs to init, and kill(-pid) can no longer reach it.
    const child = spawn('/bin/sh', ['-c', `exec -a ${MARKER} sleep 30`], {detached: true, stdio: 'ignore'});
    child.unref();
    await settle(300);
    expect(alive(child.pid!)).toBe(true);

    expect(killMatching(MARKER)).toBeGreaterThan(0);
    await settle(500);
    expect(alive(child.pid!)).toBe(false);
  });

  test('no match is not an error', () => {
    // pgrep exits 1 when nothing matches; treating that as a failure would make setup
    // throw on the ordinary case of a clean machine.
    expect(() => killMatching(`${MARKER}-absent`)).not.toThrow();
    expect(killMatching(`${MARKER}-absent`)).toBe(0);
  });

  test('the runner is never in the selection, however broad the pattern', () => {
    // Asked, not performed. The first version of this test ran the kill with the runner's
    // own command line as the pattern — which matches every node process — and SIGTERMed
    // eleven sibling jest workers. Selecting is a question; killing is an act.
    const self = execFileSync('ps', ['-o', 'command=', '-p', String(process.pid)], {encoding: 'utf8'}).trim();
    const word = self.split(/\s+/)[0]; // the node binary path: matches the whole worker pool
    const picked = matchingPids(word);
    expect(picked.length).toBeGreaterThan(0); // the pattern really is that broad
    expect(picked).not.toContain(process.pid);
  });
});
