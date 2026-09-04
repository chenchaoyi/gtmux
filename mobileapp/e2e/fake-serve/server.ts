// A fake `gtmux serve`, for exercising the flows that CHANGE something.
//
// Why it exists: the mutating half of the app — send, the reply options, focus, and now
// knowledge land/retire — could only be driven against the commander's real machine,
// where a test that types into a pane interrupts live work. So it was never driven at
// all, and every one of those paths reached the phone tested only by hand.
//
// What keeps it honest (it is a second implementation, and a second implementation drifts):
//   - handlers return the SAME TypeScript types the app's client consumes, so a change to
//     the client's expectations fails to compile here;
//   - the error cases are first-class, not an afterthought: a pane that is gone, a key
//     that is not allow-listed, a guest scope, a knowledge id with no pending promotion.
//     A fake that only serves the happy path teaches a test suite that failure never
//     happens;
//   - contract.test.ts compares these shapes against a REAL serve when one is reachable.
//
// It is deliberately not a simulation of tmux. It records what was asked of it, which is
// what a test needs to assert: not "the screen changed" but "the app sent C-c to %12".

import {createServer, IncomingMessage, Server, ServerResponse} from 'http';
import {AddressInfo} from 'net';
import {World} from './world';

/**
 * A guest link's two allowlists, kept separate because the real serve keeps them separate:
 * `view` gates what may be SEEN (/api/agents, /api/panes, /api/pane, /api/attach) and
 * `input` gates what may be TYPED INTO (/api/send, via EnrolledDevice.MayInput). A link
 * can show a pane without granting the keyboard, and the fake collapsing the two would
 * quietly test a permission model the product does not have.
 */
export const GUEST_VIEW = ['%12', '%13'];
export const GUEST_INPUT = ['%12'];
/** Back-compat alias for the view list. */
export const GUEST_PANES = GUEST_VIEW;

/**
 * The ONE place a guest's reach is decided. Every handler asks this rather than testing
 * the allowlist itself: the fake's first version made that judgment per-handler, and the
 * radar handler simply forgot to — a guest saw the whole fleet. One rule has one place to
 * be wrong, and one place to fix.
 */
export function mayGuest(kind: 'view' | 'input', pane: string): boolean {
  return (kind === 'view' ? GUEST_VIEW : GUEST_INPUT).includes(pane);
}

const KEYS = ['Enter', 'C-c', 'Escape', 'Tab', 'Up', 'Down', 'Left', 'Right', 'Space', 'BSpace', 'C-d', 'C-z', 'C-l'];

export interface Fake {
  url: string;
  token: string;
  world: World;
  /** Announce a fleet change to connected clients, as the real serve does on every one. */
  bumpAgents: () => void;
  /**
   * dropStreams cuts every open SSE connection, the way a tunnel hiccup or a sleeping
   * phone does. The app must come back to `live` on its own — a client that needs a
   * relaunch to reconnect looks, to its reader, exactly like a Mac that went away.
   */
  dropStreams: () => number;
  close: () => Promise<void>;
}

const json = (res: ServerResponse, code: number, body: unknown): void => {
  const b = JSON.stringify(body);
  res.writeHead(code, {'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(b)});
  res.end(b);
};

async function readBody(req: IncomingMessage): Promise<Record<string, unknown>> {
  const chunks: Buffer[] = [];
  for await (const c of req) chunks.push(c as Buffer);
  if (chunks.length === 0) return {};
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    return {};
  }
}

/**
 * start boots the fake on an ephemeral port.
 *
 * `guest` flips the caller's scope: every OWNER surface then answers 403, which is how a
 * test checks that the app hides what a guest may not see rather than merely not asking
 * for it.
 */
export async function startFake(opts: {guest?: boolean} = {}): Promise<Fake> {
  const world = new World();
  const token = 'fake-token';
  const streams = new Set<ServerResponse>();
  let rev = 1;
  /**
   * Tell every connected client the fleet changed, exactly as the real serve does: a bare
   * revision number that makes the client re-fetch /api/agents.
   *
   * Without this the fake changed its world in silence and the app only noticed on its own
   * poll timer — which made an answered question look like it had not been answered for
   * however long that timer is. A fake that mutates without announcing it is not a slower
   * serve, it is a different one.
   */
  const bumpAgents = (): void => {
    rev += 1;
    const frame = `event: agents\ndata: {"rev":${rev}}\n\n`;
    streams.forEach(r => r.write(frame));
  };

  const server: Server = createServer((req, res) => {
    void handle(req, res).catch(() => json(res, 500, {error: 'fake failed'}));
  });

  async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const url = new URL(req.url ?? '/', 'http://127.0.0.1');
    const path = url.pathname;
    const q = url.searchParams;
    const owner = !opts.guest;

    if (path === '/api/health') return json(res, 200, {ok: true});

    // Every other endpoint is authenticated. An unauthenticated request must not reveal
    // whether the path exists.
    const auth = (req.headers.authorization ?? '').replace(/^Bearer\s+/i, '');
    if (auth !== token) return json(res, 401, {error: 'unauthorized'});

    /** What this caller may see: everything for an owner, the view allowlist for a guest. */
    const visible = <T extends {pane_id: string}>(rows: T[]): T[] =>
      owner ? rows : rows.filter(r => mayGuest('view', r.pane_id));
    const mayReach = (kind: 'view' | 'input', pane: string): boolean => owner || mayGuest(kind, pane);

    const ownerOnly = (): boolean => {
      if (owner) return true;
      json(res, 403, {error: 'forbidden: not shared'});
      return false;
    };

    if (path === '/api/events') {
      res.writeHead(200, {'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', Connection: 'keep-alive'});
      res.write(': hello\n\n');
      const beat = setInterval(() => res.write(': heartbeat\n\n'), 2000);
      streams.add(res);
      req.on('close', () => {
        clearInterval(beat);
        streams.delete(res);
      });
      return;
    }

    if (req.method === 'GET') {
      switch (path) {
        case '/api/share':
          return json(res, 200, owner ? {all: true, panes: []} : {all: false, panes: GUEST_VIEW});
        case '/api/agents':
          // A guest sees ONLY the panes on its own link's view allowlist — the real serve
          // filters here (server.go, filterAgentsForGuest). The first version of this fake
          // returned the whole fleet to a guest, which is worse than a missing feature: a
          // suite written against it would have PROVED that a guest sees every session's
          // name, task and error, and gone on passing if the real filter ever broke.
          return json(res, 200, visible(world.agents));
        case '/api/panes':
          return json(res, 200, visible(world.agents).filter(a => a.pane_id.startsWith('%')).map(a => ({
            pane_id: a.pane_id, session: a.session, window: a.window, pane: a.pane ?? a.session, agent: a.agent,
          })));
        case '/api/pane': {
          const id = q.get('id') ?? '';
          if (!mayReach('view', id)) return json(res, 403, {error: 'forbidden: pane not shared'});
          const text = world.screens.get(id);
          if (text == null) return json(res, 404, {error: 'no such pane'});
          return json(res, 200, {id, text, cols: 80, rows: 30});
        }
        case '/api/options': {
          const id = q.get('id') ?? '';
          if (!mayReach('view', id)) return json(res, 403, {error: 'forbidden: pane not shared'});
          const a = world.agent(id);
          // `{options: [...]}`, not a bare array — the first version of this fake
          // answered an array and the client quietly read nothing, which is precisely the
          // drift contract.test.ts exists to catch (so /api/options is on its list now).
          if (!a || a.status !== 'waiting') return json(res, 200, {options: []});
          return json(res, 200, {
            options: [
              {n: 1, label: '可以,提到 red'},
              {n: 2, label: '不用,保持 amber'},
              {n: 3, label: '让我看看再说'},
            ],
          });
        }
        case '/api/transcript': {
          const id = q.get('id') ?? '';
          if (!mayReach('view', id)) return json(res, 403, {error: 'forbidden: pane not shared'});
          return json(res, 200, [
            {prompt: '把知识库接到手机上', response: '⟣ 已经接好了,三个端点都在。', time: new Date(Date.now() - 600_000).toISOString()},
          ]);
        }
        case '/api/theme':
          return json(res, 200, {bg: '#000000', fg: '#ffffff'});
        case '/api/awake':
          if (!ownerOnly()) return;
          return json(res, 200, {awake: false});
        case '/api/usage':
          if (!ownerOnly()) return;
          return json(res, 200, {
            limits: {windows: [{label: 'session', pct_used: 24}, {label: 'week (all models)', pct_used: 54}]},
            resource: {machine: {disk_free_gb: 16, mem_tier: 'ok'}},
          });
        case '/api/digest':
          if (!ownerOnly()) return;
          return json(res, 200, world.agents.map(a => ({
            ...a,
            verdict: a.role === 'supervisor' ? {state: 'normal', workers: world.agents.length - 1, waiting: 1} : undefined,
          })));
        case '/api/hq/board':
          if (!ownerOnly()) return;
          return json(res, 200, world.board);
        case '/api/hq/events':
          if (!ownerOnly()) return;
          return json(res, 200, []);
        case '/api/hq/knowledge': {
          if (!ownerOnly()) return;
          const live = world.knowledge.entries;
          const counts = new Map<string, number>();
          live.forEach(e => counts.set(e.topic, (counts.get(e.topic) ?? 0) + 1));
          const pending = live.filter(e => e.promoted_at && !e.landed_at);
          const oldest = pending.length ? Math.min(...pending.map(e => e.promoted_at!)) : 0;
          return json(res, 200, {
            entries: [...live].reverse().map(({body, ...row}) => row),
            topics: ['accounts', 'workflows', 'best-practices', 'pitfalls', 'corrections', 'environment'].map(name => ({
              name, count: counts.get(name) ?? 0, builtin: true,
            })),
            promotions: {pending: pending.length, oldest_sec: oldest ? Math.floor(Date.now() / 1000) - oldest : 0},
            candidates: {pending: 2, oldest_sec: 400},
          });
        }
        case '/api/hq/knowledge/entry': {
          if (!ownerOnly()) return;
          const id = q.get('id') ?? '';
          if (!id) return json(res, 400, {error: 'id required'});
          const e = world.knowledge.entries.find(x => x.id === id);
          if (!e) return json(res, 404, {error: 'no such entry'});
          return json(res, 200, e);
        }
        default:
          return json(res, 404, {error: 'not found'});
      }
    }

    if (req.method === 'POST') {
      const body = await readBody(req);
      world.record(path, body);
      const rigged = world.failNext.get(path);
      if (rigged) {
        world.failNext.delete(path);
        return json(res, rigged.status, {error: rigged.error});
      }
      switch (path) {
        case '/api/send': {
          const id = String(body.id ?? '');
          const a = world.agent(id);
          if (!id) return json(res, 400, {error: 'missing id'});
          // A guest types only where its link granted the keyboard — a SEPARATE list from
          // what it may look at (real serve: EnrolledDevice.MayInput).
          if (!mayReach('input', id)) return json(res, 403, {error: 'forbidden: pane not shared'});
          // The wording is the real serve's, not a paraphrase: the app classifies a
          // refusal by what the server SAYS (ui/sendFailure), so a fake that invents its
          // own sentences tests a classifier against strings no server sends.
          if (!a) return json(res, 400, {error: 'send failed: pane not found'});
          const key = body.key == null ? '' : String(body.key);
          const text = body.text == null ? '' : String(body.text);
          if (!key && !text) return json(res, 400, {error: 'nothing to send'});
          if (key && !KEYS.includes(key)) return json(res, 400, {error: 'send failed: key not allowed'});
          // The draft guard, as the core states it: a paste APPENDS, so delivering into
          // someone's half-written line would submit THEIR text with yours.
          if (text && world.drafts.get(id)) {
            return json(res, 400, {
              error: 'send failed: not sent: that pane has unsent text in its input box — clear it or send from the Mac',
            });
          }
          // Answering a numbered menu is what a real agent does with a lone digit: the
          // choice commits and the session stops waiting. Modelled here so a test can ask
          // the question that matters after the tap — did the needs-you mark clear —
          // rather than only whether the request was made.
          if (a.status === 'waiting' && /^[1-9]$/.test(text.trim())) {
            a.status = 'working';
            a.since = Math.floor(Date.now() / 1000);
            world.answered.set(id, text.trim());
            bumpAgents();
          }
          return json(res, 200, {status: 'ok'});
        }
        case '/api/focus': {
          const id = q.get('id') ?? String(body.id ?? '');
          world.record('/api/focus', {id});
          if (!mayReach('view', id)) return json(res, 403, {error: 'forbidden: pane not shared'});
          if (!world.agent(id)) return json(res, 400, {error: 'no such pane'});
          return json(res, 200, {ok: true});
        }
        case '/api/hq/knowledge/act': {
          if (!ownerOnly()) return;
          const op = String(body.op ?? '');
          const id = String(body.id ?? '');
          const e = world.knowledge.entries.find(x => x.id === id);
          if (op !== 'land' && op !== 'retire') return json(res, 400, {error: 'unknown op (land|retire)'});
          if (!e) return json(res, 400, {error: `no live entry "${id}"`});
          if (op === 'land') {
            const ref = String(body.ref ?? '');
            if (!ref) return json(res, 400, {error: 'land needs a ref'});
            if (!e.promoted_at || e.landed_at) {
              return json(res, 400, {error: `${id} has no pending promotion to land (gtmux knowledge promotions)`});
            }
            e.landed_at = Math.floor(Date.now() / 1000);
            e.landed_ref = ref;
            return json(res, 200, {ok: true});
          }
          if (!String(body.why ?? '')) return json(res, 400, {error: 'retire needs a reason'});
          world.knowledge.entries = world.knowledge.entries.filter(x => x.id !== id);
          return json(res, 200, {ok: true});
        }
        case '/api/push/register':
        case '/api/push/activity':
          return json(res, 200, {ok: true});
        default:
          return json(res, 404, {error: 'not found'});
      }
    }
    return json(res, 405, {error: 'method not allowed'});
  }

  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const port = (server.address() as AddressInfo).port;
  return {
    url: `http://127.0.0.1:${port}`,
    token,
    world,
    bumpAgents,
    dropStreams: () => {
      const n = streams.size;
      streams.forEach(r => r.destroy());
      streams.clear();
      return n;
    },
    close: () =>
      new Promise<void>(resolve => {
        server.closeAllConnections?.();
        server.close(() => resolve());
      }),
  };
}
