import {createJourneyGuard} from './journeyGuard';

test('wait holds the next journey until outstanding cleanup completes', async () => {
  const guard = createJourneyGuard();
  let release!: () => void;
  const pending = new Promise<void>(resolve => {release = resolve;});
  let cleaned = false;
  const first = guard.run(async () => {await pending; cleaned = true;});
  let nextStarted = false;
  const next = guard.wait().then(() => {nextStarted = true;});
  await Promise.resolve();
  expect(nextStarted).toBe(false);
  release();
  await first;
  await next;
  expect(cleaned).toBe(true);
  expect(nextStarted).toBe(true);
});

test('a failed journey releases the guard without concealing its error', async () => {
  const guard = createJourneyGuard();
  const failed = guard.run(async () => {throw new Error('device failed');});
  await expect(failed).rejects.toThrow('device failed');
  await expect(guard.wait()).resolves.toBeUndefined();
});
