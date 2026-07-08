// GtmuxClient — every /api/* call sends `Authorization: Bearer <token>`.
// Mirrors api/contract.md. `focus` selects a pane; `send` types into one (a WRITE
// gated only by the bearer token).

import {Agent, PaneResponse, ReplyOption, TermTheme, toAgent} from './types';
import {Debug} from '../debug';

export interface SendPayload {
  text?: string;
  key?: string;
  enter?: boolean;
}

// A chat-history turn (GET /api/transcript): one user instruction, the agent's
// final text reply, and the intermediate tool calls folded into collapsible steps.
export interface TranscriptStep {
  kind: string; // "tool"
  title: string; // tool name (Edit, Bash, exec…)
  detail?: string; // short arg summary (path / command head)
}
// One chronological piece of a reply: an assistant text bubble + the tool steps
// that ran after it. The chat renders each as its own speech bubble so interleaved
// process (steps between texts) reads clearly.
export interface TranscriptSegment {
  text?: string;
  steps?: TranscriptStep[];
}
export interface TranscriptTurn {
  prompt: string;
  response: string; // joined segment texts (fallback / web sig)
  segments?: TranscriptSegment[];
  time?: string; // prompt's RFC3339 timestamp (agent log) — for the chat time label
}

// tfetch is fetch + optional debug logging (method · path · status · ms). It
// records the path only (host stripped, token/id query values redacted) — never
// the bearer token or request body. No-op overhead when Debug.logNet is off.
async function tfetch(url: string, init?: RequestInit): Promise<Response> {
  if (!Debug.logNet) return fetch(url, init);
  const method = (init?.method || 'GET').toUpperCase();
  const path = url.replace(/^https?:\/\/[^/]+/, '').replace(/([?&](?:token|id)=)[^&]*/g, '$1…');
  const t0 = Date.now();
  try {
    const r = await fetch(url, init);
    Debug.record({event: 'net', method, path, status: r.status, ms: Date.now() - t0});
    return r;
  } catch (e: any) {
    Debug.record({event: 'net', method, path, error: String(e?.message || e), ms: Date.now() - t0});
    throw e;
  }
}

// ApiError carries the HTTP status so callers can tell an AUTH rejection (401/403 —
// the token was revoked or is wrong → re-pair) from a plain network/offline failure.
export class ApiError extends Error {
  constructor(public status: number, where: string) {
    super(`${where}: HTTP ${status}`);
    this.name = 'ApiError';
  }
  get isAuth(): boolean {
    return this.status === 401 || this.status === 403;
  }
}

// isAuthError reports whether a caught error is an auth rejection (revoked/wrong token).
export function isAuthError(e: unknown): boolean {
  return e instanceof ApiError && e.isAuth;
}

export class GtmuxClient {
  constructor(
    public base: string,
    private token: string,
  ) {}

  private h(): Record<string, string> {
    return {Authorization: `Bearer ${this.token}`};
  }

  // Unauthenticated reachability check.
  async health(): Promise<boolean> {
    try {
      const r = await tfetch(`${this.base}/api/health`);
      return r.ok;
    } catch {
      return false;
    }
  }

  async agents(): Promise<Agent[]> {
    const r = await tfetch(`${this.base}/api/agents`, {headers: this.h()});
    if (!r.ok) throw new ApiError(r.status, 'agents');
    const raw = await r.json();
    return Array.isArray(raw) ? raw.map(toAgent) : [];
  }

  // id is a pane id like "%12"; encodeURIComponent turns "%" into "%25".
  async pane(id: string): Promise<PaneResponse> {
    const r = await tfetch(
      `${this.base}/api/pane?id=${encodeURIComponent(id)}`,
      {headers: this.h()},
    );
    if (!r.ok) throw new Error(`pane: HTTP ${r.status}`);
    return r.json();
  }

  async focus(id: string): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/focus?id=${encodeURIComponent(id)}`, {
      method: 'POST',
      headers: this.h(),
    });
    return r.ok;
  }

  // options returns a waiting pane's interactive 1/2/3 choice block (parsed by the
  // SAME Go parser the menu-bar uses), for the approval card. [] when none parse.
  async options(id: string): Promise<ReplyOption[]> {
    const r = await tfetch(`${this.base}/api/options?id=${encodeURIComponent(id)}`, {
      headers: this.h(),
    });
    if (!r.ok) return [];
    const j = await r.json().catch(() => null);
    return Array.isArray(j?.options) ? j.options : [];
  }

  // enrollMint mints a short-lived single-use pairing code (for handing this paired
  // server off to a computer browser via `${base}/#c=<code>`). null on failure.
  async enrollMint(): Promise<string | null> {
    const r = await tfetch(`${this.base}/api/enroll/mint`, {method: 'POST', headers: this.h()});
    if (!r.ok) return null;
    const j = await r.json().catch(() => null);
    return typeof j?.enrollCode === 'string' ? j.enrollCode : null;
  }

  // theme returns the host terminal's resolved appearance (colors + font) so the
  // pane mirror matches the user's real terminal. null on failure.
  async theme(): Promise<TermTheme | null> {
    const r = await tfetch(`${this.base}/api/theme`, {headers: this.h()});
    if (!r.ok) return null;
    return r.json().catch(() => null);
  }

  // diff fetches a unified `git diff` of the pane's cwd ("what the agent changed").
  // Empty string when the cwd isn't a git repo.
  async diff(id: string): Promise<string> {
    const r = await tfetch(`${this.base}/api/diff?id=${encodeURIComponent(id)}`, {headers: this.h()});
    if (!r.ok) throw new Error(`diff: HTTP ${r.status}`);
    const j = await r.json();
    return typeof j?.diff === 'string' ? j.diff : '';
  }

  // transcript fetches the pane's parsed chat history (prompt → collapsed steps →
  // final response). [] when the pane has no resumable session / no agent log.
  async transcript(id: string): Promise<TranscriptTurn[]> {
    const r = await tfetch(`${this.base}/api/transcript?id=${encodeURIComponent(id)}`, {headers: this.h()});
    if (!r.ok) return [];
    const j = await r.json().catch(() => null);
    return Array.isArray(j) ? j : [];
  }

  // send types into a pane (a WRITE): a named control key, or literal text (+Enter).
  // Returns the post-send pane snapshot the server captures after a brief settle
  // (text + cursor) so the caller can render the echo in ONE round-trip instead of
  // a separate pane() fetch — the latency win over a remote tunnel. null on failure.
  async send(id: string, payload: SendPayload): Promise<PaneResponse | null> {
    const r = await tfetch(`${this.base}/api/send`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({id, ...payload}),
    });
    if (!r.ok) return null;
    try {
      return (await r.json()) as PaneResponse;
    } catch {
      return {id, text: ''}; // sent ok, but no snapshot body — caller falls back to poll
    }
  }

  // upload sends a file to the Mac and returns the saved path (to reference to an
  // agent), or null. Multipart — don't set Content-Type (RN adds the boundary).
  async upload(uri: string, name: string, type: string): Promise<string | null> {
    const form = new FormData();
    form.append('file', {uri, name, type} as any);
    try {
      const r = await tfetch(`${this.base}/api/upload`, {method: 'POST', headers: this.h(), body: form});
      if (!r.ok) return null;
      const j = await r.json();
      return typeof j?.path === 'string' ? j.path : null;
    } catch {
      return null;
    }
  }

  // iconUri is an authed <Image> source for an agent's official icon (served from
  // the Mac's installed app, like the menu-bar app). 404 → caller falls back.
  iconUri(agentName: string): {uri: string; headers: Record<string, string>} {
    return {uri: `${this.base}/api/icon?agent=${encodeURIComponent(agentName)}`, headers: this.h()};
  }

  // registerPush registers the APNs token + which alert kinds the device wants
  // ([] = all). serve filters per-device, so you can opt out of e.g. "done".
  async registerPush(deviceToken: string, kinds?: string[], env?: string): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/push/register`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token: deviceToken, platform: 'ios', kinds: kinds ?? [], env}),
    });
    return r.ok;
  }

  // registerActivityToken hands the Mac a Live Activity push token so the relay
  // can push-to-update the lock-screen tally even when the app is closed.
  async registerActivityToken(token: string, env?: string): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/push/activity`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token, env}),
    });
    return r.ok;
  }

  // testPush asks the Mac to send a test notification to this device.
  async testPush(): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/push/test`, {method: 'POST', headers: this.h()});
    return r.ok;
  }
}
