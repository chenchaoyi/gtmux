// The opt-in names the action, not merely the availability of a live token.
function manualMode(env) {
  if (env.GTMUX_E2E_MANUAL !== '1' || !['capture', 'diagnostics'].includes(env.GTMUX_E2E_TOOL)) {
    throw new Error('Manual tools require GTMUX_E2E_MANUAL=1 and GTMUX_E2E_TOOL=capture|diagnostics');
  }
  return env.GTMUX_E2E_TOOL;
}
function sendTarget(env) {
  const pane = env.GTMUX_E2E_PANE;
  if (env.GTMUX_E2E_OWNED_FIXTURE !== '1' || !/^%[0-9]+$/.test(pane || '') || !env.GTMUX_E2E_TOKEN) {
    throw new Error('Send diagnostic requires an owned synthetic fixture and its exact GTMUX_E2E_PANE');
  }
  const url = new URL(env.GTMUX_E2E_URL || '');
  if (url.protocol !== 'http:' || !['127.0.0.1', '[::1]', 'localhost'].includes(url.hostname) || url.username || url.password || url.search || url.hash || url.pathname !== '/') {
    throw new Error('Send diagnostic requires a loopback HTTP fixture origin');
  }
  return {pane, origin: url.origin, token: env.GTMUX_E2E_TOKEN};
}
async function verifySendTarget(env, request = fetch) {
  const target = sendTarget(env);
  const response = await request(target.origin + '/api/health', {redirect:'error', signal:AbortSignal.timeout(3000)});
  const health = await response.json();
  if (!response.ok || health.fixture !== 'gtmux-e2e-fake-serve') throw new Error('Endpoint is not the synthetic gtmux fixture');
  const agentsResponse = await request(target.origin + '/api/agents', {headers:{Authorization:'Bearer ' + target.token}, redirect:'error', signal:AbortSignal.timeout(3000)});
  const agents = await agentsResponse.json();
  if (!agentsResponse.ok || !Array.isArray(agents) || agents.filter(a=>a.pane_id===target.pane).length!==1) throw new Error('Exact diagnostic pane is absent or ambiguous');
  return target;
}
async function setupManual(env, verify, boot) {
  const mode = manualMode(env);
  if (mode === 'diagnostics') await verify(env);
  return boot();
}
module.exports = {manualMode, sendTarget, verifySendTarget, setupManual};
