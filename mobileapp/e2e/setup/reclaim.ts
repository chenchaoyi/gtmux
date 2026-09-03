// reclaim — clear what a previous e2e run left behind, and clear our own on the way out.
//
// The leak this exists for was measured on 2026-09-04: a WebDriverAgent xcodebuild had
// been running for 25 hours and 50 minutes, restarting the app on the simulator all that
// time, and it was the largest single source of process churn on the machine.
//
// Why the old teardown could not catch it: it group-killed the Appium server
// (`kill(-pid)`), whose comment claimed that caught "the WebDriverAgent xcodebuild
// grandchild". It does — but only while Appium is alive to anchor the group. When a run
// is INTERRUPTED (jest killed, a tool timeout, a Ctrl-C), teardown never runs at all;
// Appium dies with the runner, WDA is re-parented to launchd (measured: ppid=1), and from
// that moment no group kill can reach it. Two runs later there were two orphaned Appium
// servers and a WDA nobody owned.
//
// So the fix is not a better teardown. Teardown is the half that does not run when it is
// needed most. The reliable point is SETUP, which always runs: reclaim first, then start
// clean. Teardown still cleans up so a normal run leaves nothing, but correctness no
// longer depends on it.

import {execFileSync} from 'child_process';

/** Processes are matched by their command line, so the predicate is the contract. */
const WDA_PATTERN = 'appium-webdriveragent/WebDriverAgent.xcodeproj';

/**
 * killMatching is the mechanism, separated from WHAT to kill so it can be tested against
 * a process of the test's own making. A test that exercised the real predicate would kill
 * whatever WebDriverAgent happened to be running on the developer's machine, which is a
 * side effect no unit test may have.
 *
 * Returns how many it signalled.
 */
export function killMatching(pattern: string, signal: NodeJS.Signals = 'SIGTERM'): number {
  const pids = matchingPids(pattern);
  kill(pids, signal);
  return pids.length;
}

/**
 * matchingPids is the SELECTION, separated from the killing so a test can assert what
 * would be signalled without signalling it.
 *
 * That separation is not academic: the first version of this module's test asked
 * killMatching to match the runner's own command line to prove self-exclusion, and
 * SIGTERMed eleven sibling jest workers — a broad pattern over full command lines matches
 * every node process on the machine. Selecting is a question; killing is an act.
 */
export function matchingPids(pattern: string): number[] {
  return pgrep(pattern);
}

function pgrep(pattern: string): number[] {
  try {
    const out = execFileSync('pgrep', ['-f', pattern], {encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore']});
    return out
      .split('\n')
      .map(s => parseInt(s.trim(), 10))
      .filter(n => Number.isFinite(n) && n !== process.pid);
  } catch {
    return []; // pgrep exits 1 when nothing matches
  }
}

function kill(pids: number[], signal: NodeJS.Signals): void {
  for (const pid of pids) {
    try {
      process.kill(pid, signal);
    } catch {
      /* already gone */
    }
  }
}

/**
 * killWebDriverAgents ends every WDA runner, orphaned or not.
 *
 * It matches on the WebDriverAgent project path rather than on a device id: a runner
 * whose session is gone still names the device it was started for, and leaving one
 * behind for "some other device" is how the 25-hour one survived several cleanups that
 * only looked at the current run. The harness owns one simulator; a WDA on this machine
 * is either ours or a previous ours.
 */
export function killWebDriverAgents(): number {
  return killMatching(WDA_PATTERN);
}

/** killStrayAppium ends Appium servers left by an interrupted run (they hold the port). */
export function killStrayAppium(keepPid?: number): number {
  const pids = pgrep('appium --port').filter(p => p !== keepPid);
  kill(pids, 'SIGTERM');
  return pids.length;
}

/**
 * reclaimBefore runs at SETUP: nothing from an earlier run may still be holding the port
 * or driving the simulator. This is the load-bearing half — see the header.
 */
export function reclaimBefore(): void {
  const appium = killStrayAppium();
  const wda = killWebDriverAgents();
  if (appium || wda) {
    // eslint-disable-next-line no-console
    console.log(`[e2e] reclaimed ${appium} stray appium + ${wda} WebDriverAgent from an earlier run`);
  }
}

/**
 * reclaimAfter runs at TEARDOWN, after the Appium group kill: the WDA may already be
 * orphaned by then, so it is killed by predicate rather than by group.
 */
export function reclaimAfter(): void {
  const wda = killWebDriverAgents();
  if (wda) {
    // eslint-disable-next-line no-console
    console.log(`[e2e] stopped ${wda} WebDriverAgent`);
  }
}
