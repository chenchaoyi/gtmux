import {createJourneyGuard} from '../setup/journeyGuard';
import {startFake, Fake} from '../fake-serve/server';
import {captureSceneJourney, SceneJourney, usageSceneLink} from '../setup/sceneJourneys';
let fake: Fake;
let current: SceneJourney;
const guard = createJourneyGuard();
beforeAll(async () => {fake = await startFake(); fake.world.seedUsage();});
beforeEach(() => {current = {fake, surface: 'ordinary-phone', steps: []};});
afterEach(async () => {
  await guard.wait();
  const label = expect.getState().currentTestName?.startsWith('opens a cold') ? 'cold-link' : 'warm-link';
  await captureSceneJourney(current, label);
}, 930000);
afterAll(async () => {await fake?.close();});
it('opens a cold usage link through the scene', () => guard.run(() => usageSceneLink(current, true)));
it('opens a warm usage link through the scene', () => guard.run(() => usageSceneLink(current, false)));
