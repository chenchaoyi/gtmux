// The diagnostic record, said in words.
//
// The buffer keeps entries in the Mac's log shape: an event name, an outcome and a few
// attributes. That is the right thing to COPY (it reads beside `gtmux logs --json`) and
// the wrong thing to show: "api.failed · route=GET /api/agents · status=0" tells someone
// who came here because their phone stopped updating nothing they can use. This turns
// each entry into a sentence and keeps the raw attributes underneath it.
//
// The entries themselves are never translated on the Mac (they are what a bug report
// quotes). Here they are, because this screen is read by the person holding the phone,
// not pasted into an issue — Copy still hands over the untouched JSON lines.

import {Entry} from './index';

export interface DiagLine {
  id: string;
  /** Local clock as the entry was stamped: 14:06. */
  clock: string;
  /** The entry's day, 2026-09-20, for the section headings. */
  day: string;
  title: string;
  detail?: string;
  /** A warning or an error: the two the "Problems only" filter keeps. */
  problem: boolean;
}

export interface DiagSection {
  day: string;
  title: string;
  lines: DiagLine[];
}

const num = (v: unknown): number | undefined => (typeof v === 'number' ? v : undefined);
const str = (v: unknown): string | undefined =>
  typeof v === 'string' && v.length > 0 ? v : undefined;

// seconds, as a person says them: 6s, 0.4s, 5 minutes.
function secs(ms: number | undefined, zh: boolean): string {
  if (ms === undefined) return '';
  if (ms >= 1000) return `${Math.round(ms / 100) / 10}s`;
  return zh ? `${ms} 毫秒` : `${ms}ms`;
}

function spell(seconds: number | undefined, zh: boolean): string {
  if (!seconds || seconds < 0) return '';
  if (seconds < 90) return zh ? `${Math.round(seconds)} 秒` : `${Math.round(seconds)} seconds`;
  const m = Math.round(seconds / 60);
  return zh ? `${m} 分钟` : `${m} minute${m === 1 ? '' : 's'}`;
}

// Why a pairing did not go through, in the words the pairing screen itself uses.
function pairReason(reason: string | undefined, status: number | undefined, zh: boolean): string {
  switch (reason) {
    case 'codeInvalid':
      return zh
        ? '这个配对码没被接受。码用过一次或者过了 5 分钟就不能再用，重新扫一个'
        : 'the code was not accepted. A code works once and for five minutes; scan a fresh one';
    case 'unreachable':
      return zh ? '那个地址没有任何回应' : 'nothing answered at that address';
    case 'tunnelDown':
      return zh
        ? 'Mac 的隧道没有回应，多半是 Mac 那边的 gtmux 没在跑'
        : "the Mac's tunnel did not answer, which usually means gtmux is not running over there";
    case 'noToken':
      return zh ? 'Mac 回了，但没给 token' : 'the Mac answered, but without a token';
    default:
      return status ? `HTTP ${status}` : '';
  }
}

/** describeEntry is one entry as a title and, where there is one, a line of detail. */
export function describeEntry(e: Entry, zh: boolean): {title: string; detail?: string} {
  const a = e.attrs ?? {};
  const join = (...parts: (string | undefined)[]) => {
    const kept = parts.filter(p => p && p.length > 0);
    return kept.length ? kept.join(zh ? ' · ' : ' · ') : undefined;
  };

  switch (e.event) {
    case 'phone.start': {
      const v = str(a.version);
      return {
        title: zh ? 'app 启动' : 'The app started',
        detail: v ? (zh ? `版本 ${v}` : `version ${v}`) : undefined,
      };
    }
    case 'api.failed': {
      const status = num(a.status);
      const route = str(a.route) ?? '';
      const repeats = num(a.repeats);
      const ms = num(a.ms);
      // A request that fails in 18ms did not time out, it was refused — saying "did not
      // answer after 18ms" reads as a timeout that never happened, so the elapsed time is
      // only worth printing once it is long enough to BE a wait.
      const waited = ms !== undefined && ms >= 1000;
      const answered = status
        ? zh
          ? `${route} 回了 HTTP ${status}`
          : `${route} answered HTTP ${status}`
        : waited
        ? zh
          ? `${route} 等了 ${secs(ms, zh)} 没有回应`
          : `${route} did not answer after ${secs(ms, zh)}`
        : zh
        ? `${route} 连都没连上`
        : `${route} could not be reached at all`;
      const again = repeats
        ? zh
          ? `之后一分钟内又失败了 ${repeats} 次`
          : `then ${repeats} more times in a minute`
        : undefined;
      return {
        title: status
          ? zh
            ? 'Mac 拒绝了一个请求'
            : 'The Mac turned a request away'
          : zh
          ? '联系不上 Mac'
          : 'Could not reach the Mac',
        // The library's own error string stays out of the sentence: it is English
        // whatever the reader's language, and "Network request failed" adds nothing to
        // "could not be reached". Copy still hands over the untouched entry.
        detail: join(answered, again),
      };
    }
    case 'api.recovered':
      return {
        title: zh ? 'Mac 又能回应了' : 'The Mac is answering again',
        detail: join(
          str(a.route),
          spell(num(a.failingSec), zh) &&
            (zh ? `失败了 ${spell(num(a.failingSec), zh)}` : `it had been failing for ${spell(num(a.failingSec), zh)}`),
        ),
      };
    case 'sse.disconnected':
      return {
        title: zh ? '实时连接断了' : 'The live connection dropped',
        detail: num(a.status) ? `HTTP ${num(a.status)}` : undefined,
      };
    case 'sse.connected': {
      const down = spell(num(a.downSec), zh);
      return {
        title: zh ? '实时连接回来了' : 'The live connection is back',
        detail: down ? (zh ? `断了 ${down}` : `it was down for ${down}`) : undefined,
      };
    }
    case 'act.pair': {
      const host = e.target ?? '';
      if (e.outcome === 'ok') {
        return {title: zh ? `和 ${host} 配对成功` : `Paired with ${host}`};
      }
      return {
        title: zh ? '一次配对没成' : 'A pairing did not go through',
        detail: join(host, pairReason(str(a.reason), num(a.status), zh)),
      };
    }
    case 'act.push.register': {
      const kinds = str(a.kinds);
      if (e.outcome === 'ok') {
        return {
          title: zh ? '已登记推送通知' : 'Registered for push notifications',
          detail: kinds ? kinds.split(',').join(zh ? '、' : ' and ') : undefined,
        };
      }
      return {
        title: zh ? '推送通知没登记上' : 'Push notifications could not be registered',
        detail: num(a.status) ? `HTTP ${num(a.status)}` : undefined,
      };
    }
    default: {
      // An event this file has not been taught yet still has to read as something. The
      // entry's own message is a sentence already; the event name is the fallback.
      const rest = Object.entries(a)
        .map(([k, v]) => `${k}=${v}`)
        .join(' · ');
      return {title: e.msg || e.event, detail: rest || undefined};
    }
  }
}

/** diagLines turns the buffer into lines, newest first. */
export function diagLines(entries: Entry[], zh: boolean): DiagLine[] {
  const out: DiagLine[] = [];
  entries.forEach((e, i) => {
    const {title, detail} = describeEntry(e, zh);
    const [day, rest] = e.ts.split('T');
    out.push({
      id: `${e.ts}#${i}`,
      clock: (rest ?? '').slice(0, 5),
      day: day ?? '',
      title,
      detail,
      problem: e.level === 'warn' || e.level === 'error',
    });
  });
  return out.reverse();
}

/** groupByDay splits lines into day sections, with today and yesterday named. */
export function groupByDay(lines: DiagLine[], now: Date, zh: boolean): DiagSection[] {
  const key = (d: Date) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  const today = key(now);
  const yesterday = key(new Date(now.getTime() - 86400_000));
  const out: DiagSection[] = [];
  for (const l of lines) {
    let s = out[out.length - 1];
    if (!s || s.day !== l.day) {
      s = {
        day: l.day,
        title: l.day === today ? (zh ? '今天' : 'Today') : l.day === yesterday ? (zh ? '昨天' : 'Yesterday') : l.day,
        lines: [],
      };
      out.push(s);
    }
    s.lines.push(l);
  }
  return out;
}

/** countProblems is what the Settings row leads with when it is not zero. */
export function countProblems(entries: Entry[]): number {
  return entries.filter(e => e.level === 'warn' || e.level === 'error').length;
}

/**
 * describeRecord is the Settings row's right-hand value: the problems if there are any,
 * else how much is kept. The row is a door, so it says what is behind it in three words,
 * and the page behind it says the rest.
 */
export function describeRecord(st: {count: number; bytes: number}, problems: number, zh: boolean): string {
  if (st.count === 0) return zh ? '还没有记录' : 'Nothing yet';
  if (problems > 0) {
    return zh ? `${problems} 个问题` : `${problems} problem${problems === 1 ? '' : 's'}`;
  }
  return zh ? `${st.count} 条` : `${st.count} entries`;
}
