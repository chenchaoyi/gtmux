/**
 * Says out loud how much of the e2e suite actually ran.
 *
 * Most of these suites need a live `gtmux serve` and skip themselves without one
 * (`process.env.GTMUX_E2E_URL ? it : it.skip`). That is the right behaviour — the
 * committed tests hold no secret and no address — but jest's summary line for it
 * ("19 skipped, 8 passed") reads almost exactly like a full green run, and the exit
 * code is 0 either way. A measured run on 2026-09-10 executed 14 of 35 cases and
 * said nothing a reader would stop at.
 *
 * So the run now ends with what it did and did not cover, by name, plus the setting
 * that would have covered the rest. Set GTMUX_E2E_REQUIRE_LIVE=1 to make an
 * incomplete run fail instead of merely saying so — for a caller that means to
 * exercise everything and wants to hear about it when it didn't.
 */
const {readFileSync} = require('fs');
const {basename} = require('path');

// Which setting a suite waits for, read from the suite itself so this cannot drift
// from what the files actually test for.
function gateOf(path) {
  let src = '';
  try {
    src = readFileSync(path, 'utf8');
  } catch {
    return 'unknown';
  }
  if (src.includes('GTMUX_SHOTS')) return 'shots';
  if (src.includes('GTMUX_E2E_URL')) return 'live';
  return 'unknown';
}

const NAME = p => basename(p).replace(/\.test\.ts$/, '');

class CoverageReporter {
  onRunComplete(_contexts, results) {
    const ran = [];
    const idle = []; // every case skipped

    for (const r of results.testResults) {
      if (r.testResults.length === 0) continue;
      const skipped = r.testResults.every(t => t.status === 'pending');
      (skipped ? idle : ran).push(r.testFilePath);
    }
    if (ran.length === 0 && idle.length === 0) return;

    const total = ran.length + idle.length;
    const byGate = {live: [], shots: [], unknown: []};
    for (const p of idle) byGate[gateOf(p)].push(NAME(p));

    const out = [];
    out.push('');
    out.push(`[e2e] ${ran.length} of ${total} suites ran; ${idle.length} did not.`);
    out.push('');
    if (ran.length) out.push(`  ran: ${ran.map(NAME).sort().join(', ')}`);
    if (byGate.live.length) {
      out.push('');
      out.push(`  ${byGate.live.length} suites waited for a live gtmux serve — set GTMUX_E2E_URL and`);
      out.push('  GTMUX_E2E_TOKEN to run them:');
      out.push(`    ${byGate.live.sort().join(', ')}`);
    }
    if (byGate.shots.length) {
      out.push('');
      out.push(`  ${byGate.shots.length} capture screenshots only — they also need GTMUX_SHOTS=1:`);
      out.push(`    ${byGate.shots.sort().join(', ')}`);
    }
    if (byGate.unknown.length) {
      out.push('');
      out.push(`  ${byGate.unknown.length} skipped for a reason this reporter could not read:`);
      out.push(`    ${byGate.unknown.sort().join(', ')}`);
    }
    if (idle.length) {
      out.push('');
      out.push('  A run without those settings drives the app against the bundled fake server.');
      out.push('  It reaches no real pane, so nothing here says the radar, the terminal, HQ,');
      out.push('  the knowledge base or usage work against a real serve.');
    }
    out.push('');
    // eslint-disable-next-line no-console
    console.log(out.join('\n'));

    if (process.env.GTMUX_E2E_REQUIRE_LIVE === '1' && idle.length > 0) {
      // The caller said it meant to run everything. Saying so is not enough then.
      process.exitCode = 1;
      // eslint-disable-next-line no-console
      console.log(
        `[e2e] GTMUX_E2E_REQUIRE_LIVE=1 and ${idle.length} suites did not run — failing the run.\n`,
      );
    }
  }
}

module.exports = CoverageReporter;
