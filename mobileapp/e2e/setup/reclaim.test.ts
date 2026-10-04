import {execFileSync, spawn} from 'child_process';
import {appiumPattern, killMatching, matchingPids, wdaPattern} from './reclaim';

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
    //
    // Spawned as node with the marker as an argument, so the marker is in the command line
    // pgrep matches. The first version used `sh -c 'exec -a <marker> sleep'`, which works
    // in bash and not in dash — so on CI the probe exited immediately and the test failed
    // on its own setup rather than on the behaviour.
    const child = spawn(process.execPath, ['-e', 'setTimeout(() => {}, 30000)', MARKER], {
      detached: true,
      stdio: 'ignore',
    });
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

// A run with its own Appium port, WDA directory or simulator reclaims only its own; without
// them the harness owns the machine and reclaims every WDA and Appium server, as before.
// The patterns are pgrep -f EREs; JS RegExp reads these constructs the same way.
describe('reclaim is scoped when a run has its own port, WDA directory or simulator', () => {
  const m = (pattern: string, line: string) => new RegExp(pattern).test(line);
  it('an Appium port is matched whole', () => {
    const p = appiumPattern({GTMUX_E2E_APPIUM_PORT: '4731'});
    expect(m(p, 'node /x/appium --port 4731 --log /tmp/a.log')).toBe(true);
    expect(m(p, 'node /x/appium --port 4731')).toBe(true);
    expect(m(p, 'node /x/appium --port 47310 --log /tmp/a.log')).toBe(false);
    expect(m(p, 'node /x/appium --port 4723 --log /tmp/a.log')).toBe(false);
  });
  it('a WDA directory is matched whole, its dots literal', () => {
    const p = wdaPattern({GTMUX_E2E_WDA_DERIVED: '/tmp/wda.switch'});
    expect(m(p, 'xcodebuild -project /a/appium-webdriveragent/WebDriverAgent.xcodeproj -derivedDataPath /tmp/wda.switch -scheme W')).toBe(true);
    expect(m(p, 'xcodebuild -derivedDataPath /tmp/wda.switch2 -scheme W')).toBe(false);
    expect(m(p, 'xcodebuild -derivedDataPath /tmp/wdaXswitch -scheme W')).toBe(false);
  });
  it('without a directory, the simulator scopes it', () => {
    const p = wdaPattern({GTMUX_E2E_UDID: '28D97331-F8C8'});
    expect(m(p, 'xcodebuild -project /a/appium-webdriveragent/WebDriverAgent.xcodeproj -destination id=28D97331-F8C8 -x')).toBe(true);
    expect(m(p, 'xcodebuild -project /a/appium-webdriveragent/WebDriverAgent.xcodeproj -destination id=28D97331-F8C8-9 -x')).toBe(false);
    expect(m(p, 'xcodebuild -project /a/appium-webdriveragent/WebDriverAgent.xcodeproj -destination id=8C17EE01 -x')).toBe(false);
  });
  it('a scoped WDA pattern selects through pgrep itself, not only as a regex', async () => {
    // The WDA predicate starts with "-derivedDataPath". Handed to pgrep as is, it was read
    // as an option: pgrep exited 2 with its usage and the selection came back empty, so the
    // runner the scope named was never reclaimed — one ran on for hours (2026-10-05). A
    // regex check cannot see that; only pgrep can. The probe is the test's own process.
    const dir = `/tmp/gtmux-reclaim-probe-${process.pid}.wda`;
    const child = spawn(process.execPath, ['-e', 'setTimeout(() => {}, 30000)', '--', '-derivedDataPath', dir, '-scheme', 'W'], {
      detached: true,
      stdio: 'ignore',
    });
    child.unref();
    await settle(300);
    try {
      expect(matchingPids(wdaPattern({GTMUX_E2E_WDA_DERIVED: dir}))).toContain(child.pid);
    } finally {
      process.kill(child.pid!, 'SIGTERM');
    }
  });
  it('unscoped by default', () => {
    expect(appiumPattern({})).toBe('appium --port');
    expect(wdaPattern({})).toBe('appium-webdriveragent/WebDriverAgent.xcodeproj');
  });
});
