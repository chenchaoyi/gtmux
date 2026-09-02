import {stepsOpen, stepsOpenByDefault} from './chatSteps';

const ctx = (isLatestTurn: boolean, working: boolean) => ({isLatestTurn, working});

// Watching and archaeology are different needs and had one presentation: a 11.5pt toggle
// in the dimmest colour, which is right for digging through history and wrong for
// watching a turn that is happening now.
describe('stepsOpenByDefault', () => {
  test('the turn in flight opens itself', () => {
    expect(stepsOpenByDefault(ctx(true, true))).toBe(true);
  });

  test('a finished turn stays closed — even the newest one', () => {
    // Otherwise a conversation scrolled back through reads as a log of tool calls.
    expect(stepsOpenByDefault(ctx(true, false))).toBe(false);
  });

  test('an older turn stays closed while a newer one runs', () => {
    expect(stepsOpenByDefault(ctx(false, true))).toBe(false);
    expect(stepsOpenByDefault(ctx(false, false))).toBe(false);
  });
});

describe('stepsOpen', () => {
  test('with no choice made, the default decides', () => {
    expect(stepsOpen(undefined, ctx(true, true))).toBe(true);
    expect(stepsOpen(undefined, ctx(true, false))).toBe(false);
  });

  test("the reader's choice wins in both directions", () => {
    expect(stepsOpen(false, ctx(true, true))).toBe(false); // closed a live turn's steps
    expect(stepsOpen(true, ctx(false, false))).toBe(true); // opened an old turn's steps
  });

  // The moment a turn ends, the default flips to closed. That is right for a reader who
  // never touched it and wrong for one who deliberately opened the steps to read them.
  test('a deliberate expansion survives the turn finishing', () => {
    const chosen = true;
    expect(stepsOpen(chosen, ctx(true, true))).toBe(true);
    expect(stepsOpen(chosen, ctx(true, false))).toBe(true);
  });
});
