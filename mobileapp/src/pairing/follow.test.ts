import {findLiveAddress, mergeAddresses, normalizeAddr} from './follow';

// A Mac that moved to another Direct server keeps its account and its port; only the host
// name changes. These pin that a phone stores the alternatives and finds the Mac again
// without anyone scanning a code (openspec/changes/direct-server-choice).

describe('mergeAddresses', () => {
  it('puts the address in use first and keeps the rest in order', () => {
    expect(
      mergeAddresses('https://sh.example/p35047', ['https://la.example/p35047', 'https://hk.example/p35047']),
    ).toEqual(['https://sh.example/p35047', 'https://la.example/p35047', 'https://hk.example/p35047']);
  });
  it('is one spelling per address', () => {
    expect(mergeAddresses('https://sh.example/p35047/', ['https://sh.example/p35047'])).toEqual([
      'https://sh.example/p35047',
    ]);
  });
  it('keeps only https: the token is about to be sent to these', () => {
    expect(mergeAddresses('https://sh.example/p1', ['http://sh.example/p1', 'nonsense', ''])).toEqual([
      'https://sh.example/p1',
    ]);
  });
  it('is a pool of servers, not a phone book', () => {
    const many = Array.from({length: 20}, (_, i) => `https://s${i}.example/p1`);
    expect(mergeAddresses(many[0], many).length).toBe(8);
  });
});

describe('findLiveAddress', () => {
  it('answers with the first address that has the Mac behind it', async () => {
    const asked: string[] = [];
    const probe = async (u: string) => {
      asked.push(u);
      return u === 'https://la.example/p35047';
    };
    const got = await findLiveAddress(['https://sh.example/p35047', 'https://la.example/p35047'], probe, 50);
    expect(got).toBe('https://la.example/p35047');
    expect(asked).toEqual(['https://sh.example/p35047', 'https://la.example/p35047']);
  });
  it('gives up when no address answers, instead of hanging', async () => {
    const got = await findLiveAddress(['https://a.example/p1', 'https://b.example/p1'], () => new Promise(() => {}), 30);
    expect(got).toBeNull();
  });
  it('a probe that throws is just an address that did not answer', async () => {
    const probe = async (u: string) => {
      if (u.includes('a.example')) throw new Error('network');
      return true;
    };
    expect(await findLiveAddress(['https://a.example/p1', 'https://b.example/p1'], probe, 50)).toBe(
      'https://b.example/p1',
    );
  });
  it('stops at the first answer instead of trying the rest', async () => {
    const asked: string[] = [];
    const probe = async (u: string) => {
      asked.push(u);
      return true;
    };
    await findLiveAddress(['https://a.example/p1', 'https://b.example/p1'], probe, 50);
    expect(asked).toEqual(['https://a.example/p1']);
  });
});

describe('normalizeAddr', () => {
  it('trims space and trailing slashes', () => {
    expect(normalizeAddr('  https://x.example/p1//  ')).toBe('https://x.example/p1');
  });
});
