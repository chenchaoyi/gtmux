import type {AccessEntry} from '../api/accessHistory';

export interface AccessLogLine {entry: AccessEntry; clock: string; day?: string}

// Selected UI language governs dates; the viewing device's time zone governs
// boundaries. Keep source order (including equal timestamps) across pagination.
export function accessLogLines(entries: AccessEntry[], zh: boolean, timeZone?: string): AccessLogLine[] {
  const locale = zh ? 'zh-CN' : 'en-US';
  const day = new Intl.DateTimeFormat(locale, {year: 'numeric', month: zh ? 'long' : 'short', day: 'numeric', timeZone});
  const clock = new Intl.DateTimeFormat(locale, {hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23', timeZone});
  let previous: string | undefined;
  return entries.map(entry => {
    const date = new Date(entry.at), valid = !Number.isNaN(date.getTime());
    const label = valid ? day.format(date) : zh ? '时间未知' : 'Unknown date';
    const line = {entry, clock: valid ? clock.format(date) : entry.at, day: label === previous ? undefined : label};
    previous = label;
    return line;
  });
}

export function accessName(e: Pick<AccessEntry, 'scope' | 'name'>, zh: boolean) {
  return e.scope === 'master' ? (zh ? '所有者凭据' : 'Owner credential') : e.scope === 'anonymous' ? (zh ? '未知凭据' : 'Unknown credential') : e.name;
}
export function accessOperation(e: AccessEntry, zh: boolean) {
  const target = e.target || (e.session_id ? `Codex · ${e.session_id}` : '');
  return (zh ? e.label_zh : e.label_en) + (target ? ` · ${target}` : '');
}
export function accessOutcome(e: AccessEntry, zh: boolean) {
  const label = e.outcome === 'ok' ? (zh ? '已接受' : 'Accepted') : e.outcome === 'refused' ? (zh ? '已拒绝' : 'Refused') : (zh ? '失败' : 'Failed');
  return label + (e.status ? ` · ${e.status}` : '');
}
export function accessMetadata(e: AccessEntry, zh: boolean) {
  return [e.platform, e.source_ip, e.device_id ? `${zh ? '设备' : 'Device'} ${e.device_id}` : '', e.method,
    e.duration_ms ? `${e.duration_ms} ms` : ''].filter(Boolean).join(' · ');
}
