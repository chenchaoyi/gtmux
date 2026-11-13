// Owner-only restore jobs; a completed worker is not proof every agent started.
export interface RestoreSession {name: string; windows: number; panes: number; agents: number; resumable: number; state: string}
export interface RestorePlan {revision: string; server_time: number; saved_at: number; resume_mode: 'auto' | 'type' | 'off'; sessions: RestoreSession[]}
export interface RestoreJob {id: string; revision: string; created_at: number; updated_at: number; phase: string; code?: string; sessions: RestoreSession[]; agents?: {loc: string; pane: string; state: string}[]}
export interface RestoreSnapshot {plan: RestorePlan; job?: RestoreJob}
export const restoreTerminal = (phase: string) => ['completed', 'partial', 'failed', 'interrupted'].includes(phase);
export class RestoreError extends Error {constructor(public code: string, public status = 0) {super(code);}}
const text = (v: unknown, max = 256): v is string => typeof v === 'string' && v.length <= max;
const count = (v: unknown) => Number.isSafeInteger(v) && Number(v) >= 0;
const sessions = (v: any) => Array.isArray(v) && v.length <= 256 && v.every(s => s && text(s.name) && s.name && count(s.windows) && count(s.panes) && count(s.agents) && count(s.resumable) && text(s.state, 32));
export function parseRestoreJob(v: any): RestoreJob {
  if (!v || !text(v.id, 80) || !/^\d{10}-[A-Za-z0-9-]{16,64}$/.test(v.id) || !/^[a-f0-9]{64}$/.test(v.revision) || !count(v.created_at) || !count(v.updated_at) ||
    !['queued', 'preparing', 'layout', 'agents', 'completed', 'partial', 'failed', 'interrupted'].includes(v.phase) || !sessions(v.sessions) ||
    (v.code !== undefined && !text(v.code, 80)) || (v.agents !== undefined && (!Array.isArray(v.agents) || v.agents.length > 2048 || !v.agents.every((a: any) => a && text(a.loc) && text(a.pane, 40) && text(a.state, 40))))) throw new RestoreError('invalid_response');
  return v;
}
export function parseRestoreSnapshot(v: any): RestoreSnapshot {
  const p = v?.plan;
  if (!p || !/^[a-f0-9]{64}$/.test(p.revision) || !count(p.server_time) || !count(p.saved_at) || !['auto', 'type', 'off'].includes(p.resume_mode) || !sessions(p.sessions)) throw new RestoreError('invalid_response');
  if (v.job) parseRestoreJob(v.job);
  return v;
}
export function restoreRequestID(p: RestorePlan): string {return `${p.server_time}-${Date.now().toString(36).padStart(10, '0')}-${Math.random().toString(36).slice(2).padEnd(12, '0')}-${Math.random().toString(36).slice(2).padEnd(12, '0')}`;}
