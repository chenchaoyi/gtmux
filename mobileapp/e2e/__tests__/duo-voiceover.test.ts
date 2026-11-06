import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {launchWithFlags} from '../setup/app';
import {posture} from '../setup/duoPosture';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {createJourneyGuard} from '../setup/journeyGuard';
import {startFake, Fake} from '../fake-serve/server';

const guard = createJourneyGuard();
afterEach(guard.wait, 930000);

type Speech = {utterance: string | null};
type VoiceOverState = {enabled: boolean};
type PanelSpeech = {panel: string; startedAt: number; elapsedMs: number; initialSpeech?: Speech; utterances: Speech[]; title: boolean; tools: boolean};

it('speaks the selected session and tools on both Duo panels with VoiceOver', () => guard.run(async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  let fake: Fake | undefined;
  let enabled = false;
  const inspectFocus = async (panel: string) => {
    const startedAt = Date.now();
    const spoken: PanelSpeech = {panel, startedAt, elapsedMs: 0, utterances: [], title: false, tools: false};
    evidence.push(spoken);
    const withinBudget = () => {
      spoken.elapsedMs = Date.now() - startedAt;
      if (spoken.elapsedMs > 90000) throw new Error(`${panel} VoiceOver focus exceeded its 90s budget`);
    };
    const observe = (result: Speech) => {
      expect(result).toHaveProperty('utterance');
      expect(result.utterance === null || typeof result.utterance === 'string').toBe(true);
      spoken.utterances.push(result);
      withinBudget();
      spoken.title ||= result.utterance?.includes('整理这周的发布清单') ?? false;
      spoken.tools ||= result.utterance?.includes('Session tools') ?? false;
    };
    // Display selection alone does not identify VoiceOver's focus. First prove
    // this panel still shows the selected reader, then require fresh moves.
    await driver.$('~Show full title, 整理这周的发布清单').waitForDisplayed({timeout: 15000});
    await driver.$('~session-tools').waitForDisplayed({timeout: 10000});
    withinBudget();
    spoken.initialSpeech = await driver.execute('mobile: voiceOverCurrentSpeech') as Speech;
    withinBudget(); // Diagnostic only: speech retained from the other panel cannot pass.
    for (let moves = 0; moves < 48 && !(spoken.title && spoken.tools); moves++) {
      withinBudget();
      observe(await driver.execute('mobile: voiceOverMove', {direction: 'forward'}) as Speech);
    }
    withinBudget();
    expect(spoken.title).toBe(true);
    expect(spoken.tools).toBe(true);
    await screenshot(`duo-voiceover-${panel}`);
  };
  try {
    expect((await driver.execute('mobile: isVoiceOverEnabled') as VoiceOverState).enabled).toBe(false);
    fake = await startFake();
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    await posture(0, 'outer', evidence);
    await driver.$('~agent-row-%13').waitForDisplayed({timeout: 25000});
    await driver.$('~agent-row-%13').click();
    enabled = true; // Restore even if the enable command fails after changing state.
    await driver.execute('mobile: enableVoiceOver');
    expect((await driver.execute('mobile: isVoiceOverEnabled') as VoiceOverState).enabled).toBe(true);
    await inspectFocus('outer');
    await posture(180, 'inner', evidence);
    expect((await driver.execute('mobile: isVoiceOverEnabled') as VoiceOverState).enabled).toBe(true);
    await inspectFocus('inner');
  } catch (error) {
    await captureOnFailure('duo-voiceover-failed', error);
  } finally {
    try {
      writeFileSync(join(getArtifactsDir(), 'duo-voiceover.json'), JSON.stringify(evidence, null, 2));
    } finally {
      try {
        if (enabled) {
          await driver.execute('mobile: disableVoiceOver');
          expect((await driver.execute('mobile: isVoiceOverEnabled') as VoiceOverState).enabled).toBe(false);
        }
      } finally {
        await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
        await driver.updateSettings({currentDisplayId: null}).catch(() => {});
        await fake?.close();
      }
    }
  }
}), 300000);
