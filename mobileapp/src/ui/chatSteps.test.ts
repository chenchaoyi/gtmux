import {stepsOpen} from './chatSteps';

test('secondary activity stays folded until the reader chooses to inspect it', () => {
  expect(stepsOpen(undefined)).toBe(false);
});

test('explicit reader choices win and persist through later transcript updates', () => {
  expect(stepsOpen(false)).toBe(false);
  expect(stepsOpen(true)).toBe(true);
});
