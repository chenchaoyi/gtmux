export interface AccessEntry {
  id: string; at: string; actor: string; name: string; scope: string;
  operation: string; label_en: string; label_zh: string; outcome: string;
  device_id?: string; platform?: string; source_ip?: string; method?: string;
  target?: string; status?: number; duration_ms?: number;
  session_id?: string;
}
export interface AccessPage {entries: AccessEntry[]; next?: string; partial?: boolean; retain_days: number}
