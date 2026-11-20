// gtmux native DeepSeek Harness terminal bridge; generated, do not edit.
// The native command receives the exact agent selected by the TUI channel.
import {constants, openSync, opendirSync, readSync, writeFileSync, closeSync, fsyncSync, linkSync, fstatSync, lstatSync, mkdirSync, renameSync, unlinkSync, realpathSync} from 'node:fs';
import {homedir} from 'node:os';
import {dirname, join, resolve} from 'node:path';
import {randomBytes, createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {observe, hookSink} from '../gtmux.mjs';

export const name = 'gtmux-dsh-terminal';
const COMPONENT_COMMAND = 'gtmux.dsh-send';
const MAX_REQUEST = 1 << 20;
const HEX = /^[a-f0-9]{32}$/;

const utf8 = new TextDecoder('utf-8', {fatal: true, ignoreBOM: true});
const jsonTokens = /"(?:[^"\\\x00-\x1f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*"|-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?|[{}\[\]:,]/gu;

function parseInputJSON(bytes, integersOnly = false) {
  const text = utf8.decode(bytes);
  const result = JSON.parse(text);
  const objects = [];
  let depth = 0;
  for (const match of text.matchAll(jsonTokens)) {
    const token = match[0];
    if (token === '{' || token === '[') {
      if (++depth > 64) throw new Error('Bridge metadata is too deeply nested');
      objects.push(token === '{' ? new Set() : null);
    } else if (token === '}' || token === ']') {
      depth--;
      objects.pop();
    } else if (token[0] === '"') {
      // Compare each decoded key, so JSON escaping cannot hide a duplicate.
      const value = JSON.parse(token);
      if (/[\uD800-\uDFFF]/u.test(value)) throw new Error('Invalid Unicode in bridge metadata');
      const keys = objects.at(-1);
      let end = match.index + token.length;
      while (/[ \t\r\n]/.test(text[end] ?? '')) end++;
      if (keys && text[end] === ':') {
        if (keys.has(value)) throw new Error('Duplicate key in bridge metadata');
        keys.add(value);
      }
    } else if (integersOnly && /^[-0-9]/.test(token) &&
               (!/^-?[0-9]+$/.test(token) || !Number.isSafeInteger(Number(token)))) {
      throw new Error('Invalid integer in input record');
    }
  }
  return result;
}

function readBounded(fd, size) {
  const bytes = Buffer.alloc(size + 1);
  let length = 0;
  while (length < bytes.length) {
    const count = readSync(fd, bytes, length, bytes.length - length, null);
    if (count === 0) break;
    length += count;
  }
  if (length !== size) throw new Error('Bridge metadata changed while reading');
  return bytes.subarray(0, length);
}

function readJSONFromFD(fd, max, privateFile) {
  const info = fstatSync(fd, {bigint: true});
  if (!info.isFile() || info.size > BigInt(max) || (info.mode & 0o022n) || info.uid !== BigInt(process.getuid()) ||
      (privateFile && ((info.mode & 0o077n) || info.nlink !== 1n))) throw new Error('Untrusted bridge metadata');
  return {info, value: parseInputJSON(readBounded(fd, Number(info.size)), privateFile)};
}

export function readJSON(file, max = 256 << 10, privateFile = false) {
  const fd = openSync(file, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    return readJSONFromFD(fd, max, privateFile).value;
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

const addressFields = ['version', 'request_id', 'instance', 'home_id', 'session_id', 'pid'];
const requestFields = [...addressFields, 'pane', 'server_epoch', 'binding_epoch', 'created_at', 'text', 'text_hash', 'durable_retry', 'index_key'];
const receiptFields = [...addressFields, 'state', 'delivery', 'error', 'created_at'];

function inputRecord(value, fields) {
  return value && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).every(key => fields.includes(key) && value[key] !== null) &&
    value.version === 1 && typeof value.request_id === 'string' && HEX.test(value.request_id) &&
    typeof value.instance === 'string' && typeof value.home_id === 'string' && typeof value.session_id === 'string' &&
    Number.isSafeInteger(value.pid);
}

function validRequest(value) {
  return inputRecord(value, requestFields) &&
    typeof value.pane === 'string' && typeof value.server_epoch === 'string' &&
    Number.isSafeInteger(value.binding_epoch) && Number.isSafeInteger(value.created_at) && value.created_at > 0 &&
    typeof value.text === 'string' && Boolean(value.text.trim()) && Buffer.byteLength(value.text) <= MAX_REQUEST &&
    (value.text_hash === undefined || value.text_hash === '' ||
      value.text_hash === createHash('sha256').update(value.text).digest('hex')) &&
    (value.durable_retry === undefined || typeof value.durable_retry === 'boolean') &&
    (value.index_key === undefined || (typeof value.index_key === 'string' && HEX.test(value.index_key)));
}

function validReceipt(value) {
  return inputRecord(value, receiptFields) && ['accepted', 'refused', 'uncertain'].includes(value.state) &&
    (value.error === undefined || typeof value.error === 'string') &&
    (value.created_at === undefined || (Number.isSafeInteger(value.created_at) && value.created_at >= 0)) &&
    (value.delivery === undefined || value.delivery === '' ||
      (value.state === 'accepted' && ['active-turn', 'started-turn', 'wake-accepted'].includes(value.delivery)));
}

const UNKNOWN_INPUT = 'Input outcome is uncertain; check the conversation before retrying';
const TARGET_CHANGED = 'The target conversation or terminal binding changed; message was not sent';
const DEFAULT_DURABILITY = {syncFile:fsyncSync, syncDir:fsyncSync, link:linkSync, unlink:unlinkSync};

function withInputQueue(requestDir, expectedDirectory, ops, work) {
  const fd = openSync(requestDir, constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
  try {
    const dir = fstatSync(fd, {bigint: true});
    const identity = layoutIdentity(dir);
    if (!dir.isDirectory() || !privateLayoutEntry(dir) || (expectedDirectory && identity !== expectedDirectory)) {
      throw new Error('Terminal input store changed');
    }
    const check = () => {
      const live = lstatSync(requestDir, {bigint: true});
      if (!live.isDirectory() || !privateLayoutEntry(live) || layoutIdentity(live) !== identity) {
        throw new Error('Terminal input store changed');
      }
    };
    check();
    return work({
      check, ops,
      syncParents() {
        // The managed queue is HOME/.local/share/gtmux/<queue>. Direct
        // fixture callers have an explicitly isolated immediate parent.
        let path = requestDir;
        const depth = expectedDirectory ? 4 : 1;
        for (let i=0; i<depth; i++) {
          check();
          path = dirname(path);
          const parent = openSync(path, constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
          try { (ops.syncParent ?? fsyncSync)(parent); } finally { closeSync(parent); }
        }
        check();
      },
      syncDir() { check(); ops.syncDir(fd); check(); },
      unlink(path) { check(); ops.unlink(path); },
    });
  } finally { try { closeSync(fd); } catch {} }
}

function writeDurableInput(file, bytes, queue, temporary = false) {
  queue.check();
  const fd = openSync(file, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
  try {
    try {
      writeFileSync(fd, bytes);
      queue.ops.syncFile(fd);
    } finally { closeSync(fd); }
  } catch (error) {
    if (temporary) { try { queue.unlink(file); } catch {} }
    throw error;
  }
}

function publishReceipt(path, receipt, queue) {
  const bytes = JSON.stringify(receipt);
  if (Buffer.byteLength(bytes) > 8192) throw new Error('Input receipt exceeds its limit');
  const tmp = join(dirname(path), '.input-receipt-' + randomBytes(12).toString('hex'));
  writeDurableInput(tmp, bytes, queue, true);
  let tempPresent = true;
  try {
    queue.check();
    queue.ops.link(tmp, path);
    queue.unlink(tmp);
    tempPresent = false;
    queue.syncDir();
  } finally {
    if (tempPresent) { try { queue.unlink(tmp); } catch {} }
  }
}

function adoptReceipt(path, id, expected, invocation, queue) {
  queue.check();
  const fd = openSync(path, constants.O_RDWR | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const {info, value} = readJSONFromFD(fd, 8192, true);
    if (!validReceipt(value) || value.request_id !== id || value.instance !== expected.instance ||
        value.home_id !== expected.home_id || value.pid !== expected.pid ||
        (value.state === 'accepted' && (value.session_id !== expected.session_id || value.session_id !== invocation.agent?.session?.id))) {
      throw new Error('Invalid or mismatched input receipt');
    }
    queue.ops.syncFile(fd);
    queue.syncDir();
    const named = lstatSync(path, {bigint: true});
    if (layoutIdentity(named) !== layoutIdentity(info) || named.nlink !== 1n) {
      throw new Error('Input receipt changed');
    }
    return value;
  } catch (error) {
    // Once opened, a missing pathname during adoption is an uncertain result.
    throw new Error('Input receipt could not be verified', {cause:error});
  } finally { closeSync(fd); }
}

function matchesInputIndex(saved, request) {
  return saved && typeof saved === 'object' && saved.text === '' &&
    Object.keys(saved).every(key => requestFields.includes(key) && saved[key] !== null) &&
    requestFields.every(key => key === 'text' || saved[key] === request[key]);
}

function findInputIndex(dir, request, queue) {
  if (request.index_key) return join(dir, request.index_key+'.index.json');
  // Older v1 pending records do not name their index. Adopt only a unique
  // match, within the Go receiver's independent directory scan bound.
  queue.check();
  const listing = opendirSync(dir);
  let match;
  try {
    for (let count=0;; count++) {
      const entry = listing.readSync();
      if (!entry) break;
      if (count === 8192) throw new Error('Input index scan exceeds its limit');
      if (!/^[a-f0-9]{32}\.index\.json$/.test(entry.name)) continue;
      const path = join(dir, entry.name);
      let saved;
      try { saved = readJSON(path, 8192, true); } catch { continue; }
      if (!matchesInputIndex(saved, request)) continue;
      if (match) throw new Error('Duplicate request index');
      match = path;
    }
  } finally { listing.closeSync(); }
  if (!match) throw new Error('Input index unavailable');
  return match;
}

function syncBoundInputIndex(dir, request, queue) {
  const path = findInputIndex(dir, request, queue);
  queue.check();
  const fd = openSync(path, constants.O_RDWR | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const {info, value} = readJSONFromFD(fd, 8192, true);
    if (!matchesInputIndex(value, request)) throw new Error('Input index changed');
    queue.ops.syncFile(fd);
    queue.syncDir();
    const named = lstatSync(path, {bigint:true});
    if (layoutIdentity(named) !== layoutIdentity(info) || named.nlink !== 1n) throw new Error('Input index changed');
    queue.syncParents();
  } finally { closeSync(fd); }
}

// Exported for synthetic contract tests. The receiving process must claim
// the record durably before invoking the bound agent.
export function receive(requestDir, expected, invocation, now = Date.now(), expectedDirectory, ops = DEFAULT_DURABILITY) {
  const id = invocation.rawInput.trim();
  if (!HEX.test(id)) return {kind: 'error', text: 'Invalid gtmux request'};
  const receiptPath = join(requestDir, id+'.receipt.json');
  const pending = join(requestDir, id+'.pending.json');
  const claimed = join(requestDir, id+'.claimed.json');
  let attempted = false;
  try {
    return withInputQueue(requestDir, expectedDirectory, ops, queue => {
      try {
        const old = adoptReceipt(receiptPath, id, expected, invocation, queue);
        if (old.state === 'accepted') return {kind: 'success', text: 'Message already accepted'};
        return {kind: 'error', text: old.error || 'Request was not accepted'};
      } catch (error) {
        if (error.code !== 'ENOENT') return {kind: 'error', text: UNKNOWN_INPUT};
      }
      if (optionalStat(claimed)) return {kind:'error', text:UNKNOWN_INPUT};
      const request = readJSON(pending, 2 << 20, true);
      if (!validRequest(request)) throw new Error('Invalid input request');
      if (request.request_id !== id || request.instance !== expected.instance || request.home_id !== expected.home_id ||
          request.pid !== expected.pid) return {kind: 'error', text: TARGET_CHANGED};

      const agent = invocation.agent;
      const refused = request.session_id !== expected.session_id || agent?.session?.id !== request.session_id ||
        request.binding_epoch !== expected.binding_epoch || request.server_epoch !== expected.server_epoch || request.pane !== expected.pane ||
        now < request.created_at - 1000 || now - request.created_at > 15000 || invocation.signal?.aborted ||
        agent.session?.header?.version !== 4 || agent.session.header.origin === 'subagent' || typeof agent.followup !== 'function';

      // A refusal contends for the same claim, so it cannot race an acceptance.
      writeDurableInput(claimed, '', queue);
      syncBoundInputIndex(requestDir, request, queue);
      if (optionalStat(receiptPath)) return {kind:'error', text:UNKNOWN_INPUT};
      queue.unlink(pending);
      let state = 'refused', error = TARGET_CHANGED;
      if (!refused) {
        attempted = true;
        try {
          agent.followup({id, role: 'user', source: {kind: 'user'}, content: [{type: 'text', text: request.text}]});
          state = 'accepted';
          error = '';
        } catch {
          state = 'uncertain';
          error = UNKNOWN_INPUT;
        }
      }
      const receipt = {version: 1, request_id: id, instance: expected.instance, home_id: expected.home_id,
        session_id: request.session_id, pid: expected.pid, state, error, created_at: now};
      publishReceipt(receiptPath, receipt, queue);
      // Keep the v1 empty marker on delivery and unknown outcome. A refused
      // request can retire it only after the directory commit above.
      if (state === 'refused') { try { queue.unlink(claimed); } catch {} }
      return state === 'accepted' ? {kind:'success', text:'Message accepted by DeepSeek Harness'} : {kind:'error', text:error};
    });
  } catch (error) {
    if (attempted || error.code === 'EEXIST') return {kind:'error', text:UNKNOWN_INPUT};
    return {kind:'error', text:'Native input request is unavailable'};
  }
}

// Keep this journal, marker and identity contract aligned with
// internal/agentclient/input_store.go. A legacy receiver cannot open a marker
// as a directory. Real cutover still requires stopped ingress and drained fds.
function optionalStat(path) {
  try { return lstatSync(path, {bigint: true}); }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
}

function privateLayoutEntry(info) {
  return info && info.uid === BigInt(process.getuid()) && (info.mode & 0o077n) === 0n;
}

function layoutIdentity(info) {
  return `${info.dev}:${info.ino}`;
}

function exactFields(value, fields) {
  return value && Object.keys(value).length === fields.length &&
    fields.every(key => Object.hasOwn(value, key));
}

function identityMatches(info, expected) {
  return exactFields(expected, ['dev', 'ino']) &&
    String(info.dev) === expected.dev && String(info.ino) === expected.ino;
}

function readLayoutDocument(path, fields) {
  const fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const info = fstatSync(fd, {bigint: true});
    if (!privateLayoutEntry(info) || !info.isFile() || info.nlink !== 1n || info.size > 4096n) {
      throw new Error('Untrusted input store metadata');
    }
    const value = parseInputJSON(readBounded(fd, Number(info.size)));
    if (!exactFields(value, fields)) throw new Error('Invalid input store metadata');
    return {info, value};
  } finally { closeSync(fd); }
}

function inputStore(stateRoot) {
  const legacy = join(stateRoot, 'terminal-input');
  const active = join(stateRoot, 'terminal-input-v2');
  const journal = join(stateRoot, 'terminal-input-cutover.json');
  try {
    const root = optionalStat(stateRoot);
    if (root && (!privateLayoutEntry(root) || !root.isDirectory())) return;
    if (!optionalStat(journal)) {
      if (optionalStat(active)) return;
      const info = optionalStat(legacy);
      if (info && (!privateLayoutEntry(info) || !info.isDirectory())) return;
      return {path:legacy, generation:'', directory:info ? layoutIdentity(info) : '', marker:''};
    }
    const {value: record} = readLayoutDocument(journal, ['version', 'phase', 'token', 'directory', 'marker']);
    if (record.version !== 1 || record.phase !== 'active' || typeof record.token !== 'string' || !HEX.test(record.token)) return;
    const target = optionalStat(active);
    if (!privateLayoutEntry(target) || !target.isDirectory() || !identityMatches(target, record.directory)) return;
    const {info, value:marker} = readLayoutDocument(legacy, ['version', 'token']);
    if (info.dev !== target.dev || !identityMatches(info, record.marker) || marker.version !== 1 || marker.token !== record.token) return;
    return {path:active, generation:record.token, directory:layoutIdentity(target), marker:layoutIdentity(info)};
  } catch {}
}

function sameInputStore(before, after) {
  return after && before.path === after.path && before.generation === after.generation &&
    before.directory === after.directory && before.marker === after.marker;
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
  const before = inputStore(stateRoot);
  if (!before) return;
  try {
    directory(stateRoot);
    directory(bridgeDir);
    if (!before.directory) directory(before.path);
  } catch { return; }
  const store = inputStore(stateRoot);
  if (!store?.directory || (before.directory ? !sameInputStore(before, store) :
      store.path !== before.path || store.generation !== before.generation)) return;
  let serverEpoch;
  try { serverEpoch = execFileSync('tmux', ['display-message', '-p', '-t', pane, '#{start_time}'], {encoding: 'utf8', timeout: 1000, maxBuffer: 1024}).trim(); } catch { return; }
  if (!/^[0-9]+$/.test(serverEpoch)) return;
  const leaseFile = join(bridgeDir, `${pid}.json`);
  let current, commandHost, disposeCommand, stopped = false;
  const sink = hookSink(ctx, () => ({terminal_client: 'dst', client_pid: pid, client_instance: instance}));
  const invalidate = () => {
    current = undefined;
    try { unlinkSync(leaseFile); } catch {}
  };
  const refresh = () => {
    if (stopped) return;
    try {
      const host = ctx.get('tuiPluginHost', false);
      if (!host) {
        invalidate();
        return;
      }
      if (host !== commandHost) {
        invalidate();
        disposeCommand?.();
        commandHost = undefined;
        disposeCommand = host.registerCommand(ctx, COMPONENT_COMMAND, {
          name: 'gtmux-send', description: 'Deliver a session-targeted gtmux message', recordInput: false,
          handler: invocation => {
            refresh();
            if (host !== commandHost || !current?.input_ready) return {kind: 'error', text: 'Terminal attachment is not ready'};
            if (!sameInputStore(store, inputStore(stateRoot))) return {kind: 'error', text: 'Terminal input store changed'};
            return receive(store.path, current, invocation, Date.now(), store.directory);
          },
        });
        commandHost = host;
      }
      const selected = foreground(dataDir, pid);
      if (!selected) {
        invalidate();
        return;
      }
      const nativeSession = ctx.get('sessions', false)?.get(selected.session_id);
      if (nativeSession?.header?.version !== 4 || nativeSession.header.origin === 'subagent' || nativeSession.id !== selected.session_id) {
        invalidate();
        return;
      }
      const previous = current;
      current = {version: 1, agent: 'dsh', client: 'dst', pane, pid, instance, home_id: homeID, ...selected,
        native_session: true, server_epoch: serverEpoch, started_at: startedAt, updated_at: Date.now(),
        input_store_generation: store.generation,
        input_ready: Boolean(sameInputStore(store, inputStore(stateRoot))) && host.grants.allows(ctx, 'commands.invoke', COMPONENT_COMMAND)};
      atomicJSON(leaseFile, current);
      if (!previous || previous.session_id !== current.session_id || previous.binding_epoch !== current.binding_epoch) {
        sink('SessionStart', {session_id: current.session_id, cwd: current.cwd});
      }
    } catch {
      invalidate();
    }
  };
  observe(ctx, (token, payload) => { refresh(); sink(token, payload); });
  const timer = setInterval(refresh, 1000);
  timer.unref();
  refresh();
  ctx.on('dispose', () => {
    stopped = true;
    current = undefined;
    clearInterval(timer);
    disposeCommand?.();
    try { const old = readJSON(leaseFile, 16384); if (old.instance === instance) unlinkSync(leaseFile); } catch {}
  });
}
