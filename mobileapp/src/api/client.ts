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

  // send types into a pane (a WRITE): a named control key, or literal text (+Enter).
  async send(id: string, payload: SendPayload): Promise<boolean> {
    const r = await fetch(`${this.base}/api/send`, {
      method: 'POST',
      headers: {...this.h(), 'Content-Type': 'application/json'},
      body: JSON.stringify({id, ...payload}),
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
