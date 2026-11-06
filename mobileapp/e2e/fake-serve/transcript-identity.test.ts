import {startFake, Fake} from './server';
import {TranscriptTurn} from '../../src/api/client';

const transcript = async (fake: Fake): Promise<TranscriptTurn[]> => {
  const response = await fetch(`${fake.url}/api/transcript?id=%2513`, {headers: {Authorization: `Bearer ${fake.token}`}});
  expect(response.status).toBe(200);
  return response.json();
};

test('default transcript identity survives polling while new fixtures and explicit transcripts remain independent', async () => {
  const epoch = 1793968127000;
  const clock = jest.spyOn(Date, 'now').mockReturnValue(epoch);
  const fixtures: Fake[] = [];
  try {
    const first = await startFake();
    fixtures.push(first);
    const initial = await transcript(first);
    expect(initial).toHaveLength(4);
    expect(initial[3].time).toBe(new Date(epoch - 4 * 60000).toISOString());
    clock.mockReturnValue(epoch + 60000);
    expect(await transcript(first)).toEqual(initial);
    const second = await startFake();
    fixtures.push(second);
    const next = await transcript(second);
    expect(next[3].time).toBe(new Date(epoch + 60000 - 4 * 60000).toISOString());
    expect(next[3].time).not.toBe(initial[3].time);
    const seeded = [{prompt: 'explicit fixture', response: 'retained', time: '2026-01-01T00:00:00Z'}];
    first.world.transcripts.set('%13', seeded);
    expect(await transcript(first)).toEqual(seeded);
    expect(await transcript(second)).toEqual(next);
  } finally {
    clock.mockRestore();
    await Promise.all(fixtures.map(fake => fake.close()));
  }
});
