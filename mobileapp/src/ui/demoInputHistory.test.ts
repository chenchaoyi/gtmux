import {demoInputHistory} from './demoData';

// The composer's input history is stored in ONE global key holding the REAL messages you
// typed against your real Mac. Showing that inside Demo mode leaks it and breaks the
// demo's own rule (永不混入真实). Demo must seed a canned list instead — and the Composer
// must never persist Demo typing back to the real store (guarded in Composer.tsx via the
// `demo` prop; this pins the canned source it draws from).
describe('demo input history is mocked, never real', () => {
  test('is a non-empty canned list in both languages', () => {
    for (const zh of [true, false]) {
      const h = demoInputHistory(zh);
      expect(h.length).toBeGreaterThan(0);
      expect(h.every(s => typeof s === 'string' && s.trim() !== '')).toBe(true);
    }
  });

  test('reads as plausible sample dev messages, not a real transcript', () => {
    // The point is that it is CANNED — deterministic and the same every call, so nothing
    // real can leak through it.
    expect(demoInputHistory(false)).toEqual(demoInputHistory(false));
    expect(demoInputHistory(true)).toContain('继续');
    expect(demoInputHistory(false)).toContain('continue');
  });
});
