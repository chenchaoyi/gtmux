// GtmuxClient — every /api/* call sends `Authorization: Bearer <token>`.
// Mirrors api/contract.md. `focus` selects a pane; `send` types into one (a WRITE
// gated only by the bearer token).

import {Agent, PaneResponse, toAgent} from './types';

export interface SendPayload {
  text?: string;
  key?: string;
  enter?: boolean;
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
      const r = await fetch(`${this.base}/api/health`);
      return r.ok;
    } catch {
      return false;
    }
  }

  async agents(): Promise<Agent[]> {
    const r = await fetch(`${this.base}/api/agents`, {headers: this.h()});
    if (!r.ok) throw new Error(`agents: HTTP ${r.status}`);
    const raw = await r.json();
    return Array.isArray(raw) ? raw.map(toAgent) : [];
  }

  // id is a pane id like "%12"; encodeURIComponent turns "%" into "%25".
  async pane(id: string): Promise<PaneResponse> {
    const r = await fetch(
      `${this.base}/api/pane?id=${encodeURIComponent(id)}`,
      {headers: this.h()},
    );
    if (!r.ok) throw new Error(`pane: HTTP ${r.status}`);
    return r.json();
  }

  async focus(id: string): Promise<boolean> {
    const r = await fetch(`${this.base}/api/focus?id=${encodeURIComponent(id)}`, {
      method: 'POST',
      headers: this.h(),
    });
    return r.ok;
  }

  // diff fetches a unified `git diff` of the pane's cwd ("what the agent changed").
  // Empty string when the cwd isn't a git repo.
  async diff(id: string): Promise<string> {
    const r = await fetch(`${this.base}/api/diff?id=${encodeURIComponent(id)}`, {headers: this.h()});
    if (!r.ok) throw new Error(`diff: HTTP ${r.status}`);
    const j = await r.json();
    return typeof j?.diff === 'string' ? j.diff : '';
  }

  // send types into a pane (a WRITE): a named control key, or literal text (+Enter).
  async send(id: string, payload: SendPayload): Promise<boolean> {
    const r = await fetch(`${this.base}/api/send`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({id, ...payload}),
    });
    return r.ok;
  }

  // upload sends a file to the Mac and returns the saved path (to reference to an
  // agent), or null. Multipart — don't set Content-Type (RN adds the boundary).
  async upload(uri: string, name: string, type: string): Promise<string | null> {
    const form = new FormData();
    form.append('file', {uri, name, type} as any);
    try {
      const r = await fetch(`${this.base}/api/upload`, {method: 'POST', headers: this.h(), body: form});
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
  async registerPush(deviceToken: string, kinds?: string[]): Promise<boolean> {
    const r = await fetch(`${this.base}/api/push/register`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token: deviceToken, platform: 'ios', kinds: kinds ?? []}),
    });
    return r.ok;
  }

  // registerActivityToken hands the Mac a Live Activity push token so the relay
  // can push-to-update the lock-screen tally even when the app is closed.
  async registerActivityToken(token: string): Promise<boolean> {
    const r = await fetch(`${this.base}/api/push/activity`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token}),
    });
    return r.ok;
  }

  // testPush asks the Mac to send a test notification to this device.
  async testPush(): Promise<boolean> {
    const r = await fetch(`${this.base}/api/push/test`, {method: 'POST', headers: this.h()});
    return r.ok;
  }
}
