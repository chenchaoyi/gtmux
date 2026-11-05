import {startFake} from './server';

test('diagnostic request metadata shows routing without headers or query values', async () => {
  const fake = await startFake({token: 'fixture-secret'});
  try {
    await fetch(`${fake.url}/api/agents?secret=private`, {headers: {Authorization: 'Bearer fixture-secret'}});
    expect(fake.requests).toEqual([{method: 'GET', path: '/api/agents', authorized: true, at: expect.any(Number)}]);
    await fetch(`${fake.url}/api/send`, {method: 'POST', headers: {Authorization: 'Bearer fixture-secret'}, body: JSON.stringify({id: '%12', text: '2'})});
    expect(fake.requests[1]).toEqual(expect.objectContaining({method: 'POST', path: '/api/send', body: 'complete', bodyAt: expect.any(Number)}));
    expect(JSON.stringify(fake.requests)).not.toContain('fixture-secret');
    expect(JSON.stringify(fake.requests)).not.toContain('private');
  } finally {
    await fake.close();
  }
});
