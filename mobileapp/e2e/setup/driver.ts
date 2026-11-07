import {appendFileSync, lstatSync, mkdirSync, realpathSync} from 'fs';
import {join, relative, resolve, sep} from 'path';
import type {Browser} from 'webdriverio';

/**
 * Singleton accessor for the shared driver + artifacts dir. The Browser is
 * created in global-setup.ts and stashed on globalThis so test files can reach
 * it without re-creating a session.
 */
declare global {
  // eslint-disable-next-line no-var
  var __E2E_DRIVER__: Browser | undefined;
  // eslint-disable-next-line no-var
  var __E2E_ARTIFACTS_DIR__: string | undefined;
  // eslint-disable-next-line no-var
  var __E2E_SERVER_PID__: number | undefined;
}

export function getDriver(): Browser {
  if (!globalThis.__E2E_DRIVER__) {
    throw new Error('[e2e] No driver — did global-setup.ts run? Import driver.ts only from e2e tests.');
  }
  return globalThis.__E2E_DRIVER__;
}

export function getArtifactsDir(): string {
  if (!globalThis.__E2E_ARTIFACTS_DIR__) {
    throw new Error('[e2e] Artifacts dir not set by global-setup.ts');
  }
  return globalThis.__E2E_ARTIFACTS_DIR__;
}


/** An explicit migration journey owns one fresh session directory. Legacy local
 * runs retain their current timestamp/latest behavior. No fallback on bad input. */
export function allocateArtifacts(appRoot: string, env: NodeJS.ProcessEnv = process.env): {runDir: string; isolated: boolean} {
  const explicit = env.GTMUX_E2E_ARTIFACTS_DIR;
  const evidenceRoot = env.GTMUX_SCENE_EVIDENCE_ROOT;
  if (explicit || evidenceRoot) {
    if (!explicit || !evidenceRoot || !env.RUNNER_TEMP) throw new Error('Incomplete scene artifact identity');
    const inputParent = resolve(env.RUNNER_TEMP, 'scene-migration');
    const parent = realpathSync(inputParent);
    const child = relative(inputParent, resolve(evidenceRoot));
    if (!child || child === '..' || child.startsWith('..' + sep)) {
      throw new Error('Scene evidence root must be an owned child');
    }
    // Accept the platform's /tmp or /var alias at RUNNER_TEMP, but never a
    // symlink introduced inside this job's evidence namespace.
    let checked = parent;
    for (const component of child.split(sep)) {
      checked = join(checked, component);
      if (lstatSync(checked).isSymbolicLink()) throw new Error('Linked scene evidence directory');
    }
    const owned = realpathSync(evidenceRoot);
    if (owned !== checked || resolve(explicit) !== join(resolve(evidenceRoot), 'session')) {
      throw new Error('Scene artifact directory must be its own session');
    }
    const runDir = join(owned, 'session');
    mkdirSync(runDir); // Exclusive; never merge an earlier attempt or journey.
    return {runDir, isolated: true};
  }
  const stamp = new Date().toISOString().replace(/[-:T]/g, '').replace(/\..+$/, '');
  const runDir = join(appRoot, '.e2e-artifacts', stamp);
  mkdirSync(runDir, {recursive: true});
  return {runDir, isolated: false};
}

/** Only completed behavior assertions emit migration milestones. The coordinator
 * requires the exact reviewed set in addition to the actual Jest case results. */
export function recordSceneStep(step: string): void {
  if (process.env.GTMUX_SCENE_EVIDENCE_ROOT) {
    appendFileSync(join(getArtifactsDir(), 'behavior-steps.jsonl'), JSON.stringify({step, time: Date.now()}) + '\n');
  }
}
