import {startFake, Fake} from '../fake-serve/server';
import {createJourneyGuard} from '../setup/journeyGuard';
import {captureSceneJourney, coldSceneAction, SceneJourney, warmSceneAction} from '../setup/sceneJourneys';
let fake: Fake;
let current: SceneJourney;
const guard = createJourneyGuard();
beforeAll(async () => {fake = await startFake();});
beforeEach(() => {current = {fake, surface: 'ordinary-phone', steps: []};});
afterEach(async () => {
  await guard.wait();
  const label = expect.getState().currentTestName?.startsWith('delivers a cold') ? 'cold-action' : 'warm-action';
  await captureSceneJourney(current, label);
}, 930000);
afterAll(async () => {await fake?.close();});
it('delivers a cold notification action exactly once', () => guard.run(() => coldSceneAction(current)), 300000);
it('delivers a warm notification action exactly once', () => guard.run(() => warmSceneAction(current)), 300000);
