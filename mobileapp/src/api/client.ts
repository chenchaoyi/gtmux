// GtmuxClient — every /api/* call sends `Authorization: Bearer <token>`.
// Mirrors api/contract.md exactly. Read-only surface; `focus` only selects a pane.

import {Agent, PaneResponse, toAgent} from './types';

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

  async registerPush(deviceToken: string): Promise<boolean> {
    const r = await fetch(`${this.base}/api/push/register`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({token: deviceToken, platform: 'ios'}),
    });
    return r.ok;
  }
}
