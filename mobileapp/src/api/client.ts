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

// ShareCapability mirrors GET /api/share — the CALLER's own scope. `all:true` ⇒ a
// full caller (owner: master token or paired device) that sees + types everywhere.
// Otherwise it's a GUEST, scoped to `view_panes` (viewable) and `panes` (typable).
export interface ShareCapability {
  input: boolean; // may this caller type at all
  all?: boolean; // owner: any pane (view + input)
  panes: string[]; // guest: input-allowed panes
  view_panes: string[]; // guest: view-allowed panes
}

// ShareConfig mirrors GET /api/share/config — the HOST's consent state (owner-only,
// owner-remote-admin): the typing master switch + the global view/input allowlists.
export interface ShareConfig {
  enabled: boolean;
  panes: string[]; // global INPUT allowlist
  view_panes: string[]; // global VIEW allowlist
}

// GuestLink is a `scope:"guest"` roster entry (a share link) with its per-link scope.
export interface GuestLink {
  id: string;
  label: string;
  enrolledAt: number;
  viewPanes: string[];
  inputPanes: string[];
  expiresAt: number;
}

// PairedDevice is a `scope:"device"` roster entry (a paired phone/browser/terminal),
// shown READ-ONLY on the phone — revoking a device stays a Mac-only operation.
export interface PairedDevice {
  id: string;
  name: string;
  enrolledAt: number;
  lastSeen?: number;
}

// DigestRow mirrors internal/app digestRow (GET /api/digest) — the fleet's
// cognitive digest for the gtmux HQ command center.
export interface DigestRow {
  pane_id?: string;
  loc?: string;
  agent: string;
  source: string;
  status: string; // working | waiting | idle | running
  kind?: string; // waiting only: permission | plan | question
  role?: string; // "supervisor"
  project?: string;
  branch?: string;
  goal?: string;
  last?: string;
  ask?: string;
  error?: string;
  bg?: string;
  since?: number;
  tok?: number;
  ctx?: number;
  rate?: number;
  usage_warn?: string;
}

// UsageReport mirrors internal/app usageReport (GET /api/usage).
export interface UsageWindow {
  label: string;
  pct_used: number;
  reset_at: string;
}
export interface ResourceReport {
  machine?: {disk_free_gb?: number; mem_free_pct?: number; mem_tier?: string; load_ratio?: number; ncpu?: number; warn?: string};
  orphans?: {pid: number; rss_mb: number; comm: string; kind?: string; hint?: string}[];
}
export interface UsageReport {
  sessions?: {agent_key: string; tok: number; rate: number; usage_warn?: string}[];
  types?: {agent_key: string; sessions: number; tok: number; rate: number; usage_warn?: string}[];
  limits?: {windows?: UsageWindow[]; warn?: string; at?: number};
  resource?: ResourceReport;
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

  // share reads the caller's own scope (GET /api/share): `all:true` ⇒ owner (full),
  // else a guest scoped to view_panes/panes. Used to decide guest-vs-owner UI. Throws
  // ApiError on a non-OK (an auth rejection surfaces like any other authed call).
  async share(): Promise<ShareCapability> {
    const r = await tfetch(`${this.base}/api/share`, {headers: this.h()});
    if (!r.ok) throw new ApiError(r.status, 'share');
    const j = await r.json().catch(() => null);
    return {
      input: !!j?.input,
      all: !!j?.all,
      panes: Array.isArray(j?.panes) ? j.panes : [],
      view_panes: Array.isArray(j?.view_panes) ? j.view_panes : [],
    };
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

  // digest: the fleet's cognitive digest (GET /api/digest) — one row per agent
  // with goal/last/ask + state, the gtmux HQ command center's situational-
  // awareness source. [] on failure (the board just shows empty).
  async digest(): Promise<DigestRow[]> {
    const r = await tfetch(`${this.base}/api/digest`, {headers: this.h()});
    if (!r.ok) return [];
    const j = await r.json().catch(() => null);
    return Array.isArray(j) ? j : [];
  }

  // usage: token accounting + real subscription-window limits (GET /api/usage) —
  // HQ shows the week/plan % in its status strip. null on failure.
  async usage(): Promise<UsageReport | null> {
    const r = await tfetch(`${this.base}/api/usage`, {headers: this.h()});
    if (!r.ok) return null;
    return (await r.json().catch(() => null)) as UsageReport | null;
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
  // Uses XMLHttpRequest (not fetch) so `onProgress` can report the upload fraction
  // (0..1) — fetch+FormData exposes NO upload progress in RN, and a large photo needs
  // real feedback. Returns null on any failure (network, non-2xx, bad body) so the
  // caller can keep the attachment staged and offer a retry.
  upload(
    uri: string,
    name: string,
    type: string,
    onProgress?: (fraction: number) => void,
  ): Promise<string | null> {
    return new Promise(resolve => {
      const form = new FormData();
      form.append('file', {uri, name, type} as any);
      const xhr = new XMLHttpRequest();
      xhr.open('POST', `${this.base}/api/upload`);
      for (const [k, v] of Object.entries(this.h())) xhr.setRequestHeader(k, v);
      if (onProgress && xhr.upload) {
        xhr.upload.onprogress = e => {
          if (e.lengthComputable && e.total > 0) onProgress(e.loaded / e.total);
        };
      }
      xhr.onload = () => {
        try {
          if (xhr.status >= 200 && xhr.status < 300) {
            const j = JSON.parse(xhr.responseText);
            resolve(typeof j?.path === 'string' ? j.path : null);
          } else {
            resolve(null);
          }
        } catch {
          resolve(null);
        }
      };
      xhr.onerror = () => resolve(null);
      xhr.ontimeout = () => resolve(null);
      xhr.send(form);
    });
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

  // unregisterPush drops this device's tokens on the Mac (removing a server), so it
  // stops pushing to a phone that unpaired it: the APNs token stops alerts + silent
  // badges, and the optional Live Activity token stops lock-screen updates (the Mac
  // also ends that card). Idempotent server-side; best-effort here (the Mac may be
  // offline at removal time).
  async unregisterPush(deviceToken: string, activityToken?: string): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/push/unregister`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token: deviceToken, activityToken}),
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

  // --- owner-remote-admin: manage THIS Mac's sharing (owner/full callers only;
  // the server 403s a guest). Mirrors the menu-bar Preferences share section. ---

  // shareConfig reads the host's consent + global allowlists (GET /api/share/config).
  async shareConfig(): Promise<ShareConfig> {
    const r = await tfetch(`${this.base}/api/share/config`, {headers: this.h()});
    if (!r.ok) throw new ApiError(r.status, 'share/config');
    const j = await r.json().catch(() => null);
    return {
      enabled: !!j?.enabled,
      panes: Array.isArray(j?.panes) ? j.panes : [],
      view_panes: Array.isArray(j?.view_panes) ? j.view_panes : [],
    };
  }

  // setShareEnabled flips the typing master switch (POST /api/share/config {enabled}).
  async setShareEnabled(on: boolean): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/share/config`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({enabled: on}),
    });
    return r.ok;
  }

  // devices lists the roster (GET /api/devices), split into guest LINKS (manageable)
  // and paired DEVICES (read-only on the phone — revoking a device is Mac-only).
  async devices(): Promise<{guests: GuestLink[]; devices: PairedDevice[]}> {
    const r = await tfetch(`${this.base}/api/devices`, {headers: this.h()});
    if (!r.ok) throw new ApiError(r.status, 'devices');
    const j = await r.json().catch(() => null);
    const raw: any[] = Array.isArray(j?.devices) ? j.devices : [];
    const guests: GuestLink[] = [];
    const devices: PairedDevice[] = [];
    for (const d of raw) {
      if (d?.scope === 'guest') {
        guests.push({
          id: d.id ?? '',
          label: d.name ?? '',
          enrolledAt: d.enrolledAt ?? 0,
          viewPanes: Array.isArray(d.viewPanes) ? d.viewPanes : [],
          inputPanes: Array.isArray(d.inputPanes) ? d.inputPanes : [],
          expiresAt: d.expiresAt ?? 0,
        });
      } else {
        devices.push({id: d.id ?? '', name: d.name ?? '', enrolledAt: d.enrolledAt ?? 0, lastSeen: d.lastSeen});
      }
    }
    return {guests, devices};
  }

  // shareNew mints a guest link with an explicit per-link scope (POST /api/share/new).
  async shareNew(label: string, view: string[], input: string[]): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/share/new`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({label, view, input}),
    });
    return r.ok;
  }

  // shareSet edits ONE link's See/Type (POST /api/share/set); omitted facets untouched.
  async shareSet(id: string, view: string[], input: string[]): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/share/set`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({id, view, input}),
    });
    return r.ok;
  }

  // revokeShare kills a guest LINK (POST /api/devices/revoke). An owner may revoke a
  // guest link but NOT a paired device — the server enforces that (decision B).
  async revokeShare(id: string): Promise<boolean> {
    const r = await tfetch(`${this.base}/api/devices/revoke`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({id}),
    });
    return r.ok;
  }

  // shareLink re-hands an existing link's token (GET /api/share/link) so the owner can
  // re-copy the URL; the caller builds `${base}/#g=${token}`. Null if not found.
  async shareLink(id: string): Promise<string | null> {
    const r = await tfetch(`${this.base}/api/share/link?id=${encodeURIComponent(id)}`, {headers: this.h()});
    if (!r.ok) return null;
    const j = await r.json().catch(() => null);
    return typeof j?.token === 'string' ? j.token : null;
  }
}
