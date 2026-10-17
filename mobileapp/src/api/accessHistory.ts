export interface AccessEntry {
  id: string;
  at: string;
  actor: string;
  name: string;
  scope: string;
  operation: string;
  label_en: string;
  label_zh: string;
  outcome: string;
  device_id?: string;
  platform?: string;
  source_ip?: string;
  method?: string;
  target?: string;
  status?: number;
  duration_ms?: number;
  session_id?: string;
  origin?: 'external' | 'local' | 'unknown';
  origin_basis?: string;
}
export interface AccessPage {
  entries: AccessEntry[];
  next?: string;
  partial?: boolean;
  retain_days: number;
  groups?: AccessGroup[];
  query?: AccessFilter;
  operations?: AccessOperation[];
}
export interface AccessFilter {
  origin?: 'external';
  device_id?: string;
  actor?: string;
  operation?: string;
  outcome?: string;
  group_by?: 'device';
}
export interface AccessGroup {
  key: string;
  actor: string;
  device_id?: string;
  name: string;
  scope: string;
  platform?: string;
  source_ip?: string;
  count: number;
  accepted: number;
  refused: number;
  failed: number;
  first_at: string;
  last_at: string;
  origin?: 'external' | 'local' | 'unknown';
}
export interface AccessOperation {
  value: string;
  en: string;
  zh: string;
}
