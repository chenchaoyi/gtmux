// gtmux native DeepSeek Harness terminal bridge; generated, do not edit.
// The native command receives the exact agent selected by the TUI channel.
import {constants, openSync, readFileSync, writeFileSync, closeSync, fstatSync, lstatSync, mkdirSync, renameSync, unlinkSync, realpathSync} from 'node:fs';
import {homedir} from 'node:os';
import {join, resolve} from 'node:path';
import {randomBytes, createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {observe, hookSink} from '../gtmux.mjs';

export const name = 'gtmux-dsh-terminal';
const COMPONENT_COMMAND = 'gtmux.dsh-send';
const MAX_REQUEST = 1 << 20;
const HEX = /^[a-f0-9]{32}$/;

export function readJSON(file, max = 256 << 10) {
  const fd = openSync(file, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const stat = fstatSync(fd);
    if (!stat.isFile() || stat.size > max || (stat.mode & 0o022) || stat.uid !== process.getuid()) throw new Error('Untrusted bridge metadata');
    const bytes = readFileSync(fd);
    if (bytes.length > max) throw new Error('Bridge metadata exceeds limit');
    return JSON.parse(bytes.toString('utf8'));
  } finally { closeSync(fd); }
}

function directory(path) {
  mkdirSync(path, {recursive: true, mode: 0o700});
  const stat = lstatSync(path);
  if (!stat.isDirectory() || stat.isSymbolicLink() || stat.uid !== process.getuid() || (stat.mode & 0o077)) throw new Error('Untrusted bridge directory');
}

function atomicJSON(file, value) {
  const tmp = `${file}.${randomBytes(8).toString('hex')}.tmp`;
  try {
    writeFileSync(tmp, JSON.stringify(value), {mode: 0o600, flag: 'wx'});
    renameSync(tmp, file);
  } finally { try { unlinkSync(tmp); } catch {} }
}

// Discovery names the currently bound session, while mounts includes background
// sessions too. Qualified non-DSH IDs must never borrow a DSH history's identity.
export function foreground(dataDir, pid) {
  const servers = readJSON(join(dataDir, 'inject', 'servers.json'));
  const mounts = readJSON(join(dataDir, 'session-mounts.json'));
  if (!Array.isArray(servers) || servers.length > 256 || mounts.version !== 1 || !Array.isArray(mounts.owners) || mounts.owners.length > 256) return;
  const selected = servers.filter(s => s?.pid === pid);
  if (selected.length !== 1) return;
  const current = selected[0];
  if (typeof current.sessionId !== 'string' || !current.sessionId || current.sessionId.length > 256 || !Number.isSafeInteger(current.startedAt) || current.startedAt <= 0) return;
  const owners = mounts.owners.filter(owner => owner?.pid === pid);
  if (owners.length !== 1 || !Array.isArray(owners[0].sessionIds) || owners[0].sessionIds.length > 256) return;
  const ids = owners[0].sessionIds;
  if (!ids.includes(current.sessionId) || ids.some(id => typeof id === 'string' && id.endsWith(`:${current.sessionId}`))) return;
  return {session_id: current.sessionId, cwd: typeof current.cwd === 'string' ? current.cwd : '', binding_epoch: current.startedAt};
}

// Exported for synthetic contract tests. Claiming before delivery makes an
// ambiguous crash non-repeatable; a retry cannot enqueue the same input twice.
export function receive(requestDir, expected, invocation, now = Date.now()) {
  const id = invocation.rawInput.trim();
  if (!HEX.test(id)) return {kind: 'error', text: 'Invalid gtmux request'};
  const receiptPath = join(requestDir, `${id}.receipt.json`);
  const pending = join(requestDir, `${id}.pending.json`);
  const claimed = join(requestDir, `${id}.claimed.json`);
  const reply = (state, error = '') => {
    const receipt = {version: 1, request_id: id, instance: expected.instance, home_id: expected.home_id, session_id: invocation.agent?.session?.id || '', pid: expected.pid, state, error, created_at: now};
    atomicJSON(receiptPath, receipt);
    return state === 'accepted' ? {kind: 'success', text: 'Message accepted by DeepSeek Harness'} : {kind: 'error', text: error};
  };
  try {
    try {
      const old = readJSON(receiptPath, 8192);
      if (old.request_id === id && old.instance === expected.instance && old.home_id === expected.home_id && old.session_id === invocation.agent?.session?.id) {
        return old.state === 'accepted' ? {kind: 'success', text: 'Message already accepted'} : {kind: 'error', text: old.error || 'Request was not accepted'};
      }
      return {kind: 'error', text: 'Request receipt belongs to another target'};
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
    const request = readJSON(pending, MAX_REQUEST + 8192);
    const agent = invocation.agent;
    if (request.version !== 1 || request.request_id !== id || request.instance !== expected.instance || request.pid !== expected.pid ||
        request.home_id !== expected.home_id || request.session_id !== expected.session_id || agent?.session?.id !== request.session_id ||
        request.binding_epoch !== expected.binding_epoch || request.server_epoch !== expected.server_epoch || request.pane !== expected.pane ||
        !Number.isSafeInteger(request.created_at) || now < request.created_at - 1000 || now - request.created_at > 15000 ||
        typeof request.text !== 'string' || !request.text.trim() || Buffer.byteLength(request.text) > MAX_REQUEST || invocation.signal?.aborted ||
        agent.session?.header?.version !== 4 || agent.session.header.origin === 'subagent' || typeof agent.followup !== 'function') {
      return reply('refused', 'The target conversation or terminal binding changed; message was not sent');
    }
    // Exclusive creation prevents a racing duplicate from claiming again.
    // Rename alone overwrites an existing claim on POSIX.
    const claim = openSync(claimed, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
    closeSync(claim);
    unlinkSync(pending);
    try {
      agent.followup({id, role: 'user', source: {kind: 'user'}, content: [{type: 'text', text: request.text}]});
    } catch {
      return reply('uncertain', 'Input outcome is uncertain; do not resend automatically');
    }
    return reply('accepted');
  } catch (error) {
    return {kind: 'error', text: error.code === 'EEXIST' ? 'Request was already claimed; do not resend automatically' : 'Native input request is unavailable; message was not sent'};
  }
}

export function apply(ctx) {
  const pane = process.env.TMUX_PANE || '';
  if (!/^%[0-9]+$/.test(pane)) return;
  const pid = process.pid;
  const instance = randomBytes(16).toString('hex');
  const startedAt = Date.now();
  const dataDir = join(homedir(), '.dsh-tui');
  let home = resolve(process.env.DSH_HOME || join(homedir(), '.dsh'));
  try { home = realpathSync(home); } catch { return; }
  const homeID = createHash('sha256').update(`dsh\0${home}`).digest('hex').slice(0, 32);
  const stateRoot = join(homedir(), '.local', 'share', 'gtmux');
  const bridgeDir = join(stateRoot, 'terminal-clients');
  const requestDir = join(stateRoot, 'terminal-input');
  try { directory(stateRoot); directory(bridgeDir); directory(requestDir); } catch { return; }
  let serverEpoch;
  try { serverEpoch = execFileSync('tmux', ['display-message', '-p', '-t', pane, '#{start_time}'], {encoding: 'utf8', timeout: 1000, maxBuffer: 1024}).trim(); } catch { return; }
  if (!/^[0-9]+$/.test(serverEpoch)) return;
  const leaseFile = join(bridgeDir, `${pid}.json`);
  let current, disposeCommand, stopped = false;
  const sink = hookSink(ctx, () => ({terminal_client: 'dst', client_pid: pid, client_instance: instance}));
  const refresh = () => {
    if (stopped) return;
    try {
      const host = ctx.get('tuiPluginHost', false);
      if (!host) return;
      if (!disposeCommand) disposeCommand = host.registerCommand(ctx, COMPONENT_COMMAND, {
        name: 'gtmux-send', description: 'Deliver a session-targeted gtmux message', recordInput: false,
        handler: invocation => {
          refresh();
          if (!current?.input_ready) return {kind: 'error', text: 'Terminal attachment is not ready'};
          return receive(requestDir, current, invocation);
        },
      });
      const selected = foreground(dataDir, pid);
      if (!selected) {
        current = undefined;
        try { unlinkSync(leaseFile); } catch {}
        return;
      }
      const nativeSession = ctx.get('sessions', false)?.get(selected.session_id);
      if (nativeSession?.header?.version !== 4 || nativeSession.header.origin === 'subagent' || nativeSession.id !== selected.session_id) {
        current = undefined;
        try { unlinkSync(leaseFile); } catch {}
        return;
      }
      const previous = current;
      current = {version: 1, agent: 'dsh', client: 'dst', pane, pid, instance, home_id: homeID, ...selected,
        native_session: true, server_epoch: serverEpoch, started_at: startedAt, updated_at: Date.now(),
        input_ready: host.grants.allows(ctx, 'commands.invoke', COMPONENT_COMMAND)};
      atomicJSON(leaseFile, current);
      if (!previous || previous.session_id !== current.session_id || previous.binding_epoch !== current.binding_epoch) {
        sink('SessionStart', {session_id: current.session_id, cwd: current.cwd});
      }
    } catch {
      current = undefined;
      try { unlinkSync(leaseFile); } catch {}
    }
  };
  observe(ctx, (token, payload) => { refresh(); sink(token, payload); });
  const timer = setInterval(refresh, 1000);
  timer.unref();
  refresh();
  ctx.on('dispose', () => {
    stopped = true;
    clearInterval(timer);
    disposeCommand?.();
    try { const old = readJSON(leaseFile, 16384); if (old.instance === instance) unlinkSync(leaseFile); } catch {}
  });
}
