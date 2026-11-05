// Scheduling/report policy only. Actions still use the compiled QA app and driver.
const GROUPS = [
  ['radar', 'radar', [1, 2]],
  ['conversation', 'conversation', [3, 4, 5]],
  ['trace', 'trace-history', [6, 7, 8, 9]],
  ['diff', 'diff', [10]],
  ['conversation', 'fullscreen', [11]],
  ['connection', 'connection', [12, 13, 14]],
  ['settings', 'settings', [15]],
  ['documents', 'document-text', [16, 17]],
  ['documents', 'document-image', [18]],
  ['trace', 'trace-code', [19]],
  ['permissions', 'permissions', [20, 21]],
  ['layout', 'layout', [22]],
  ['sessions', 'sessions', [23]],
  ['settings', 'appearance', [24]],
  ['hq', 'hq', [25]],
  ['input', 'keyboard', [26]],
  ['icons', 'icons', [27]],
  ['attachments', 'file-upload', [28]],
  ['attachments', 'image-markup', [29]],
  ['documents', 'document-pdf', [30]],
  ['trace', 'trace-file', [31]],
];
const dependencies = {
  2: [1],
  4: [3],
  5: [3],
  7: [6],
  8: [7],
  9: [6],
  17: [16],
};
const cases = GROUPS.flatMap(([capability, group, ids]) =>
  ids.map(id => ({ id, capability, group, requires: dependencies[id] || [] })),
);
function selection({ only, blocks } = {}) {
  const parse = x => (x === undefined ? null : x.split(',').map(s => s.trim()));
  const requested = parse(only),
    chosenBlocks = parse(blocks);
  if (
    requested?.some(
      s => !/^\d{2}$/.test(s) || !cases.some(c => c.id === Number(s)),
    )
  )
    throw Error('Unknown Android case selection');
  if (chosenBlocks?.some(s => !GROUPS.some(g => g[0] === s)))
    throw Error('Unknown Android capability block');
  const selected = new Set(
    cases
      .filter(
        c =>
          (!requested || requested.includes(String(c.id).padStart(2, '0'))) &&
          (!chosenBlocks || chosenBlocks.includes(c.capability)),
      )
      .map(c => c.id),
  );
  if (!selected.size) throw Error('Empty Android selection');
  const include = id => {
    for (const dep of cases.find(c => c.id === id).requires) {
      selected.add(dep);
      include(dep);
    }
  };
  for (const id of [...selected]) include(id);
  return selected;
}
function createJourney({
  only,
  blocks,
  setup,
  capture,
  isDisconnected,
  onChange = () => {},
  clock = () => performance.now(),
} = {}) {
  const selected = selection({ only, blocks }),
    results = [],
    groups = new Map(),
    metrics = {};
  let lost;
  async function measure(stage, action) {
    const at = clock();
    try {
      return await action();
    } finally {
      const row = (metrics[stage] ||= { calls: 0, ms: 0 });
      row.calls++;
      row.ms += clock() - at;
    }
  }
  const snapshot = () => ({
    schema: 2,
    selection: [...selected].sort((a, b) => a - b),
    results,
    metrics,
    first_automation_failure: lost || null,
  });
  const record = row => {
    results.push(row);
    onChange(snapshot());
    return row;
  };
  async function check(name, action) {
    const spec = cases.find(c => c.id === Number(name.slice(0, 2)));
    if (!spec) throw Error('Unregistered Android check: ' + name);
    const base = { name, capability: spec.capability, group: spec.group };
    if (!selected.has(spec.id))
      return record({ ...base, status: 'not-run', reason: 'Explicit subset' });
    if (lost)
      return record({
        ...base,
        status: 'not-run',
        blocked_by: lost,
        reason: 'Automation connection unavailable',
      });
    const missing = spec.requires.find(
      id => !results.some(r => Number(r.name.slice(0, 2)) === id),
    );
    const failed = spec.requires
      .map(id => results.find(r => Number(r.name.slice(0, 2)) === id))
      .find(r => r && r.status !== 'passed');
    if (missing || failed)
      return record({
        ...base,
        status: 'blocked',
        blocked_by:
          failed?.blocked_by ||
          failed?.name ||
          String(missing).padStart(2, '0'),
        reason: 'Required check did not pass',
      });
    const state = groups.get(spec.group);
    if (state?.error)
      return record({
        ...base,
        status: 'blocked',
        blocked_by: state.root,
        reason: 'Block setup failed: ' + state.error,
      });
    if (!state) {
      try {
        await measure('setup:' + spec.group, () => setup(spec));
        groups.set(spec.group, { ok: true });
      } catch (error) {
        const root = 'setup:' + spec.group;
        groups.set(spec.group, { root, error: error.message });
        if (isDisconnected(error)) lost = root;
        try {
          await capture(root.replace(':', '-') + '-failed');
        } catch (captureError) {
          if (isDisconnected(captureError)) lost ||= root;
        }
        return record({
          ...base,
          status: 'blocked',
          blocked_by: root,
          error: error.message,
          reason: 'Block setup failed',
        });
      }
    }
    const at = clock();
    let phase = 'action',
      actionMs = 0,
      evidenceMs = 0;
    try {
      await action();
      actionMs = clock() - at;
      phase = 'evidence';
      const evidenceAt = clock();
      await capture(name);
      evidenceMs = clock() - evidenceAt;
      return record({
        ...base,
        status: 'passed',
        action_ms: actionMs,
        evidence_ms: evidenceMs,
        seconds: (clock() - at) / 1000,
      });
    } catch (error) {
      if (phase === 'action') actionMs = clock() - at;
      else evidenceMs = clock() - at - actionMs;
      if (isDisconnected(error)) lost = name;
      try {
        await capture(name + '-failed');
      } catch (captureError) {
        if (isDisconnected(captureError)) lost ||= name;
      }
      return record({
        ...base,
        status: 'failed',
        phase,
        error: error.message,
        action_ms: actionMs,
        evidence_ms: evidenceMs,
        seconds: (clock() - at) / 1000,
      });
    }
  }
  return { check, measure, snapshot, results };
}
module.exports = { cases, selection, createJourney };
