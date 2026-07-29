import {uploadAll} from './Composer';

// Pins the fix for the "input box didn't confirm → stuck forever" bug: the send box's
// `sending` flag disables the send button + gates the send handler, so it MUST always
// reset. It only stayed stuck when an attachment upload THREW (a synchronous XHR/
// FormData error rejecting the promise) and escaped the handler before reset. uploadAll
// absorbs that — a rejected upload maps to null, never a throw — so the caller's
// `setSending(false)` always runs and the box can never wedge.
describe('uploadAll — the send box can never wedge on an upload error', () => {
  const atts = [
    {id: 'a', uri: 'file://1', name: 'one.png', type: 'image/png'},
    {id: 'b', uri: 'file://2', name: 'two.png', type: 'image/png'},
  ];

  it('returns every saved path when all uploads succeed', async () => {
    const r = await uploadAll(atts, async (_u, _n, _t) => '/saved/' + _n, () => {});
    expect(r).toEqual(['/saved/one.png', '/saved/two.png']);
  });

  it('returns null (does NOT throw) when an upload REJECTS — this is the wedge fix', async () => {
    let threw = false;
    const r = await uploadAll(
      atts,
      async () => {
        throw new Error('xhr.send blew up synchronously');
      },
      () => {},
    ).catch(() => {
      threw = true;
      return undefined;
    });
    expect(threw).toBe(false); // must swallow the throw
    expect(r).toBeNull();
  });

  it('returns null when an upload resolves null (server refused)', async () => {
    const r = await uploadAll(atts, async () => null, () => {});
    expect(r).toBeNull();
  });

  it('stops at the first failure and does not upload the rest', async () => {
    const seen: string[] = [];
    const r = await uploadAll(
      atts,
      async (_u, n) => {
        seen.push(n);
        return n === 'one.png' ? null : '/saved/' + n;
      },
      () => {},
    );
    expect(r).toBeNull();
    expect(seen).toEqual(['one.png']); // never attempted 'two.png'
  });

  it('reports progress keyed by attachment id', async () => {
    const prog: Record<string, number> = {};
    await uploadAll(
      atts,
      async (_u, _n, _t, onP) => {
        onP(0.5);
        return '/saved';
      },
      (id, f) => {
        prog[id] = f;
      },
    );
    expect(prog).toEqual({a: 0.5, b: 0.5});
  });
});
